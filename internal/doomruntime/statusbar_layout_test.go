package doomruntime

import "testing"

func TestDrawStatusTallNumUsesFixedZeroWidthSpacing(t *testing.T) {
	g := &game{
		opts: Options{
			StatusPatchBank: map[string]WallTexture{
				"STTNUM0": {Width: 14, Height: 16, RGBA: make([]byte, 14*16*4)},
				"STTNUM1": {Width: 8, Height: 16, RGBA: make([]byte, 8*16*4)},
			},
		},
	}
	draws := g.statusDigitDraws("STTNUM", 100, 3, 42, 1)
	if len(draws) != 3 {
		t.Fatalf("draw count=%d want 3", len(draws))
	}
	want := []statusDigitDraw{
		{name: "STTNUM0", x: 28},
		{name: "STTNUM0", x: 14},
		{name: "STTNUM1", x: 0},
	}
	for i := range want {
		if draws[i] != want[i] {
			t.Fatalf("draw %d=%+v want %+v", i, draws[i], want[i])
		}
	}
}
