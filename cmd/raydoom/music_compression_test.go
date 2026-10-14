//go:build raylib && cgo && !js

package main

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"gddoom/internal/music"
	"gddoom/internal/wad"
)

func TestNativeMusicCompressionPCMMatchesSharedScore(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	lump, _ := wf.LumpByName("D_E1M1")
	data, err := wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := music.ParseMUSData(data)
	if err != nil {
		t.Fatal(err)
	}
	original, err := music.ParseMUSData(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []music.Backend{music.BackendImpSynth, music.BackendMeltySynth, music.BackendPCSpeaker} {
		t.Run(backend.String(), func(t *testing.T) {
			for _, ratio := range []float64{1, 3, 8, 1} {
				cfg := defaultNativeMusicConfig()
				cfg.backend, cfg.volumeCompression = backend, ratio
				cfg.soundFont = "../../soundfonts/general-midi.sf2"
				got, err := newNativeMusicState(wf, cfg, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := got.prepareRawScore(raw, "D_E1M1"); err != nil {
					t.Fatal(err)
				}
				want, err := newNativeMusicState(wf, cfg, got.bank, got.font)
				if err != nil {
					t.Fatal(err)
				}
				compressed := music.ApplyMUSVolumeCompression(raw, ratio)
				if err := want.prepareScore(compressed, "D_E1M1"); err != nil {
					t.Fatal(err)
				}
				if got.rawParsed != raw || !reflect.DeepEqual(got.parsed, compressed) {
					t.Fatalf("ratio %g did not retain source and shared compressed score", ratio)
				}
				for chunk := 0; chunk < 4; chunk++ {
					if !slices.Equal(got.pcm, want.pcm) {
						t.Fatalf("ratio %g chunk %d differs from shared compression PCM", ratio, chunk)
					}
					if chunk < 3 {
						if _, err := got.nextPCM(); err != nil {
							t.Fatal(err)
						}
						if _, err := want.nextPCM(); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
		})
	}
	if !reflect.DeepEqual(raw, original) {
		t.Fatal("compression changed the source events")
	}
}

func TestNativeMusicSettingsRetainEffectiveCompression(t *testing.T) {
	if defaultNativeMusicConfig().volumeCompression != music.DefaultMUSVolumeCompression {
		t.Fatal("native default differs from main")
	}
	for _, ratio := range []float64{0, 1, 2.5, 3, 9, math.NaN(), math.Inf(-1), math.Inf(1)} {
		s := nativeSettings{musicCompression: ratio}
		cfg := s.musicConfig()
		want := music.NormalizeMUSVolumeCompression(ratio)
		if cfg.volumeCompression != want || cfg != s.musicConfig() {
			t.Fatalf("ratio %g produced unstable or incorrect config: %+v", ratio, cfg)
		}
		cfg.backend = music.BackendPCSpeaker
		s.setMusicConfig(cfg)
		if s.musicConfig().volumeCompression != want || s.musicCompression != want {
			t.Fatal("synth change lost compression")
		}
	}
}
