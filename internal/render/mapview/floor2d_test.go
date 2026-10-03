package mapview

import "testing"

func TestFloorDirectedEdgesDoNotFillAnUnboundedHalfPlane(t *testing.T) {
	identity := func(x, y float64) (float64, float64) { return x, y }
	pix := make([]byte, 16*16*4)
	RasterizeFloor2D(pix, FloorRasterInput{
		ViewW: 16, ViewH: 16, ViewBBox: WorldBBox{0, 0, 16, 16},
		LoopSets:      []FloorLoopSet{{Edges: []WorldEdge{{WorldPt{8, 2}, WorldPt{8, 14}}}, BBox: WorldBBox{8, 2, 8, 14}}},
		ScreenToWorld: identity, WorldToScreen: identity,
	})
	for i := 3; i < len(pix); i += 4 {
		if pix[i] != 0 {
			t.Fatal("an isolated wall side filled an unbounded half-plane")
		}
	}
}

func TestFloorDirectedEdgesKeepJunctionsAndHoles(t *testing.T) {
	// The outer boundary deliberately overshoots a junction instead of
	// sharing its endpoint. A closed ring cannot be traced from these edges.
	edges := []WorldEdge{
		{WorldPt{2, 2}, WorldPt{14, 2}},
		{WorldPt{12, 2}, WorldPt{12, 12}},
		{WorldPt{12, 12}, WorldPt{2, 12}},
		{WorldPt{2, 12}, WorldPt{2, 2}},
		// Clockwise hole and a pair of internal, self-referencing sides.
		{WorldPt{5, 5}, WorldPt{5, 9}},
		{WorldPt{5, 9}, WorldPt{9, 9}},
		{WorldPt{9, 9}, WorldPt{9, 5}},
		{WorldPt{9, 5}, WorldPt{5, 5}},
		{WorldPt{3, 3}, WorldPt{11, 11}},
		{WorldPt{11, 11}, WorldPt{3, 3}},
	}
	for _, flip := range []bool{false, true} {
		transform := func(x, y float64) (float64, float64) {
			if flip {
				return y, x
			}
			return x, y
		}
		pix := make([]byte, 16*16*4)
		RasterizeFloor2D(pix, FloorRasterInput{
			ViewW: 16, ViewH: 16, ViewBBox: WorldBBox{0, 0, 16, 16},
			LoopSets:    []FloorLoopSet{{Edges: edges, BBox: WorldBBox{2, 2, 14, 12}}},
			FallbackRGB: [3]byte{50, 100, 200}, ScreenToWorld: transform, WorldToScreen: transform,
		})
		for py := 0; py < 16; py++ {
			for px := 0; px < 16; px++ {
				x, y := transform(float64(px)+0.5, float64(py)+0.5)
				want := x >= 2 && x < 12 && y >= 2 && y < 12 && !(x >= 5 && x < 9 && y >= 5 && y < 9)
				if got := pix[(py*16+px)*4+3] != 0; got != want {
					t.Fatalf("flip=%t at (%g,%g): filled=%t want %t", flip, x, y, got, want)
				}
			}
		}
	}
}

func TestFloorDirectedEdgesUseWindingForOverlappingRegions(t *testing.T) {
	var edges []WorldEdge
	for _, x := range []float64{1, 4} {
		ring := []WorldPt{{x, 1}, {x + 6, 1}, {x + 6, 7}, {x, 7}}
		for i, a := range ring {
			edges = append(edges, WorldEdge{a, ring[(i+1)%4]})
		}
	}
	pix := make([]byte, 12*10*4)
	identity := func(x, y float64) (float64, float64) { return x, y }
	RasterizeFloor2D(pix, FloorRasterInput{
		ViewW: 12, ViewH: 10, ViewBBox: WorldBBox{0, 0, 12, 10},
		LoopSets:      []FloorLoopSet{{Edges: edges, BBox: WorldBBox{1, 1, 10, 7}}},
		ScreenToWorld: identity, WorldToScreen: identity,
	})
	for py := 0; py < 10; py++ {
		for px := 0; px < 12; px++ {
			want := px >= 1 && px < 10 && py >= 1 && py < 7
			if got := pix[(py*12+px)*4+3] != 0; got != want {
				t.Fatalf("(%d,%d): filled=%t want %t", px, py, got, want)
			}
		}
	}
}

func TestFloorRingInputRetainsEvenOddHoles(t *testing.T) {
	// Even-odd ring input must retain holes regardless of ring orientation.
	pix := make([]byte, 16*16*4)
	identity := func(x, y float64) (float64, float64) { return x, y }
	RasterizeFloor2D(pix, FloorRasterInput{
		ViewW: 16, ViewH: 16, ViewBBox: WorldBBox{0, 0, 16, 16},
		LoopSets: []FloorLoopSet{{Rings: [][]WorldPt{
			{{2, 2}, {12, 2}, {12, 12}, {2, 12}},
			{{5, 5}, {9, 5}, {9, 9}, {5, 9}},
		}, BBox: WorldBBox{2, 2, 12, 12}}},
		ScreenToWorld: identity, WorldToScreen: identity,
	})
	if pix[(3*16+3)*4+3] != 255 || pix[(7*16+7)*4+3] != 0 || pix[(14*16+14)*4+3] != 0 {
		t.Fatal("ring input lost its even-odd fill")
	}
}
