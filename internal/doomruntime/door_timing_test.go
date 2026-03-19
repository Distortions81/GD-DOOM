package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func newDoorTimingGame(doorSec int) *game {
	return &game{
		m: &mapdata.Map{
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: -64},
				{X: 0, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Flags: mlTwoSided, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 1},
				{Sector: 0},
			},
			Segs: []mapdata.Seg{
				{StartVertex: 0, EndVertex: 1, Linedef: 0, Direction: 0},
				{StartVertex: 0, EndVertex: 1, Linedef: 0, Direction: 1},
			},
			SubSectors: []mapdata.SubSector{
				{SegCount: 1, FirstSeg: 0},
				{SegCount: 1, FirstSeg: 1},
			},
			Nodes: []mapdata.Node{
				{X: 0, Y: -64, DX: 0, DY: 128, ChildID: [2]uint16{0x8000, 0x8001}},
			},
			Sectors: []mapdata.Sector{{}, {}},
		},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
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
				flags:    mlTwoSided,
				sideNum0: 0,
				sideNum1: 1,
			},
		},
		physForLine: []int{0},
		lineValid:   make([]int, 1),
		doors:       map[int]*doorThinker{},
		p: player{
			x:      -32 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}
}

func TestTickDoors_NormalDoorOpensWaitsThenCloses(t *testing.T) {
	g := newDoorTimingGame(1)
	g.sectorCeil[1] = 64 * fracUnit
	g.doors[1] = &doorThinker{
		sector:    1,
		typ:       doorNormal,
		direction: 1,
		topHeight: 72 * fracUnit,
		topWait:   3,
		speed:     2 * fracUnit,
	}

	g.tickDoors()
	if got := g.sectorCeil[1]; got != 66*fracUnit {
		t.Fatalf("after tick1 ceil=%d want=%d", got, 66*fracUnit)
	}
	g.tickDoors()
	if got := g.sectorCeil[1]; got != 68*fracUnit {
		t.Fatalf("after tick2 ceil=%d want=%d", got, 68*fracUnit)
	}
	g.tickDoors()
	if got := g.sectorCeil[1]; got != 70*fracUnit {
		t.Fatalf("after tick3 ceil=%d want=%d", got, 70*fracUnit)
	}
	g.tickDoors()
	d := g.doors[1]
	if got := g.sectorCeil[1]; got != 72*fracUnit {
		t.Fatalf("after tick4 ceil=%d want=%d", got, 72*fracUnit)
	}
	if d == nil || d.direction != 1 || d.topCountdown != 0 {
		t.Fatalf("at exact top direction/countdown=%v/%v want 1/0", d.direction, d.topCountdown)
	}
	g.tickDoors()
	if d.direction != 0 || d.topCountdown != 3 {
		t.Fatalf("after overshoot tick direction/countdown=%d/%d want 0/3", d.direction, d.topCountdown)
	}
	g.tickDoors()
	g.tickDoors()
	g.tickDoors()
	if d.direction != -1 {
		t.Fatalf("after wait direction=%d want=-1", d.direction)
	}
	g.tickDoors()
	if got := g.sectorCeil[1]; got != 70*fracUnit {
		t.Fatalf("after first close tick ceil=%d want=%d", got, 70*fracUnit)
	}
}

func TestTickDoors_Close30ThenOpenWaitsThirtySecondsAtBottom(t *testing.T) {
	g := newDoorTimingGame(1)
	g.doors[1] = &doorThinker{
		sector:    1,
		typ:       doorClose30ThenOpen,
		direction: -1,
		topHeight: 128 * fracUnit,
		topWait:   vDoorWaitTic,
		speed:     2 * fracUnit,
	}

	g.tickDoors()
	if got := g.sectorCeil[1]; got != 126*fracUnit {
		t.Fatalf("after tick1 ceil=%d want=%d", got, 126*fracUnit)
	}
	d := g.doors[1]
	for i := 0; i < 63; i++ {
		g.tickDoors()
	}
	if got := g.sectorCeil[1]; got != 0 {
		t.Fatalf("at bottom ceil=%d want=0", got)
	}
	if d.direction != 0 || d.topCountdown != 35*30 {
		t.Fatalf("bottom wait direction/countdown=%d/%d want 0/%d", d.direction, d.topCountdown, 35*30)
	}
	for i := 0; i < 35*30-1; i++ {
		g.tickDoors()
	}
	if d.direction != 0 || d.topCountdown != 1 {
		t.Fatalf("before reopen direction/countdown=%d/%d want 0/1", d.direction, d.topCountdown)
	}
	g.tickDoors()
	if d.direction != 1 {
		t.Fatalf("reopen direction=%d want=1", d.direction)
	}
}

func TestTickDoors_BlazeRaiseUsesFourTimesSpeed(t *testing.T) {
	g := newDoorTimingGame(1)
	g.sectorCeil[1] = 0
	g.doors[1] = &doorThinker{
		sector:    1,
		typ:       doorBlazeRaise,
		direction: 1,
		topHeight: 16 * fracUnit,
		topWait:   vDoorWaitTic,
		speed:     vDoorSpeed * 4,
	}

	g.tickDoors()
	if got := g.sectorCeil[1]; got != 8*fracUnit {
		t.Fatalf("after tick1 blaze ceil=%d want=%d", got, 8*fracUnit)
	}
	g.tickDoors()
	d := g.doors[1]
	if got := g.sectorCeil[1]; got != 16*fracUnit {
		t.Fatalf("after tick2 blaze ceil=%d want=%d", got, 16*fracUnit)
	}
	if d == nil || d.direction != 1 || d.topCountdown != 0 {
		t.Fatalf("blaze exact top direction/countdown=%d/%d want 1/0", d.direction, d.topCountdown)
	}
	g.tickDoors()
	if d.direction != 0 || d.topCountdown != vDoorWaitTic {
		t.Fatalf("blaze wait direction/countdown=%d/%d want 0/%d", d.direction, d.topCountdown, vDoorWaitTic)
	}
}

func TestTickDoors_NormalDoorReopensWhenPlayerOverlapsDoorwayFromAdjacentSector(t *testing.T) {
	g := newDoorTimingGame(1)
	g.p.x = -8 * fracUnit
	g.p.y = 0
	g.p.z = 0
	g.p.floorz = 0
	g.p.ceilz = 128 * fracUnit
	g.sectorCeil[1] = playerHeight
	g.doors[1] = &doorThinker{
		sector:    1,
		typ:       doorNormal,
		direction: -1,
		topHeight: 128 * fracUnit,
		topWait:   vDoorWaitTic,
		speed:     2 * fracUnit,
	}

	g.tickDoors()

	d := g.doors[1]
	if d == nil {
		t.Fatal("expected active door thinker")
	}
	if got := g.sectorCeil[1]; got != playerHeight {
		t.Fatalf("door ceiling moved into overlapping player: got %d want %d", got, playerHeight)
	}
	if d.direction != 1 {
		t.Fatalf("blocking normal door should reverse open, direction=%d", d.direction)
	}
}
