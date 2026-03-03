package automap

import "testing"

func TestMakeSpans_OpenCloseSimple(t *testing.T) {
	g := &game{viewW: 8, viewH: 8}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 160}
	pl := g.floorVisplaneForKey(key)

	// Rect from x=2..5, y=3..4.
	for x := 2; x <= 5; x++ {
		ix := x + 1
		pl.top[ix] = 3
		pl.bottom[ix] = 4
	}
	pl.minX = 2
	pl.maxX = 5

	spans := makePlaneSpans(pl, g.viewH, nil)
	if len(spans) != 2 {
		t.Fatalf("span count=%d want=2", len(spans))
	}
	byY := map[int]floorSpan{}
	for _, s := range spans {
		byY[s.y] = s
	}
	if s, ok := byY[3]; !ok || s.x1 != 2 || s.x2 != 5 {
		t.Fatalf("row y=3 span=%+v ok=%t want x1=2 x2=5", s, ok)
	}
	if s, ok := byY[4]; !ok || s.x1 != 2 || s.x2 != 5 {
		t.Fatalf("row y=4 span=%+v ok=%t want x1=2 x2=5", s, ok)
	}
}

func TestMakeSpans_HandlesSentinelBoundaries(t *testing.T) {
	g := &game{viewW: 6, viewH: 6}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 160}
	pl := g.floorVisplaneForKey(key)

	// Single-column fill at right edge.
	x := 5
	ix := x + 1
	pl.top[ix] = 1
	pl.bottom[ix] = 3
	pl.minX = x
	pl.maxX = x

	spans := makePlaneSpans(pl, g.viewH, nil)
	if len(spans) != 3 {
		t.Fatalf("span count=%d want=3", len(spans))
	}
	byY := map[int]floorSpan{}
	for _, s := range spans {
		byY[s.y] = s
	}
	for _, y := range []int{1, 2, 3} {
		if s, ok := byY[y]; !ok || s.x1 != 5 || s.x2 != 5 {
			t.Fatalf("row y=%d span=%+v ok=%t want x1=5 x2=5", y, s, ok)
		}
	}
}

func TestMakeSpans_NoNegativeWidth(t *testing.T) {
	out := appendSpanIfValid(nil, floorPlaneKey{}, 2, 7, 3)
	if len(out) != 0 {
		t.Fatalf("expected no span for negative width, got %d", len(out))
	}
}
