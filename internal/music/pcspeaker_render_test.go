package music

import (
	"testing"

	"gddoom/internal/sound"
)

type renderSingleVoicePatchBank struct {
	patch Patch
}

func (b renderSingleVoicePatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	return b.patch
}

func (b renderSingleVoicePatchBank) PatchVoices(program uint8, percussion bool, note uint8) []NotePatch {
	return []NotePatch{{Patch: b.patch}}
}

func TestPCSpeakerRendererInterleavesVoicesByPriority(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 120})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 64, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 2, A: 67, B: 110})

	first := r.toneForSubTick(0)
	second := r.toneForSubTick(1)
	third := r.toneForSubTick(3)
	if !first.Active || !second.Active || !third.Active {
		t.Fatalf("expected active tones across scheduled substeps: first=%+v second=%+v third=%+v", first, second, third)
	}
	if first.Divisor == second.Divisor {
		t.Fatalf("expected lower-priority note to get a scheduled slot, first=%+v second=%+v", first, second)
	}
	if first.Divisor == third.Divisor {
		t.Fatalf("expected third voice to get a scheduled slot, first=%+v third=%+v", first, third)
	}
}

func TestPCSpeakerRendererPrefersMusicalPriorityWhenManyVoicesActive(t *testing.T) {
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
	if top.ch != 0 || top.note != normalizePCSpeakerNote(60) {
		t.Fatalf("top candidate=(ch=%d note=%d) want channel 0 note %d", top.ch, top.note, normalizePCSpeakerNote(60))
	}
}

func TestPCSpeakerRendererPercussionBecomesBurst(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 9, A: 35, B: 100})
	if !r.toneForSubTick(0).Active {
		t.Fatal("expected percussion burst on first substep")
	}
	if !r.toneForSubTick(1).Active {
		t.Fatal("expected kick burst to continue on second active substep")
	}
	if !r.toneForSubTick(2).Active {
		t.Fatal("expected kick burst to include a third thump before the gap")
	}
	if r.toneForSubTick(3).Active {
		t.Fatal("expected kick burst to include a gap")
	}
	if !r.toneForSubTick(4).Active {
		t.Fatal("expected kick burst to resume after the gap")
	}
}

func TestPCSpeakerRendererPercussionDoesNotHideMelody(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 64, B: 100})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 9, A: 38, B: 100})

	first := r.toneForSubTick(0)
	second := r.toneForSubTick(1)
	third := r.toneForSubTick(2)
	if !first.Active {
		t.Fatal("expected percussion accent on first substep")
	}
	if first.Divisor == second.Divisor {
		t.Fatal("expected melody to resume immediately on percussion gap")
	}
	if !second.Active {
		t.Fatal("expected melody on percussion gap")
	}
	if !third.Active || third.Divisor == second.Divisor {
		t.Fatal("expected later percussion hit to interrupt melody again")
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

func TestPCSpeakerRendererPrefersPrimaryInstrumentVoice(t *testing.T) {
	r := newPCSpeakerRenderer(doubleVoiceBank{})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 100})

	candidates := r.activeCandidates()
	if len(candidates) != 1 {
		t.Fatalf("candidate len=%d want 1 primary voice only", len(candidates))
	}
	if candidates[0].note != 60 {
		t.Fatalf("candidate note=%d want 60", candidates[0].note)
	}
}

func TestPCSpeakerRendererNewHighPriorityNoteInterruptsImmediately(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 100})
	low := r.toneForSubTick(0)
	if !low.Active {
		t.Fatal("expected initial note active")
	}

	r.applyEvent(Event{Type: EventNoteOn, Channel: 1, A: 72, B: 100})
	high := r.toneForSubTick(0)
	if !high.Active {
		t.Fatal("expected higher-priority note active immediately")
	}
	if high.Divisor == low.Divisor {
		t.Fatalf("expected new high note to interrupt current note, low=%+v high=%+v", low, high)
	}
}

func TestPCSpeakerRendererDoesNotLetRecencyBeatStrongerLead(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.activeNotes[pcSpeakerNoteKey(0, 76)] = pcSpeakerNoteState{
		channel:  0,
		note:     76,
		velocity: 112,
		playNote: 76,
		patch:    Patch{Car40: 0x00, Car60: 0xF4, Car80: 0x21},
		start:    0,
	}
	r.activeNotes[pcSpeakerNoteKey(1, 52)] = pcSpeakerNoteState{
		channel:  1,
		note:     52,
		velocity: 72,
		playNote: 52,
		patch:    Patch{Car40: 0x18, Car60: 0x82, Car80: 0xD7},
		start:    23,
	}
	r.renderStep = 24

	candidates := r.activeCandidates()
	if len(candidates) != 2 {
		t.Fatalf("candidate len=%d want 2", len(candidates))
	}
	if candidates[0].note != normalizePCSpeakerNote(76) {
		t.Fatalf("top note=%d want lead note %d", candidates[0].note, normalizePCSpeakerNote(76))
	}
}

func TestPCSpeakerRendererUsesChannelAndPatchPriority(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.activeNotes[pcSpeakerNoteKey(0, 72)] = pcSpeakerNoteState{
		channel:  0,
		note:     72,
		velocity: 96,
		playNote: 72,
		patch:    Patch{Car40: 0x00, Car60: 0xE4, Car80: 0x32, C0: 0x06},
		start:    0,
	}
	r.activeNotes[pcSpeakerNoteKey(4, 48)] = pcSpeakerNoteState{
		channel:  4,
		note:     48,
		velocity: 112,
		playNote: 48,
		patch:    Patch{Car40: 0x10, Car60: 0x62, Car80: 0xD8, C0: 0x00},
		start:    20,
	}
	r.renderStep = 24

	candidates := r.activeCandidates()
	if len(candidates) != 2 {
		t.Fatalf("candidate len=%d want 2", len(candidates))
	}
	if candidates[0].ch != 0 {
		t.Fatalf("top channel=%d want lead channel 0", candidates[0].ch)
	}
}

func TestPCSpeakerRendererAlternatesClosePriorityNotes(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.activeNotes[pcSpeakerNoteKey(0, 76)] = pcSpeakerNoteState{
		channel:  0,
		note:     76,
		velocity: 104,
		playNote: 76,
		patch:    Patch{Car40: 0x00, Car60: 0xD4, Car80: 0x31, C0: 0x04},
		start:    0,
	}
	r.activeNotes[pcSpeakerNoteKey(1, 72)] = pcSpeakerNoteState{
		channel:  1,
		note:     72,
		velocity: 100,
		playNote: 72,
		patch:    Patch{Car40: 0x02, Car60: 0xC4, Car80: 0x32, C0: 0x03},
		start:    1,
	}

	first := r.toneForSubTick(0)
	if !first.Active {
		t.Fatalf("expected first note active: %+v", first)
	}
	period := pcSpeakerInterleaveHoldSubsteps
	var later sound.PCSpeakerTone
	for i := 0; i < period+2; i++ {
		later = r.toneForSubTick(i % pcSpeakerMusicSubstepsPerTick)
	}
	if !later.Active {
		t.Fatalf("expected later note active after interleave window: %+v", later)
	}
	if first.Divisor == later.Divisor {
		t.Fatalf("expected close-priority notes to alternate over time, first=%+v later=%+v", first, later)
	}
}

func TestPCSpeakerRendererDropsDecayedSilentVoice(t *testing.T) {
	shortPatch := Patch{Car40: 0x00, Car60: 0x0F, Car80: 0xF0}
	r := newPCSpeakerRenderer(renderSingleVoicePatchBank{patch: shortPatch})
	r.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 72, B: 100})

	for i := 0; i < 48; i++ {
		_ = r.toneForSubTick(i % pcSpeakerMusicSubstepsPerTick)
	}
	candidates := r.activeCandidates()
	if len(candidates) != 1 {
		t.Fatalf("candidate len=%d want 1 aged note retained for fallback", len(candidates))
	}
	if candidates[0].audible {
		t.Fatal("expected short-decay voice to age out of the audible pool")
	}
}

func TestPCSpeakerRendererDoesNotLetInaudibleVoiceBlockAudibleOne(t *testing.T) {
	r := newPCSpeakerRenderer(nil)
	r.activeNotes[pcSpeakerNoteKey(0, 72)] = pcSpeakerNoteState{
		channel:  0,
		note:     72,
		velocity: 100,
		playNote: 72,
		patch:    Patch{Car40: 0x00, Car60: 0x0F, Car80: 0xF0},
		start:    0,
	}
	r.activeNotes[pcSpeakerNoteKey(1, 60)] = pcSpeakerNoteState{
		channel:  1,
		note:     60,
		velocity: 100,
		playNote: 60,
		patch:    Patch{Car40: 0x00, Car60: 0x00, Car80: 0x00},
		start:    0,
	}
	r.renderStep = 24

	candidates := r.activeCandidates()
	if len(candidates) != 2 {
		t.Fatalf("candidate len=%d want 2", len(candidates))
	}
	if !candidates[0].audible {
		t.Fatal("expected audible candidate to sort ahead of inaudible one")
	}
}
