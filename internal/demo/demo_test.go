package demo

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestHeaderForRecordingWritesAllFlags(t *testing.T) {
	h, err := HeaderForRecording("E1M1", RecordingOptions{
		Skill:           3,
		Deathmatch:      true,
		FastMonsters:    true,
		RespawnMonsters: true,
		NoMonsters:      true,
	})
	if err != nil {
		t.Fatalf("HeaderForRecording: %v", err)
	}
	if !h.Deathmatch {
		t.Error("Deathmatch should be true")
	}
	if !h.Respawn {
		t.Error("Respawn should be true")
	}
	if !h.Fast {
		t.Error("Fast should be true")
	}
	if !h.NoMonsters {
		t.Error("NoMonsters should be true")
	}
}

func TestHeaderForRecordingDefaultsFalse(t *testing.T) {
	h, err := HeaderForRecording(mapdata.MapName("MAP01"), RecordingOptions{Skill: 2})
	if err != nil {
		t.Fatalf("HeaderForRecording: %v", err)
	}
	if h.Respawn || h.NoMonsters || h.Deathmatch || h.Fast {
		t.Errorf("flags should all be false by default: %+v", h)
	}
}

func TestParseAllowsTrailingBytesAfterMarker(t *testing.T) {
	data := []byte{
		Version109,
		4, 0, 24,
		0, 0, 0, 0,
		0,
		1, 0, 0, 0,
		10, 20, 30, 40,
		Marker,
		'D', 'S', 'D', 'A',
	}

	script, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(script.Tics) != 1 {
		t.Fatalf("tic count=%d want 1", len(script.Tics))
	}
	if script.Header.Map != 24 {
		t.Fatalf("map=%d want 24", script.Header.Map)
	}
	if got := script.Tics[0]; got != (Tic{Forward: 10, Side: 20, AngleTurn: 30 << 8, Buttons: 40}) {
		t.Fatalf("tic=%+v want %+v", got, Tic{Forward: 10, Side: 20, AngleTurn: 30 << 8, Buttons: 40})
	}
}
