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

func TestDMXFrequencyWordRange(t *testing.T) {
	for note := 0; note <= 127; note++ {
		freq := dmxFrequencyWord(note, 0)
		fnum := int(freq & 0x03FF)
		block := int((freq >> 10) & 0x07)
		if fnum < 1 || fnum > 1023 {
			t.Fatalf("note=%d fnum=%d out of range", note, fnum)
		}
		if block < 0 || block > 7 {
			t.Fatalf("note=%d block=%d out of range", note, block)
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

func TestDriverRenderMUS(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	score := []byte{
		0x40, 0, 0, // program 0 on channel 0
		0x90,      // note on, last
		0xBC, 110, // note 60 with velocity 110
		0x14,       // delta 20 tics
		0x00, 0x3C, // note off
		0x60, // end
	}
	mus := buildMUSTestLump(score)
	pcm, err := d.RenderMUS(mus)
	if err != nil {
		t.Fatalf("RenderMUS() error: %v", err)
	}
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
		t.Fatal("expected non-zero PCM from MUS render")
	}
}

func TestDriverRenderMUSS16LE(t *testing.T) {
	d := NewOutputDriver(nil)
	d.Reset()
	score := []byte{
		0x10, 60, // note on ch0 default velocity
		0x80, 60, // note off ch0 with delay flag
		0x08, // delay 8 tics
		0x60, // end
	}
	mus := buildMUSTestLump(score)
	pcm, err := d.RenderMUSS16LE(mus)
	if err != nil {
		t.Fatalf("RenderMUSS16LE() error: %v", err)
	}
	if len(pcm) == 0 {
		t.Fatal("expected non-empty byte PCM")
	}
	if len(pcm)%2 != 0 {
		t.Fatalf("pcm byte len=%d should be even", len(pcm))
	}
}

type capturePercussionPatchBank struct {
	percussionFlags []bool
}

func (b *capturePercussionPatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	b.percussionFlags = append(b.percussionFlags, percussion)
	return DefaultPatchBank{}.Patch(program, percussion, note)
}

func TestDriverPercussionChannelUsesPercussionPatch(t *testing.T) {
	bank := &capturePercussionPatchBank{}
	d := NewOutputDriver(bank)
	d.Reset()
	_ = d.Render([]Event{
		{Type: EventNoteOn, Channel: 9, A: 35, B: 100},
		{Type: EventEnd},
	})
	if len(bank.percussionFlags) == 0 {
		t.Fatal("expected at least one patch lookup")
	}
	if !bank.percussionFlags[0] {
		t.Fatalf("percussion flag=%v want=true", bank.percussionFlags[0])
	}
}

type doubleVoiceBank struct{}

func (doubleVoiceBank) Patch(program uint8, percussion bool, note uint8) Patch {
	return DefaultPatchBank{}.Patch(program, percussion, note)
}

func (doubleVoiceBank) PatchVoices(program uint8, percussion bool, note uint8) []NotePatch {
	return []NotePatch{
		{Patch: DefaultPatchBank{}.Patch(program, percussion, note)},
		{Patch: DefaultPatchBank{}.Patch(program, percussion, note), BaseNoteOffset: 12},
	}
}

func TestDriverNoteOffStopsAllVoicesForNote(t *testing.T) {
	d := NewDriver(49716, doubleVoiceBank{})
	d.Reset()
	_ = d.Render([]Event{
		{Type: EventNoteOn, Channel: 0, A: 60, B: 100},
		{Type: EventNoteOff, Channel: 0, A: 60},
		{Type: EventEnd},
	})
	for i, v := range d.voices {
		if v.active {
			t.Fatalf("voice %d still active after note off", i)
		}
	}
}

func TestResolveVoiceNote(t *testing.T) {
	if got := resolveVoiceNote(64, false, NotePatch{BaseNoteOffset: -12}); got != 52 {
		t.Fatalf("offset note=%d want=52", got)
	}
	if got := resolveVoiceNote(64, true, NotePatch{}); got != 60 {
		t.Fatalf("percussion note=%d want=60", got)
	}
	if got := resolveVoiceNote(64, false, NotePatch{Fixed: true, FixedNote: 40}); got != 40 {
		t.Fatalf("fixed note=%d want=40", got)
	}
	if got := resolveVoiceNote(120, false, NotePatch{}); got != 84 {
		t.Fatalf("wrapped note=%d want=84", got)
	}
}

func TestDriverPitchBendUsesMSB(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	// LSB differs; MSB same => same bend in DMX semantics.
	d.applyEvent(Event{Type: EventPitchBend, Channel: 0, A: 0x00, B: 0x40})
	b0 := d.ch[0].pitchBend
	d.applyEvent(Event{Type: EventPitchBend, Channel: 0, A: 0x7F, B: 0x40})
	b1 := d.ch[0].pitchBend
	if b0 != 0 || b1 != 0 {
		t.Fatalf("center bend b0=%d b1=%d want=0", b0, b1)
	}
	d.applyEvent(Event{Type: EventPitchBend, Channel: 0, A: 0x00, B: 0x41})
	if got := d.ch[0].pitchBend; got != 1 {
		t.Fatalf("bend=%d want=1", got)
	}
}

func TestClampMIDI7(t *testing.T) {
	if got := clampMIDI7(-5); got != 0 {
		t.Fatalf("clamp -5=%d want=0", got)
	}
	if got := clampMIDI7(200); got != 127 {
		t.Fatalf("clamp 200=%d want=127", got)
	}
	if got := clampMIDI7(64); got != 64 {
		t.Fatalf("clamp 64=%d want=64", got)
	}
}
