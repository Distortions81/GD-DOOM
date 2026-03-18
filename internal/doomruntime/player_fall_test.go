package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestTryMove_PlayerStepsOffLedgeThenFalls(t *testing.T) {
	g := &game{
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
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: -64, CeilingHeight: 128},
			},
		},
		p: player{
			x:          -32 * fracUnit,
			y:          0,
			angle:      0,
			viewHeight: playerViewHeight,
		},
		soundQueue: make([]soundEvent, 0, 2),
	}
	g.initPhysics()

	if !g.tryMove(48*fracUnit, 0) {
		t.Fatal("tryMove should cross into lower adjacent sector")
	}
	if got := g.p.floorz; got != -64*fracUnit {
		t.Fatalf("floorz=%d want=%d after crossing ledge", got, -64*fracUnit)
	}
	if got := g.p.z; got != 0 {
		t.Fatalf("z=%d want=0 before gravity applies", got)
	}

	g.zMovement()
	if got := g.p.z; got != 0 {
		t.Fatalf("first airborne z=%d want=0", got)
	}
	if got := g.p.momz; got != -2*playerGravity {
		t.Fatalf("first airborne momz=%d want=%d", got, -2*playerGravity)
	}
	if len(g.soundQueue) != 0 {
		t.Fatalf("unexpected landing sound while stepping off ledge: %v", g.soundQueue)
	}

	g.zMovement()
	if got := g.p.z; got != -2*fracUnit {
		t.Fatalf("second airborne z=%d want=%d", got, -2*fracUnit)
	}
	if got := g.p.momz; got != -3*playerGravity {
		t.Fatalf("second airborne momz=%d want=%d", got, -3*playerGravity)
	}
}

func TestRunGameplayTic_NoAirControlWhileFalling(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:          0,
			y:          0,
			z:          8 * fracUnit,
			floorz:     0,
			ceilz:      128 * fracUnit,
			angle:      0,
			viewHeight: playerViewHeight,
		},
	}

	g.updatePlayer(moveCmd{forward: 0x32, side: 0x28})
	if g.p.x != 0 || g.p.y != 0 {
		t.Fatalf("input stage changed position to (%d,%d)", g.p.x, g.p.y)
	}
	if g.p.momx != 0 || g.p.momy != 0 {
		t.Fatalf("airborne thrust should not apply during input stage, got momx=%d momy=%d", g.p.momx, g.p.momy)
	}
	if g.p.momz != 0 {
		t.Fatalf("input stage changed momz to %d", g.p.momz)
	}

	g.runGameplayTic(moveCmd{}, false, false)

	if g.p.x != 0 || g.p.y != 0 {
		t.Fatalf("airborne movement changed position to (%d,%d)", g.p.x, g.p.y)
	}
	if g.p.momx != 0 || g.p.momy != 0 {
		t.Fatalf("airborne thrust should remain zero, got momx=%d momy=%d", g.p.momx, g.p.momy)
	}
	if got := g.p.momz; got != -2*playerGravity {
		t.Fatalf("momz=%d want=%d after one falling tick", got, -2*playerGravity)
	}
}

func TestTryMove_PlayerStepUpDefersZChangeToZMovement(t *testing.T) {
	g := &game{
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
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 24, CeilingHeight: 128},
			},
		},
		p: player{
			x:          -32 * fracUnit,
			y:          0,
			z:          0,
			floorz:     0,
			ceilz:      128 * fracUnit,
			viewHeight: playerViewHeight,
		},
	}
	g.initPhysics()

	if !g.tryMove(48*fracUnit, 0) {
		t.Fatal("tryMove should cross onto 24-unit step")
	}
	if got := g.p.floorz; got != 24*fracUnit {
		t.Fatalf("floorz=%d want=%d after crossing step", got, 24*fracUnit)
	}
	if got := g.p.z; got != 0 {
		t.Fatalf("z=%d want=0 before zMovement resolves step-up", got)
	}

	g.zMovement()

	if got := g.p.z; got != 24*fracUnit {
		t.Fatalf("z=%d want=%d after zMovement step-up", got, 24*fracUnit)
	}
	if g.p.viewHeight >= playerViewHeight {
		t.Fatalf("viewHeight=%d want less than %d after smooth step-up", g.p.viewHeight, playerViewHeight)
	}
}

func TestXYMovement_StepOffLedgePreservesMomentumWhileAirborne(t *testing.T) {
	g := &game{
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
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: -64, CeilingHeight: 128},
			},
		},
		p: player{
			x:          -32 * fracUnit,
			y:          0,
			z:          0,
			viewHeight: playerViewHeight,
		},
	}
	g.initPhysics()
	if !g.tryMove(48*fracUnit, 0) {
		t.Fatal("tryMove should cross into lower adjacent sector")
	}
	g.p.momx = 24 * fracUnit

	g.xyMovement()

	if got := g.p.floorz; got != -64*fracUnit {
		t.Fatalf("floorz=%d want=%d after ledge crossing", got, -64*fracUnit)
	}
	if got := g.p.z; got != 0 {
		t.Fatalf("z=%d want=0 immediately after ledge crossing", got)
	}
	if got := g.p.momx; got != 24*fracUnit {
		t.Fatalf("momx=%d want=%d with no airborne friction", got, 24*fracUnit)
	}
}

func TestXYMovement_KeepsLowMomentumWhileMoveInputHeld(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		p: player{
			x:          0,
			y:          0,
			z:          0,
			floorz:     0,
			ceilz:      128 * fracUnit,
			momx:       618,
			momy:       2067,
			viewHeight: playerViewHeight,
		},
		currentMoveCmd: moveCmd{forward: 1},
	}

	g.xyMovement()

	if g.p.momx == 0 || g.p.momy == 0 {
		t.Fatalf("momentum was zeroed with move input held: momx=%d momy=%d", g.p.momx, g.p.momy)
	}
}

func TestPlayerHardLanding_UsesOofAndViewSquatWithoutDamage(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 256},
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{256 * fracUnit},
		p: player{
			z:          8 * fracUnit,
			floorz:     0,
			ceilz:      256 * fracUnit,
			momz:       -9 * playerGravity,
			viewHeight: playerViewHeight,
		},
		stats:      playerStats{Health: 100},
		soundQueue: make([]soundEvent, 0, 2),
	}

	g.zMovement()

	if got := g.p.z; got != 0 {
		t.Fatalf("landed z=%d want=0", got)
	}
	if got := g.p.momz; got != 0 {
		t.Fatalf("landed momz=%d want=0", got)
	}
	if got := g.stats.Health; got != 100 {
		t.Fatalf("health=%d want=100; vanilla Doom has no fall damage", got)
	}
	if !hasSoundEvent(g.soundQueue, soundEventOof) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventOof)
	}
	if g.p.deltaViewHeight >= 0 {
		t.Fatalf("deltaViewHeight=%d want negative landing squat", g.p.deltaViewHeight)
	}

	g.tickPlayerViewHeight()
	if g.playerViewZ >= playerViewHeight {
		t.Fatalf("playerViewZ=%d want less than default eye height after hard landing", g.playerViewZ)
	}

	for i := 0; i < 32; i++ {
		g.tickPlayerViewHeight()
	}
	if got := g.p.viewHeight; got != playerViewHeight {
		t.Fatalf("viewHeight=%d want=%d after recovery", got, playerViewHeight)
	}
}
