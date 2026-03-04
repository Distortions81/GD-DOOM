package music

import "testing"

func TestDriverRenderSimpleNote(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	events := []Event{
		{Type: EventProgramChange, Channel: 0, A: 0},
		{Type: EventNoteOn, Channel: 0, A: 60, B: 120},
		{Type: EventNoteOff, Channel: 0, A: 60, DeltaTics: 35},
		{Type: EventEnd},
	}
	pcm := d.Render(events)
	if len(pcm) == 0 {
		t.Fatal("expected non-empty PCM")
	}
	nonZero := false
	for _, s := range pcm {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("expected non-zero PCM samples")
	}
}

func TestNoteToFnumBlockRange(t *testing.T) {
	for note := 0; note <= 127; note++ {
		f, b := noteToFnumBlock(note, 0)
		if f < 1 || f > 1023 {
			t.Fatalf("note=%d fnum=%d out of range", note, f)
		}
		if b < 0 || b > 7 {
			t.Fatalf("note=%d block=%d out of range", note, b)
		}
	}
}

func TestVoiceStealKeepsRendering(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	var evs []Event
	// Force more notes than voices.
	for n := 24; n < 24+24; n++ {
		evs = append(evs, Event{Type: EventNoteOn, Channel: 0, A: uint8(n), B: 100, DeltaTics: 1})
	}
	evs = append(evs, Event{Type: EventEnd})
	pcm := d.Render(evs)
	if len(pcm) == 0 {
		t.Fatal("expected PCM after voice stealing")
	}
}
