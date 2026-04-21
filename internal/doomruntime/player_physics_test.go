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

func TestTryMoveWithPickupProbe_NoClipCrossesBlockingLine(t *testing.T) {
	g := &game{
		noClip: true,
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{Flags: mlBlocking, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128},
			},
			SubSectors: []mapdata.SubSector{
				{SegCount: 1, FirstSeg: 0},
				{SegCount: 1, FirstSeg: 1},
			},
			Segs: []mapdata.Seg{
				{Linedef: 0, Direction: 0},
				{Linedef: 0, Direction: 1},
			},
			Nodes: []mapdata.Node{
				{DX: 0, DY: 128, ChildID: [2]uint16{0x8001, 0x8000}},
			},
		},
		subSectorSec: []int{0, 1},
		lines: []physLine{
			{
				idx:      0,
				x1:       0,
				y1:       -64 * fracUnit,
				x2:       0,
				y2:       64 * fracUnit,
				dx:       0,
				dy:       128 * fracUnit,
				bbox:     [4]int64{64 * fracUnit, -64 * fracUnit, 0, 0},
				slope:    slopeVertical,
				flags:    mlBlocking,
				sideNum0: 0,
				sideNum1: 1,
			},
		},
		lineSpecial: []uint16{0},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
		p: player{
			x:      -32 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
			sector: 0,
		},
	}

	if !g.tryMove(32*fracUnit, 0) {
		t.Fatal("noclip move should cross blocking line")
	}
	if got, want := g.p.x, int64(32*fracUnit); got != want {
		t.Fatalf("player x=%d want %d", got, want)
	}
	if got, want := g.playerSector(), 1; got != want {
		t.Fatalf("player sector=%d want %d", got, want)
	}
}

func TestTryMoveWithPickupProbe_BlockedMoveDoesNotCollectDroppedPickup(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2001, X: 32, Y: 0},
				{Type: 3002, X: 32, Y: 0, Flags: skillMediumBits},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false, false},
		thingDropped:   []bool{true, false},
		thingHP:        []int{0, 150},
		thingDead:      []bool{false, false},
		p: player{
			x:          -32 * fracUnit,
			y:          0,
			z:          0,
			floorz:     0,
			ceilz:      128 * fracUnit,
			viewHeight: playerViewHeight,
		},
	}
	g.initPlayerState()
	g.initPhysics()

	if g.tryMoveWithPickupProbe(16*fracUnit, 0, true) {
		t.Fatal("blocked move should fail")
	}
	if g.thingCollected[0] {
		t.Fatal("dropped pickup should remain when the speculative move is blocked")
	}
}
