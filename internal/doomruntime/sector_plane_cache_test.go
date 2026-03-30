package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestSectorPlaneCache_RebuildsDynamicSectorOnHeightChange(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors:    []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
			SubSectors: []mapdata.SubSector{{FirstSeg: 0, SegCount: 4}},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},

		subSectorSec:     []int{0},
		subSectorPlaneID: []int{0},
		sectorSubSectors: [][]int{{0}},
		subSectorPoly: [][]worldPt{{
			{x: 0, y: 0},
			{x: 64, y: 0},
			{x: 64, y: 64},
			{x: 0, y: 64},
		}},
		subSectorTris: [][][3]int{{
			{0, 1, 2},
			{0, 2, 3},
		}},
		dynamicSectorMask: []bool{true},
	}

	g.buildSectorPlaneTriCache()
	g.initSectorPlaneLevelCache()
	if got := len(g.sectorPlaneCache); got != 1 {
		t.Fatalf("sectorPlaneCache len=%d want=1", got)
	}
	if got := len(g.sectorPlaneCache[0].tris); got != 2 {
		t.Fatalf("initial tri count=%d want=2", got)
	}
	oldX := g.sectorPlaneCache[0].tris[0].a.x

	// Simulate a geometry update and door height change in the same sector.
	g.subSectorPoly[0][0] = worldPt{x: 10, y: 0}
	g.sectorCeil[0] = 120 * fracUnit
	g.markDynamicSectorPlaneCacheDirty(0)
	g.refreshDynamicSectorPlaneCache()

	if got := g.sectorPlaneCache[0].tris[0].a.x; got == oldX {
		t.Fatalf("cached tris were not rebuilt after dynamic sector update: got x=%.1f", got)
	}
}

func TestSectorPlaneCacheLightingRefreshesPerTick(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{Light: 160}},
		},
		worldTic: 1,
	}

	g.initSectorPlaneLevelCache()
	if got := g.sectorLightLevelCached(0); got != 160 {
		t.Fatalf("initial cached light=%d want=160", got)
	}

	g.m.Sectors[0].Light = 96
	if got := g.sectorLightLevelCached(0); got != 160 {
		t.Fatalf("same-tick cached light=%d want=160", got)
	}

	g.worldTic = 2
	if got := g.sectorLightLevelCached(0); got != 96 {
		t.Fatalf("next-tick cached light=%d want=96", got)
	}
}

func TestSectorPlaneCache_StaticSectorIgnoresDirtyMark(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		sectorFloor:         []int64{0},
		sectorCeil:          []int64{128 * fracUnit},
		sectorPlaneTris:     [][]worldTri{{}},
		dynamicSectorMask:   []bool{false},
		sectorSubSectors:    [][]int{{}},
		subSectorPlaneID:    []int{},
		subSectorSec:        []int{},
		subSectorPoly:       [][]worldPt{},
		subSectorTris:       [][][3]int{},
		holeFillPolys:       nil,
		sectorPlaneCache:    nil,
		staticSubSectorMask: nil,
	}
	g.initSectorPlaneLevelCache()
	g.markDynamicSectorPlaneCacheDirty(0)
	if g.sectorPlaneCache[0].dirty {
		t.Fatal("static sector should not be marked dirty")
	}
}

func TestSectorPlaneCache_TracksLightingTypeAndBrightness(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{Light: 96}},
		},
		sectorLightFx: []sectorLightEffect{{kind: sectorLightEffectStrobe}},
		sectorPlaneTris: [][]worldTri{
			{},
		},
	}
	g.initSectorPlaneLevelCache()
	if got := g.sectorLightLevelCached(0); got != 96 {
		t.Fatalf("cached light=%d want=96", got)
	}
	if got := g.sectorLightKindCached(0); got != sectorLightEffectStrobe {
		t.Fatalf("cached light kind=%d want=%d", got, sectorLightEffectStrobe)
	}

	g.m.Sectors[0].Light = 144
	g.sectorLightFx[0].kind = sectorLightEffectGlow
	g.refreshSectorPlaneCacheLighting()

	if got := g.sectorLightLevelCached(0); got != 144 {
		t.Fatalf("updated cached light=%d want=144", got)
	}
	if got := g.sectorLightKindCached(0); got != sectorLightEffectGlow {
		t.Fatalf("updated cached light kind=%d want=%d", got, sectorLightEffectGlow)
	}
}

func TestSectorLightForRender_PrefersCachedLight(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{Light: 96}},
		},
		sectorPlaneCache: []sectorPlaneCacheEntry{
			{light: 144},
		},
	}

	if got := g.sectorLightForRender(0, &g.m.Sectors[0]); got != 144 {
		t.Fatalf("render light=%d want=144", got)
	}
}

func TestPlane3DKeyForSector_UsesCachedLight(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{
				FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 96,
			}},
		},
		sectorPlaneCache: []sectorPlaneCacheEntry{
			{light: 160},
		},
	}

	key := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], true)
	if key.light != 160 {
		t.Fatalf("plane key light=%d want=160", key.light)
	}
}

func TestSectorLightLevelCached_InterpolatesInSourcePortMode(t *testing.T) {
	g := &game{
		opts: Options{SourcePortMode: true},
		sectorPlaneCache: []sectorPlaneCacheEntry{{
			lightKind: sectorLightEffectStrobe,
			prevLight: 96,
			light:     160,
		}},
		renderAlpha: 0.5,
	}

	if got := g.sectorLightLevelCached(0); got != 128 {
		t.Fatalf("interpolated light=%d want=128", got)
	}
}

func TestSectorLightMulCached_InterpolatesInSourcePortMode(t *testing.T) {
	g := &game{
		opts: Options{SourcePortMode: true},
		sectorPlaneCache: []sectorPlaneCacheEntry{{
			lightKind:    sectorLightEffectStrobe,
			prevLightMul: 96,
			lightMul:     160,
		}},
		renderAlpha: 0.5,
	}

	if got := g.sectorLightMulCached(0); got != 128 {
		t.Fatalf("interpolated light mul=%d want=128", got)
	}
}

func TestSectorLightLevelCached_DoesNotInterpolateOutsideSourcePortMode(t *testing.T) {
	g := &game{
		opts: Options{SourcePortMode: false},
		sectorPlaneCache: []sectorPlaneCacheEntry{{
			lightKind: sectorLightEffectStrobe,
			prevLight: 96,
			light:     160,
		}},
		renderAlpha: 0.5,
	}

	if got := g.sectorLightLevelCached(0); got != 160 {
		t.Fatalf("classic mode light=%d want=160", got)
	}
}

func TestSectorLightLevelCached_DoesNotInterpolateWithoutAnimatedEffect(t *testing.T) {
	g := &game{
		opts: Options{SourcePortMode: true},
		sectorPlaneCache: []sectorPlaneCacheEntry{{
			lightKind: sectorLightEffectNone,
			prevLight: 96,
			light:     160,
		}},
		renderAlpha: 0.5,
	}

	if got := g.sectorLightLevelCached(0); got != 160 {
		t.Fatalf("static light=%d want=160", got)
	}
}

func TestSectorLightMulCached_DoesNotInterpolateWithoutAnimatedEffect(t *testing.T) {
	g := &game{
		opts: Options{SourcePortMode: true},
		sectorPlaneCache: []sectorPlaneCacheEntry{{
			lightKind:    sectorLightEffectNone,
			prevLightMul: 96,
			lightMul:     160,
		}},
		renderAlpha: 0.5,
	}

	if got := g.sectorLightMulCached(0); got != 160 {
		t.Fatalf("static light mul=%d want=160", got)
	}
}

func TestSectorsLightDifferForRender_UsesInterpolatedCachedLight(t *testing.T) {
	prevSectorLighting := doomSectorLighting
	doomSectorLighting = true
	t.Cleanup(func() { doomSectorLighting = prevSectorLighting })

	g := &game{
		opts: Options{SourcePortMode: true},
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{Light: 160},
				{Light: 96},
			},
		},
		sectorPlaneCache: []sectorPlaneCacheEntry{
			{
				lightKind: sectorLightEffectGlow,
				prevLight: 96,
				light:     160,
			},
			{
				lightKind: sectorLightEffectGlow,
				prevLight: 96,
				light:     160,
			},
		},
		renderAlpha: 0.5,
	}

	if got := g.sectorsLightDifferForRender(0, 1, &g.m.Sectors[0], &g.m.Sectors[1]); got {
		t.Fatal("expected identical interpolated render lights despite different raw sector lights")
	}
}
