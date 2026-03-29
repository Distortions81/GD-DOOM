package music

import (
	"path/filepath"
	"testing"
)

func TestMeltySynthDriverResetReusesSynth(t *testing.T) {
	sf, err := ParseSoundFontFile(filepath.Join("..", "..", "soundfonts", "SC55.sf2"))
	if err != nil {
		t.Fatalf("ParseSoundFontFile() error: %v", err)
	}
	d, err := NewMeltySynthDriver(OutputSampleRate, sf)
	if err != nil {
		t.Fatalf("NewMeltySynthDriver() error: %v", err)
	}
	if d.synth == nil {
		t.Fatal("expected initialized synth")
	}
	first := d.synth
	d.ApplyEvent(Event{Type: EventProgramChange, Channel: 0, A: 5})
	d.ApplyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 100})
	d.Reset()
	if d.synth == nil {
		t.Fatal("expected synth after reset")
	}
	if d.synth != first {
		t.Fatal("reset rebuilt synth; want reuse")
	}
}
