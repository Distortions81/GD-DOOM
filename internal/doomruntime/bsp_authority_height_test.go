package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func authorityHeightVisibilityGame(direction uint16) *game {
	return &game{opts: Options{SourcePortMode: true}, m: &mapdata.Map{
		Sectors: []mapdata.Sector{
			{CeilingHeight: 128, FloorPic: "FLOOR", CeilingPic: "CEIL", Light: 128},
			{CeilingHeight: 0, FloorPic: "FLOOR", CeilingPic: "CEIL", Light: 128},
		},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
		Linedefs: []mapdata.Linedef{{SideNum: [2]int16{0, 1}}},
		Segs:     []mapdata.Seg{{Linedef: 0, Direction: direction}},
	}, sectorFloor: []int64{0, 0}, sectorCeil: []int64{128 * fracUnit, 0}}
}

func TestAuthorityBSPKeepsFractionalRenderedDoorOpeningVisible(t *testing.T) {
	for _, direction := range []uint16{0, 1} {
		g := authorityHeightVisibilityGame(direction)
		if !g.segCoarseOpaque(0) {
			t.Fatal("single-player closed door no longer occludes")
		}
		g.authorityRender = &authorityRenderState{
			from:  authorityRenderFrame{sectors: []authorityRenderSector{{0, 128 * fracUnit}, {0, 2 * fracUnit}}},
			to:    authorityRenderFrame{sectors: []authorityRenderSector{{0, 128 * fracUnit}, {0, 0}}},
			alpha: 0.75,
		}
		if g.segCoarseOpaque(0) {
			t.Fatal("canonical closed door culled a still-visible half-unit rendered opening")
		}
		g.authorityRender.alpha = 1
		if !g.segCoarseOpaque(0) {
			t.Fatal("rendered closed door did not occlude")
		}
		g.authorityRender = nil
		g.clientPrediction = &ClientPrediction{}
		g.sectorCeil[1] = fracUnit / 2
		if g.segCoarseOpaque(0) {
			t.Fatal("first client baseline truncated its canonical fractional opening")
		}
		g.clientPrediction = nil
		if !g.segCoarseOpaque(0) {
			t.Fatal("single-player coarse visibility changed to presentation heights")
		}
	}
}

func TestAuthorityBSPPortalSplitUsesRenderedFloorAndCeiling(t *testing.T) {
	g := authorityHeightVisibilityGame(0)
	g.m.Sectors[1].CeilingHeight, g.sectorCeil[1] = 128, 128*fracUnit
	if g.segPortalSplitPseudo3D(0) {
		t.Fatal("equal single-player sectors unexpectedly split")
	}
	for _, floor := range []bool{true, false} {
		from := []authorityRenderSector{{0, 128 * fracUnit}, {0, 128 * fracUnit}}
		to := []authorityRenderSector{{0, 128 * fracUnit}, {0, 128 * fracUnit}}
		if floor {
			from[1].floor = -fracUnit
		} else {
			from[1].ceil += fracUnit
		}
		g.authorityRender = &authorityRenderState{from: authorityRenderFrame{sectors: from}, to: authorityRenderFrame{sectors: to}, alpha: 0.75}
		if !g.segPortalSplitPseudo3D(0) {
			t.Fatal("canonical-equal sectors hid a quarter-unit rendered portal split")
		}
		g.authorityRender.alpha = 1
		if g.segPortalSplitPseudo3D(0) {
			t.Fatal("matching rendered endpoints retained an obsolete portal split")
		}
	}
	// Conversely, a newly arrived height change must not create a boundary
	// while both sides still share the buffered starting height.
	g.m.Sectors[1].FloorHeight, g.sectorFloor[1] = 4, 4*fracUnit
	g.authorityRender.to.sectors[1].floor = 4 * fracUnit
	g.authorityRender.from.sectors[1].ceil = 128 * fracUnit
	g.authorityRender.alpha = 0
	if g.segPortalSplitPseudo3D(0) {
		t.Fatal("latest collision floor created a premature rendered portal split")
	}
	g.authorityRender = nil
	if !g.segPortalSplitPseudo3D(0) {
		t.Fatal("single-player floor split no longer uses canonical map heights")
	}
}
