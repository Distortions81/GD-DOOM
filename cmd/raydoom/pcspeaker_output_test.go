//go:build raylib && cgo && !js

package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"gddoom/internal/audiofx"
	"gddoom/internal/doomruntime"
	"gddoom/internal/sound"
)

type nativeSpeakerProbe struct {
	volume         float64
	variant        audiofx.PCSpeakerVariant
	paused         bool
	effects, music []sound.PCSpeakerTone
	rate           int
	loop           bool
}

func (p *nativeSpeakerProbe) Play(seq []sound.PCSpeakerTone) { p.effects = seq }
func (p *nativeSpeakerProbe) SetMusic(seq []sound.PCSpeakerTone, rate int, loop bool) {
	p.music, p.rate, p.loop = seq, rate, loop
}
func (p *nativeSpeakerProbe) ClearMusic()                           { p.music = nil }
func (p *nativeSpeakerProbe) ClearEffects()                         { p.effects = nil }
func (p *nativeSpeakerProbe) Stop()                                 { p.ClearMusic(); p.ClearEffects() }
func (p *nativeSpeakerProbe) Close() error                          { p.Stop(); return nil }
func (p *nativeSpeakerProbe) SetVolume(v float64)                   { p.volume = v }
func (p *nativeSpeakerProbe) SetVariant(v audiofx.PCSpeakerVariant) { p.variant = v }
func (p *nativeSpeakerProbe) SetPaused(paused bool)                 { p.paused = paused }

func TestNativeSpeakerOutputSelectionAndFallback(t *testing.T) {
	for _, output := range []audiofx.PCSpeakerOutput{audiofx.PCSpeakerOutputEmulated, audiofx.PCSpeakerOutputLinux} {
		for _, unavailable := range []bool{false, true} {
			physical, emulated := &nativeSpeakerProbe{}, &nativeSpeakerProbe{}
			linuxCalls, emulatedCalls := 0, 0
			var warning bytes.Buffer
			p, backend, err := chooseNativeSpeakerOutput(output, .75, audiofx.PCSpeakerVariantSmallSpeaker, &warning,
				func() (nativeSpeaker, error) {
					linuxCalls++
					if unavailable {
						return nil, errors.New("no writable speaker device")
					}
					return physical, nil
				},
				func(volume float64, variant audiofx.PCSpeakerVariant) (nativeSpeaker, *nativeSpeakerBackend, error) {
					emulatedCalls++
					emulated.SetVolume(volume)
					emulated.SetVariant(variant)
					return emulated, nil, nil
				})
			if err != nil || backend != nil {
				t.Fatalf("unexpected output: %v", err)
			}
			if output == audiofx.PCSpeakerOutputLinux && !unavailable {
				if p != physical || linuxCalls != 1 || emulatedCalls != 0 || physical.volume != .75 || warning.Len() != 0 {
					t.Fatal("physical output initialized an emulated stream or lost gain")
				}
			} else if p != emulated || emulatedCalls != 1 || emulated.volume != .75 || emulated.variant != audiofx.PCSpeakerVariantSmallSpeaker {
				t.Fatal("emulated output or fallback did not retain its settings")
			}
			if output == audiofx.PCSpeakerOutputEmulated && linuxCalls != 0 {
				t.Fatal("default output touched hardware")
			}
			if output == audiofx.PCSpeakerOutputLinux && unavailable && !strings.Contains(warning.String(), "falling back to emulated output: no writable speaker device") {
				t.Fatal("missing main-compatible fallback diagnostic")
			}
		}
	}
}

func TestNativePhysicalSpeakerEffectsDoNotRequirePCMBackend(t *testing.T) {
	probe := &nativeSpeakerProbe{}
	seq := []sound.PCSpeakerTone{{Active: true, Divisor: 1000}}
	probe.SetMusic(seq, 140, true)
	a := &nativeAudio{speaker: probe, volume: .7, pcBank: map[string][]sound.PCSpeakerTone{"DSPISTOL": seq}}
	if err := a.ConfigureSpeaker(true, .8, "passthrough"); err != nil {
		t.Fatal(err)
	}
	a.Play(nil, doomruntime.NativeSound{Name: "DSPISTOL"})
	if len(probe.effects) != 1 {
		t.Fatal("physical effect did not play")
	}
	a.SetPaused(true)
	probe.ClearEffects()
	a.Play(nil, doomruntime.NativeSound{Name: "DSPISTOL"})
	if !probe.paused || len(probe.effects) != 0 {
		t.Fatal("paused physical output accepted host effects")
	}
	a.Update(nil)
	a.SetPaused(false)
	a.Play(nil, doomruntime.NativeSound{Name: "DSPISTOL"})
	a.Stop()
	if len(probe.effects) != 0 || len(probe.music) != 1 {
		t.Fatal("clearing host effects lost music")
	}
}
