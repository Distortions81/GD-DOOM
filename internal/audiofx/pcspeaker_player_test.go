package audiofx

import (
	"testing"
	"time"

	"gddoom/internal/sound"
	gobeep86 "github.com/Distortions81/GoBeep86"
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
