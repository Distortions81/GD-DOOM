package doomruntime

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func TestClassicBBoxRetainsFrustumEdgeColumn(t *testing.T) {
	g := &game{viewW: 320}
	n := mapdata.Node{BBoxR: [4]int16{128, 128, -1088, -960}}
	l, r, ok := g.classicBBoxRange(n, 0, -896*fracUnit, 64*fracUnit, math.Cos(math.Pi/2), math.Sin(math.Pi/2))
	if !ok || l != 0 || r != 0 {
		t.Fatalf("edge bbox=%d..%d %v, want column zero", l, r, ok)
	}
}

func TestClassicSkyHeightTransitionDoesNotMarkCeiling(t *testing.T) {
	front := &mapdata.Sector{CeilingPic: "F_SKY1", FloorPic: "FLAT", Light: 160}
	back := *front
	g := &game{}
	got := g.classifyWallPortalCached(-1, front, &back, -1, 41, 0, 128, 0, 64)
	if got.MarkCeiling || got.TopWall || got.WorldTop != 23 {
		t.Fatalf("classic sky seam: %+v", got)
	}
	g.opts.SourcePortMode = true
	if !g.classifyWallPortalCached(-1, front, &back, -1, 41, 0, 128, 0, 64).MarkCeiling {
		t.Fatal("source port sky behavior changed")
	}
}

func TestClassicBSPOcclusionUsesDrawnColumns(t *testing.T) {
	// E1M2 SEG 835 viewed from the north-facing spawn. The float projection
	// covers 144..151, but classic draws only 145..150. Marking either extra
	// column opaque can discard a farther subsector and move its flat spans.
	g := &game{viewW: 320, m: &mapdata.Map{
		Vertexes: []mapdata.Vertex{{X: -88, Y: 320}, {X: -64, Y: 320}},
		Segs:     []mapdata.Seg{{StartVertex: 0, EndVertex: 1}},
	}}
	l, r, ok := g.segScreenRange(0, -32*fracUnit, -240*fracUnit, 0, 1, 2, 160)
	if !ok || l != 145 || r != 150 {
		t.Fatalf("occlusion range %d..%d visible=%t, want 145..150", l, r, ok)
	}
	g.m.Segs[0].StartVertex, g.m.Segs[0].EndVertex = 1, 0
	if _, _, ok := g.segScreenRange(0, -32*fracUnit, -240*fracUnit, 0, 1, 2, 160); ok {
		t.Fatal("back-facing segment must not occlude")
	}
}

func classicPlaneTestGame() *game {
	return &game{viewW: 8, viewH: 4, plane3DVisBuckets: make(map[plane3DKey]plane3DVisBucket)}
}

func TestClassicPlaneRetainsSplitWithinSubsector(t *testing.T) {
	g := classicPlaneTestGame()
	order := g.beginPlane3DFrame(8)
	key := plane3DKey{height: 0, light: 160, flatID: 1, floor: true}
	first := g.findClassicPlane(key, &order)
	first = g.checkClassicPlane(first, 0, 3, &order)
	first.top[1], first.bottom[1] = 2, 3
	split := g.checkClassicPlane(first, 0, 3, &order)
	if split == first {
		t.Fatal("overlap must split the plane")
	}
	split.top[2], split.bottom[2] = 2, 3
	// The next wall in this subsector must extend the current split, even
	// though the first plane could also accommodate columns 4..7.
	if got := g.checkClassicPlane(split, 4, 7, &order); got != split {
		t.Fatal("lost subsector's current plane")
	}
	if split.minX != 0 || split.maxX != 7 {
		t.Fatalf("range %d..%d", split.minX, split.maxX)
	}
	// The next subsector starts from the first plane. An overlap makes a new
	// split, not a search for another existing plane with a free column zero.
	next := g.findClassicPlane(key, &order)
	if next != first {
		t.Fatal("R_FindPlane must select the first plane")
	}
	next = g.checkClassicPlane(next, 0, 0, &order)
	if next == first || next == split || len(order) != 3 {
		t.Fatal("reused an earlier split instead of allocating a new one")
	}
}

func TestClassicPlaneChecksOnlyClippedVisibleRange(t *testing.T) {
	g := classicPlaneTestGame()
	order := g.beginPlane3DFrame(8)
	key := plane3DKey{height: 0, light: 160, flatID: 1, floor: true}
	first := g.findClassicPlane(key, &order)
	first = g.checkClassicPlane(first, 0, 2, &order)
	first.top[1], first.bottom[1] = 2, 3
	// A wall's original range overlaps column zero, but solid clipping leaves
	// only 3..7. R_CheckPlane must see that visible fragment, not the full wall.
	if got := g.checkClassicPlane(first, 3, 7, &order); got != first || len(order) != 1 {
		t.Fatal("unnecessary split for an unobstructed fragment")
	}
	// floor/ceiling role is not part of the original lookup key. Retain the
	// first plane's role only as metadata for the existing debug overlay.
	key.floor = false
	if got := g.findClassicPlane(key, &order); got != first || !got.key.floor {
		t.Fatal("lookup identity or debug metadata changed")
	}
}

func TestClassicSkyFloorSharesSkyIdentity(t *testing.T) {
	g := classicPlaneTestGame()
	sec := &mapdata.Sector{FloorHeight: -24, CeilingHeight: 128, FloorPic: "F_SKY1", CeilingPic: "F_SKY1", Light: 128}
	floor := g.plane3DKeyForSector(sec, true)
	ceiling := g.plane3DKeyForSector(sec, false)
	if !floor.sky || floor.height != 0 || floor.light != 0 {
		t.Fatalf("sky floor treated as flat: %+v", floor)
	}
	order := g.beginPlane3DFrame(g.viewW)
	if g.findClassicPlane(floor, &order) != g.findClassicPlane(ceiling, &order) {
		t.Fatal("sky floor and ceiling must share a visplane")
	}
}

func TestClassicSubsectorFrontComesFromFirstSeg(t *testing.T) {
	// MAP21 has a subsector whose first SEG belongs to the 200-high sector,
	// while a later visible SEG's sidedef names the 256-high neighbor.
	g := &game{m: &mapdata.Map{
		Sectors:    []mapdata.Sector{{CeilingHeight: 200}, {CeilingHeight: 256}},
		Sidedefs:   []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
		Linedefs:   []mapdata.Linedef{{SideNum: [2]int32{0, -1}}, {SideNum: [2]int32{1, 0}}},
		Segs:       []mapdata.Seg{{Linedef: 0}, {Linedef: 1}},
		SubSectors: []mapdata.SubSector{{FirstSeg: 0, SegCount: 2}},
	}}
	front, back := g.wallRenderSectorIndices(1)
	if front != 0 || back != 0 {
		t.Fatalf("classic front/back=%d/%d, want 0/0", front, back)
	}
	g.opts.SourcePortMode = true
	front, back = g.wallRenderSectorIndices(1)
	if front != 1 || back != 0 {
		t.Fatalf("source port front/back=%d/%d, want 1/0", front, back)
	}
}

func TestClassicUnlitPreservesLightBoundaryPlanes(t *testing.T) {
	resetLightingMathState(t)
	disableDoomLighting()
	g := &game{}
	front := &mapdata.Sector{FloorPic: "FLAT", CeilingPic: "F_SKY1", Light: 128}
	back := *front
	back.Light = 160
	if !g.sectorsLightDifferForRender(-1, -1, front, &back) {
		t.Fatal("unlit discarded the classic light boundary")
	}
	state := g.classifyWallPortalCached(-1, front, &back, -1, 41, 0, 128, 0, 128)
	if !state.MarkFloor || !state.MarkCeiling {
		t.Fatalf("unlit changed visplane boundaries: %+v", state)
	}
	g.opts.SourcePortMode = true
	if g.sectorsLightDifferForRender(-1, -1, front, &back) {
		t.Fatal("Source Port lighting toggle changed")
	}
}
