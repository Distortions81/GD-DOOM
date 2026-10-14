//go:build raylib && cgo && !js

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"gddoom/internal/audiofx"
	"gddoom/internal/music"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const nativeSpeakerFrames = 1024

// Both physical and emulated outputs share effect/music arbitration and the
// pause/clear operations used by the native host.
type nativeSpeaker interface {
	audiofx.PCSpeaker
	SetVariant(audiofx.PCSpeakerVariant)
	ClearEffects()
	SetPaused(bool)
}

var _ nativeSpeaker = (*audiofx.PCSpeakerPlayer)(nil)
var _ nativeSpeaker = (*audiofx.LinuxPCSpeakerPlayer)(nil)

func newNativeSpeakerOutput(output audiofx.PCSpeakerOutput, volume float64, variant audiofx.PCSpeakerVariant, stderr io.Writer) (nativeSpeaker, *nativeSpeakerBackend, error) {
	return chooseNativeSpeakerOutput(output, volume, variant, stderr,
		func() (nativeSpeaker, error) { return audiofx.NewLinuxPCSpeakerPlayer() },
		func(v float64, variant audiofx.PCSpeakerVariant) (nativeSpeaker, *nativeSpeakerBackend, error) {
			return newNativeSpeaker(v, variant)
		})
}

func chooseNativeSpeakerOutput(output audiofx.PCSpeakerOutput, volume float64, variant audiofx.PCSpeakerVariant, stderr io.Writer,
	linux func() (nativeSpeaker, error), emulated func(float64, audiofx.PCSpeakerVariant) (nativeSpeaker, *nativeSpeakerBackend, error)) (nativeSpeaker, *nativeSpeakerBackend, error) {
	if output == audiofx.PCSpeakerOutputLinux {
		p, err := linux()
		if err == nil {
			p.SetVolume(volume)
			return p, nil, nil
		}
		if stderr != nil {
			fmt.Fprintf(stderr, "linux pc speaker unavailable, falling back to emulated output: %v\n", err)
		}
	}
	return emulated(volume, variant)
}

// nativeSpeakerBackend supplies the Raylib transport for the main host's
// PCSpeakerPlayer. Source playback and mixed effect/music selection stay shared.
type nativeSpeakerBackend struct {
	source          io.ReadSeeker
	stream          rl.AudioStream
	bytes           []byte
	pcm             []int16
	started, paused bool
	emptyBuffers    int
	volume          float64
	err             error
}

func newNativeSpeaker(volume float64, variant audiofx.PCSpeakerVariant) (*audiofx.PCSpeakerPlayer, *nativeSpeakerBackend, error) {
	var backend *nativeSpeakerBackend
	p, err := audiofx.NewPCSpeakerWithBackend(volume, variant, func(src io.ReadSeeker) (audiofx.PCSpeakerBackend, error) {
		rl.SetAudioStreamBufferSizeDefault(nativeSpeakerFrames)
		stream := rl.LoadAudioStream(music.OutputSampleRate, 16, 2)
		if !rl.IsAudioStreamValid(stream) {
			return nil, fmt.Errorf("PC speaker: audio stream could not initialize")
		}
		backend = &nativeSpeakerBackend{source: src, stream: stream, bytes: make([]byte, nativeSpeakerFrames*4), pcm: make([]int16, nativeSpeakerFrames*2)}
		return backend, nil
	})
	return p, backend, err
}

func (p *nativeSpeakerBackend) Play() {
	if p.started {
		p.paused = false
		rl.ResumeAudioStream(p.stream)
		return
	}
	// Play resets processed flags. Prime only after it, while playback is paused.
	rl.PlayAudioStream(p.stream)
	rl.PauseAudioStream(p.stream)
	p.started, p.paused = true, false
	if err := p.fill(); err != nil {
		p.err = err
		p.Pause()
		return
	}
	rl.ResumeAudioStream(p.stream)
}
func (p *nativeSpeakerBackend) Pause() {
	p.paused = true
	if p.started {
		rl.PauseAudioStream(p.stream)
	}
}
func (p *nativeSpeakerBackend) Rewind() error {
	if p.started {
		rl.ResumeAudioStream(p.stream)
		rl.StopAudioStream(p.stream)
	}
	p.started, p.paused, p.emptyBuffers, p.err = false, false, 0, nil
	_, err := p.source.Seek(0, io.SeekStart)
	return err
}
func (p *nativeSpeakerBackend) SetBufferSize(time.Duration) {}
func (p *nativeSpeakerBackend) SetVolume(v float64) {
	p.volume = v
	rl.SetAudioStreamVolume(p.stream, float32(v))
}
func (p *nativeSpeakerBackend) IsPlaying() bool { return p.started && !p.paused }
func (p *nativeSpeakerBackend) Close() error {
	p.Pause()
	rl.UnloadAudioStream(p.stream)
	p.started = false
	return nil
}
func (p *nativeSpeakerBackend) fill() error {
	for i := 0; i < 2 && rl.IsAudioStreamProcessed(p.stream); i++ {
		clear(p.bytes)
		n, err := io.ReadFull(p.source, p.bytes)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}
		if n == 0 {
			p.emptyBuffers++
		} else {
			p.emptyBuffers = 0
		}
		if p.emptyBuffers >= 2 {
			// Both previous halves have drained; a finite effect has ended.
			rl.ResumeAudioStream(p.stream)
			rl.StopAudioStream(p.stream)
			p.started = false
			return nil
		}
		for i := range p.pcm {
			p.pcm[i] = int16(binary.LittleEndian.Uint16(p.bytes[2*i : 2*i+2]))
		}
		rl.UpdateAudioStream(p.stream, p.pcm[:nativeSpeakerFrames])
	}
	return nil
}
func (p *nativeSpeakerBackend) Update() error {
	if p == nil {
		return nil
	}
	if p.err != nil {
		return p.err
	}
	if !p.started || p.paused {
		return nil
	}
	return p.fill()
}
