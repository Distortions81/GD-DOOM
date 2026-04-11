package music

import "testing"

func TestPCSpeakerRendererInterleavesVoicesByPriority(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 120})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 64, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 2, A: 67, B: 110})

	got := r.toneForSubTick(0)
	if !got.Active || got.Divisor == 0 {
		t.Fatalf("expected active top-priority tone, got=%+v", got)
	}
}

func TestPCSpeakerRendererPrefersNewerHigherVoicesWhenTooManyActive(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 120})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 62, B: 110})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 2, A: 64, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 3, A: 65, B: 90})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 4, A: 67, B: 10})

	candidates := r.activeCandidates()
	if len(candidates) != 5 {
		t.Fatalf("candidate len=%d want 5", len(candidates))
	}
	top, ok := r.topCandidate()
	if !ok {
		t.Fatal("expected top candidate")
	}
	if top.note != normalizePCSpeakerNote(67) {
		t.Fatalf("top note=%d want=%d", top.note, normalizePCSpeakerNote(67))
	}
}

func TestPCSpeakerRendererPercussionBecomesBurst(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 9, A: 35, B: 100})
	if !r.toneForSubTick(0).Active {
		t.Fatal("expected percussion burst on first substep")
	}
	if r.toneForSubTick(1).Active {
		t.Fatal("expected percussion click to end immediately")
	}
}

func TestPCSpeakerRendererPercussionDoesNotHideMelody(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 64, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 9, A: 35, B: 100})

	first := r.toneForSubTick(0)
	second := r.toneForSubTick(1)
	if !first.Active {
		t.Fatal("expected melody on first substep")
	}
	if first.Divisor == pcSpeakerPercussionDivisor {
		t.Fatal("percussion should not replace melody when melodic candidates exist")
	}
	if !second.Active || second.Divisor != first.Divisor {
		t.Fatal("expected melody to continue uninterrupted")
	}
}

func TestPCSpeakerRendererIgnoresPercussionInPitchPool(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 9, A: 35, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 64, B: 100})

	candidates := r.activeCandidates()
	if len(candidates) != 1 {
		t.Fatalf("candidate len=%d want 1", len(candidates))
	}
	if candidates[0].note != 64 {
		t.Fatalf("candidate note=%d want 64", candidates[0].note)
	}
}

func TestPCSpeakerRendererNormalizesOutOfRangeNotes(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 36, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 96, B: 100})

	candidates := r.activeCandidates()
	if len(candidates) != 2 {
		t.Fatalf("candidate len=%d want 2", len(candidates))
	}
	for _, c := range candidates {
		if c.note < pcSpeakerMinNote || c.note > pcSpeakerMaxNote {
			t.Fatalf("normalized note=%d out of range", c.note)
		}
	}
}

func TestPCSpeakerRendererResumesLowerPriorityNoteWhenTopEnds(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 67, B: 100})

	top := r.toneForSubTick(0)
	if !top.Active {
		t.Fatal("expected top note active")
	}

	r.applyEvent(Event{Type: EventNoteOff, Channel: 1, A: 67})
	next := r.toneForSubTick(0)
	if !next.Active {
		t.Fatal("expected lower-priority note to resume")
	}
	if next.Divisor == top.Divisor {
		t.Fatal("expected resumed note to differ from ended top note")
	}
}
