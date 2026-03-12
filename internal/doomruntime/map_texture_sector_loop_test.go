package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestBuildSectorLoopSets_SimpleSquare(t *testing.T) {
	m := &mapdata.Map{
		Vertexes: []mapdata.Vertex{
			{X: 0, Y: 0},
			{X: 128, Y: 0},
			{X: 128, Y: 128},
			{X: 0, Y: 128},
		},
		Sectors: []mapdata.Sector{
			{FloorPic: "FLOOR4_8"},
		},
		Sidedefs: []mapdata.Sidedef{
			{Sector: 0},
		},
		Linedefs: []mapdata.Linedef{
			{V1: 0, V2: 1, SideNum: [2]int16{0, -1}},
			{V1: 1, V2: 2, SideNum: [2]int16{0, -1}},
			{V1: 2, V2: 3, SideNum: [2]int16{0, -1}},
			{V1: 3, V2: 0, SideNum: [2]int16{0, -1}},
		},
	}
	g := &game{m: m}
	sets := g.buildSectorLoopSets()
	if len(sets) != 1 {
		t.Fatalf("len(sets)=%d want 1", len(sets))
	}
	if got := len(sets[0].rings); got != 1 {
		t.Fatalf("len(rings)=%d want 1", got)
	}
	r := sets[0].rings[0]
	if got := len(r); got != 4 {
		t.Fatalf("ring vertices=%d want 4", got)
	}
}

func TestBuildSectorLoopSets_HoleEvenOdd(t *testing.T) {
	// Sector 0 is an outer square with an inner square hole (sector 1 pillar).
	m := &mapdata.Map{
		Vertexes: []mapdata.Vertex{
			// outer
			{X: 0, Y: 0},
			{X: 256, Y: 0},
			{X: 256, Y: 256},
			{X: 0, Y: 256},
			// inner
			{X: 96, Y: 96},
			{X: 160, Y: 96},
			{X: 160, Y: 160},
			{X: 96, Y: 160},
		},
		Sectors: []mapdata.Sector{
			{FloorPic: "FLOOR4_8"},
			{FloorPic: "NUKAGE1"},
		},
		Sidedefs: []mapdata.Sidedef{
			{Sector: 0}, // outer boundary sidedef
			{Sector: 1}, // inner square front (pillar)
			{Sector: 0}, // inner square back (hole boundary for sector 0)
		},
		Linedefs: []mapdata.Linedef{
			// outer one-sided lines for sector 0
			{V1: 0, V2: 1, SideNum: [2]int16{0, -1}},
			{V1: 1, V2: 2, SideNum: [2]int16{0, -1}},
			{V1: 2, V2: 3, SideNum: [2]int16{0, -1}},
			{V1: 3, V2: 0, SideNum: [2]int16{0, -1}},
			// inner two-sided lines; sector 0 is on back side
			{V1: 4, V2: 5, SideNum: [2]int16{1, 2}},
			{V1: 5, V2: 6, SideNum: [2]int16{1, 2}},
			{V1: 6, V2: 7, SideNum: [2]int16{1, 2}},
			{V1: 7, V2: 4, SideNum: [2]int16{1, 2}},
		},
	}
	g := &game{m: m}
	sets := g.buildSectorLoopSets()
	if len(sets) != 2 {
		t.Fatalf("len(sets)=%d want 2", len(sets))
	}
	if got := len(sets[0].rings); got != 2 {
		t.Fatalf("sector0 rings=%d want 2", got)
	}
	// Inside outer but outside hole.
	if !pointInRingsEvenOdd(32, 32, sets[0].rings) {
		t.Fatal("expected inside sector 0 at (32,32)")
	}
	// Inside hole should evaluate to outside by even-odd.
	if pointInRingsEvenOdd(128, 128, sets[0].rings) {
		t.Fatal("expected hole at (128,128) to be outside sector 0")
	}
	// Outside outer.
	if pointInRingsEvenOdd(300, 300, sets[0].rings) {
		t.Fatal("expected outside at (300,300)")
	}
}
