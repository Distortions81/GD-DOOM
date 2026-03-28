package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestUseSpecialLine_ActivatesFloorSpecialAndConsumesOneShot(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 18, Tag: 7}},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorHeight: 0, CeilingHeight: 128},
			},
		},
		lineSpecial: []uint16{18},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
	}
	g.useSpecialLine(0, 0)
	if g.floors == nil || len(g.floors) != 1 {
		t.Fatalf("expected floor thinker, got %d", len(g.floors))
	}
	if g.lineSpecial[0] != 0 {
		t.Fatalf("one-shot floor special should be consumed, got %d", g.lineSpecial[0])
	}
}

func TestUseSpecialLine_ActivatesRepeatPlatformButton(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 62, Tag: 7}},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorHeight: 32, CeilingHeight: 128},
			},
		},
		lineSpecial: []uint16{62},
		sectorFloor: []int64{32 * fracUnit},
		sectorCeil:  []int64{128 * fracUnit},
	}
	g.useSpecialLine(0, 0)
	if g.plats == nil || len(g.plats) != 1 {
		t.Fatalf("expected plat thinker, got %d", len(g.plats))
	}
	if g.lineSpecial[0] != 62 {
		t.Fatalf("repeat platform special should not be consumed, got %d", g.lineSpecial[0])
	}
}

func TestActivatePlatLine_DownWaitUpStayStartsActiveWithZeroCount(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 88, Tag: 7}},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
			},
		},
		lineSpecial: []uint16{88},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
	}

	if !g.activatePlatLine(0, mapdata.PlatInfo{Action: mapdata.PlatDownWaitUpStay, UsesTag: true}) {
		t.Fatal("expected plat activation")
	}
	pt := g.plats[0]
	if pt == nil {
		t.Fatal("expected plat thinker")
	}
	if pt.status != platStatusDown {
		t.Fatalf("status=%v want %v", pt.status, platStatusDown)
	}
	if pt.oldStatus != platStatusInStasis {
		t.Fatalf("oldStatus=%v want %v", pt.oldStatus, platStatusInStasis)
	}
	if pt.count != 0 {
		t.Fatalf("count=%d want 0", pt.count)
	}
	if pt.wait != platWaitTics {
		t.Fatalf("wait=%d want %d", pt.wait, platWaitTics)
	}
}

func TestActivatePlatLine_DownWaitUpStayReusesStalePlatFieldsLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 88, Tag: 7}},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
			},
		},
		lineSpecial: []uint16{88},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		platFree: []*platThinker{{
			count:     platWaitTics,
			oldStatus: platStatusInStasis,
		}},
	}

	if !g.activatePlatLine(0, mapdata.PlatInfo{Action: mapdata.PlatDownWaitUpStay, UsesTag: true}) {
		t.Fatal("expected plat activation")
	}
	pt := g.plats[0]
	if pt == nil {
		t.Fatal("expected plat thinker")
	}
	if pt.status != platStatusDown {
		t.Fatalf("status=%v want %v", pt.status, platStatusDown)
	}
	if pt.count != platWaitTics {
		t.Fatalf("count=%d want stale %d from recycled plat", pt.count, platWaitTics)
	}
	if pt.oldStatus != platStatusInStasis {
		t.Fatalf("oldStatus=%v want stale %v from recycled plat", pt.oldStatus, platStatusInStasis)
	}
}

func TestActivatePlatLine_AfterPlatPhaseTicksImmediately(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{Special: 88, Tag: 7, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
				{FloorHeight: -64, CeilingHeight: 128},
			},
		},
		lineSpecial:       []uint16{88},
		sectorFloor:       []int64{0, -64 * fracUnit},
		sectorCeil:        []int64{128 * fracUnit, 128 * fracUnit},
		platTickedThisTic: true,
	}

	if !g.activatePlatLine(0, mapdata.PlatInfo{Action: mapdata.PlatDownWaitUpStay, UsesTag: true}) {
		t.Fatal("expected plat activation")
	}
	if got, want := g.sectorFloor[0], int64(-4*fracUnit); got != want {
		t.Fatalf("floor after same-tic plat activation=%d want %d", got, want)
	}
	pt := g.plats[0]
	if pt == nil {
		t.Fatal("expected plat thinker")
	}
	if pt.status != platStatusDown {
		t.Fatalf("status=%v want %v", pt.status, platStatusDown)
	}
	if pt.count != 0 {
		t.Fatalf("count=%d want 0", pt.count)
	}
}

func TestActivatePlatLine_BlazeDownWaitUpStayUsesBlazeTypeAndSpeed(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 120, Tag: 7}},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
			},
		},
		lineSpecial: []uint16{120},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
	}

	if !g.activatePlatLine(0, mapdata.PlatInfo{Action: mapdata.PlatBlazeDownWaitUpStay, UsesTag: true}) {
		t.Fatal("expected plat activation")
	}
	pt := g.plats[0]
	if pt == nil {
		t.Fatal("expected plat thinker")
	}
	if pt.typ != platTypeBlazeDownWaitUpStay {
		t.Fatalf("plat type=%v want %v", pt.typ, platTypeBlazeDownWaitUpStay)
	}
	if got, want := pt.speed, int64(8*platMoveSpeed); got != want {
		t.Fatalf("speed=%d want %d", got, want)
	}
}

func TestCheckWalkSpecialLines_PlayerTriggersBlazePlatWhenRadiusTouchesLineExtension(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{Special: 120, Tag: 2, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 96, CeilingHeight: 192, Tag: 2},
				{FloorHeight: 160, CeilingHeight: 192},
			},
		},
		lineSpecial: []uint16{120},
		sectorFloor: []int64{96 * fracUnit, 160 * fracUnit},
		sectorCeil:  []int64{192 * fracUnit, 192 * fracUnit},
		lines: []physLine{{
			idx:      0,
			x1:       0,
			y1:       64 * fracUnit,
			x2:       0,
			y2:       0,
			dx:       0,
			dy:       -64 * fracUnit,
			bbox:     [4]int64{64 * fracUnit, 0, 0, 0},
			slope:    slopeVertical,
			special:  120,
			tag:      2,
			sideNum0: 0,
			sideNum1: 1,
		}},
		p: player{
			x:      4 * fracUnit,
			y:      -9 * fracUnit,
			z:      160 * fracUnit,
			floorz: 160 * fracUnit,
			ceilz:  192 * fracUnit,
		},
	}

	g.checkWalkSpecialLines(4*fracUnit, -9*fracUnit, -9*fracUnit, -9*fracUnit)

	if got, want := len(g.plats), 1; got != want {
		t.Fatalf("plat count=%d want %d", got, want)
	}
	pt := g.plats[0]
	if pt == nil {
		t.Fatal("expected blaze plat thinker")
	}
	if pt.typ != platTypeBlazeDownWaitUpStay {
		t.Fatalf("plat type=%v want %v", pt.typ, platTypeBlazeDownWaitUpStay)
	}
	if got := g.lineSpecial[0]; got != 120 {
		t.Fatalf("repeat walk special consumed: got %d want 120", got)
	}
}

func TestSetSectorCeilingHeight_DoesNotHeightClipNeighborSectorThings(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 72},
				{FloorHeight: 0, CeilingHeight: 72},
			},
			Things: []mapdata.Thing{
				{X: 96, Y: 64, Type: 3004},
			},
		},
		sectorFloor:       []int64{0, 0},
		sectorCeil:        []int64{72 * fracUnit, 72 * fracUnit},
		sectorBBox:        []worldBBox{{minX: 0, minY: 0, maxX: 127, maxY: 127}, {minX: 160, minY: 0, maxX: 255, maxY: 127}},
		bmapOriginX:       0,
		bmapOriginY:       0,
		bmapWidth:         2,
		bmapHeight:        1,
		thingCollected:    []bool{false},
		thingX:            []int64{96 * fracUnit},
		thingY:            []int64{64 * fracUnit},
		thingZState:       []int64{0},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{68 * fracUnit},
		thingSupportValid: []bool{true},
		p: player{
			x:      96 * fracUnit,
			y:      64 * fracUnit,
			z:      0,
			floorz: 0,
			ceilz:  72 * fracUnit,
		},
	}
	g.thingBlockCells = make([][]int, g.bmapWidth*g.bmapHeight)
	g.rebuildThingBlockmap()

	g.setSectorCeilingHeight(1, 71*fracUnit)

	if got, want := g.thingCeilState[0], int64(68*fracUnit); got != want {
		t.Fatalf("neighbor thing ceilingz=%d want %d", got, want)
	}
}

func TestHeightClipThing_RemovesDroppedItemsThatNoLongerFitLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 8},
			},
			Things: []mapdata.Thing{
				{X: 32, Y: 32, Type: 2001},
			},
		},
		sectorFloor:       []int64{0},
		sectorCeil:        []int64{8 * fracUnit},
		thingCollected:    []bool{false},
		thingDropped:      []bool{true},
		thingX:            []int64{32 * fracUnit},
		thingY:            []int64{32 * fracUnit},
		thingZState:       []int64{0},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{8 * fracUnit},
		thingSupportValid: []bool{true},
		bmapWidth:         1,
		bmapHeight:        1,
		thingBlockCell:    []int{-1},
		thingBlockCells:   make([][]int, 1),
	}
	g.rebuildThingBlockmap()

	if !g.heightClipThing(0, g.m.Things[0]) {
		t.Fatal("heightClipThing returned false for dropped item")
	}
	if !g.thingCollected[0] {
		t.Fatal("dropped item should be removed when it no longer fits")
	}
}

func TestRunGameplayTic_UseIsEdgeTriggeredLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 1, SideNum: [2]int16{0, 1}}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		lineSpecial: []uint16{1},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
		lines: []physLine{{
			idx:      0,
			x1:       64 * fracUnit,
			y1:       -64 * fracUnit,
			x2:       64 * fracUnit,
			y2:       64 * fracUnit,
			dx:       0,
			dy:       128 * fracUnit,
			flags:    mlTwoSided,
			special:  1,
			sideNum0: 0,
			sideNum1: 1,
			bbox:     [4]int64{64 * fracUnit, -64 * fracUnit, 64 * fracUnit, 64 * fracUnit},
			slope:    slopeVertical,
		}},
		p: player{x: 128 * fracUnit, y: 0, angle: doomAng180, floorz: 0, ceilz: 128 * fracUnit, subsector: -1, sector: 0, viewHeight: playerViewHeight},
	}
	g.physForLine = []int{0}
	g.doors = map[int]*doorThinker{
		1: {
			sector:    1,
			typ:       doorNormal,
			direction: 1,
			speed:     vDoorSpeed,
			topWait:   vDoorWaitTic,
			topHeight: 124 * fracUnit,
		},
	}
	d := g.doors[1]

	g.runGameplayTic(moveCmd{}, true, false)
	if d.direction != -1 {
		t.Fatalf("first use on active manual door direction=%d want=-1", d.direction)
	}

	g.runGameplayTic(moveCmd{}, true, false)
	if d.direction != -1 {
		t.Fatalf("held use should not retrigger active manual door, direction=%d want=-1", d.direction)
	}

	g.runGameplayTic(moveCmd{}, false, false)
	g.runGameplayTic(moveCmd{}, true, false)
	if d.direction != 1 {
		t.Fatalf("use after release should retrigger active manual door, direction=%d want=1", d.direction)
	}
}

func TestUseSpecialLine_ClearsStalePlatTickLatchBeforeActivation(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{Special: 20, Tag: 7, SideNum: [2]int16{0, 1}},
				{SideNum: [2]int16{2, 3}},
				{SideNum: [2]int16{3, 4}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 3},
				{Sector: 4},
				{Sector: 0},
				{Sector: 1},
				{Sector: 2},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: -32, CeilingHeight: 128, Tag: 7},
				{FloorHeight: -32, CeilingHeight: 128, Tag: 7},
				{FloorHeight: 88, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "STARTAN3"},
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		lineSpecial:       []uint16{20, 0, 0},
		sectorFloor:       []int64{-32 * fracUnit, -32 * fracUnit, 88 * fracUnit, 0, 0},
		sectorCeil:        []int64{128 * fracUnit, 128 * fracUnit, 128 * fracUnit, 128 * fracUnit, 128 * fracUnit},
		platTickedThisTic: true,
	}

	g.platTickedThisTic = false
	g.useSpecialLine(0, 0)

	if got, want := g.sectorFloor[0], int64(-32*fracUnit); got != want {
		t.Fatalf("sector 0 floor=%d want %d before world tick", got, want)
	}
	pt := g.plats[1]
	if pt == nil {
		t.Fatal("expected sector 1 plat thinker")
	}
	if got, want := pt.high, int64(88*fracUnit); got != want {
		t.Fatalf("sector 1 plat high=%d want %d", got, want)
	}
	if got := g.lineSpecial[0]; got != 0 {
		t.Fatalf("one-shot use special should be consumed, got %d", got)
	}
}

func TestUseSpecialLine_ReusesDoorThinkerCountdownOnRespawn(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: -64},
				{X: 0, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Special: 1, Flags: mlTwoSided, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
	}
	g.initPhysics()
	g.doors[1] = &doorThinker{
		sector:       1,
		typ:          doorNormal,
		direction:    0,
		topHeight:    124 * fracUnit,
		topWait:      vDoorWaitTic,
		topCountdown: 16,
		speed:        vDoorSpeed,
	}

	d := g.doors[1]
	d.pendingRemove = true
	g.prunePendingDoors()
	if len(g.doors) != 0 {
		t.Fatalf("doors remaining=%d want=0 after prune", len(g.doors))
	}
	if !g.useSpecialLineForActor(0, 0, true) {
		t.Fatal("expected manual door reopen to spawn a new thinker")
	}
	d = g.doors[1]
	if d == nil {
		t.Fatal("expected respawned door thinker")
	}
	if d.direction != 1 {
		t.Fatalf("respawned door direction=%d want=1", d.direction)
	}
	if d.topCountdown != 16 {
		t.Fatalf("respawned door topcountdown=%d want=16", d.topCountdown)
	}
}

func TestActivatePlatRaiseToNearestAndChangeClearsSectorDamageImmediately(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 62, Tag: 7, SideNum: [2]int16{0, -1}}},
			Sidedefs: []mapdata.Sidedef{{Sector: 1}},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorHeight: 0, CeilingHeight: 128, Special: 7},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1"},
			},
		},
		lineSpecial: []uint16{62},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
	}

	if !g.activatePlatLine(0, mapdata.PlatInfo{Action: mapdata.PlatRaiseToNearestAndChange}) {
		t.Fatal("activatePlatLine returned false")
	}
	if got := g.m.Sectors[0].Special; got != 0 {
		t.Fatalf("sector special=%d want=0 immediately after activation", got)
	}
}

func TestTickPlats_RaiseToNearestAndChangeRemovesAfterOvershootTick(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		sectorFloor: []int64{31 * fracUnit},
		sectorCeil:  []int64{128 * fracUnit},
		plats: map[int]*platThinker{
			0: {
				sector: 0,
				typ:    platTypeRaiseToNearestAndChange,
				status: platStatusUp,
				speed:  fracUnit,
				high:   32 * fracUnit,
			},
		},
	}

	g.tickPlats()
	if _, ok := g.plats[0]; !ok {
		t.Fatal("plat removed at exact destination; want one more tic like Doom")
	}
	if got, want := g.sectorFloor[0], int64(32*fracUnit); got != want {
		t.Fatalf("floor after exact destination tic=%d want=%d", got, want)
	}

	g.tickPlats()
	if _, ok := g.plats[0]; ok {
		t.Fatal("plat not removed on overshoot tic")
	}
}

func TestTickPlats_DownWaitUpStayRemovesAfterOvershootTick(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		sectorFloor: []int64{-1 * fracUnit},
		sectorCeil:  []int64{128 * fracUnit},
		plats: map[int]*platThinker{
			0: {
				sector: 0,
				typ:    platTypeDownWaitUpStay,
				status: platStatusUp,
				speed:  fracUnit,
				high:   0,
				wait:   platWaitTics,
			},
		},
	}

	g.tickPlats()
	if _, ok := g.plats[0]; !ok {
		t.Fatal("plat removed at exact destination; want one more tic like Doom")
	}
	if got, want := g.sectorFloor[0], int64(0); got != want {
		t.Fatalf("floor after exact destination tic=%d want=%d", got, want)
	}

	g.tickPlats()
	if _, ok := g.plats[0]; ok {
		t.Fatal("downWaitUpStay plat not removed on overshoot tic")
	}
}

func TestTickFloors_RemoveOnOvershootTicNotExactArrival(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		floors: map[int]*floorThinker{
			0: {
				direction:  -1,
				speed:      8 * fracUnit,
				destHeight: -16 * fracUnit,
			},
		},
	}

	g.tickFloors()
	if got := g.sectorFloor[0]; got != -8*fracUnit {
		t.Fatalf("first tick floor=%d want=%d", got, -8*fracUnit)
	}
	if _, ok := g.floors[0]; !ok {
		t.Fatal("floor thinker removed too early after first step")
	}

	g.tickFloors()
	if got := g.sectorFloor[0]; got != -16*fracUnit {
		t.Fatalf("second tick floor=%d want=%d", got, -16*fracUnit)
	}
	if _, ok := g.floors[0]; !ok {
		t.Fatal("floor thinker removed on exact-arrival tic")
	}

	g.tickFloors()
	if got := g.sectorFloor[0]; got != -16*fracUnit {
		t.Fatalf("overshoot tick floor=%d want=%d", got, -16*fracUnit)
	}
	if _, ok := g.floors[0]; ok {
		t.Fatal("floor thinker not removed on overshoot tic")
	}
}

func TestCheckWalkSpecialLines_TriggersTeleport(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 128, Y: 64, Angle: 90, Type: 14},
			},
			Linedefs: []mapdata.Linedef{
				{Special: 97, Tag: 7},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
			},
		},
		lineSpecial: []uint16{97},
		lines: []physLine{
			{
				idx:   0,
				x1:    0,
				y1:    64,
				x2:    0,
				y2:    -64,
				dx:    0,
				dy:    -128,
				slope: slopeVertical,
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:      -32 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}
	g.checkWalkSpecialLines(-32*fracUnit, 0, 32*fracUnit, 0)
	if g.p.x != 128*fracUnit || g.p.y != 64*fracUnit {
		t.Fatalf("player teleported to (%d,%d), want (%d,%d)", g.p.x, g.p.y, 128*fracUnit, 64*fracUnit)
	}
	if g.p.angle != thingDegToWorldAngle(90) {
		t.Fatalf("player angle=%d want %d", g.p.angle, thingDegToWorldAngle(90))
	}
	if g.p.reactionTime != 18 {
		t.Fatalf("player reactionTime=%d want 18", g.p.reactionTime)
	}
	if got := len(g.hitscanPuffs); got != 2 {
		t.Fatalf("teleport fog count=%d want=2", got)
	}
	if g.hitscanPuffs[0].kind != hitscanFxTeleport || g.hitscanPuffs[1].kind != hitscanFxTeleport {
		t.Fatalf("teleport fog kinds=%d,%d want=%d,%d", g.hitscanPuffs[0].kind, g.hitscanPuffs[1].kind, hitscanFxTeleport, hitscanFxTeleport)
	}
	if got := len(g.soundQueue); got != 2 {
		t.Fatalf("teleport sound count=%d want=2", got)
	}
	if g.soundQueue[0] != soundEventTeleport || g.soundQueue[1] != soundEventTeleport {
		t.Fatalf("teleport sounds=%v want=%v", g.soundQueue, []soundEvent{soundEventTeleport, soundEventTeleport})
	}
	if g.lineSpecial[0] != 97 {
		t.Fatalf("repeat teleporter special should not be consumed, got %d", g.lineSpecial[0])
	}
}

func TestCheckWalkSpecialLines_DoesNotTriggerRemoteTeleport(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 1056, Y: 1936, Angle: 270, Type: 14},
			},
			Linedefs: []mapdata.Linedef{
				{Special: 97, Tag: 13},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 13},
			},
		},
		lineSpecial: []uint16{97},
		lines: []physLine{
			{
				idx:   0,
				x1:    960 * fracUnit,
				y1:    384 * fracUnit,
				x2:    960 * fracUnit,
				y2:    448 * fracUnit,
				dx:    0,
				dy:    64 * fracUnit,
				bbox:  [4]int64{448 * fracUnit, 384 * fracUnit, 960 * fracUnit, 960 * fracUnit},
				slope: slopeVertical,
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:      1550 * fracUnit,
			y:      325 * fracUnit,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}

	g.checkWalkSpecialLines(1550*fracUnit, 325*fracUnit, 1551*fracUnit, 326*fracUnit)

	if g.p.x != 1550*fracUnit || g.p.y != 325*fracUnit {
		t.Fatalf("player unexpectedly teleported to (%d,%d)", g.p.x, g.p.y)
	}
	if got := len(g.hitscanPuffs); got != 0 {
		t.Fatalf("teleport fog count=%d want=0", got)
	}
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("teleport sound count=%d want=0", got)
	}
}

func TestCheckWalkSpecialLines_DoesNotTriggerTeleportFromBackSide(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 128, Y: 64, Angle: 90, Type: 14},
			},
			Linedefs: []mapdata.Linedef{
				{Special: 97, Tag: 7},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
			},
		},
		lineSpecial: []uint16{97},
		lines: []physLine{
			{
				idx:   0,
				x1:    0,
				y1:    64,
				x2:    0,
				y2:    -64,
				dx:    0,
				dy:    -128,
				slope: slopeVertical,
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:      32 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}

	g.checkWalkSpecialLines(32*fracUnit, 0, -32*fracUnit, 0)

	if g.p.x != 32*fracUnit || g.p.y != 0 {
		t.Fatalf("player unexpectedly teleported to (%d,%d)", g.p.x, g.p.y)
	}
	if got := len(g.hitscanPuffs); got != 0 {
		t.Fatalf("teleport fog count=%d want=0", got)
	}
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("teleport sound count=%d want=0", got)
	}
}

func TestCheckWalkSpecialLinesForActor_NonPlayerTeleportDoesNotMovePlayer(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 32, Y: 0, Angle: 0, Type: 3004},
				{X: 128, Y: 64, Angle: 90, Type: 14},
			},
			Linedefs: []mapdata.Linedef{
				{Special: 97, Tag: 7},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
			},
		},
		lineSpecial: []uint16{97},
		lines: []physLine{
			{
				idx:   0,
				x1:    0,
				y1:    64,
				x2:    0,
				y2:    -64,
				dx:    0,
				dy:    -128,
				slope: slopeVertical,
			},
		},
		sectorFloor:       []int64{0},
		sectorCeil:        []int64{128 * fracUnit},
		thingX:            []int64{32 * fracUnit, 128 * fracUnit},
		thingY:            []int64{0, 64 * fracUnit},
		thingZState:       []int64{0, 0},
		thingFloorState:   []int64{0, 0},
		thingCeilState:    []int64{128 * fracUnit, 128 * fracUnit},
		thingSupportValid: []bool{true, true},
		thingAngleState:   []uint32{0, thingDegToWorldAngle(90)},
		thingMoveDir:      []monsterMoveDir{monsterDirEast, monsterDirNoDir},
		thingMoveCount:    []int{7, 0},
		p: player{
			x:      320 * fracUnit,
			y:      320 * fracUnit,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}

	g.checkWalkSpecialLinesForActor(-32*fracUnit, 0, 32*fracUnit, 0, 0, false)

	if g.p.x != 320*fracUnit || g.p.y != 320*fracUnit {
		t.Fatalf("player unexpectedly teleported to (%d,%d)", g.p.x, g.p.y)
	}
	if g.m.Things[0].X != 128 || g.m.Things[0].Y != 64 {
		t.Fatalf("monster teleported to (%d,%d), want (%d,%d)", g.m.Things[0].X, g.m.Things[0].Y, 128, 64)
	}
	if g.thingMoveDir[0] != monsterDirEast || g.thingMoveCount[0] != 7 {
		t.Fatalf("monster move state=%v/%d want east/7", g.thingMoveDir[0], g.thingMoveCount[0])
	}
	if g.p.reactionTime != 0 {
		t.Fatalf("player reactionTime=%d want 0", g.p.reactionTime)
	}
	if got := len(g.hitscanPuffs); got != 2 {
		t.Fatalf("teleport fog count=%d want=2", got)
	}
	if got, want := g.hitscanPuffs[0].z, g.sectorFloor[0]; got != want {
		t.Fatalf("source teleport fog z=%d want=%d", got, want)
	}
	if got := len(g.soundQueue); got != 2 {
		t.Fatalf("teleport sound count=%d want=2", got)
	}
}

func TestUpdatePlayer_DoesNotApplyThrustWhileTeleportFrozen(t *testing.T) {
	g := &game{
		p: player{
			z:            0,
			floorz:       0,
			angle:        0,
			reactionTime: 18,
		},
	}

	g.updatePlayer(moveCmd{forward: forwardMove[1], side: sideMove[1]})

	if g.p.momx != 0 || g.p.momy != 0 {
		t.Fatalf("player momentum=(%d,%d) want (0,0)", g.p.momx, g.p.momy)
	}
}

func TestUpdatePlayer_DoesNotTurnWhileTeleportFrozen(t *testing.T) {
	g := &game{
		p: player{
			angle:        0xE0000000,
			reactionTime: 1,
		},
	}

	g.updatePlayer(moveCmd{turnRaw: -(1 << 24)})

	if got, want := g.p.angle, uint32(0xE0000000); got != want {
		t.Fatalf("player angle=%#x want %#x", got, want)
	}
}

func TestTickPlayerCounters_DecrementsReactionTime(t *testing.T) {
	g := &game{
		p: player{reactionTime: 2},
	}

	g.tickPlayerCounters()
	if g.p.reactionTime != 1 {
		t.Fatalf("reactionTime after first tick=%d want 1", g.p.reactionTime)
	}
	g.tickPlayerCounters()
	if g.p.reactionTime != 0 {
		t.Fatalf("reactionTime after second tick=%d want 0", g.p.reactionTime)
	}
	g.tickPlayerCounters()
	if g.p.reactionTime != 0 {
		t.Fatalf("reactionTime after third tick=%d want 0", g.p.reactionTime)
	}
}

func TestCheckWalkSpecialLines_DoesNotTriggerTeleportOutsidePlayerSubsectors(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 128, Y: 64, Angle: 90, Type: 14},
			},
			Linedefs: []mapdata.Linedef{
				{Special: 97, Tag: 7},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7},
			},
			SubSectors: []mapdata.SubSector{
				{SegCount: 0, FirstSeg: 0},
				{SegCount: 0, FirstSeg: 0},
			},
			Nodes: []mapdata.Node{
				{ChildID: [2]uint16{0x8000, 0x8001}},
			},
		},
		lineSpecial: []uint16{97},
		lines: []physLine{
			{
				idx:   0,
				x1:    0,
				y1:    64,
				x2:    0,
				y2:    -64,
				dx:    0,
				dy:    -128,
				bbox:  [4]int64{64, -64, 0, 0},
				slope: slopeVertical,
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:      -32 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}

	g.checkWalkSpecialLines(-32*fracUnit, 0, 32*fracUnit, 0)

	if g.p.x != -32*fracUnit || g.p.y != 0 {
		t.Fatalf("player unexpectedly teleported to (%d,%d)", g.p.x, g.p.y)
	}
	if got := len(g.hitscanPuffs); got != 0 {
		t.Fatalf("teleport fog count=%d want=0", got)
	}
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("teleport sound count=%d want=0", got)
	}
}

func TestUseSpecialLine_WalkOnlySpecialDoesNotReportUnsupported(t *testing.T) {
	g := &game{
		lineSpecial: []uint16{97},
	}
	g.useSpecialLine(0, 0)
	if g.useText != "USE: no change" {
		t.Fatalf("useText=%q want %q", g.useText, "USE: no change")
	}
}

func TestUseSpecialLine_ReportsUnsupportedSpecialNumber(t *testing.T) {
	g := &game{
		lineSpecial: []uint16{242},
		soundQueue:  make([]soundEvent, 0, 1),
	}
	g.useSpecialLine(0, 0)
	if g.useText != "USE: unsupported special 242" {
		t.Fatalf("useText=%q want %q", g.useText, "USE: unsupported special 242")
	}
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("soundQueue=%v want empty", g.soundQueue)
	}
}

func TestUseLines_NoLineDoesNotPlayNoWaySound(t *testing.T) {
	g := &game{
		soundQueue: make([]soundEvent, 0, 1),
	}
	g.useLines()
	if g.useText != "USE: no line" {
		t.Fatalf("useText=%q want %q", g.useText, "USE: no line")
	}
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("soundQueue=%v want empty", g.soundQueue)
	}
}

func TestUseSpecialLine_ActivatesCeilingSpecial(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 41, Tag: 7}},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorHeight: 0, CeilingHeight: 128},
			},
		},
		lineSpecial: []uint16{41},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
	}
	g.useSpecialLine(0, 0)
	if g.ceilings == nil || len(g.ceilings) != 1 {
		t.Fatalf("expected ceiling thinker, got %d", len(g.ceilings))
	}
	if g.lineSpecial[0] != 0 {
		t.Fatalf("one-shot ceiling special should be consumed, got %d", g.lineSpecial[0])
	}
}

func TestUseSpecialLine_ButtonLightTurnOffUsesMinNeighbor(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 139, Tag: 7, SideNum: [2]int16{0, 1}}, {SideNum: [2]int16{2, 3}}},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0}, {Sector: 1},
				{Sector: 0}, {Sector: 2},
			},
			Sectors: []mapdata.Sector{
				{Tag: 7, Light: 160},
				{Light: 64},
				{Light: 96},
			},
		},
		lineSpecial:   []uint16{139, 0},
		sectorLightFx: make([]sectorLightEffect, 3),
	}
	g.useSpecialLine(0, 0)
	if got := g.m.Sectors[0].Light; got != 64 {
		t.Fatalf("light=%d want=64", got)
	}
	if g.lineSpecial[0] != 139 {
		t.Fatalf("repeat light special should not be consumed, got %d", g.lineSpecial[0])
	}
}

func TestActivateLightLine_StartStrobingSkipsActiveLightThinker(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 17, Tag: 7}},
			Sectors: []mapdata.Sector{
				{Tag: 7, Light: 160},
			},
		},
		lineSpecial:   []uint16{17},
		sectorLightFx: []sectorLightEffect{{kind: sectorLightEffectGlow, minLight: 64, maxLight: 160, direction: -1}},
	}
	g.useSpecialLine(0, 0)
	if got := g.sectorLightFx[0].kind; got != sectorLightEffectGlow {
		t.Fatalf("light thinker kind=%d want=%d", got, sectorLightEffectGlow)
	}
}

func TestActivateStairsLine_DoesNotLoopBetweenAdjacentSectors(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{Special: 8, Tag: 7, SideNum: [2]int16{0, -1}},
				{SideNum: [2]int16{1, 2}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorPic: "STEP1"},
				{FloorPic: "STEP1"},
			},
		},
		lineSpecial: []uint16{8, 0},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
	}

	if !g.activateStairsLine(0, mapdata.StairsInfo{Action: mapdata.StairsBuild8, UsesTag: true}) {
		t.Fatal("expected stair special to activate")
	}
	if len(g.floors) != 2 {
		t.Fatalf("stairs created %d floor thinkers want 2", len(g.floors))
	}
	if got := g.floors[0].destHeight; got != 8*fracUnit {
		t.Fatalf("sector 0 dest=%d want=%d", got, 8*fracUnit)
	}
	if got := g.floors[1].destHeight; got != 16*fracUnit {
		t.Fatalf("sector 1 dest=%d want=%d", got, 16*fracUnit)
	}
}

func TestActivateStairsLine_OnlyTraversesFrontToBack(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{Special: 8, Tag: 7, SideNum: [2]int16{0, -1}},
				{SideNum: [2]int16{1, 2}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
				{Sector: 0},
			},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorPic: "STEP1"},
				{FloorPic: "STEP1"},
			},
		},
		lineSpecial: []uint16{8, 0},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
	}

	if !g.activateStairsLine(0, mapdata.StairsInfo{Action: mapdata.StairsBuild8, UsesTag: true}) {
		t.Fatal("expected stair special to activate")
	}
	if len(g.floors) != 1 {
		t.Fatalf("stairs created %d floor thinkers want 1", len(g.floors))
	}
	if _, ok := g.floors[1]; ok {
		t.Fatal("back-to-front neighbor should not be added to stair chain")
	}
}

func TestSetSectorFloorHeight_PlayerOnFloorMovesWithLoweringFloor(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:      0,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}

	g.setSectorFloorHeight(0, -16*fracUnit)

	if got := g.p.floorz; got != -16*fracUnit {
		t.Fatalf("floorz=%d want=%d", got, -16*fracUnit)
	}
	if got := g.p.z; got != -16*fracUnit {
		t.Fatalf("z=%d want=%d", got, -16*fracUnit)
	}
}

func TestSetSectorCeilingHeight_AirbornePlayerClipsToNewCeiling(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:      0,
			y:      0,
			z:      80 * fracUnit,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}

	g.setSectorCeilingHeight(0, 96*fracUnit)

	if got := g.p.ceilz; got != 96*fracUnit {
		t.Fatalf("ceilz=%d want=%d", got, 96*fracUnit)
	}
	if got := g.p.z; got != 40*fracUnit {
		t.Fatalf("z=%d want=%d after ceiling clip", got, 40*fracUnit)
	}
}
