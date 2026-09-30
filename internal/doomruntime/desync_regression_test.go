package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestMissileSpawnLinksSubsectorOnlyAfterSuccessfulMove(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		g := &game{
			m: &mapdata.Map{
				Sectors:    []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 128}},
				SubSectors: []mapdata.SubSector{{}, {}},
				Nodes:      []mapdata.Node{{DY: 128, ChildID: [2]uint16{0x8001, 0x8000}}},
			},
			subSectorSec: []int{0, 1}, sectorFloor: []int64{0, 0},
			sectorCeil: []int64{128 * fracUnit, 128 * fracUnit},
			p:          player{x: 1000 * fracUnit}, stats: playerStats{Health: 100},
		}
		if blocked {
			g.lines = []physLine{{idx: 0, x1: 0, y1: -64 * fracUnit, x2: 0, y2: 64 * fracUnit,
				dy: 128 * fracUnit, slope: slopeVertical, sideNum1: -1,
				bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 0, 0}}}
		}
		p := projectile{x: -fracUnit, z: 32 * fracUnit, vx: 4 * fracUnit,
			radius: 2 * fracUnit, height: 8 * fracUnit, floorz: 0, ceilz: 128 * fracUnit,
			kind: projectilePlayerPlasma, sourcePlayer: true, sourceThing: -1}
		if got := g.finishProjectileSpawn(&p, true); got == blocked {
			t.Fatalf("blocked=%t spawn success=%t", blocked, got)
		}
		if blocked {
			fx := g.projectileImpacts[0]
			if fx.x != fracUnit || fx.subsector != 1 {
				t.Fatalf("failed spawn position/link=(%d,%d), want=(%d,1)", fx.x, fx.subsector, fracUnit)
			}
			if trace := g.demoTraceMobjs(); trace[len(trace)-1].Sector != 0 {
				t.Fatal("failed spawn trace recomputed its sector from the advanced coordinates")
			}
			file := saveFile{Version: saveGameVersion, Game: gameSaveState{ProjectileImpacts: captureProjectileImpacts(g.projectileImpacts)}}
			data, err := encodeSnapshot(saveGameMagic, file)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeSnapshot(data, saveGameMagic)
			if err != nil {
				t.Fatal(err)
			}
			if restored := restoreProjectileImpacts(decoded.Game.ProjectileImpacts)[0]; restored.subsector != fx.subsector {
				t.Fatal("snapshot lost the failed-spawn subsector link")
			}
		} else {
			if p.x != fracUnit || p.subsector != 2 {
				t.Fatalf("successful spawn position/link=(%d,%d), want=(%d,2)", p.x, p.subsector, fracUnit)
			}
			g.projectiles = []projectile{p}
			if trace := g.demoTraceMobjs(); trace[len(trace)-1].Sector != 1 {
				t.Fatal("successful spawn trace did not enter the destination sector")
			}
			file := saveFile{Version: saveGameVersion, Game: gameSaveState{Projectiles: captureProjectiles(g.projectiles)}}
			data, err := encodeSnapshot(saveGameMagic, file)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeSnapshot(data, saveGameMagic)
			if err != nil {
				t.Fatal(err)
			}
			if restored := restoreProjectiles(decoded.Game.Projectiles)[0]; restored.subsector != p.subsector {
				t.Fatal("snapshot lost the live missile's subsector link")
			}
		}
	}
}

func TestLethalPlayerHitFromBelowConsumesForwardFallRandom(t *testing.T) {
	for _, height := range []int64{64 * fracUnit, 65 * fracUnit} {
		doomrand.Clear()
		g := &game{p: player{x: 64 * fracUnit, z: height}, stats: playerStats{Health: 6}, playerMobjHealth: 6}
		g.damagePlayerFromWithInflictorZ(9, "hit", 0, 0, true, -1, 0)
		wantDraws := 1 // Death state shortening.
		wantMomX := fixedMul(9*(fracUnit>>3), doomFineCosine(0))
		if height > 64*fracUnit {
			// The first random value is even, so this draw does not reverse thrust.
			wantDraws++
		}
		if _, got := doomrand.State(); got != wantDraws {
			t.Fatalf("height=%d RNG=%d want=%d", height, got, wantDraws)
		}
		if g.p.momx != wantMomX {
			t.Fatalf("height=%d momx=%d want=%d", height, g.p.momx, wantMomX)
		}
	}
	doomrand.Clear()
	_ = doomrand.PRandom() // Next draw is odd and reverses/quadruples thrust.
	g := &game{p: player{x: 64 * fracUnit, z: 65 * fracUnit}, stats: playerStats{Health: 6}, playerMobjHealth: 6}
	g.damagePlayerFromWithInflictorZ(9, "hit", 0, 0, true, -1, 0)
	if want := fixedMul(4*9*(fracUnit>>3), doomFineCosine(doomAng180)); g.p.momx != want {
		t.Fatalf("forward-fall momx=%d want=%d", g.p.momx, want)
	}
}

func TestFaceTargetInvisiblePlayerUsesOneDoomShadowJitter(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m:                 &mapdata.Map{Things: []mapdata.Thing{{Type: 3002}}},
		thingTargetPlayer: []bool{true},
		inventory:         playerInventory{InvisTics: 100},
	}
	g.faceMonsterToward(0, 0, 0, 128*fracUnit, 0)
	jitter := int32(8-109) << 21
	want := doomPointToAngle2(0, 0, 128*fracUnit, 0) + uint32(jitter)
	if got := g.thingWorldAngle(0, g.m.Things[0]); got != want {
		t.Fatalf("angle=%d want=%d", got, want)
	}
	if _, index := doomrand.State(); index != 2 {
		t.Fatalf("play RNG index=%d want=2", index)
	}
}

func TestRevenantTracerRetainsMonsterTargetAcrossSnapshot(t *testing.T) {
	g := &game{
		m:        &mapdata.Map{Things: []mapdata.Thing{{Type: 3005, X: 128, Y: 128}}},
		thingHP:  []int{400},
		p:        player{x: 128 * fracUnit, y: -128 * fracUnit},
		demoTick: 5,
	}
	p := projectile{kind: projectileTracer, sourceThing: -1, tracerThingTarget: 1,
		z: 32 * fracUnit, vx: 20 * fracUnit}
	file := saveFile{Version: saveGameVersion, Game: gameSaveState{Projectiles: captureProjectiles([]projectile{p})}}
	data, err := encodeSnapshot(saveGameMagic, file)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	p = restoreProjectiles(restored.Game.Projectiles)[0]
	g.tickProjectileSpecial(&p)
	if p.vy <= 0 {
		t.Fatalf("missile should turn toward monster north of it, momy=%d", p.vy)
	}
	g.thingHP[0] = 0
	angle, vx, vy, vz := p.angle, p.vx, p.vy, p.vz
	g.tickProjectileSpecial(&p)
	if p.angle != angle || p.vx != vx || p.vy != vy || p.vz != vz {
		t.Fatal("missile changed course after its target died")
	}
}

func TestProjectileDeathThinkerClipsToSupportFloor(t *testing.T) {
	g := &game{}
	fx := projectileImpact{kind: projectilePlasmaBall, sourceType: 68,
		z: 32 * fracUnit, floorz: 36 * fracUnit, ceilz: 128 * fracUnit,
		tics: 25, phaseTics: 5}
	g.advanceProjectileImpactTic(&fx)
	if fx.z != 36*fracUnit {
		t.Fatalf("impact z=%d want=%d", fx.z, 36*fracUnit)
	}
}

func TestRepeatingCrusherWaitsPastDestinationAndKeepsThinker(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 12}}},
		sectorFloor: []int64{0}, sectorCeil: []int64{12 * fracUnit},
		p:        player{x: 1024 * fracUnit},
		ceilings: map[int]*ceilingThinker{},
	}
	ct := &ceilingThinker{action: mapdata.CeilingSilentCrushRaise, direction: -1,
		crush: true, speed: fracUnit, bottomHeight: 8 * fracUnit, topHeight: 12 * fracUnit}
	g.ceilings[0] = ct
	for i := 0; i < 4; i++ {
		g.tickCeiling(0, ct)
	}
	if ct.direction != -1 || g.sectorCeil[0] != ct.bottomHeight {
		t.Fatal("crusher reversed on equality")
	}
	g.tickCeiling(0, ct)
	if ct.direction != 1 {
		t.Fatal("crusher did not reverse past bottom")
	}
	for i := 0; i < 4; i++ {
		g.tickCeiling(0, ct)
	}
	if ct.direction != 1 || g.sectorCeil[0] != ct.topHeight {
		t.Fatal("crusher reversed on top equality")
	}
	g.tickCeiling(0, ct)
	if ct.direction != -1 || g.ceilings[0] != ct {
		t.Fatal("crusher failed to begin its next cycle")
	}
}

func TestDuplicateBackpackSuppliesEveryAmmoKind(t *testing.T) {
	g := &game{inventory: playerInventory{Backpack: true, ReadyWeapon: weaponBFG}}
	_, _, picked := g.applyPickup(8, false)
	if !picked || g.stats.Bullets != 10 || g.stats.Shells != 4 || g.stats.Rockets != 1 || g.stats.Cells != 20 {
		t.Fatalf("duplicate backpack: picked=%v stats=%+v", picked, g.stats)
	}
	// Vanilla consumes a backpack even when all ammo is full.
	g.stats.Bullets, g.stats.Shells, g.stats.Rockets, g.stats.Cells = ammoCaps(true)
	if _, _, picked := g.applyPickup(8, false); !picked {
		t.Fatal("full-ammo backpack remained in the world")
	}
}

func TestDuplicatePlasmaGunSuppliesTwoCellClips(t *testing.T) {
	g := &game{inventory: playerInventory{ReadyWeapon: weaponBFG, Weapons: map[int16]bool{2004: true}}}
	if _, _, picked := g.applyPickup(2004, false); !picked || g.stats.Cells != 40 {
		t.Fatalf("duplicate plasma: picked=%v cells=%d", picked, g.stats.Cells)
	}
}

func TestSkullCollisionIncludesCorpseUntilFallAction(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}},
			Things: []mapdata.Thing{{Type: 3006}, {Type: 3006, X: 16}}},
		thingCollected: []bool{false, false}, thingDead: []bool{false, true},
		thingHP: []int{100, -5}, thingStatePhase: []int{0, 2},
		sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
	}
	probe := g.probeSkullFlyMove(0, 3006, 0, 0)
	if !probe.hitTarget || probe.target.idx != 1 {
		t.Fatal("charging skull missed a solid corpse")
	}
	g.thingStatePhase[1] = 3
	probe = g.probeSkullFlyMove(0, 3006, 0, 0)
	if probe.hitTarget {
		t.Fatal("charging skull hit corpse after A_Fall")
	}
}

func TestProjectileUsesVanillaBlockmapSearchRadius(t *testing.T) {
	g := &game{
		m:       &mapdata.Map{Things: []mapdata.Thing{{Type: 68, X: 128, Y: 128}}},
		thingHP: []int{500}, thingCollected: []bool{false},
		bmapWidth: 3, bmapHeight: 3,
	}
	p := projectile{sourceThing: -1, radius: 6 * fracUnit, height: 8 * fracUnit}
	if _, hit := g.projectileThingHitAtPosition(p, 89*fracUnit, 89*fracUnit, 0); hit {
		t.Fatal("hit oversized actor whose origin is outside MAXRADIUS cells")
	}
	g.bmapWidth, g.bmapHeight = 0, 0
	if _, hit := g.projectileThingHitAtPosition(p, 89*fracUnit, 89*fracUnit, 0); !hit {
		t.Fatal("fixture should overlap the oversized actor geometrically")
	}
}

func TestKillingChargingSkullClearsChargeFlag(t *testing.T) {
	g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 3006}}},
		thingHP: []int{1}, thingCollected: []bool{false}}
	g.ensureMonsterAIState()
	g.thingSkullFly[0] = true
	g.damageMonster(0, 2)
	if !g.thingDead[0] || g.thingSkullFly[0] {
		t.Fatal("killed charging skull retained MF_SKULLFLY")
	}
}

func TestOneUseWalkTriggerConsumedWhenSectorIsBusy(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Linedefs: []mapdata.Linedef{{Special: 36, Tag: 7}},
			Sectors: []mapdata.Sector{{Tag: 7, CeilingHeight: 128}}},
		lineSpecial: []uint16{36}, sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
		floors: map[int]*floorThinker{0: {}},
		lines: []physLine{{idx: 0, x1: 0, y1: 64 * fracUnit, x2: 0, y2: -64 * fracUnit,
			dx: 0, dy: -128 * fracUnit, slope: slopeVertical,
			bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 0, 0}}},
	}
	g.checkWalkSpecialLinesForActorWithCandidates(-8*fracUnit, 0, 8*fracUnit, 0, -1, true, []int{0})
	if g.lineSpecial[0] != 0 {
		t.Fatal("busy tagged sector left W1 special active")
	}
}

func TestArchVileFireCapturesVictimAndRunsOrderedStates(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m:                 &mapdata.Map{Things: []mapdata.Thing{{Type: 64}, {Type: 3005, X: 128}}},
		thingTargetPlayer: []bool{false, false}, thingTargetIdx: []int{1, -1},
		sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
		p: player{y: -128 * fracUnit}, nextThinkerOrder: 5,
	}
	g.spawnArchVileFire(0)
	if len(g.projectileImpacts) != 1 {
		t.Fatal("missing fire")
	}
	fx := g.projectileImpacts[0]
	if fx.order != 5 || fx.phaseTics != 2 || fx.fireTargetThing != 2 {
		t.Fatalf("fire order=%d tics=%d target=%d", fx.order, fx.phaseTics, fx.fireTargetThing)
	}
	if _, index := doomrand.State(); index != 1 {
		t.Fatalf("RNG index=%d want=1", index)
	}
	g.thingTargetPlayer[0] = true
	// Changing the caster's target must leave this fire following the monster.
	g.followArchVileFire(&fx)
	if fx.x < 128*fracUnit || fx.y != fixedMul(24*fracUnit, doomFineSineAtAngle(0)) {
		t.Fatalf("fire followed wrong victim: (%d,%d)", fx.x, fx.y)
	}
	file := saveFile{Version: saveGameVersion, Game: gameSaveState{ProjectileImpacts: captureProjectileImpacts([]projectileImpact{fx})}}
	data, err := encodeSnapshot(saveGameMagic, file)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	got := restoreProjectileImpacts(restored.Game.ProjectileImpacts)[0]
	if got != fx {
		t.Fatalf("fire changed across snapshot: got=%+v want=%+v", got, fx)
	}
	for tic := 1; tic < 60; tic++ {
		if !g.advanceProjectileImpactTic(&got) {
			t.Fatalf("fire removed at tic %d", tic)
		}
	}
	if got.phase != 29 || got.phaseTics != 1 {
		t.Fatalf("final state=%d tics=%d", got.phase, got.phaseTics)
	}
	if g.advanceProjectileImpactTic(&got) {
		t.Fatal("fire survives S_FIRE30")
	}
}

func TestFullStatsStillConsumeSoulAndMegaspheres(t *testing.T) {
	for _, typ := range []int16{2013, 83} {
		g := &game{stats: playerStats{Health: 200, Armor: 200, ArmorType: 2}}
		if _, _, consumed := g.applyPickup(typ, false); !consumed {
			t.Fatalf("type=%d left behind at full stats", typ)
		}
		if g.stats.Health != 200 || g.stats.Armor != 200 || g.stats.ArmorType != 2 {
			t.Fatalf("type=%d altered full stats: %+v", typ, g.stats)
		}
	}
}
