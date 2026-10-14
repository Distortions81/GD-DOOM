//go:build linux

package audiofx

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gddoom/internal/sound"
)

func TestLinuxPCSpeakerPrepareDivisorChangeLocked(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "pcspkr-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	p := &LinuxPCSpeakerPlayer{f: f}

	gotFile, changed, err := p.prepareDivisorChangeLocked(1234)
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	if !changed {
		t.Fatal("expected first divisor change to be emitted")
	}
	if gotFile != f {
		t.Fatal("expected original file handle")
	}

	p.lastDivisor = 1234
	gotFile, changed, err = p.prepareDivisorChangeLocked(1234)
	if err != nil {
		t.Fatalf("repeat prepare: %v", err)
	}
	if changed {
		t.Fatal("expected repeated divisor to be suppressed")
	}
	if gotFile != nil {
		t.Fatal("expected no file when no change is needed")
	}

	p.lastDivisor = 1234
	gotFile, changed, err = p.prepareDivisorChangeLocked(0)
	if err != nil {
		t.Fatalf("silence prepare: %v", err)
	}
	if !changed {
		t.Fatal("expected silence to be emitted when stopping an active tone")
	}
	if gotFile != f {
		t.Fatal("expected original file handle for silence write")
	}
}

func TestLinuxPCSpeakerPauseAndClearPreserveCursors(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "speaker-events-*")
	if err != nil {
		t.Fatal(err)
	}
	p := &LinuxPCSpeakerPlayer{f: f}
	defer p.Close()
	effects := make([]sound.PCSpeakerTone, 8)
	music := make([]sound.PCSpeakerTone, 8)
	for i := range effects {
		effects[i] = sound.PCSpeakerTone{Active: true, Divisor: 1000}
		music[i] = sound.PCSpeakerTone{Active: true, Divisor: 2000}
	}
	p.Play(effects)
	p.SetMusic(music, 140, true)
	div := p.stepDivisor()
	if err := p.setDivisor(div); err != nil {
		t.Fatal(err)
	}
	effectPos, musicPos, mixTick := p.effectTickPos, p.musicTickPos, p.mixTick
	p.SetPaused(true)
	for range 50 {
		if p.stepDivisor() != 0 {
			t.Fatal("paused speaker produced a tone")
		}
		// Simulate a tone computed before the pause reached the device writer.
		if err := p.setDivisor(1000); err != nil {
			t.Fatal(err)
		}
	}
	if p.effectTickPos != effectPos || p.musicTickPos != musicPos || p.mixTick != mixTick || p.lastDivisor != 0 {
		t.Fatal("pause advanced playback or restored sound")
	}
	p.ClearEffects()
	if len(p.effectSeq) != 0 || p.musicTickPos != musicPos || len(p.musicSeq) != len(music) {
		t.Fatal("clearing effects also cleared music")
	}
	p.SetPaused(false)
	if got := p.stepDivisor(); got != 2000 || p.musicTickPos != musicPos+1 {
		t.Fatalf("music did not resume at its retained cursor: divisor %d", got)
	}
	p.ClearMusic()
	if got := p.stepDivisor(); got != 0 {
		t.Fatalf("cleared music retained a tone: %d", got)
	}
}

func TestLinuxPCSpeakerConcurrentPauseAndCloseSilence(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "speaker-events-*")
	if err != nil {
		t.Fatal(err)
	}
	p := &LinuxPCSpeakerPlayer{f: f, stopCh: make(chan struct{})}
	if err := p.setDivisor(1000); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				_ = p.setDivisor(1000)
				p.SetPaused(true)
				_ = p.setDivisor(2000)
				p.SetPaused(false)
			}
		}()
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	size := binary.Size(linuxInputEvent{})
	if len(data)%(2*size) != 0 {
		t.Fatal("concurrent writes interleaved tone/report event pairs")
	}
	if len(data) != 0 {
		// The last event pair must be silence followed by SYN_REPORT.
		value := int32(binary.LittleEndian.Uint32(data[len(data)-size-4 : len(data)-size]))
		if value != 0 {
			t.Fatalf("close left an active hardware tone: %d", value)
		}
	}
}

func TestLinuxPCSpeakerPrepareDivisorChangeLockedClosed(t *testing.T) {
	t.Parallel()

	p := &LinuxPCSpeakerPlayer{}
	if _, _, err := p.prepareDivisorChangeLocked(42); err == nil {
		t.Fatal("expected closed player error")
	}
}

func TestLinuxPCSpeakerShouldSilenceOnCloseLocked(t *testing.T) {
	t.Parallel()

	t.Run("unused speaker skips silence", func(t *testing.T) {
		t.Parallel()

		p := &LinuxPCSpeakerPlayer{}
		if p.shouldSilenceOnCloseLocked() {
			t.Fatal("shouldSilenceOnCloseLocked() = true, want false")
		}
	})

	t.Run("used speaker emits silence", func(t *testing.T) {
		t.Parallel()

		p := &LinuxPCSpeakerPlayer{usedSpeaker: true}
		if !p.shouldSilenceOnCloseLocked() {
			t.Fatal("shouldSilenceOnCloseLocked() = false, want true")
		}
	})
}

func TestFindLinuxPCSpeakerDeviceFromPatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	devicePath := filepath.Join(dir, "platform-pcspkr-event-spkr")
	if err := os.WriteFile(devicePath, nil, 0o600); err != nil {
		t.Fatalf("write test device: %v", err)
	}

	got, err := findLinuxPCSpeakerDeviceFromPatterns([]string{
		filepath.Join(dir, "*pcspkr*-event-spkr"),
	})
	if err != nil {
		t.Fatalf("findLinuxPCSpeakerDeviceFromPatterns() error = %v", err)
	}
	if got != devicePath {
		t.Fatalf("findLinuxPCSpeakerDeviceFromPatterns() = %q want %q", got, devicePath)
	}
}

func TestWriteLinuxPCSpeakerTone(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "pcspkr-event-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	if err := writeLinuxPCSpeakerTone(f, 0); err != nil {
		t.Fatalf("writeLinuxPCSpeakerTone(silence): %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var tone linuxInputEvent
	if err := binary.Read(f, binary.LittleEndian, &tone); err != nil {
		t.Fatalf("read tone event: %v", err)
	}
	if tone.Type != linuxInputEventTypeSound || tone.Code != linuxInputSoundTone || tone.Value != 0 {
		t.Fatalf("tone event = %+v, want type=%d code=%d value=0", tone, linuxInputEventTypeSound, linuxInputSoundTone)
	}
	var sync linuxInputEvent
	if err := binary.Read(f, binary.LittleEndian, &sync); err != nil {
		t.Fatalf("read sync event: %v", err)
	}
	if sync.Type != linuxInputEventTypeSync || sync.Code != linuxInputSyncReport {
		t.Fatalf("sync event = %+v, want type=%d code=%d", sync, linuxInputEventTypeSync, linuxInputSyncReport)
	}
}
