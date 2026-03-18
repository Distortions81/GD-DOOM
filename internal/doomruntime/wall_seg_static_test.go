package doomruntime

import (
	"testing"

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
