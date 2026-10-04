//go:build raylib && cgo && integration && !js

package main

import (
	"encoding/binary"
	"io"
	"os"
	"runtime"
	"slices"
	"testing"

	"gddoom/internal/audiofx"
	"gddoom/internal/music"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
	gobeep86 "github.com/Distortions81/GoBeep86"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeSharedPCSpeakerMusicAndEffectsMatchMainPCM(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("requires native audio")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	audiofx.SetPCSpeakerInterleaveHz(280)
	defer audiofx.SetPCSpeakerInterleaveHz(140)
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"passthrough", "paper-speaker", "small-buzzer"} {
		t.Run(variant, func(t *testing.T) {
			a := newNativeAudio(wf, .7)
			if a == nil {
				t.Fatal("audio unavailable")
			}
			defer a.Close()
			if err := a.ConfigureSpeaker(true, .75, variant); err != nil {
				t.Fatal(err)
			}
			cfg := defaultNativeMusicConfig()
			cfg.backend, cfg.speakerVariant = music.BackendPCSpeaker, variant
			m, err := newNativeMusicWithConfig(wf, .75, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			m.UseSharedSpeaker(a.speaker)
			if err := m.PlayMap("E1M1"); err != nil {
				t.Fatal(err)
			}
			backend := a.speakerBackend
			if !backend.IsPlaying() || rl.IsAudioStreamPlaying(m.stream) || backend.volume != .75 {
				t.Fatal("PC music did not use the shared speaker transport")
			}
			reference := gobeep86.NewSource(audiofx.ParsePCSpeakerVariant(variant))
			reference.SetGain(.75)
			tones := make([]gobeep86.Tone, len(m.speakerSequence))
			for i, tone := range m.speakerSequence {
				tones[i] = gobeep86.Tone{Active: tone.Active, Divisor: tone.ToneDivisor()}
			}
			reference.SetMusic(tones, music.OutputSampleRate, m.speakerRate, true)
			// The shared player's SetMusic rewinds its reader before playback.
			if _, err := reference.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			want := make([]byte, nativeSpeakerFrames*4)
			for range 2 {
				if _, err := io.ReadFull(reference, want); err != nil {
					t.Fatal(err)
				}
			}
			for i, sample := range backend.pcm {
				if sample != int16(binary.LittleEndian.Uint16(want[i*2:i*2+2])) {
					t.Fatalf("primed music differs at sample %d", i)
				}
			}
			m.SetPaused(true)
			before := append([]int16(nil), backend.pcm...)
			if err := backend.Update(); err != nil {
				t.Fatal(err)
			}
			if backend.IsPlaying() || !slices.Equal(before, backend.pcm) {
				t.Fatal("pause advanced the shared speaker")
			}
			m.SetPaused(false)
			if !backend.IsPlaying() {
				t.Fatal("shared speaker did not resume")
			}
			seq := []sound.PCSpeakerTone{}
			for range 50 {
				seq = append(seq, sound.PCSpeakerTone{Active: true, Divisor: 2000})
			}
			a.speaker.Play(seq)
			effects := make([]gobeep86.Tone, len(seq))
			for i, tone := range seq {
				effects[i] = gobeep86.Tone{Active: tone.Active, Divisor: tone.ToneDivisor()}
			}
			reference.SetEffectMixed(effects, music.OutputSampleRate, 140)
			compare := func() {
				t.Helper()
				got := make([]byte, len(want))
				if _, err := io.ReadFull(backend.source, got); err != nil {
					t.Fatal(err)
				}
				if _, err := io.ReadFull(reference, want); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, want) {
					t.Fatal("shared effect/music PCM differs from the main speaker model")
				}
			}
			compare()
			bad := cfg
			bad.backend, bad.soundFont = music.BackendMeltySynth, "missing-font.sf2"
			if err := m.Configure(bad); err == nil || m.track != "D_E1M1" || !backend.source.(*gobeep86.Source).MusicIsActive() {
				t.Fatal("failed synth switch replaced shared speaker playback")
			}
			compare()
			cfg.backend = music.BackendImpSynth
			if err := m.Configure(cfg); err != nil {
				t.Fatal(err)
			}
			reference.ClearMusic()
			if backend.source.(*gobeep86.Source).MusicIsActive() || !backend.IsPlaying() {
				t.Fatal("synth switch retained PC music or stopped its effect")
			}
			compare()
			a.Stop()
			if len(a.voices) != 0 {
				t.Fatal("speaker effects created digital aliases")
			}
		})
	}
}
