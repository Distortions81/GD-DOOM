package audiofx

import (
	"errors"
	"io"
	"testing"
	"time"

	"gddoom/internal/sound"
	gobeep86 "github.com/Distortions81/GoBeep86"
)

func TestPCSpeakerBackendFactoryAndPauseRetainPlayback(t *testing.T) {
	backend := &fakePCSpeakerBackend{}
	var reader io.ReadSeeker
	p, err := NewPCSpeakerWithBackend(.75, PCSpeakerVariantSmallSpeaker, func(src io.ReadSeeker) (PCSpeakerBackend, error) {
		reader = src
		return backend, nil
	})
	if err != nil || reader == nil || backend.buffer <= 0 {
		t.Fatalf("backend initialization: %v", err)
	}
	p.SetMusic([]sound.PCSpeakerTone{{Active: true, Divisor: 96}}, 140, true)
	if !backend.playing || backend.volume != .75 || backend.rewound != 1 {
		t.Fatal("injected transport did not start shared music")
	}
	p.Play([]sound.PCSpeakerTone{{Active: true, Divisor: 20}})
	if backend.rewound != 1 {
		t.Fatal("mixed effect rewound music")
	}
	p.SetPaused(true)
	if backend.playing {
		t.Fatal("pause did not reach transport")
	}
	p.SetPaused(false)
	if !backend.playing || backend.rewound != 1 {
		t.Fatal("resume restarted the source")
	}
	p.ClearEffects()
	if !p.src.MusicIsActive() {
		t.Fatal("clearing effects cleared music")
	}
	p.ClearMusic()
	if p.src.MusicIsActive() {
		t.Fatal("music did not clear")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	want := errors.New("transport failed")
	if p, err := NewPCSpeakerWithBackend(1, PCSpeakerVariantClean, func(io.ReadSeeker) (PCSpeakerBackend, error) { return nil, want }); p != nil || !errors.Is(err, want) {
		t.Fatal("failed transport initialization accepted")
	}
}

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

func TestPCSpeakerRenderDelegatesToExternalLibrary(t *testing.T) {
	seq := []sound.PCSpeakerTone{{Active: true, Divisor: 96}, {Active: true, Divisor: 96}}
	pcm, err := RenderPCSpeakerSequenceToPCM(seq, 140, PCSpeakerVariantSmallSpeaker)
	if err != nil {
		t.Fatalf("RenderPCSpeakerSequenceToPCM() error = %v", err)
	}
	if len(pcm) == 0 {
		t.Fatal("expected PCM output")
	}
}

func TestPCSpeakerPlayUsesMixedPathWhenMusicIsActive(t *testing.T) {
	backend := &fakePCSpeakerBackend{playing: true}
	src := gobeep86.NewSource(gobeep86.VariantSmallSpeaker)
	src.SetMusic([]gobeep86.Tone{{Active: true, Divisor: 96}}, gobeep86.DefaultOutputSampleRate, 140, false)
	p := &PCSpeakerPlayer{player: backend, src: src, volume: 0.75}

	p.Play([]sound.PCSpeakerTone{{Active: true, Divisor: 20}})

	if backend.paused != 0 {
		t.Fatalf("paused=%d want 0", backend.paused)
	}
	if backend.rewound != 0 {
		t.Fatalf("rewound=%d want 0", backend.rewound)
	}
}

func TestInterleavePCSpeakerSequencesReturnsOutput(t *testing.T) {
	out, tickRate := InterleavePCSpeakerSequences(
		[]sound.PCSpeakerTone{{Active: true, Divisor: 20}}, 140,
		[]sound.PCSpeakerTone{{Active: true, Divisor: 96}}, 140,
	)
	if len(out) == 0 {
		t.Fatal("expected interleaved output")
	}
	if tickRate <= 0 {
		t.Fatalf("tickRate=%d want > 0", tickRate)
	}
}
