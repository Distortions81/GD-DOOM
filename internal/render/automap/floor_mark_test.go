package automap

import "testing"

func TestMarkFloorColumnRange_ClampsToClipBuffers(t *testing.T) {
	g := &game{viewW: 5, viewH: 10}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 160}
	pl := g.floorVisplaneForKey(key)

	g.ceilingClip[2] = 1
	g.floorClip[2] = 8
	if ok := markFloorColumnRange(pl, 2, -4, 40, g.floorClip, g.ceilingClip); !ok {
		t.Fatal("expected markFloorColumnRange to succeed")
	}
	if got, want := pl.top[3], int16(2); got != want {
		t.Fatalf("top=%d want=%d", got, want)
	}
	if got, want := pl.bottom[3], int16(7); got != want {
		t.Fatalf("bottom=%d want=%d", got, want)
	}
	if got, want := pl.minX, 2; got != want {
		t.Fatalf("minX=%d want=%d", got, want)
	}
	if got, want := pl.maxX, 2; got != want {
		t.Fatalf("maxX=%d want=%d", got, want)
	}
}

func TestMarkFloorColumnRange_RejectsInvalidTopBottom(t *testing.T) {
	g := &game{viewW: 4, viewH: 8}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 160}
	pl := g.floorVisplaneForKey(key)

	g.ceilingClip[1] = 5
	g.floorClip[1] = 6
	if ok := markFloorColumnRange(pl, 1, 0, 20, g.floorClip, g.ceilingClip); ok {
		t.Fatal("expected invalid clamped range to be rejected")
	}
	if got := pl.top[2]; got != floorUnset {
		t.Fatalf("top should remain unset, got %d", got)
	}
	if got := pl.bottom[2]; got != floorUnset {
		t.Fatalf("bottom should remain unset, got %d", got)
	}
}

func TestMarkScreenPolygonColumns_Rectangle(t *testing.T) {
	g := &game{viewW: 20, viewH: 20}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 160}
	pl := g.floorVisplaneForKey(key)
	poly := []screenPt{
		{x: 4, y: 6},
		{x: 10, y: 6},
		{x: 10, y: 12},
		{x: 4, y: 12},
	}
	g.markScreenPolygonColumns(pl, poly)
	if pl.minX > pl.maxX {
		t.Fatal("expected marked columns for rectangle")
	}
	if pl.minX > 4 || pl.maxX < 9 {
		t.Fatalf("unexpected x coverage min=%d max=%d", pl.minX, pl.maxX)
	}
	mid := 7 + 1
	if pl.top[mid] == floorUnset || pl.bottom[mid] == floorUnset {
		t.Fatal("expected middle column to be marked")
	}
}
