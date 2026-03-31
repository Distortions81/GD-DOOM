package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestNextWorldThinkerAfter_SeesNewThinkersAddedMidWalk(t *testing.T) {
	g := &game{
		floors: map[int]*floorThinker{
			1: {order: 1, sector: 1},
		},
	}

	first, ok := g.nextWorldThinkerAfter(0)
	if !ok {
		t.Fatal("expected first thinker")
	}
	if first.kind != worldThinkerFloor || first.key != 1 || first.order != 1 {
		t.Fatalf("first=%+v", first)
	}

	g.doors = map[int]*doorThinker{
		2: {order: 2, sector: 2},
	}

	second, ok := g.nextWorldThinkerAfter(first.order)
	if !ok {
		t.Fatal("expected newly added thinker")
	}
	if second.kind != worldThinkerDoor || second.key != 2 || second.order != 2 {
		t.Fatalf("second=%+v", second)
	}
}

func TestCheckPositionForActor_BlockedByThingPreservesSectorPlanes(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{FloorHeight: -8, CeilingHeight: 96}},
			Things: []mapdata.Thing{
				{X: 0, Y: 0, Type: 2002},
				{X: 0, Y: 0, Type: 2035},
			},
		},
		sectorFloor:       []int64{-8 * fracUnit},
		sectorCeil:        []int64{96 * fracUnit},
		thingCollected:    []bool{false, false},
		thingDropped:      []bool{true, false},
		thingBlockCell:    []int{-1, -1},
		thingBlockOrder:   []int64{1, 2},
		thingBlockCells:   [][]int{{0, 1}},
		thingX:            []int64{0, 0},
		thingY:            []int64{0, 0},
		thingZState:       []int64{-8 * fracUnit, -8 * fracUnit},
		thingFloorState:   []int64{-8 * fracUnit, -8 * fracUnit},
		thingCeilState:    []int64{96 * fracUnit, 96 * fracUnit},
		thingSupportValid: []bool{true, true},
		bmapWidth:         1,
		bmapHeight:        1,
	}

	floor, ceil, drop, ok := g.checkPositionForActor(0, 0, 20*fracUnit, false, 0, false)
	if ok {
		t.Fatal("probe should be blocked by overlapping solid thing")
	}
	if floor != -8*fracUnit || ceil != 96*fracUnit || drop != -8*fracUnit {
		t.Fatalf("blocked probe planes=(%d,%d,%d) want (%d,%d,%d)", floor, ceil, drop, -8*fracUnit, 96*fracUnit, -8*fracUnit)
	}
}
