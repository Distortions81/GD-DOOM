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
