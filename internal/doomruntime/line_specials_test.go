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
	if g.thingMoveDir[0] != monsterDirNoDir || g.thingMoveCount[0] != 0 {
		t.Fatalf("monster move state=%v/%d want nodir/0", g.thingMoveDir[0], g.thingMoveCount[0])
	}
	if g.p.reactionTime != 0 {
		t.Fatalf("player reactionTime=%d want 0", g.p.reactionTime)
	}
	if got := len(g.hitscanPuffs); got != 2 {
		t.Fatalf("teleport fog count=%d want=2", got)
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
