package automap

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview/presenter"
)

func TestStyleForThing_PlayerStart(t *testing.T) {
	s := styleForThing(testThing(1))
	if s.Glyph != presenter.GlyphSquare {
		t.Fatalf("glyph=%v want %v", s.Glyph, presenter.GlyphSquare)
	}
}

func TestStyleForThing_Monster(t *testing.T) {
	s := styleForThing(testThing(3004))
	if s.Glyph != presenter.GlyphTriangle {
		t.Fatalf("glyph=%v want %v", s.Glyph, presenter.GlyphTriangle)
	}
}

func TestStyleForThing_Key(t *testing.T) {
	s := styleForThing(testThing(13))
	if s.Glyph != presenter.GlyphStar {
		t.Fatalf("glyph=%v want %v", s.Glyph, presenter.GlyphStar)
	}
	if s.Color != presenter.ThingKeyRed {
		t.Fatalf("key color=%v want %v", s.Color, presenter.ThingKeyRed)
	}
}

func TestStyleForThing_Item(t *testing.T) {
	s := styleForThing(testThing(2012))
	if s.Glyph != presenter.GlyphDiamond {
		t.Fatalf("glyph=%v want %v", s.Glyph, presenter.GlyphDiamond)
	}
}

func TestStyleForThing_MegasphereItem(t *testing.T) {
	s := styleForThing(testThing(83))
	if s.Glyph != presenter.GlyphDiamond {
		t.Fatalf("glyph=%v want %v", s.Glyph, presenter.GlyphDiamond)
	}
}

func TestStyleForThing_MiscFallback(t *testing.T) {
	s := styleForThing(testThing(9999))
	if s.Glyph != presenter.GlyphCross {
		t.Fatalf("glyph=%v want %v", s.Glyph, presenter.GlyphCross)
	}
}

func TestRelativeThingAngle(t *testing.T) {
	// Thing and view aligned -> "up" in rotate mode.
	got := relativeThingAngle(90, degToAngle(90))
	if got != 0 {
		t.Fatalf("relativeThingAngle aligned = %d, want 0", got)
	}

	// With view north, thing west appears to the left.
	got = relativeThingAngle(180, degToAngle(90))
	if got != -90 {
		t.Fatalf("relativeThingAngle offset = %d, want -90", got)
	}
}

func TestWorldThingAngle(t *testing.T) {
	if got := worldThingAngle(90); got != 0 {
		t.Fatalf("worldThingAngle(90)=%d want=0", got)
	}
	if got := worldThingAngle(0); got != 90 {
		t.Fatalf("worldThingAngle(0)=%d want=90", got)
	}
	if got := worldThingAngle(180); got != -90 {
		t.Fatalf("worldThingAngle(180)=%d want=-90", got)
	}
}

func testThing(typ int16) mapdata.Thing {
	return mapdata.Thing{Type: typ}
}
