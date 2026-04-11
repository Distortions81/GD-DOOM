package music

import (
	"testing"

	"gddoom/internal/sound"
)

func TestComboPattern(t *testing.T) {
	tests := []struct {
		candidates []pcSpeakerCandidate
		want       []int
	}{
		{nil, nil},
		{[]pcSpeakerCandidate{{}}, []int{0}},
		{[]pcSpeakerCandidate{{}, {}}, []int{0, 0, 1, 0, 0, 1}},
		{[]pcSpeakerCandidate{{}, {}, {}}, []int{0, 0, 1, 0, 2, 1, 0, 0, 1}},
		{[]pcSpeakerCandidate{{}, {}, {}, {}}, []int{0, 0, 1, 0, 2, 1, 0, 3, 1, 0, 2, 1}},
	}
	for _, tc := range tests {
		got := comboPattern(tc.candidates)
		if len(got) != len(tc.want) {
			t.Fatalf("comboPattern len=%d want=%d", len(got), len(tc.want))
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("comboPattern[%d]=%d want=%d", i, got[i], tc.want[i])
			}
		}
	}
}

func TestPCSpeakerRendererInterleavesVoicesByPriority(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 120})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 64, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 2, A: 67, B: 80})

	got := make([]byte, 4)
	for i := range got {
		got[i] = byte(r.toneForSubTick(i).ToneDivisor() & 0xff)
	}

	if got[0] == 0 || got[1] == 0 {
		t.Fatalf("expected non-zero tones, got=%v", got)
	}
	if got[0] != got[1] {
		t.Fatalf("expected base tone to repeat first, got=%v", got)
	}
	if got[2] == got[0] {
		t.Fatalf("expected interruption tone at tick 2, got=%v", got)
	}
	if got[3] != got[0] {
		t.Fatalf("expected return to base tone at tick 3, got=%v", got)
	}
}

func TestPCSpeakerRendererPrefersStrongerVoicesWhenTooManyActive(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 120})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 62, B: 110})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 2, A: 64, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 3, A: 65, B: 90})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 4, A: 67, B: 10})

	candidates := r.interleaveCandidates()
	if len(candidates) != 4 {
		t.Fatalf("candidate len=%d want 4", len(candidates))
	}
	weakestDivisor := uint16(0)
	for _, vi := range r.driver.allocList {
		v := r.driver.voices[vi]
		if v.active && v.velocity == 10 {
			weakestDivisor = sound.PITDivisorForFrequency(oplFreqWordFrequency(v.freqWord))
			break
		}
	}
	for _, c := range candidates {
		if weakestDivisor != 0 && c.divisor == weakestDivisor {
			t.Fatal("weakest voice should have been dropped from interleave set")
		}
	}
}
