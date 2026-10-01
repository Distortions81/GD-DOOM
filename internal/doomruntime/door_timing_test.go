package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestTaggedDoorSkipsSectorsWithOtherMovers(t *testing.T) {
	for _, mover := range []string{"floor", "platform", "stopped-platform", "ceiling", "stopped-ceiling"} {
		t.Run(mover, func(t *testing.T) {
			g := newDoorTimingGame(0)
			switch mover {
			case "floor":
				g.floors = map[int]*floorThinker{0: {direction: 1}}
			case "platform", "stopped-platform":
				status := platStatusWaiting
				if mover == "stopped-platform" {
					status = platStatusInStasis
				}
				g.plats = map[int]*platThinker{0: {status: status}}
			case "ceiling", "stopped-ceiling":
				direction := 1
				if mover == "stopped-ceiling" {
					direction = 0
				}
				g.ceilings = map[int]*ceilingThinker{0: {direction: direction}}
			}
			if g.activateDoorSectors([]int{0}, mapdata.DoorBlazeRaise) {
				t.Fatal("tagged door activated on a sector with another mover")
			}
			if !g.activateDoorSectors([]int{0, 1}, mapdata.DoorBlazeRaise) {
				t.Fatal("busy sector prevented activation of the idle tagged sector")
			}
			if g.doors[0] != nil || g.doors[1] == nil {
				t.Fatalf("doors=%v; want only the idle sector to have a door", g.doors)
			}
			oldCeil := g.sectorCeil[0]
			g.tickDoors()
			if g.sectorCeil[0] != oldCeil {
				t.Fatal("tagged door changed the busy sector's ceiling")
			}
		})
	}
}

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

func TestOpeningDoorPastDestinationRestoresBlockedSnapAndCompletes(t *testing.T) {
	for _, typ := range []doorType{doorOpen, doorNormal} {
		for _, blocked := range []bool{false, true} {
			g := &game{m: &mapdata.Map{
				Sectors: []mapdata.Sector{{CeilingHeight: 128}},
				Things:  []mapdata.Thing{{Type: 9, X: 100}},
			}, thingHP: []int{30}, thingCollected: []bool{false}, isDead: true}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p.x = 1000 * fracUnit
			dest := int64(80 * fracUnit)
			if blocked {
				dest = 32 * fracUnit
			}
			d := &doorThinker{sector: 0, typ: typ, direction: 1, speed: 2 * fracUnit,
				topHeight: dest, topWait: 150}
			g.doors = map[int]*doorThinker{0: d}
			g.tickDoor(0, d)
			want := dest
			if blocked {
				want = 128 * fracUnit
			}
			_, _, thingCeil := g.thingSupportState(0, g.m.Things[0])
			if g.sectorCeil[0] != want || thingCeil != want {
				t.Fatalf("blocked=%t: ceiling=%d actor ceiling=%d want=%d", blocked, g.sectorCeil[0], thingCeil, want)
			}
			if typ == doorNormal {
				if d.direction != 0 || d.topCountdown != 150 {
					t.Fatal("blocked pastdest must still enter the normal door wait")
				}
			} else if g.activeDoorThinker(0) != nil {
				t.Fatal("blocked pastdest must still remove an open-only door")
			}
		}
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
	if d.direction != -1 {
		t.Fatal("door stopped on the floor before stepping past its destination")
	}
	g.tickDoors()
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

func TestInitTimedDoorSpecialsSpawnsCloseInThirtySeconds(t *testing.T) {
	g := newDoorTimingGame(1)
	g.m.Sectors[1].Special = 10

	g.initTimedDoorSpecials()

	d := g.doors[1]
	if d == nil {
		t.Fatal("special 10 did not spawn a door thinker")
	}
	if d.typ != doorNormal || d.direction != 0 || d.speed != vDoorSpeed || d.topCountdown != 30*doomTicsPerSecond {
		t.Fatalf("door=%+v want normal inactive 30-second door", *d)
	}
	if got := g.m.Sectors[1].Special; got != 0 {
		t.Fatalf("sector special=%d want cleared", got)
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

func TestSetSectorCeilingHeight_LeavesNoBlockmapProjectileCachesUnchanged(t *testing.T) {
	g := newDoorTimingGame(1)
	g.sectorCeil[1] = 64 * fracUnit

	x := int64(32 * fracUnit)
	if got := g.sectorAt(x, 0); got != 1 {
		x = -32 * fracUnit
		if got := g.sectorAt(x, 0); got != 1 {
			t.Fatalf("failed to find sample point in sector 1, got sectors %d and %d", g.sectorAt(32*fracUnit, 0), g.sectorAt(-32*fracUnit, 0))
		}
	}

	g.projectiles = []projectile{{
		x:      x,
		y:      0,
		z:      32 * fracUnit,
		radius: 11 * fracUnit,
		height: 8 * fracUnit,
		floorz: 0,
		ceilz:  64 * fracUnit,
	}}
	g.projectileImpacts = []projectileImpact{{
		x:      x,
		y:      0,
		z:      32 * fracUnit,
		kind:   projectileRocket,
		floorz: 0,
		ceilz:  64 * fracUnit,
	}}

	g.setSectorCeilingHeight(1, 96*fracUnit)

	if got := g.projectiles[0].ceilz; got != 64*fracUnit {
		t.Fatalf("projectile ceilz=%d want cached value 64", got)
	}
	if got := g.projectileImpacts[0].ceilz; got != 64*fracUnit {
		t.Fatalf("impact ceilz=%d want cached value 64", got)
	}
}
