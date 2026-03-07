package automap

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
	if g.lineSpecial[0] != 97 {
		t.Fatalf("repeat teleporter special should not be consumed, got %d", g.lineSpecial[0])
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
