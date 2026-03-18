package doomruntime

import (
	"testing"

	"gddoom/internal/render/scene"
	"gddoom/internal/mapdata"
)

func TestCachedSegPortalSplit(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 72, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
			},
		},
		dynamicSectorMask: []bool{false, false, true, false},
		sectorPlaneCache: []sectorPlaneCacheEntry{
			{lightKind: sectorLightEffectNone},
			{lightKind: sectorLightEffectNone},
			{lightKind: sectorLightEffectNone},
			{lightKind: sectorLightEffectGlow},
		},
	}

	if ok, split := g.cachedSegPortalSplit(0, 1); !ok || split {
		t.Fatalf("equal static sectors = (%v, %v), want (true, false)", ok, split)
	}
	if ok, split := g.cachedSegPortalSplit(0, 2); ok || split {
		t.Fatalf("dynamic sector should not be cached, got (%v, %v)", ok, split)
	}
	if ok, split := g.cachedSegPortalSplit(0, 3); ok || split {
		t.Fatalf("animated light sector should not be cached, got (%v, %v)", ok, split)
	}

	g.sectorPlaneCache[3].lightKind = sectorLightEffectNone
	if ok, split := g.cachedSegPortalSplit(0, 3); !ok || !split {
		t.Fatalf("height mismatch should cache split=true, got (%v, %v)", ok, split)
	}
}

func TestSegPortalSplitAtTick_RecomputesAnimatedLightOncePerTick(t *testing.T) {
	oldSectorLighting := doomSectorLighting
	doomSectorLighting = true
	t.Cleanup(func() {
		doomSectorLighting = oldSectorLighting
	})

	g := &game{
		worldTic: 7,
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 128},
			},
		},
		wallSegStaticCache: []wallSegStatic{{valid: true}},
	}

	if !g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("expected initial tick with differing light to split")
	}
	g.m.Sectors[1].Light = 160
	if !g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("expected same-tick result to stay cached across frames")
	}

	g.worldTic = 8
	if g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("expected new tick to recompute light-sensitive split")
	}
}

func TestSegPortalSplitAtTick_IgnoresStaticCacheAfterLightEffectStarts(t *testing.T) {
	oldSectorLighting := doomSectorLighting
	doomSectorLighting = true
	t.Cleanup(func() {
		doomSectorLighting = oldSectorLighting
	})

	g := &game{
		worldTic: 1,
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
			},
		},
		sectorPlaneCache: []sectorPlaneCacheEntry{
			{lightKind: sectorLightEffectNone},
			{lightKind: sectorLightEffectNone},
		},
		wallSegStaticCache: []wallSegStatic{{
			valid:             true,
			portalSplitStatic: true,
			portalSplit:       false,
		}},
	}

	if g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("expected equal-light sectors to start unsplit")
	}

	g.sectorPlaneCache[1].lightKind = sectorLightEffectStrobe
	g.m.Sectors[1].Light = 96
	g.worldTic = 2

	if !g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("expected active light effect to bypass stale static portal split cache")
	}

	g.m.Sectors[1].Light = 160
	if !g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("expected same-tic animated-light result to stay cached across frames")
	}

	g.worldTic = 3
	if g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("expected next tic to recompute animated-light portal split")
	}
}

func TestBuildWallSegPrepassSingle_DoesNotEarlyOutForAnimatedLight(t *testing.T) {
	oldSectorLighting := doomSectorLighting
	doomSectorLighting = true
	t.Cleanup(func() {
		doomSectorLighting = oldSectorLighting
	})

	ld := mapdata.Linedef{SideNum: [2]int16{0, 1}}
	g := &game{
		viewW: 320,
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 96},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Linedefs: []mapdata.Linedef{ld},
		},
		sectorPlaneCache: []sectorPlaneCacheEntry{
			{lightKind: sectorLightEffectNone},
			{lightKind: sectorLightEffectStrobe},
		},
		wallSegStaticCache: []wallSegStatic{{
			valid:             true,
			ld:                ld,
			input:             scene.NewWallPrepassWorldInput(64, -32, 64, 32, 0, 64, 0),
			frontSectorIdx:    0,
			backSectorIdx:     1,
			portalSplitStatic: true,
			portalSplit:       false,
		}},
	}

	_ = g.buildWallSegPrepassSingle(0, 0, 0, 1, 0, 160, 0.01)
	if !g.wallSegStaticCache[0].lightTickValid {
		t.Fatal("expected animated light to reach per-tic portal split evaluation")
	}
	if !g.wallSegStaticCache[0].lightTickSplit {
		t.Fatal("expected per-tic portal split evaluation to detect differing animated light")
	}
}
