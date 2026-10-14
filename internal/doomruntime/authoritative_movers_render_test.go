package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestAuthorityMoverWallsAndPlanesShareFractionalHeight(t *testing.T) {
	g := &game{
		opts:        Options{SourcePortMode: true},
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{FloorHeight: 1, CeilingHeight: 66, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1"}}},
		sectorFloor: []int64{fracUnit},
		sectorCeil:  []int64{66 * fracUnit},
		doors:       map[int]*doorThinker{0: {direction: 1, speed: 2 * fracUnit, topHeight: 128 * fracUnit}},
		authorityRender: &authorityRenderState{
			from:  authorityRenderFrame{sectors: []authorityRenderSector{{floor: 0, ceil: 64 * fracUnit}}},
			to:    authorityRenderFrame{sectors: []authorityRenderSector{{floor: fracUnit, ceil: 66 * fracUnit}}},
			alpha: 0.25,
		},
	}
	// Local input alpha wraps every command tic. Both wall edges and visplanes
	// must stay on the snapshot cursor through each wrap.
	for _, alpha := range []float64{0, 0.5, 0.99, 0, 0.2} {
		g.renderAlpha = alpha
		floor, ceil, ok := g.sectorHeightRenderSnapshot(0)
		if !ok || floor != fracUnit/4 || ceil != 64*fracUnit+fracUnit/2 {
			t.Fatalf("local alpha %v moved authoritative wall: floor=%d ceil=%d", alpha, floor, ceil)
		}
		for _, isFloor := range []bool{true, false} {
			key := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], isFloor)
			want := float64(ceil) / fracUnit
			if isFloor {
				want = float64(floor) / fracUnit
			}
			if key.height != want {
				t.Fatalf("floor=%v plane height=%v does not meet wall=%v", isFloor, key.height, want)
			}
		}
	}
	if floor, ceil, _ := g.sectorHeightSnapshot(0); floor != fracUnit || ceil != 66*fracUnit {
		t.Fatal("rendering changed collision heights")
	}
}

func TestAuthorityMoverFirstBaselineHoldsConfirmedDoor(t *testing.T) {
	g := &game{
		opts:             Options{SourcePortMode: true},
		m:                &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 64}}},
		sectorFloor:      []int64{0},
		sectorCeil:       []int64{64 * fracUnit},
		doors:            map[int]*doorThinker{0: {direction: 1, speed: 2 * fracUnit, topHeight: 128 * fracUnit}},
		clientPrediction: &ClientPrediction{},
		renderAlpha:      0.5,
	}
	if _, ceil, _ := g.sectorHeightRenderSnapshot(0); ceil != 64*fracUnit {
		t.Fatalf("first baseline extrapolated door: %d", ceil)
	}
	// Preserve the existing single-player interpolation behavior.
	g.clientPrediction = nil
	if _, ceil, _ := g.sectorHeightRenderSnapshot(0); ceil != 65*fracUnit {
		t.Fatalf("single-player door interpolation changed: %d", ceil)
	}
}

func TestAuthorityMoverPortalSplitTracksEachRenderedFrame(t *testing.T) {
	sec := mapdata.Sector{CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160}
	g := &game{
		opts:     Options{SourcePortMode: true},
		worldTic: 12,
		m:        &mapdata.Map{Sectors: []mapdata.Sector{sec, sec}},
		wallSegStaticCache: []wallSegStatic{{
			valid: true, portalSplitStatic: true, portalSplit: false,
			lightTickValid: true, lightTick: 12, lightTickSplit: false,
		}},
		authorityRender: &authorityRenderState{
			from: authorityRenderFrame{sectors: []authorityRenderSector{
				{floor: 0, ceil: 128 * fracUnit}, {floor: fracUnit, ceil: 128 * fracUnit},
			}},
			to: authorityRenderFrame{sectors: []authorityRenderSector{
				{floor: 0, ceil: 128 * fracUnit}, {floor: 0, ceil: 128 * fracUnit},
			}},
			alpha: 0.75,
		},
	}
	if !g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("stale static/per-tic cache hid a quarter-unit rendered lift edge")
	}
	g.authorityRender.alpha = 1
	if g.segPortalSplitAtTick(0, true, 0, 1) {
		t.Fatal("portal split did not update when the displayed lift reached the floor within the same world tic")
	}
	if g.worldTic != 12 || g.m.Sectors[1].FloorHeight != 0 {
		t.Fatal("presentation changed world state")
	}
}
