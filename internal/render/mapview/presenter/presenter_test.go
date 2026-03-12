package presenter

import "testing"

func TestShouldDrawThings(t *testing.T) {
	if ShouldDrawThings(1) {
		t.Fatalf("iddt1 should not draw things")
	}
	if !ShouldDrawThings(2) {
		t.Fatalf("iddt2 should draw things")
	}
}

func TestRelativeThingAngle(t *testing.T) {
	got := RelativeThingAngle(90, degToAngle(90))
	if got != 0 {
		t.Fatalf("RelativeThingAngle aligned = %d, want 0", got)
	}

	got = RelativeThingAngle(180, degToAngle(90))
	if got != -90 {
		t.Fatalf("RelativeThingAngle offset = %d, want -90", got)
	}
}

func TestWorldThingAngle(t *testing.T) {
	if got := WorldThingAngle(90); got != 0 {
		t.Fatalf("WorldThingAngle(90)=%d want=0", got)
	}
	if got := WorldThingAngle(0); got != 90 {
		t.Fatalf("WorldThingAngle(0)=%d want=90", got)
	}
	if got := WorldThingAngle(180); got != -90 {
		t.Fatalf("WorldThingAngle(180)=%d want=-90", got)
	}
}

func degToAngle(deg int) uint32 {
	return uint32(uint64(deg) * 4294967296 / 360)
}

func TestStyleForThingType(t *testing.T) {
	tests := []struct {
		name          string
		typ           int16
		isPlayerStart bool
		isMonster     bool
		wantGlyph     Glyph
		wantColor     [3]uint8
	}{
		{name: "player", typ: 1, isPlayerStart: true, wantGlyph: GlyphSquare, wantColor: [3]uint8{ThingPlayerColor.R, ThingPlayerColor.G, ThingPlayerColor.B}},
		{name: "monster", typ: 3004, isMonster: true, wantGlyph: GlyphTriangle, wantColor: [3]uint8{ThingMonsterColor.R, ThingMonsterColor.G, ThingMonsterColor.B}},
		{name: "red key", typ: 13, wantGlyph: GlyphStar, wantColor: [3]uint8{ThingKeyRed.R, ThingKeyRed.G, ThingKeyRed.B}},
		{name: "item", typ: 2012, wantGlyph: GlyphDiamond, wantColor: [3]uint8{ThingItemColor.R, ThingItemColor.G, ThingItemColor.B}},
		{name: "misc", typ: 9999, wantGlyph: GlyphCross, wantColor: [3]uint8{ThingMiscColor.R, ThingMiscColor.G, ThingMiscColor.B}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StyleForThingType(tt.typ, tt.isPlayerStart, tt.isMonster)
			if got.Glyph != tt.wantGlyph {
				t.Fatalf("glyph=%v want %v", got.Glyph, tt.wantGlyph)
			}
			if [3]uint8{got.Color.R, got.Color.G, got.Color.B} != tt.wantColor {
				t.Fatalf("color=%v want %v", [3]uint8{got.Color.R, got.Color.G, got.Color.B}, tt.wantColor)
			}
		})
	}
}
