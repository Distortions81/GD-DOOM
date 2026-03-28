package demo

import "testing"

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
