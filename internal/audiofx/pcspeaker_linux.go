//go:build linux

package audiofx

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gddoom/internal/sound"

	"golang.org/x/sys/unix"
)

const linuxPCSpeakerBaseTickRate = 280
const linuxPCSpeakerClockHz = 1193182

const (
	linuxInputEventTypeSound = 0x12
	linuxInputSoundTone      = 0x02
	linuxInputEventTypeSync  = 0x00
	linuxInputSyncReport     = 0x00
)

var linuxPCSpeakerDevicePatterns = []string{
	"/dev/input/by-path/*pcspkr*-event-spkr",
	"/dev/input/by-path/*-event-spkr",
	"/dev/input/by-id/*pcspkr*",
}

type linuxInputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

type LinuxPCSpeakerPlayer struct {
	mu             sync.Mutex
	f              *os.File
	effectSeq      []sound.PCSpeakerTone
	effectTickRate int
	effectTickPos  int
	musicSeq       []sound.PCSpeakerTone
	musicTickRate  int
	musicTickPos   int
	musicLoop      bool
	mixTick        uint64
	currentSource  int
	wakeCh         chan struct{}
	stopCh         chan struct{}
	lastDivisor    uint16
	usedSpeaker    bool
}

const (
	linuxPCSpeakerSourceNone = iota
	linuxPCSpeakerSourceEffect
	linuxPCSpeakerSourceMusic
)

func NewLinuxPCSpeakerPlayer() (*LinuxPCSpeakerPlayer, error) {
	path, err := findLinuxPCSpeakerDevice()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open linux pc speaker device %s: %w", path, err)
	}
	p := &LinuxPCSpeakerPlayer{
		f:      f,
		wakeCh: make(chan struct{}, 1),
		stopCh: make(chan struct{}),
	}
	go p.loop()
	return p, nil
}

func (p *LinuxPCSpeakerPlayer) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.f == nil {
		p.mu.Unlock()
		return nil
	}
	close(p.stopCh)
	f := p.f
	usedSpeaker := p.shouldSilenceOnCloseLocked()
	p.f = nil
	p.mu.Unlock()
	if usedSpeaker {
		_ = writeLinuxPCSpeakerTone(f, 0)
	}
	err := f.Close()
	p.f = nil
	return err
}

func (p *LinuxPCSpeakerPlayer) shouldSilenceOnCloseLocked() bool {
	return p.usedSpeaker
}

func (p *LinuxPCSpeakerPlayer) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.effectSeq = nil
	p.effectTickPos = 0
	p.musicSeq = nil
	p.musicTickPos = 0
	p.musicLoop = false
	p.mixTick = 0
	p.currentSource = linuxPCSpeakerSourceNone
	p.mu.Unlock()
	p.notify()
	_ = p.setDivisor(0)
}

func (p *LinuxPCSpeakerPlayer) PlaySequence(seq []sound.PCSpeakerTone, tickRate int) error {
	p.Stop()
	p.SetMusic(seq, tickRate, false)
	total := totalTicksAtRate(len(seq), tickRate, normalizeLinuxPCSpeakerTickRate(tickRate))
	time.Sleep(time.Duration(total)*time.Second/time.Duration(normalizeLinuxPCSpeakerTickRate(tickRate)) + 50*time.Millisecond)
	return nil
}

func (p *LinuxPCSpeakerPlayer) Play(seq []sound.PCSpeakerTone) {
	if p == nil || len(seq) == 0 {
		return
	}
	p.mu.Lock()
	p.effectSeq = append([]sound.PCSpeakerTone(nil), seq...)
	p.effectTickRate = 140
	p.effectTickPos = 0
	p.mu.Unlock()
	p.notify()
}

func (p *LinuxPCSpeakerPlayer) SetMusic(seq []sound.PCSpeakerTone, tickRate int, loop bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.musicSeq = append([]sound.PCSpeakerTone(nil), seq...)
	p.musicTickRate = tickRate
	p.musicTickPos = 0
	p.musicLoop = loop
	p.mixTick = 0
	p.currentSource = linuxPCSpeakerSourceNone
	p.mu.Unlock()
	p.notify()
}

func (p *LinuxPCSpeakerPlayer) ClearMusic() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.musicSeq = nil
	p.musicTickPos = 0
	p.musicLoop = false
	p.mixTick = 0
	p.currentSource = linuxPCSpeakerSourceNone
	p.mu.Unlock()
	p.notify()
}

func (p *LinuxPCSpeakerPlayer) SetVolume(v float64) {}

func (p *LinuxPCSpeakerPlayer) loop() {
	var nextTick time.Time
	for {
		p.mu.Lock()
		if p.f == nil {
			p.mu.Unlock()
			return
		}
		active := len(p.effectSeq) > 0 || len(p.musicSeq) > 0
		rate := normalizeLinuxPCSpeakerTickRate(max(p.effectTickRate, p.musicTickRate))
		p.mu.Unlock()
		if !active {
			_ = p.setDivisor(0)
			nextTick = time.Time{}
			select {
			case <-p.wakeCh:
				continue
			case <-p.stopCh:
				return
			}
		}
		tickDur := time.Second / time.Duration(rate)
		now := time.Now()
		if nextTick.IsZero() || now.After(nextTick.Add(4*tickDur)) {
			nextTick = now
		}
		div := p.stepDivisor()
		_ = p.setDivisor(div)
		nextTick = nextTick.Add(tickDur)
		wait := time.Until(nextTick)
		if wait < 0 {
			wait = 0
		}
		select {
		case <-time.After(wait):
		case <-p.wakeCh:
			nextTick = time.Time{}
		case <-p.stopCh:
			_ = p.setDivisor(0)
			return
		}
	}
}

func (p *LinuxPCSpeakerPlayer) stepDivisor() uint16 {
	p.mu.Lock()
	defer p.mu.Unlock()
	rate := normalizeLinuxPCSpeakerTickRate(max(p.effectTickRate, p.musicTickRate))
	effectTone, effectOK := p.currentEffectToneLocked(rate)
	musicTone, musicOK := p.currentMusicToneLocked(rate)
	switch {
	case effectOK && musicOK:
		effectActive := effectTone.Active
		musicActive := musicTone.Active
		switch {
		case effectActive && musicActive:
			// Only switch ownership during true overlap windows.
			if p.currentSource != linuxPCSpeakerSourceEffect && p.currentSource != linuxPCSpeakerSourceMusic {
				p.currentSource = linuxPCSpeakerSourceEffect
			} else {
				hold := pcSpeakerToneInterleaveHoldTicks(effectTone, musicTone, rate)
				next := pcSpeakerToneMixPattern[int((p.mixTick/uint64(hold))%uint64(len(pcSpeakerToneMixPattern)))]
				p.mixTick++
				if next == 0 {
					p.currentSource = linuxPCSpeakerSourceEffect
				} else {
					p.currentSource = linuxPCSpeakerSourceMusic
				}
			}
			if p.currentSource == linuxPCSpeakerSourceMusic {
				return musicTone.ToneDivisor()
			}
			return effectTone.ToneDivisor()
		case effectActive:
			p.currentSource = linuxPCSpeakerSourceEffect
			return effectTone.ToneDivisor()
		case musicActive:
			p.currentSource = linuxPCSpeakerSourceMusic
			return musicTone.ToneDivisor()
		default:
			p.currentSource = linuxPCSpeakerSourceNone
			return 0
		}
	case effectOK:
		p.currentSource = linuxPCSpeakerSourceEffect
		return effectTone.ToneDivisor()
	case musicOK:
		p.currentSource = linuxPCSpeakerSourceMusic
		return musicTone.ToneDivisor()
	default:
		p.currentSource = linuxPCSpeakerSourceNone
		return 0
	}
}

func (p *LinuxPCSpeakerPlayer) currentEffectToneLocked(outTickRate int) (sound.PCSpeakerTone, bool) {
	total := totalTicksAtRate(len(p.effectSeq), p.effectTickRate, outTickRate)
	if total <= 0 || p.effectTickPos >= total {
		p.effectSeq = nil
		p.effectTickPos = 0
		return sound.PCSpeakerTone{}, false
	}
	tone, ok := toneAtTick(p.effectSeq, p.effectTickRate, outTickRate, p.effectTickPos)
	p.effectTickPos++
	if p.effectTickPos >= total {
		p.effectSeq = nil
		p.effectTickPos = 0
	}
	return tone, ok
}

func (p *LinuxPCSpeakerPlayer) currentMusicToneLocked(outTickRate int) (sound.PCSpeakerTone, bool) {
	total := totalTicksAtRate(len(p.musicSeq), p.musicTickRate, outTickRate)
	if total <= 0 {
		p.musicSeq = nil
		p.musicTickPos = 0
		return sound.PCSpeakerTone{}, false
	}
	if p.musicTickPos >= total {
		if !p.musicLoop {
			p.musicSeq = nil
			p.musicTickPos = 0
			return sound.PCSpeakerTone{}, false
		}
		p.musicTickPos = 0
	}
	tone, ok := toneAtTick(p.musicSeq, p.musicTickRate, outTickRate, p.musicTickPos)
	p.musicTickPos++
	if p.musicLoop && p.musicTickPos >= total {
		p.musicTickPos = 0
	}
	return tone, ok
}

func (p *LinuxPCSpeakerPlayer) notify() {
	if p == nil {
		return
	}
	select {
	case p.wakeCh <- struct{}{}:
	default:
	}
}

func (p *LinuxPCSpeakerPlayer) setDivisor(div uint16) error {
	if p == nil {
		return fmt.Errorf("linux pc speaker player is closed")
	}
	p.mu.Lock()
	f, changed, err := p.prepareDivisorChangeLocked(div)
	p.mu.Unlock()
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := writeLinuxPCSpeakerTone(f, div); err != nil {
		return err
	}
	p.mu.Lock()
	if p.f == f {
		p.lastDivisor = div
		if div != 0 {
			p.usedSpeaker = true
		}
	}
	p.mu.Unlock()
	return nil
}

func (p *LinuxPCSpeakerPlayer) prepareDivisorChangeLocked(div uint16) (*os.File, bool, error) {
	if p.f == nil {
		return nil, false, fmt.Errorf("linux pc speaker player is closed")
	}
	if div == p.lastDivisor {
		return nil, false, nil
	}
	return p.f, true, nil
}

func normalizeLinuxPCSpeakerTickRate(rate int) int {
	if rate <= 0 || rate > linuxPCSpeakerBaseTickRate {
		return linuxPCSpeakerBaseTickRate
	}
	return rate
}

func findLinuxPCSpeakerDevice() (string, error) {
	return findLinuxPCSpeakerDeviceFromPatterns(linuxPCSpeakerDevicePatterns)
}

func findLinuxPCSpeakerDeviceFromPatterns(patterns []string) (string, error) {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", fmt.Errorf("glob pc speaker devices %q: %w", pattern, err)
		}
		for _, path := range matches {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("pc speaker device not found under /dev/input; require write access to /dev/input/by-path/*pcspkr*-event-spkr")
}

func writeLinuxPCSpeakerTone(f *os.File, div uint16) error {
	if f == nil {
		return fmt.Errorf("linux pc speaker player is closed")
	}
	value := int32(0)
	if div != 0 {
		value = int32(linuxPCSpeakerClockHz / int(div))
		if value <= 0 {
			value = 1
		}
	}
	if err := binary.Write(f, binary.LittleEndian, linuxInputEvent{
		Type:  linuxInputEventTypeSound,
		Code:  linuxInputSoundTone,
		Value: value,
	}); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, linuxInputEvent{
		Type: linuxInputEventTypeSync,
		Code: linuxInputSyncReport,
	}); err != nil {
		return err
	}
	return nil
}
