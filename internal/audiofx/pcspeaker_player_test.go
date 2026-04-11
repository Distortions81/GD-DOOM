package audiofx

import (
	"encoding/binary"
	"io"
	"math"
	"testing"
	"time"

	"gddoom/internal/sound"
)

type fakePCSpeakerBackend struct {
	playing bool
	paused  int
	rewound int
	played  int
	volume  float64
	buffer  time.Duration
}

func (f *fakePCSpeakerBackend) Play()                         { f.playing = true; f.played++ }
func (f *fakePCSpeakerBackend) Pause()                        { f.playing = false; f.paused++ }
func (f *fakePCSpeakerBackend) Rewind() error                 { f.rewound++; return nil }
func (f *fakePCSpeakerBackend) SetBufferSize(d time.Duration) { f.buffer = d }
func (f *fakePCSpeakerBackend) SetVolume(v float64)           { f.volume = v }
func (f *fakePCSpeakerBackend) IsPlaying() bool               { return f.playing }
func (f *fakePCSpeakerBackend) Close() error                  { f.playing = false; return nil }

func TestPCSpeakerVariantsProducePCM(t *testing.T) {
	t.Parallel()

	seq := make([]sound.PCSpeakerTone, 140)
	for i := range seq {
		seq[i] = sound.PCSpeakerTone{Active: true, ToneValue: 96}
	}

	check := func(t *testing.T, variant PCSpeakerVariant, minPeak int) {
		t.Helper()
		src := &pcSpeakerSource{variant: variant, model: modelForVariant(variant), reverb: newCaseReverb(), streamGain: 1}
		src.load(seq, 44100)

		buf := make([]byte, 4096)
		maxAbs := 0
		for {
			n, err := src.Read(buf)
			for i := 0; i+3 < n; i += 4 {
				v := int(int16(buf[i]) | int16(buf[i+1])<<8)
				if v < 0 {
					v = -v
				}
				if v > maxAbs {
					maxAbs = v
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read failed for %s: %v", variant.String(), err)
			}
		}
		if maxAbs < minPeak {
			t.Fatalf("%s peak too low: got %d want >= %d", variant.String(), maxAbs, minPeak)
		}
	}

	check(t, PCSpeakerVariantClean, 1000)
	check(t, PCSpeakerVariantSmallSpeaker, 1000)
	check(t, PCSpeakerVariantPiezo, 1000)
}

func TestPCSpeakerEffectsInterruptMusic(t *testing.T) {
	src := &pcSpeakerSource{
		rate:           44100,
		effectSeq:      []sound.PCSpeakerTone{{Active: true, ToneValue: 20}},
		effectTickRate: 140,
		musicSeq:       []sound.PCSpeakerTone{{Active: true, ToneValue: 96}},
		musicTickRate:  140,
	}

	tone, direct, drive := src.currentToneLocked()
	if direct || drive != 0 {
		t.Fatalf("expected effect tone path, got direct=%v drive=%v", direct, drive)
	}
	if !tone.Active || tone.ToneValue != 20 {
		t.Fatalf("effect should interrupt music, got active=%v tone=%d", tone.Active, tone.ToneValue)
	}

	src.effectSeq = nil
	src.effectSamplePos = 0
	tone, direct, drive = src.currentToneLocked()
	if direct || drive != 0 {
		t.Fatalf("expected music tone path, got direct=%v drive=%v", direct, drive)
	}
	if !tone.Active || tone.ToneValue != 96 {
		t.Fatalf("music should resume after effect, got active=%v tone=%d", tone.Active, tone.ToneValue)
	}
}

func TestPCSpeakerSilentEffectTailDoesNotBlockMusic(t *testing.T) {
	src := &pcSpeakerSource{
		rate:           44100,
		effectSeq:      []sound.PCSpeakerTone{{Active: true, ToneValue: 20}, {}},
		effectTickRate: 140,
		musicSeq:       []sound.PCSpeakerTone{{Active: true, ToneValue: 96}},
		musicTickRate:  140,
	}

	first, direct, drive := src.currentToneLocked()
	if direct || drive != 0 || !first.Active || first.ToneValue != 20 {
		t.Fatalf("expected first active effect tone, got tone=%+v direct=%v drive=%v", first, direct, drive)
	}

	samplesPerTick := int(math.Round(float64(src.rate) / float64(src.effectTickRate)))
	src.effectSamplePos = samplesPerTick

	next, direct, drive := src.currentToneLocked()
	if direct || drive != 0 {
		t.Fatalf("expected music fallback, got direct=%v drive=%v", direct, drive)
	}
	if !next.Active || next.ToneValue != 96 {
		t.Fatalf("expected music after silent effect tail, got active=%v tone=%d", next.Active, next.ToneValue)
	}
}

func TestPCSpeakerMusicPCMUnderrunHoldsLastTarget(t *testing.T) {
	src := &pcSpeakerSource{
		rate:           44100,
		musicPCMRate:   11025,
		musicPCMActive: true,
	}
	src.musicPCM = make([]byte, 4)
	binary.LittleEndian.PutUint16(src.musicPCM[0:2], uint16(int16(12000)))
	binary.LittleEndian.PutUint16(src.musicPCM[2:4], uint16(int16(12000)))

	first, ok := src.nextMusicPCMDriveLocked()
	if !ok {
		t.Fatal("expected first PCM drive sample")
	}
	for i := 0; i < 6; i++ {
		drive, ok := src.nextMusicPCMDriveLocked()
		if !ok {
			t.Fatalf("unexpected underrun silence at step %d", i)
		}
		if drive != first {
			t.Fatalf("expected held drive during underrun, got %v want %v", drive, first)
		}
	}

	src.appendMusicPCM(src.musicPCM)
	if _, ok := src.nextMusicPCMDriveLocked(); !ok {
		t.Fatal("expected resumed PCM drive after append")
	}
}

func TestPCSpeakerEffectMixesIntoMusicPCMPath(t *testing.T) {
	src := &pcSpeakerSource{
		rate:           44100,
		musicPCMRate:   11025,
		musicPCMActive: true,
		effectSeq:      []sound.PCSpeakerTone{{Active: true, ToneValue: 96}},
		effectTickRate: 140,
	}
	src.musicPCM = make([]byte, 4)

	tone, direct, drive := src.currentToneLocked()
	if !direct {
		t.Fatalf("expected mixed direct path, got tone=%+v", tone)
	}
	if drive == 0 {
		t.Fatal("expected effect energy to reach music PCM path")
	}
	if len(src.effectSeq) == 0 && src.effectSamplePos != 0 {
		t.Fatal("effect sequence state invalid after mixed read")
	}
}

func TestPCSpeakerPlayDoesNotInterruptMusicPCM(t *testing.T) {
	backend := &fakePCSpeakerBackend{playing: true}
	src := &pcSpeakerSource{musicPCMActive: true}
	p := &PCSpeakerPlayer{
		player: backend,
		src:    src,
		volume: 0.75,
	}

	p.Play([]sound.PCSpeakerTone{{Active: true, ToneValue: 96}})

	if backend.paused != 0 {
		t.Fatalf("paused=%d want 0", backend.paused)
	}
	if backend.rewound != 0 {
		t.Fatalf("rewound=%d want 0", backend.rewound)
	}
	if len(src.effectSeq) != 1 {
		t.Fatalf("effect seq len=%d want 1", len(src.effectSeq))
	}
	if !src.musicPCMActive {
		t.Fatal("music PCM should remain active")
	}
}

func TestPCSpeakerEffectSurvivesMusicPCMUnderrun(t *testing.T) {
	src := &pcSpeakerSource{
		rate:           44100,
		musicPCMRate:   11025,
		musicPCMActive: true,
		effectSeq:      []sound.PCSpeakerTone{{Active: true, ToneValue: 96}},
		effectTickRate: 140,
	}

	drive, ok := src.nextMusicPCMDriveLocked()
	if !ok {
		t.Fatal("expected effect-only target during music underrun")
	}
	if drive == 0 {
		t.Fatal("expected non-zero drive during music underrun")
	}
}

func TestPCSpeakerSetMusicClearsStalePCMState(t *testing.T) {
	src := &pcSpeakerSource{
		rate:           44100,
		musicPCMActive: true,
		musicPCMRate:   11025,
	}

	src.setMusic([]sound.PCSpeakerTone{{Active: true, ToneValue: 96}}, 44100, 140, true)

	if src.musicPCMActive {
		t.Fatal("expected PCM mode cleared when setting tone-sequence music")
	}
	if len(src.musicPCM) != 0 {
		t.Fatalf("musicPCM len=%d want 0", len(src.musicPCM))
	}
	tone, direct, drive := src.currentToneLocked()
	if direct || drive != 0 {
		t.Fatalf("expected tone-sequence music path, got direct=%v drive=%v", direct, drive)
	}
	if !tone.Active || tone.ToneValue != 96 {
		t.Fatalf("expected tone-sequence music after clearing PCM, got active=%v tone=%d", tone.Active, tone.ToneValue)
	}
}

func TestPCSpeakerSetMusicRewindsBackend(t *testing.T) {
	backend := &fakePCSpeakerBackend{}
	p := &PCSpeakerPlayer{
		player: backend,
		src:    &pcSpeakerSource{},
		volume: 0.5,
	}

	p.SetMusic([]sound.PCSpeakerTone{{Active: true, ToneValue: 96}}, 140, true)

	if backend.rewound != 1 {
		t.Fatalf("rewound=%d want 1", backend.rewound)
	}
	if backend.played != 1 {
		t.Fatalf("played=%d want 1", backend.played)
	}
}
