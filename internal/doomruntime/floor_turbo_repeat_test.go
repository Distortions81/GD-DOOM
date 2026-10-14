package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func turboWalkFloorGame(special uint16) *game {
	return &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, Flags: 7}},
			Linedefs: []mapdata.Linedef{
				{Special: special, Tag: 7, Flags: mlTwoSided, SideNum: [2]int16{0, 1}},
				{Flags: mlTwoSided, SideNum: [2]int16{0, 2}},
			},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}, {Sector: 2}},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 16, CeilingHeight: 128},
				{FloorHeight: 32, CeilingHeight: 128},
			},
		},
		lineSpecial: []uint16{special, 0},
		sectorFloor: []int64{0, 16 * fracUnit, 32 * fracUnit},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit, 128 * fracUnit},
		lines: []physLine{{
			idx: 0, x1: 0, y1: -64 * fracUnit, x2: 0, y2: 64 * fracUnit,
			dy: 128 * fracUnit, bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 0, 0},
			slope: slopeVertical, special: special, tag: 7, sideNum0: 0, sideNum1: 1,
		}},
		p: player{sector: 2, floorz: 32 * fracUnit, z: 32 * fracUnit, ceilz: 128 * fracUnit},
	}
}

func TestWalkTurboFloor129RaisesToNextHeightOnEachCrossing(t *testing.T) {
	g := turboWalkFloorGame(129)
	// P_CrossSpecialLine allows players, from either side, to trigger 129.
	// Each completed raise must leave it usable for the next higher neighbor.
	for step, target := range []int64{16 * fracUnit, 32 * fracUnit} {
		from, to := int64(-fracUnit), int64(fracUnit)
		if step == 1 {
			from, to = to, from
		}
		g.checkWalkSpecialLines(from, 0, to, 0)
		if g.floors[0] == nil {
			t.Fatalf("crossing %d did not raise the tagged floor", step+1)
		}
		before := g.sectorFloor[0]
		g.tickFloors()
		if got := g.sectorFloor[0]; got != before+4*fracUnit {
			t.Fatalf("crossing %d first movement=%d, want turbo speed 4 map units", step+1, got-before)
		}
		for range 8 {
			g.tickFloors()
		}
		if g.sectorFloor[0] != target || g.floors[0] != nil {
			t.Fatalf("crossing %d ended at %d, want %d and no active mover", step+1, g.sectorFloor[0], target)
		}
		if g.lineSpecial[0] != 129 {
			t.Fatal("repeat crossing consumed special 129")
		}
	}
}

func TestWalkTurboFloor129SurvivesBusySectorAndRejectsNonWalkActivators(t *testing.T) {
	g := turboWalkFloorGame(129)
	g.checkWalkSpecialLinesForActor(-fracUnit, 0, fracUnit, 0, 0, false)
	if len(g.floors) != 0 || g.lineSpecial[0] != 129 {
		t.Fatal("monster crossing activated or consumed the player-only trigger")
	}
	if g.useSpecialLineForActor(0, 0, true) || len(g.floors) != 0 {
		t.Fatal("use activated the walk-only trigger")
	}
	busy := &floorThinker{sector: 0}
	g.floors = map[int]*floorThinker{0: busy}
	g.checkWalkSpecialLines(-fracUnit, 0, fracUnit, 0)
	if g.floors[0] != busy || g.lineSpecial[0] != 129 {
		t.Fatal("busy crossing replaced the active mover or consumed the repeat trigger")
	}
	delete(g.floors, 0)
	g.checkWalkSpecialLines(fracUnit, 0, -fracUnit, 0)
	if g.floors[0] == nil || g.lineSpecial[0] != 129 {
		t.Fatal("repeat trigger did not work once the sector became free")
	}
}
