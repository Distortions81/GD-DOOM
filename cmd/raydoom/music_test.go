//go:build raylib && cgo && !js

package main

import (
	"encoding/binary"
	"gddoom/internal/music"
	"gddoom/internal/wad"
	gobeep86 "github.com/Distortions81/GoBeep86"
	"io"
	"slices"
	"testing"
)

func TestNativeMusicChunksMatchSharedSynth(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	lump, _ := wf.LumpByName("GENMIDI")
	data, _ := wf.LumpData(lump)
	bank, err := music.ParseGENMIDIOP2PatchBank(data)
	if err != nil {
		t.Fatal(err)
	}
	lump, _ = wf.LumpByName("D_E1M1")
	data, _ = wf.LumpData(lump)
	parsed, err := music.ParseMUSData(data)
	if err != nil {
		t.Fatal(err)
	}
	parsed = music.ApplyMUSVolumeCompression(parsed, music.DefaultMUSVolumeCompression)
	a := music.NewDriver(music.OutputSampleRate, bank)
	b := music.NewDriver(music.OutputSampleRate, bank)
	renderer, err := music.NewParsedMUSStreamRenderer(b, parsed)
	if err != nil {
		t.Fatal(err)
	}
	m := &nativeMusic{driver: a, parsed: parsed, pcm: make([]int16, nativeMusicFrames*2)}
	audible := false
	for range 4 {
		got, err := m.nextPCM()
		if err != nil {
			t.Fatal(err)
		}
		expected, _, err := renderer.NextChunkS16LE(nativeMusicFrames)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int16, len(expected)/2)
		for i := range want {
			want[i] = int16(binary.LittleEndian.Uint16(expected[i*2 : i*2+2]))
		}
		if !slices.Equal(got, want) {
			t.Fatal("native music chunk differs from the shared synthesizer")
		}
		for _, sample := range got {
			audible = audible || sample != 0
		}
	}
	if !audible {
		t.Fatal("music stream is silent")
	}
}

func TestNativeMusicLoopsInsideBufferAndRejectsEmptyScore(t *testing.T) {
	// End after one MUS tic. Chunks cross many loop boundaries and must stay
	// full-sized instead of adding an underfilled Raylib buffer's silence.
	data := make([]byte, 16)
	copy(data, []byte{'M', 'U', 'S', 0x1a})
	binary.LittleEndian.PutUint16(data[4:6], 5)
	binary.LittleEndian.PutUint16(data[6:8], 16)
	data = append(data, 0x90, 0xbc, 100, 1, 0x60) // note-on, one-tic delay, end.
	parsed, err := music.ParseMUSData(data)
	if err != nil {
		t.Fatal(err)
	}
	m := &nativeMusic{driver: music.NewDriver(music.OutputSampleRate, nil), parsed: parsed, pcm: make([]int16, nativeMusicFrames*2)}
	pcm, err := m.nextPCM()
	if err != nil || len(pcm) != nativeMusicFrames*2 {
		t.Fatalf("looping buffer: %d samples %v", len(pcm), err)
	}
	loopSamples := music.OutputSampleRate / 140 * 2
	if !slices.Equal(pcm[:loopSamples], pcm[loopSamples:loopSamples*2]) {
		t.Fatal("loop boundary introduced silence or changed the repeated score")
	}
	binary.LittleEndian.PutUint16(data[4:6], 1)
	data = append(data[:16], 0x60)
	m.parsed, err = music.ParseMUSData(data)
	if err != nil {
		t.Fatal(err)
	}
	m.renderer = nil
	if _, err := m.nextPCM(); err == nil {
		t.Fatal("zero-duration score was not rejected")
	}
}

func TestNativeMusicAlternateSynthPCMMatchesMain(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	lump, _ := wf.LumpByName("D_E1M1")
	data, _ := wf.LumpData(lump)
	parsed, err := music.ParseMUSData(data)
	if err != nil {
		t.Fatal(err)
	}
	parsed = music.ApplyMUSVolumeCompression(parsed, music.DefaultMUSVolumeCompression)
	for _, backend := range []music.Backend{music.BackendImpSynth, music.BackendMeltySynth, music.BackendPCSpeaker} {
		t.Run(backend.String(), func(t *testing.T) {
			cfg := defaultNativeMusicConfig()
			cfg.backend = backend
			cfg.soundFont = "../../soundfonts/general-midi.sf2"
			m, err := newNativeMusicState(wf, cfg, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.prepareScore(parsed, "D_E1M1"); err != nil {
				t.Fatal(err)
			}
			var next func() []byte
			if backend == music.BackendPCSpeaker {
				seq, rate := music.RenderParsedMUSToPCSpeaker(m.bank, parsed)
				tones := make([]gobeep86.Tone, len(seq))
				for i, tone := range seq {
					tones[i] = gobeep86.Tone{Active: tone.Active, Divisor: tone.ToneDivisor()}
				}
				source := gobeep86.NewSource(gobeep86.ParseVariant(cfg.speakerVariant))
				source.SetMusic(tones, music.OutputSampleRate, rate, true)
				next = func() []byte {
					out := make([]byte, nativeMusicFrames*4)
					if _, err := io.ReadFull(source, out); err != nil {
						t.Fatal(err)
					}
					return out
				}
			} else {
				var driver nativeMusicDriver
				if backend == music.BackendMeltySynth {
					driver, err = music.NewMeltySynthDriver(music.OutputSampleRate, m.font)
				} else {
					driver, err = music.NewDriverWithBackend(music.OutputSampleRate, m.bank, backend)
				}
				if err != nil {
					t.Fatal(err)
				}
				driver.SetMUSPanMax(.8)
				driver.SetOutputGain(2.5)
				reference, err := music.NewParsedMUSStreamRenderer(driver, parsed)
				if err != nil {
					t.Fatal(err)
				}
				next = func() []byte {
					out, _, err := reference.NextChunkS16LE(nativeMusicFrames)
					if err != nil {
						t.Fatal(err)
					}
					return out
				}
			}
			audible := false
			for i := 0; i < 4; i++ {
				got := m.pcm
				if i > 0 {
					got, err = m.nextPCM()
					if err != nil {
						t.Fatal(err)
					}
				}
				expected := next()
				if len(got)*2 != len(expected) {
					t.Fatal("underfilled stereo buffer")
				}
				for i, sample := range got {
					if sample != int16(binary.LittleEndian.Uint16(expected[i*2:i*2+2])) {
						t.Fatalf("PCM differs at sample %d", i)
					}
					audible = audible || sample != 0
				}
			}
			if !audible {
				t.Fatal("synth is silent")
			}
		})
	}
}
