package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestStyleForThing_PlayerStart(t *testing.T) {
	s := styleForThing(testThing(1))
	if s.glyph != thingGlyphSquare {
		t.Fatalf("glyph=%v want %v", s.glyph, thingGlyphSquare)
	}
}

func TestStyleForThing_Monster(t *testing.T) {
	s := styleForThing(testThing(3004))
	if s.glyph != thingGlyphTriangle {
		t.Fatalf("glyph=%v want %v", s.glyph, thingGlyphTriangle)
	}
}

func TestStyleForThing_Key(t *testing.T) {
	s := styleForThing(testThing(13))
	if s.glyph != thingGlyphStar {
		t.Fatalf("glyph=%v want %v", s.glyph, thingGlyphStar)
	}
	if s.clr != thingKeyRed {
		t.Fatalf("key color=%v want %v", s.clr, thingKeyRed)
	}
}

func TestStyleForThing_Item(t *testing.T) {
	s := styleForThing(testThing(2012))
	if s.glyph != thingGlyphDiamond {
		t.Fatalf("glyph=%v want %v", s.glyph, thingGlyphDiamond)
	}
}

func TestStyleForThing_MiscFallback(t *testing.T) {
	s := styleForThing(testThing(9999))
	if s.glyph != thingGlyphCross {
		t.Fatalf("glyph=%v want %v", s.glyph, thingGlyphCross)
	}
}

func testThing(typ int16) mapdata.Thing {
	return mapdata.Thing{Type: typ}
}
