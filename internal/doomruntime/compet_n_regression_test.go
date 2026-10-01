package doomruntime

import (
	"fmt"
	"reflect"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestLostSoulChargeAimsAtPlayerCorpseHeight(t *testing.T) {
	for _, dead := range []bool{false, true} {
		t.Run(fmt.Sprint(dead), func(t *testing.T) {
			g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 3006}}},
				thingHP: []int{9}, thingCollected: []bool{false}, thingDead: []bool{false},
				thingTargetPlayer: []bool{true}, thingTargetIdx: []int{-1}, isDead: dead,
				p: player{x: 166110, y: 80708577, z: 8388608}}
			g.ensureMonsterAIState()
			g.thingX[0], g.thingY[0] = -4198292, 2196010
			g.setThingSupportState(0, 11655138, 8388608, 16515072)
			if !g.startLostSoulCharge(0) {
				t.Fatal("retained player target must allow the pending charge")
			}
			want := int64(-23467)
			if dead {
				want = -46029
			}
			if g.thingMomZ[0] != want {
				t.Fatalf("charge momz=%d want=%d (dead=%v)", g.thingMomZ[0], want, dead)
			}
		})
	}
}

func TestLethalCrusherBloodUsesPlayerCorpseHeight(t *testing.T) {
	g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{FloorHeight: 16, CeilingHeight: 128}}}}
	g.initPhysics()
	g.initPlayerState()
	g.stats.Health, g.playerMobjHealth = 10, 10
	g.worldTic = 1
	g.p.z, g.p.floorz = 16*fracUnit, 16*fracUnit
	g.setSectorCeilingHeightWithCrush(0, 65*fracUnit+3*fracUnit/4, true)
	if !g.isDead || len(g.hitscanPuffs) != 1 {
		t.Fatalf("lethal crusher did not spawn blood: dead=%t effects=%d", g.isDead, len(g.hitscanPuffs))
	}
	if got, want := g.hitscanPuffs[0].z, g.p.z+7*fracUnit; got != want {
		t.Fatalf("blood z=%d want corpse center=%d", got, want)
	}
}

func TestRuntimeMonstersRemainSolidWithNoMonstersEnabled(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	g := newGame(&mapdata.Map{Name: "MAP30",
		Things:  []mapdata.Thing{{Type: 3005, X: 100, Flags: 7}, {Type: 1}},
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
	}, Options{Width: doomLogicalW, Height: doomLogicalH, SkillLevel: 4, NoMonsters: true})
	g.p.x = -1000 * fracUnit
	// A_SpawnFly uses P_SpawnMobj, bypassing the map's -nomonsters filter.
	other := g.appendRuntimeThing(mapdata.Thing{Type: 3005, X: 65, Flags: 7}, false)
	g.thingHP[other] = 400
	mover := g.appendRuntimeThing(mapdata.Thing{Type: 3002, Flags: 7}, false)
	g.thingHP[mover] = 150
	if !g.thingActiveInSession(other) || !g.thingBlocksInSession(other) {
		t.Fatal("brain-spawned monster was filtered from active collision actors")
	}
	if g.probeMonsterMove(mover, 3002, 5*fracUnit, 0).ok {
		t.Fatal("demon moved through a brain-spawned cacodemon")
	}
	if g.thingActiveInSession(0) || g.thingBlocksInSession(0) {
		t.Fatal("original map monster escaped the -nomonsters filter")
	}
}

func TestRaisedImpReacquiresTargetWithoutChasingOnRaiseExit(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	g := newGame(&mapdata.Map{Name: "MAP20",
		Things:  []mapdata.Thing{{Type: 3001, X: 100, Flags: 7}, {Type: 1}},
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
	}, Options{Width: doomLogicalW, Height: doomLogicalH, SkillLevel: 4})
	g.ensureMonsterAIState()
	g.p.x = -1000 * fracUnit
	g.thingState[0], g.thingStatePhase[0], g.thingStateTics[0] = monsterStateRaise, 4, 1
	g.thingAggro[0], g.thingTargetPlayer[0], g.thingTargetIdx[0] = false, false, -1
	g.thingMoveCount[0], g.thingMoveDir[0], g.thingThreshold[0] = 9, monsterDirSouthWest, 96
	g.thingLastLook[0] = 0
	x, y := g.thingPosFixed(0, g.m.Things[0])
	doomrand.SetState(0, 61)
	g.tickMonsterRaiseOrHeal(0, g.m.Things[0])
	if !g.thingTargetPlayer[0] || g.thingThreshold[0] != 0 {
		t.Fatal("raise-exit A_Chase must reacquire the player and clear its old threshold")
	}
	if g.thingMoveCount[0] != 9 || g.thingMoveDir[0] != monsterDirSouthWest {
		t.Fatalf("raise exit also chased: count=%d dir=%d", g.thingMoveCount[0], g.thingMoveDir[0])
	}
	if ax, ay := g.thingPosFixed(0, g.m.Things[0]); ax != x || ay != y {
		t.Fatal("raise exit moved after reacquiring the player")
	}
	if _, prnd := doomrand.State(); prnd != 61 {
		t.Fatalf("raise-exit direct reacquisition consumed gameplay RNG: %d", prnd)
	}
	if g.thingState[0] != monsterStateSee || g.thingStatePhase[0] != 0 || g.thingStateTics[0] != 3 {
		t.Fatal("raise exit did not preserve the full newly entered RUN1 frame")
	}
}

func TestRaisedArachnotronEntersRunBeforeReacquiringTarget(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(fmt.Sprint(visible), func(t *testing.T) {
			t.Cleanup(doomrand.Clear)
			g := newGame(&mapdata.Map{Name: "MAP23",
				Things:  []mapdata.Thing{{Type: 68, X: 100, Flags: 7}, {Type: 1}},
				Sectors: []mapdata.Sector{{CeilingHeight: 128}},
			}, Options{Width: doomLogicalW, Height: doomLogicalH, SkillLevel: 4})
			g.ensureMonsterAIState()
			g.p.x = -1000 * fracUnit
			if !visible {
				g.m.RejectMatrix = &mapdata.RejectMatrix{SectorCount: 1, Data: []byte{1}}
			}
			g.sectorSoundTarget = []bool{true}
			g.thingState[0], g.thingStatePhase[0], g.thingStateTics[0] = monsterStateRaise, 6, 1
			g.thingDoomState[0] = 666
			g.thingAggro[0], g.thingTargetPlayer[0], g.thingTargetIdx[0] = false, false, -1
			g.thingMoveCount[0], g.thingMoveDir[0], g.thingThreshold[0] = 915, monsterDirNoDir, 20
			doomrand.SetState(0, 61)
			g.tickMonsterRaiseOrHeal(0, g.m.Things[0])
			wantState, wantTics := 634, 20
			if visible {
				wantState, wantTics = 635, 3
			}
			if !g.thingTargetPlayer[0] || g.thingThreshold[0] != 0 || g.thingDoomState[0] != wantState || g.thingStateTics[0] != wantTics {
				t.Fatalf("raise exit target/threshold/state/tics=%t/%d/%d/%d want player/0/%d/%d", g.thingTargetPlayer[0], g.thingThreshold[0], g.thingDoomState[0], g.thingStateTics[0], wantState, wantTics)
			}
			if g.thingMoveCount[0] != 915 || g.thingMoveDir[0] != monsterDirNoDir || g.thingX[0] != 100*fracUnit {
				t.Fatal("raise exit moved after direct target reacquisition")
			}
			if _, prnd := doomrand.State(); prnd != 61 {
				t.Fatal("raise exit consumed gameplay RNG")
			}
		})
	}
}

func TestPlayerCorpsePreservesSlidingMomentumAcrossStep(t *testing.T) {
	for _, dead := range []bool{false, true} {
		for _, momentum := range []int64{fracUnit, fracUnit / 4} {
			g := newDoorTimingGame(1)
			g.initPhysics()
			g.sectorFloor[1] = 64 * fracUnit
			g.p.x, g.p.y = -8*fracUnit, 0
			g.p.z, g.p.floorz, g.p.ceilz = 64*fracUnit, 64*fracUnit, 128*fracUnit
			g.p.momx, g.p.momy, g.isDead = momentum, -momentum, dead
			g.xyMovement()
			want := fixedMul(momentum, friction)
			if dead && momentum > fracUnit/4 {
				want = momentum
			}
			if g.p.momx != want || g.p.momy != -want {
				t.Fatalf("dead=%v momentum=%d got=%d,%d want=%d,%d", dead, momentum, g.p.momx, g.p.momy, want, -want)
			}
		}
	}
}

func TestArchVileSelfDamageDoesNotTickNewPainStateTwice(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Things:  []mapdata.Thing{{Type: 64, X: 64}},
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
	}, opts: Options{SkillLevel: 4}, thingHP: []int{700}, thingCollected: []bool{false},
		thingDead: []bool{false}, thingAggro: []bool{true},
		sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
		p: player{ceilz: 128 * fracUnit}, stats: playerStats{Health: 1000}, playerMobjHealth: 1000,
		invulnerable: true}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.thingTargetPlayer[0] = true
	g.thingTargetIdx[0] = -1
	g.thingState[0] = monsterStateAttack
	g.thingStateTics[0] = 1
	g.thingAttackTics[0] = 94
	g.thingAttackPhase[0] = 8
	g.spawnArchVileFire(0)
	// The same pain roll as MAP14 pa14-043's self-inflicted blast.
	doomrand.SetState(0, 95)
	defer doomrand.Clear()
	g.tickGenericMonsterState(0, g.m.Things[0])
	if g.thingHP[0] >= 700 || g.thingState[0] != monsterStatePain {
		t.Fatalf("blast must damage the vile and enter pain: hp=%d state=%d", g.thingHP[0], g.thingState[0])
	}
	if g.thingStateTics[0] != 5 || g.thingPainTics[0] != 10 || g.thingStatePhase[0] != 0 {
		t.Fatalf("new pain state advanced on its entry tic: frame tics=%d remaining=%d phase=%d", g.thingStateTics[0], g.thingPainTics[0], g.thingStatePhase[0])
	}
	g.tickGenericMonsterState(0, g.m.Things[0])
	if g.thingStateTics[0] != 4 || g.thingPainTics[0] != 9 {
		t.Fatalf("next thinker must advance pain once: frame tics=%d remaining=%d", g.thingStateTics[0], g.thingPainTics[0])
	}
}

func TestSkullCollisionResetRunsDemonLookChaseBeforeNormalThinker(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		skill, wantTics, wantSteps int
	}{
		{"normal", 4, 1, 1},
		{"nightmare", 5, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{m: &mapdata.Map{
				Things:  []mapdata.Thing{{Type: 3002, Angle: 0}},
				Sectors: []mapdata.Sector{{CeilingHeight: 128}},
			}, opts: Options{SkillLevel: tc.skill}, thingHP: []int{150},
				thingCollected: []bool{false}, thingDead: []bool{false}, thingAggro: []bool{true},
				stats: playerStats{Health: 100}, playerMobjHealth: 100}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p = player{x: 256 * fracUnit, ceilz: 128 * fracUnit}
			g.thingTargetPlayer[0] = true
			g.thingTargetIdx[0] = -1
			g.thingMoveDir[0] = monsterDirEast
			g.thingMoveCount[0] = 10
			g.thingState[0] = monsterStateSee
			doomrand.Clear()
			defer doomrand.Clear()
			// PIT_CheckThing can reset the damaged demon through shared tmthing.
			// P_SetMobjState(spawnstate) runs A_Look and its nested A_Chase now.
			g.resetLostSoulCharge(0, 3002)
			x, _ := g.thingPosFixed(0, g.m.Things[0])
			if x != 10*fracUnit || g.thingMoveCount[0] != 9 || g.thingStatePhase[0] != 0 {
				t.Fatalf("reset must run initial chase immediately: x=%d count=%d phase=%d", x, g.thingMoveCount[0], g.thingStatePhase[0])
			}
			// A later thinker decrements the installed frame. Nightmare's one-tic
			// run state enters RUN2 and chases again; normal mode keeps RUN1.
			g.tickThingThinker(0, g.m.Things[0])
			x, _ = g.thingPosFixed(0, g.m.Things[0])
			if x != int64(tc.wantSteps)*10*fracUnit || g.thingMoveCount[0] != 10-tc.wantSteps ||
				g.thingStatePhase[0] != tc.wantSteps-1 || g.thingStateTics[0] != tc.wantTics {
				t.Fatalf("normal thinker: x=%d count=%d phase=%d tics=%d", x, g.thingMoveCount[0], g.thingStatePhase[0], g.thingStateTics[0])
			}
		})
	}
}

func TestEffectSpawningPreservesLiveThinkersBeyond64(t *testing.T) {
	for _, tc := range []struct {
		name     string
		spawn    func(*game, int64)
		impacts  bool
		perSpawn int
	}{
		{"puff", func(g *game, x int64) { g.spawnHitscanPuff(x, 0, 32*fracUnit) }, false, 1},
		{"blood", func(g *game, x int64) { g.spawnHitscanBlood(x, 0, 32*fracUnit, 20) }, false, 1},
		{"tracer", func(g *game, x int64) { g.spawnTracerSmokeTrail(x, 0, 32*fracUnit, 0, 0) }, false, 2},
		{"teleport", func(g *game, x int64) { g.spawnTeleportFog(x, 0, 0) }, false, 1},
		{"impact", func(g *game, x int64) { g.spawnProjectileImpact(projectileRocket, x, 0, 32*fracUnit, 0) }, true, 1},
		{"deferred-impact", func(g *game, x int64) { g.spawnProjectileImpactDeferredRandom(projectileRocket, x, 0, 32*fracUnit, 0) }, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}},
				sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit}}
			doomrand.Clear()
			defer doomrand.Clear()
			for i := 0; i < 65; i++ {
				tc.spawn(g, int64(i)*fracUnit)
			}
			want := 65 * tc.perSpawn
			if tc.impacts {
				if len(g.projectileImpacts) != want || g.projectileImpacts[0].x != 0 || g.projectileImpacts[0].order != 1 {
					t.Fatalf("live impacts evicted: count=%d want=%d first=%+v", len(g.projectileImpacts), want, g.projectileImpacts[0])
				}
			} else if len(g.hitscanPuffs) != want || g.hitscanPuffs[0].x != 0 || g.hitscanPuffs[0].order != 1 {
				t.Fatalf("live effects evicted: count=%d want=%d first=%+v", len(g.hitscanPuffs), want, g.hitscanPuffs[0])
			}
		})
	}
}

func TestNewMissileRunsAtItsThinkerOrderBeforeLaterFloor(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		missileOrder, floorOrder int64
		wantFloor                int64
	}{
		{"missile-before-floor", 1, 2, 136 * fracUnit},
		{"floor-before-missile", 2, 1, 135 * fracUnit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{
				m:           &mapdata.Map{Sectors: []mapdata.Sector{{FloorHeight: 136, CeilingHeight: 232}}},
				sectorFloor: []int64{136 * fracUnit}, sectorCeil: []int64{232 * fracUnit},
				p: player{x: -1024 * fracUnit},
				floors: map[int]*floorThinker{0: {order: tc.floorOrder, sector: 0,
					direction: -1, speed: fracUnit, destHeight: 0}},
				projectiles: []projectile{{order: tc.missileOrder, deferredTick: true,
					x: 128 * fracUnit, y: 128 * fracUnit, z: 168 * fracUnit,
					floorz: 136 * fracUnit, ceilz: 232 * fracUnit,
					vx: fracUnit, radius: 6 * fracUnit, height: 8 * fracUnit,
					kind: projectileFireball, frameTics: 3, sourceThing: -1}},
			}
			g.tickThinkers()
			if len(g.projectiles) != 1 {
				t.Fatalf("projectile count=%d want=1", len(g.projectiles))
			}
			p := g.projectiles[0]
			if p.floorz != tc.wantFloor || g.sectorFloor[0] != 135*fracUnit {
				t.Fatalf("missile floor=%d want=%d; sector floor=%d", p.floorz, tc.wantFloor, g.sectorFloor[0])
			}
			if p.x != 129*fracUnit || p.frameTics != 2 || p.deferredTick {
				t.Fatalf("missile must advance exactly once: x=%d tics=%d deferred=%v", p.x, p.frameTics, p.deferredTick)
			}
		})
	}
}

func TestSkullCollisionVisitsPlayerInBlockmapOrder(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		monsterX, playerX, probeX int16
		monsterOrder, playerOrder int64
		want                      lineAttackTargetKind
	}{
		{"monster-newer-in-same-cell", 40, 32, 32, 3, 2, lineAttackTargetThing},
		{"player-newer-in-same-cell", 40, 32, 32, 2, 3, lineAttackTargetPlayer},
		{"earlier-cell-before-newer-player", 96, 144, 120, 2, 3, lineAttackTargetThing},
		{"earlier-player-cell-before-newer-monster", 144, 96, 120, 3, 2, lineAttackTargetPlayer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{m: &mapdata.Map{
				Sectors: []mapdata.Sector{{CeilingHeight: 128}},
				Things: []mapdata.Thing{{Type: 3006, X: 224, Y: 32, Flags: skillMask},
					{Type: 58, X: tc.monsterX, Y: 32, Flags: skillMask}},
			}, opts: Options{SkillLevel: 3}, stats: playerStats{Health: 100}, playerMobjHealth: 100,
				p:       player{x: int64(tc.playerX) * fracUnit, y: 32 * fracUnit},
				thingHP: []int{100, 150}, thingCollected: []bool{false, false},
				thingDead: []bool{false, false}, bmapWidth: 2, bmapHeight: 1,
				thingBlockOrder: []int64{1, tc.monsterOrder}, playerBlockOrder: tc.playerOrder}
			g.rebuildThingBlockmap()
			probe := g.probeSkullFlyMove(0, 3006, int64(tc.probeX)*fracUnit, 32*fracUnit)
			if !probe.hitTarget || probe.target.kind != tc.want || (tc.want == lineAttackTargetThing && probe.target.idx != 1) {
				t.Fatalf("collision target=%+v, want kind=%d", probe, tc.want)
			}
		})
	}
}

func TestDamagedChargingSkullSlamsDuringChaseMovement(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 3006, X: 96, Y: 32, Flags: skillMask}},
	}, opts: Options{SkillLevel: 3}, stats: playerStats{Health: 100}, playerMobjHealth: 100,
		thingHP: []int{90}, thingCollected: []bool{false}, thingDead: []bool{false}, thingAggro: []bool{true}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.p = player{x: 64 * fracUnit, y: 32 * fracUnit, ceilz: 128 * fracUnit}
	g.thingTargetPlayer[0] = true
	g.thingSkullFly[0] = true
	g.setThingMomentum(0, fracUnit, -fracUnit, fracUnit)
	g.setThingSupportState(0, 0, 0, 128*fracUnit)
	g.thingDoomState[0] = monsterDoomSeeState(3006)
	g.thingState[0] = monsterStateSee
	// A retained charge is already within one walking step of its target.
	// Its damaged see-state action must perform the same slam as momentum.
	doomrand.SetState(0, 0)
	defer doomrand.SetState(0, 0)
	beforeX, beforeY := g.thingPosFixed(0, g.m.Things[0])
	if g.monsterMoveInDir(0, 3006, monsterDirWest) {
		t.Fatal("skull slam unexpectedly accepted the walking move")
	}
	if g.stats.Health != 97 || g.playerMobjHealth != 97 {
		t.Fatalf("health=%d/%d, want 3 points of charge damage", g.stats.Health, g.playerMobjHealth)
	}
	if g.thingSkullFly[0] || g.thingMomX[0] != 0 || g.thingMomY[0] != 0 || g.thingMomZ[0] != 0 {
		t.Fatal("walking collision did not end the skull charge")
	}
	if x, y := g.thingPosFixed(0, g.m.Things[0]); x != beforeX || y != beforeY {
		t.Fatalf("blocked skull moved to (%d,%d)", x, y)
	}
	// Entering the spawn state immediately runs A_Look, then A_Chase.
	// With this visible target it starts a fresh attack windup, without
	// restoring MF_SKULLFLY until the next attack frame's action.
	if g.thingDoomState[0] != monsterDoomMissileState(3006) {
		t.Fatalf("skull state=%d, want a fresh attack windup", g.thingDoomState[0])
	}
	if _, rng := doomrand.State(); rng != 3 {
		t.Fatalf("gameplay RNG=%d, want damage, player pain, and fresh missile-range rolls", rng)
	}
}

func TestHitscanBoundaryNudgeAlsoMovesThingInterceptRay(t *testing.T) {
	// Original Doom's MAP25 shotgun ray at tic 127 starts on a block row.
	// Its one-unit Y nudge makes this pellet miss the player's corner.
	g := &game{m: &mapdata.Map{BlockMap: &mapdata.BlockMap{Cells: make([][]int16, 16)}},
		bmapOriginX: 640 * fracUnit, bmapOriginY: 528 * fracUnit, bmapWidth: 4, bmapHeight: 4,
		p: player{x: 48947745, y: 39776634}, playerBlockOrder: 5}
	actor := lineAttackActor{x: 44040192, y: 51380224, shootZ: -1835008,
		thingIdx: -1, targetMask: lineAttackMaskPlayer}
	if got := g.lineAttackPath(actor, 3571231456, 2048*fracUnit); got != (divline{x: 44040192, y: 51445760, dx: 65751040, dy: -117073920}) {
		t.Fatalf("shot path=%+v, want original Doom's nudged ray with unchanged endpoint", got)
	}
	for _, in := range g.collectLineAttackIntercepts(actor, 3571231456, 2048*fracUnit) {
		if !in.isLine && in.target.kind == lineAttackTargetPlayer {
			t.Fatal("boundary ray used its unnudged origin and incorrectly crossed the player")
		}
	}
}

func TestFloatingMonsterUsesRetainedDeadMonsterTarget(t *testing.T) {
	for _, typ := range []int16{3006, 3005, 71} {
		t.Run(fmt.Sprint(typ), func(t *testing.T) {
			g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: typ}, {Type: 3001, X: 16}}},
				thingHP: []int{100, -4}, thingDead: []bool{false, true}, thingCollected: []bool{false, false},
				thingTargetPlayer: []bool{false, false}, thingTargetIdx: []int{1, -1},
				thingX: []int64{0, 16 * fracUnit}, thingY: []int64{0, 0},
				thingZState: []int64{80 * fracUnit, 0}, thingFloorState: []int64{0, 0}, thingCeilState: []int64{256 * fracUnit, 256 * fracUnit},
				thingSupportValid: []bool{true, true}, thingInFloat: []bool{false, false}}
			if g.monsterHasTarget(0) {
				t.Fatal("corpse should not be a live attack target")
			}
			g.tickMonsterZMovement(0, g.m.Things[0], 80*fracUnit, 0, 256*fracUnit, 0)
			z, _, _ := g.thingSupportState(0, g.m.Things[0])
			if z != 76*fracUnit {
				t.Fatalf("height=%d, want four-unit descent toward retained corpse", z)
			}
		})
	}
}

func TestArchVileBlastPreservesLastLinkedFireSubsector(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Things:     []mapdata.Thing{{Type: 64, X: 64}},
		Sectors:    []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 128}},
		SubSectors: []mapdata.SubSector{{}, {}},
		Nodes:      []mapdata.Node{{DY: 128, ChildID: [2]uint16{0x8000, 0x8001}}},
	}, opts: Options{SkillLevel: 4}, thingHP: []int{700}, thingCollected: []bool{false},
		thingTargetPlayer: []bool{true}, thingTargetIdx: []int{-1},
		sectorFloor: []int64{0, 0}, sectorCeil: []int64{128 * fracUnit, 128 * fracUnit},
		p: player{x: 8 * fracUnit, ceilz: 128 * fracUnit}, stats: playerStats{Health: 1000}, playerMobjHealth: 1000}
	doomrand.Clear()
	defer doomrand.Clear()
	g.spawnArchVileFire(0)
	fx := g.archVileFireForSource(0)
	linked := g.subSectorAtFixed(fx.x, fx.y) + 1
	if fx.subsector != linked {
		t.Fatalf("initial fire subsector=%d, want %d", fx.subsector, linked)
	}
	if !g.archVileBlast(0, 64*fracUnit, 0) {
		t.Fatal("visible target should receive the blast")
	}
	fx = g.archVileFireForSource(0)
	if fx.x >= 0 || g.subSectorAtFixed(fx.x, fx.y)+1 == linked {
		t.Fatal("fixture should move blast coordinates across the BSP partition")
	}
	if fx.subsector != linked {
		t.Fatal("A_VileAttack incorrectly relinked the fire")
	}
	g.p.x = -64 * fracUnit
	g.followArchVileFire(fx)
	if fx.subsector == linked || fx.subsector != g.subSectorAtFixed(fx.x, fx.y)+1 {
		t.Fatal("subsequent A_Fire failed to relink the fire at its new position")
	}
}

func TestChargingSkullSlamsDuringSectorHeightClip(t *testing.T) {
	for _, targetType := range []int16{30, 2008} {
		for _, charging := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/charging=%v", targetType, charging), func(t *testing.T) {
				g := &game{m: &mapdata.Map{
					Sectors: []mapdata.Sector{{CeilingHeight: 192}},
					Things: []mapdata.Thing{{Type: 3006, X: 64, Y: 32, Flags: skillMask},
						{Type: targetType, X: 40, Y: 32, Flags: skillMask}},
				}, opts: Options{SkillLevel: 5}, stats: playerStats{Health: 100}, playerMobjHealth: 100,
					thingHP: []int{100, 1000}, thingCollected: []bool{false, false},
					thingDead: []bool{false, false}, thingAggro: []bool{true, false}}
				g.initPhysics()
				g.ensureMonsterAIState()
				g.p = player{x: 256 * fracUnit, y: 32 * fracUnit, ceilz: 192 * fracUnit}
				g.sectorSoundTarget = []bool{true}
				g.thingTargetPlayer[0] = true
				g.thingDoomState[0], g.thingState[0], g.thingStateTics[0] = 590, monsterStateAttack, 4
				g.thingSkullFly[0], g.thingJustAtk[0] = charging, true
				g.setThingMomentum(0, 20*fracUnit, 0, fracUnit)
				// The skull is above the object's vertical bounds. Original
				// MF_SKULLFLY collisions still test only their XY overlap.
				g.setThingSupportState(0, 96*fracUnit, 0, 192*fracUnit)
				g.setThingSupportState(1, 0, 0, 192*fracUnit)
				doomrand.Clear()
				defer doomrand.Clear()
				if !g.heightClipThing(0, g.m.Things[0]) {
					t.Fatal("skull should still fit the moving sector opening")
				}
				_, rng := doomrand.State()
				if charging {
					if g.thingSkullFly[0] || g.thingMomX[0] != 0 || g.thingMomY[0] != 0 || g.thingMomZ[0] != 0 || g.thingDoomState[0] != 587 || g.thingStateTics[0] != 6 || rng != 1 {
						t.Fatalf("height clip failed to end the charge: flag=%v momentum=%d,%d,%d state=%d tics=%d rng=%d", g.thingSkullFly[0], g.thingMomX[0], g.thingMomY[0], g.thingMomZ[0], g.thingDoomState[0], g.thingStateTics[0], rng)
					}
				} else if g.thingDoomState[0] != 590 || g.thingMomX[0] != 20*fracUnit || rng != 0 {
					t.Fatal("ordinary height clip incorrectly performed a skull slam")
				}
				if g.thingHP[1] != 1000 || g.thingCollected[1] || g.stats.Health != 100 {
					t.Fatal("non-shootable slam target should remain intact without hurting the distant player")
				}
			})
		}
	}
}

func TestHeightClipRetainsNestedSkullChaseOpeningAndFloatHeight(t *testing.T) {
	for _, highFloor := range []int64{65, 80} {
		t.Run(fmt.Sprint(highFloor), func(t *testing.T) {
			g := &game{m: &mapdata.Map{
				Sectors:  []mapdata.Sector{{FloorHeight: 24, CeilingHeight: 200}, {FloorHeight: int16(highFloor), CeilingHeight: 200}},
				Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
				Things: []mapdata.Thing{{Type: 3006, X: 64, Y: 32, Flags: skillMask},
					{Type: 30, X: 40, Y: 32, Flags: skillMask}},
			}, opts: Options{SkillLevel: 3}, stats: playerStats{Health: 100}, playerMobjHealth: 100,
				thingHP: []int{100, 1000}, thingCollected: []bool{false, false},
				thingDead: []bool{false, false}, thingAggro: []bool{true, false}}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p = player{x: 256 * fracUnit, y: 32 * fracUnit, z: 24 * fracUnit, floorz: 24 * fracUnit, ceilz: 200 * fracUnit}
			g.lines = []physLine{{idx: 0, x1: 80 * fracUnit, y1: 64 * fracUnit,
				x2: 80 * fracUnit, y2: -64 * fracUnit, dy: -128 * fracUnit,
				slope: slopeVertical, sideNum0: 0, sideNum1: 1, flags: mlTwoSided,
				bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 80 * fracUnit, 80 * fracUnit}}}
			g.sectorSoundTarget = []bool{true, true}
			g.thingTargetPlayer[0] = true
			g.thingDoomState[0], g.thingState[0], g.thingStateTics[0] = 590, monsterStateAttack, 4
			g.thingSkullFly[0] = true
			g.thingMoveDir[0], g.thingMoveCount[0] = monsterDirEast, 2
			g.setThingSupportState(0, 48*fracUnit, 24*fracUnit, 200*fracUnit)
			g.setThingSupportState(1, 24*fracUnit, 24*fracUnit, 200*fracUnit)
			doomrand.Clear()
			defer doomrand.Clear()
			if !g.heightClipThing(0, g.m.Things[0]) {
				t.Fatal("skull should fit the nested chase opening")
			}
			wantX, wantZ := int64(72*fracUnit), int64(48*fracUnit)
			if highFloor == 80 {
				wantX, wantZ = 64*fracUnit, 52*fracUnit
			}
			z, floor, _ := g.thingSupportState(0, g.m.Things[0])
			if g.thingX[0] != wantX || z != wantZ || floor != highFloor*fracUnit {
				t.Fatalf("nested clip x/z/floor=%d/%d/%d want=%d/%d/%d", g.thingX[0], z, floor, wantX, wantZ, highFloor*fracUnit)
			}
		})
	}
}

func TestBlockMonstersLineClipsPickupsAndBlood(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprint(blocked), func(t *testing.T) {
			g := newDoorTimingGame(0)
			g.sectorCeil[0] = 80 * fracUnit
			g.p.x = 1000 * fracUnit
			g.m.Things = []mapdata.Thing{{Type: 2008, X: 4, Flags: skillMask}}
			if blocked {
				g.lines[0].flags |= mlBlockMonsters
			}
			g.setThingSupportState(0, 0, 0, 128*fracUnit)
			if !g.heightClipThing(0, g.m.Things[0]) {
				t.Fatal("pickup did not fit its support opening")
			}
			wantCeil := int64(80 * fracUnit)
			if blocked {
				wantCeil = 128 * fracUnit
			}
			if _, _, ceil := g.thingSupportState(0, g.m.Things[0]); ceil != wantCeil {
				t.Fatalf("pickup ceiling=%d, want %d", ceil, wantCeil)
			}
			p := hitscanPuff{kind: hitscanFxBlood, state: 90, tics: 8, x: 4 * fracUnit,
				z: 32 * fracUnit, momx: fracUnit, floorz: 0, ceilz: 128 * fracUnit}
			doomrand.Clear()
			if !g.tickHitscanPuff(&p) {
				t.Fatal("blood disappeared before its collision check")
			}
			wantX, wantMomX := int64(5*fracUnit), int64(fracUnit)
			if blocked {
				wantX, wantMomX = 4*fracUnit, 0
			}
			if p.x != wantX || p.momx != wantMomX {
				t.Fatalf("blood x/momx=%d/%d, want %d/%d", p.x, p.momx, wantX, wantMomX)
			}
			if _, rng := doomrand.State(); rng != 0 {
				t.Fatalf("blood collision consumed gameplay RNG: %d", rng)
			}
		})
	}
}

func TestSectorHeightClipFollowsSkullRelinkIntoAnotherCell(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things: []mapdata.Thing{
			{Type: 2028, X: 90, Y: 100, Flags: skillMask},
			{Type: 71, X: 200, Y: 100, Flags: skillMask},
			{Type: 3006, X: 129, Y: 63, Angle: 225, Flags: skillMask},
		},
	}, opts: Options{SkillLevel: 3}, thingHP: []int{1000, 400, 100},
		thingCollected: []bool{false, false, false}, thingDead: []bool{false, false, false},
		thingAggro: []bool{false, false, false},
		thingX:     []int64{90 * fracUnit, 200 * fracUnit, 129 * fracUnit},
		thingY:     []int64{100 * fracUnit, 100 * fracUnit, 63 * fracUnit},
	}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.bmapWidth, g.bmapHeight = 2, 1
	g.sectorBBox = []worldBBox{{minX: 170, maxX: 216, minY: 64, maxY: 64}}
	g.thingBlockOrder = []int64{2, 3, 4}
	g.nextBlockmapOrder = 5
	g.playerBlockOrder = 1
	g.p.x, g.p.y = 100*fracUnit, 32*fracUnit
	g.stats.Health = 1000
	g.playerMobjHealth = 1000
	g.rebuildThingBlockmap()
	for i := range g.m.Things {
		g.setThingSupportState(i, 0, 0, 128*fracUnit)
	}
	g.thingSkullFly[2], g.thingJustAtk[2], g.thingAggro[2], g.thingTargetPlayer[2] = true, true, true, true
	g.thingMoveDir[2] = monsterDirNorthWest
	g.thingReactionTics[2] = 0
	doomrand.SetState(0, 0)
	g.setSectorFloorHeight(0, fracUnit)
	if g.thingBlockCell[2] != 0 {
		t.Fatalf("height clipping did not relink the skull into the other cell: x=%d y=%d", g.thingX[2], g.thingY[2])
	}
	if _, floor, _ := g.thingSupportState(1, g.m.Things[1]); floor != 0 {
		t.Fatal("blocklinks traversal visited the Pain Elemental left behind in the old cell")
	}
	if _, floor, _ := g.thingSupportState(0, g.m.Things[0]); floor != fracUnit {
		t.Fatal("blocklinks traversal skipped the decoration in the skull's new cell")
	}
}

func TestBSPNodeSideWrapsFixedCoordinateDifferenceLikeDoom(t *testing.T) {
	// Original E4M8 tic 2158: an escaped Imp fireball reaches this point.
	// Subtracting the root node's negative Y exceeds signed fixed_t and wraps.
	node := divline{x: 1048576, y: -19398656, dx: -8912896, dy: -1572864}
	if got := doomPointOnNodeSide(-140334634, 2147126461, node); got != 1 {
		t.Fatalf("original E4M8 root node side=%d, want 1", got)
	}
}

func TestLongSightInterceptPreservesOriginalFixedPointOverflow(t *testing.T) {
	// Captured from original MAP32 Nightmare tic 427, line 178. Widening
	// the denominator makes this fraction positive and incorrectly opens LOS.
	ray := divline{x: 6900224, y: 239752960, dx: 30746894, dy: 310505186}
	line := divline{x: -33554432, y: 398458880, dx: 125829120}
	if frac := interceptVector(ray, line); frac != -39674 {
		t.Fatalf("sight intercept=%d, want original fixed_t result -39674", frac)
	}
	g := &game{m: &mapdata.Map{
		Vertexes:   []mapdata.Vertex{{X: -512, Y: 6080}, {X: 1408, Y: 6080}},
		Linedefs:   []mapdata.Linedef{{V1: 0, V2: 1, Flags: mlTwoSided, SideNum: [2]int16{0, 1}}},
		Sidedefs:   []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
		Sectors:    []mapdata.Sector{{CeilingHeight: 256}, {CeilingHeight: 128}},
		Segs:       []mapdata.Seg{{StartVertex: 0, EndVertex: 1, Linedef: 0}},
		SubSectors: []mapdata.SubSector{{SegCount: 1}},
		Nodes:      []mapdata.Node{{DX: 1, ChildID: [2]uint16{0x8000, 0x8000}}},
	}}
	g.initPhysics()
	if g.actorHasLOS(ray.x, ray.y, 0, 56*fracUnit, ray.x+ray.dx, ray.y+ray.dy, 0, playerHeight) {
		t.Fatal("overflowed sight intercept must close the original slope window")
	}
}

func TestRuntimeSpawnCorpseKeepsCrushedBoundsForLaterHeightClips(t *testing.T) {
	g := newDoorTimingGame(0)
	g.opts.SkillLevel = 5
	for i := range g.m.Sectors {
		g.m.Sectors[i].CeilingHeight = 128
	}
	g.m.Things = []mapdata.Thing{{Type: 9, X: 4, Flags: skillMask}}
	g.thingHP, g.thingDead, g.thingCollected = []int{0}, []bool{true}, []bool{true}
	g.thingGibbed, g.thingGibTick = []bool{true}, []int{7}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.setThingSupportState(0, 0, 0, 128*fracUnit)
	g.p.x = 1000 * fracUnit
	i := g.appendRuntimeThing(mapdata.Thing{Type: 9, X: 4, Flags: skillMask}, false)
	g.ensureMonsterAIState()
	if len(g.thingGibbed) != len(g.m.Things) || len(g.thingGibTick) != len(g.m.Things) ||
		!g.thingGibbed[0] || g.thingGibTick[0] != 7 || g.thingGibbed[i] || g.thingGibTick[i] != -1 {
		t.Fatal("runtime spawn did not initialize crusher state while preserving older corpses")
	}
	g.thingHP[i], g.thingDead[i] = 0, true
	g.sectorFloor[0], g.sectorCeil[0] = 24*fracUnit, 26*fracUnit
	g.setThingSupportState(i, 0, 0, 128*fracUnit)
	if !g.heightClipThing(i, g.m.Things[i]) || !g.thingGibbed[i] ||
		g.thingCurrentRadius(i, g.m.Things[i]) != 0 || g.thingCurrentHeight(i, g.m.Things[i]) != 0 {
		t.Fatal("crushed runtime corpse retained its old bounds")
	}
	if !g.heightClipThing(i, g.m.Things[i]) {
		t.Fatal("crushed runtime corpse did not fit on its next height clip")
	}
	if _, _, ceil := g.thingSupportState(i, g.m.Things[i]); ceil != 128*fracUnit {
		t.Fatalf("next height clip kept adjacent ceiling=%d, want own subsector ceiling", ceil)
	}
}

func TestNewlyCrushedCorpseReclipsWithinSameTic(t *testing.T) {
	g := newDoorTimingGame(0)
	g.worldTic = 99
	g.sectorFloor[0], g.sectorCeil[0] = 24*fracUnit, 26*fracUnit
	g.p.x = 1000 * fracUnit
	g.m.Things = []mapdata.Thing{{Type: 3004, X: 4, Flags: skillMask}}
	g.thingHP, g.thingDead, g.thingCollected = []int{-1}, []bool{true}, []bool{false}
	g.thingGibbed, g.thingGibTick = []bool{false}, []int{-1}
	g.ensureMonsterAIState()
	g.setThingSupportState(0, 0, 0, 128*fracUnit)
	if !g.heightClipThing(0, g.m.Things[0]) || !g.thingGibbed[0] {
		t.Fatal("first plane did not crush the corpse")
	}
	if z, floor, ceil := g.thingSupportState(0, g.m.Things[0]); z != 24*fracUnit || floor != 24*fracUnit || ceil != 26*fracUnit {
		t.Fatalf("first clipping support=%d/%d/%d", z, floor, ceil)
	}
	// A second P_ChangeSector in this tic sees the new zero-radius body.
	// It no longer crosses the neighboring sector's line and must refresh
	// support from its own subsector rather than keeping the first opening.
	if !g.heightClipThing(0, g.m.Things[0]) {
		t.Fatal("zero-height body did not fit after its second height clip")
	}
	if z, floor, ceil := g.thingSupportState(0, g.m.Things[0]); z != 0 || floor != 0 || ceil != 128*fracUnit {
		t.Fatalf("second clipping support=%d/%d/%d, want 0/0/128 units", z, floor, ceil)
	}
}

func TestRepeatedCrusherStopPreservesDirectionForRestart(t *testing.T) {
	for _, action := range []mapdata.CeilingAction{mapdata.CeilingCrushRaise, mapdata.CeilingFastCrushRaise, mapdata.CeilingSilentCrushRaise} {
		for _, direction := range []int{-1, 1} {
			t.Run(fmt.Sprintf("%s/%d", action, direction), func(t *testing.T) {
				g := newDoorTimingGame(0)
				g.m.Linedefs[0].Tag, g.m.Sectors[0].Tag = 7, 7
				g.sectorCeil[0] = 128 * fracUnit
				ct := &ceilingThinker{order: 17, sector: 0, action: action, speed: 2 * fracUnit,
					direction: direction, bottomHeight: 8 * fracUnit, topHeight: 160 * fracUnit, crush: true}
				g.ceilings = map[int]*ceilingThinker{0: ct}
				stop := mapdata.CeilingInfo{Action: mapdata.CeilingCrushStop, UsesTag: true}
				if !g.activateCeilingLine(0, stop) || ct.direction != 0 || ct.oldDirection != direction {
					t.Fatal("first stop did not retain the moving ceiling's direction")
				}
				if g.activateCeilingLine(0, stop) || ct.oldDirection != direction {
					t.Fatal("repeated stop overwrote the saved direction or reported an activation")
				}
				if !g.activateCeilingLine(0, mapdata.CeilingInfo{Action: action, UsesTag: true}) || ct.direction != direction || g.ceilings[0] != ct || ct.order != 17 {
					t.Fatal("restart did not resume the original thinker in its saved direction")
				}
				g.tickCeiling(0, ct)
				if want := int64(128+direction*2) * fracUnit; g.sectorCeil[0] != want {
					t.Fatalf("restarted ceiling=%d want=%d", g.sectorCeil[0], want)
				}
			})
		}
	}
}

func TestFastDemonAttackAndPainUseOriginalHalvedStates(t *testing.T) {
	for _, typ := range []int16{3002, 58} {
		for _, nightmare := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/nightmare=%t", typ, nightmare), func(t *testing.T) {
				g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}},
					Things: []mapdata.Thing{{Type: typ, X: 200, Flags: skillMask}}},
					opts:    Options{SkillLevel: 4, FastMonsters: true},
					thingHP: []int{150}, thingDead: []bool{false}, thingCollected: []bool{false}, thingAggro: []bool{true},
					stats: playerStats{Health: 100}, playerMobjHealth: 100}
				if nightmare {
					g.opts.SkillLevel, g.opts.FastMonsters = 5, false
				}
				g.initPhysics()
				g.ensureMonsterAIState()
				g.setMonsterTargetPlayer(0)
				g.startMonsterAttackAnimWithMode(0, typ, false)
				if g.thingStateTics[0] != 4 || g.thingAttackTics[0] != 12 {
					t.Fatalf("attack frame/total=%d/%d, want 4/12", g.thingStateTics[0], g.thingAttackTics[0])
				}
				doomrand.Clear()
				g.damageMonsterFrom(0, 1, false, -1, 0, 0, false)
				if g.thingState[0] != monsterStatePain || g.thingStateTics[0] != 1 || g.thingPainTics[0] != 2 {
					t.Fatalf("pain state/frame/total=%d/%d/%d, want pain/1/2", g.thingState[0], g.thingStateTics[0], g.thingPainTics[0])
				}
				if state := demoTraceThingState(g, 0, typ); state != 488 {
					t.Fatalf("first fast pain frame trace state=%d, want S_SARG_PAIN (488)", state)
				}
				g.thingStatePhase[0], g.thingStateTics[0] = 1, 1
				g.syncMonsterPainTics(0, typ)
				if state := demoTraceThingState(g, 0, typ); state != 489 || g.thingPainTics[0] != 1 {
					t.Fatalf("second fast pain frame trace state/remaining=%d/%d, want 489/1", state, g.thingPainTics[0])
				}
			})
		}
	}
	doomrand.Clear()
}

func TestGenericPostAttackChaseSkipsDirectionSelectionInFastModes(t *testing.T) {
	for _, typ := range []int16{3001, 3002, 58, 3003, 69, 66, 67, 16} {
		for _, mode := range []struct {
			name string
			opts Options
			fast bool
		}{
			{"normal", Options{SkillLevel: 4}, false},
			{"uv-fast", Options{SkillLevel: 4, FastMonsters: true}, true},
			{"nightmare", Options{SkillLevel: 5}, true},
		} {
			t.Run(fmt.Sprintf("%d/%s", typ, mode.name), func(t *testing.T) {
				g := &game{m: &mapdata.Map{
					Sectors: []mapdata.Sector{{CeilingHeight: 128}},
					Things:  []mapdata.Thing{{Type: typ, X: 96, Y: 32}},
				}, opts: mode.opts, stats: playerStats{Health: 100},
					thingHP: []int{monsterSpawnHealth(typ)}, thingCollected: []bool{false},
					thingDead: []bool{false}, thingAggro: []bool{true}}
				g.initPhysics()
				g.ensureMonsterAIState()
				g.p = player{x: 256 * fracUnit, y: 128 * fracUnit, ceilz: 128 * fracUnit}
				g.thingTargetPlayer[0] = true
				g.thingState[0], g.thingStatePhase[0], g.thingStateTics[0] = monsterStateSee, 0, 1
				g.thingMoveDir[0], g.thingMoveCount[0], g.thingJustAtk[0] = monsterDirNorth, 8, true
				g.setThingSupportState(0, 0, 0, 128*fracUnit)
				doomrand.SetState(0, 42)
				defer doomrand.Clear()
				g.tickGenericMonsterState(0, g.m.Things[0])
				if g.thingJustAtk[0] || len(g.projectiles) != 0 {
					t.Fatal("post-attack chase must clear JUSTATTACKED and return without attacking")
				}
				if g.thingWorldAngle(0, g.m.Things[0]) != degToAngle(45) {
					t.Fatal("post-attack chase must still turn toward its existing movement direction")
				}
				x, y := g.thingPosFixed(0, g.m.Things[0])
				_, rng := doomrand.State()
				if mode.fast {
					if x != 96*fracUnit || y != 32*fracUnit || g.thingMoveDir[0] != monsterDirNorth || g.thingMoveCount[0] != 8 || rng != 42 {
						t.Fatalf("fast post-attack chase moved or rolled RNG: pos=%d,%d dir=%d count=%d rng=%d", x, y, g.thingMoveDir[0], g.thingMoveCount[0], rng)
					}
				} else if (x == 96*fracUnit && y == 32*fracUnit) || rng == 42 {
					t.Fatal("normal post-attack chase must choose and walk a new direction")
				}
			})
		}
	}
}

func TestBlockThingsIteratorFollowsLinksChangedByItsCallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		want []int
	}{
		{"relink-same-cell", []int{2, 1, 2, 0}},
		{"move-to-other-cell", []int{2, 1, 3}},
		{"move-off-map", []int{2, 1}},
		{"unlink-current", []int{2, 1, 0}},
		{"unlink-next", []int{2, 1}},
		{"insert-new-head", []int{2, 1, 0}},
		{"stop", []int{2, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{m: &mapdata.Map{Things: []mapdata.Thing{
				{Type: barrelThingType, X: 32}, {Type: barrelThingType, X: 48},
				{Type: barrelThingType, X: 64}, {Type: barrelThingType, X: 160},
			}}, bmapWidth: 2, bmapHeight: 1, thingBlockOrder: []int64{1, 2, 3, 4}}
			g.rebuildThingBlockmap()
			var visits []int
			changed := false
			completed := g.blockThingsIterator(0, 0, func(i int) bool {
				visits = append(visits, i)
				if len(visits) > 16 {
					t.Fatal("iterator failed to reach the end of the changed chain")
				}
				if i != 1 || changed {
					return true
				}
				changed = true
				switch tc.name {
				case "relink-same-cell":
					g.setThingPosFixed(i, 48*fracUnit, 0)
				case "move-to-other-cell":
					g.setThingPosFixed(i, 192*fracUnit, 0)
				case "move-off-map":
					g.setThingPosFixed(i, -32*fracUnit, 0)
				case "unlink-current":
					g.removeThingFromBlockCell(0, i)
					g.thingBlockCell[i] = -1
				case "unlink-next":
					g.removeThingFromBlockCell(0, 0)
					g.thingBlockCell[0] = -1
				case "insert-new-head":
					g.setThingPosFixed(3, 96*fracUnit, 0)
				case "stop":
					return false
				}
				return true
			})
			if !reflect.DeepEqual(visits, tc.want) || completed != (tc.name != "stop") {
				t.Fatalf("visits=%v completed=%t want=%v", visits, completed, tc.want)
			}
		})
	}
}

func TestSelectingHeldOrUnavailableWeaponKeepsPendingSwitch(t *testing.T) {
	for _, slot := range []int{2, 5, 0, 8} {
		t.Run(fmt.Sprint(slot), func(t *testing.T) {
			g := &game{m: &mapdata.Map{Name: "MAP08"}, stats: playerStats{Health: 100, Cells: 20},
				inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{2004: true}}}
			advanceWeaponToReady(g)
			g.selectWeaponSlot(6)
			g.tickWeaponOverlay()
			if g.weaponState != weaponStatePistolDown || g.inventory.PendingWeapon != weaponPlasma {
				t.Fatal("fixture did not begin lowering the pistol for plasma")
			}
			g.selectWeaponSlot(slot)
			if g.inventory.PendingWeapon != weaponPlasma {
				t.Fatal("ignored selection cancelled the pending plasma switch")
			}
			for tic := 0; tic < 32 && g.inventory.ReadyWeapon == weaponPistol; tic++ {
				g.tickWeaponOverlay()
			}
			if g.inventory.ReadyWeapon != weaponPlasma || g.inventory.PendingWeapon != 0 {
				t.Fatal("pending switch did not finish when the pistol reached the bottom")
			}
			g.selectWeaponSlot(2)
			if g.inventory.PendingWeapon != weaponPistol {
				t.Fatal("pistol could not be selected after plasma became the held weapon")
			}
		})
	}
}

func TestCrushingClearsCorpseSolidityBeforeItsDeathAnimationFinishes(t *testing.T) {
	g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things: []mapdata.Thing{{Type: 58, X: 32, Flags: 7}}},
		thingCollected: []bool{false}, thingHP: []int{-10}, thingDead: []bool{true},
		thingGibbed: []bool{false}, thingGibTick: []int{-1},
		opts: Options{SkillLevel: 3}, stats: playerStats{Health: 100}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.p.x = 1000 * fracUnit
	g.thingState[0], g.thingStatePhase[0], g.thingStateTics[0] = monsterStateDeath, 2, 5
	p := projectile{sourceThing: -1, radius: 6 * fracUnit, height: 8 * fracUnit}
	check := func(want bool) {
		t.Helper()
		_, missileBlocked := g.projectileThingHitAtPosition(p, 32*fracUnit, 0, 0)
		if g.actorBlockedByThings(32*fracUnit, 0, 30*fracUnit, -1, true) != want ||
			g.monsterCorpseStillSolid(0) != want || missileBlocked != want {
			t.Fatalf("corpse actor/skull/missile solidity differs from %t in state %d", want, g.thingState[0])
		}
	}
	check(true)
	g.sectorCeil[0] = 8 * fracUnit
	if !g.heightClipThing(0, g.m.Things[0]) || g.thingState[0] != monsterStateGibs ||
		g.thingCurrentHeight(0, g.m.Things[0]) != 0 || g.thingCurrentRadius(0, g.m.Things[0]) != 0 {
		t.Fatal("lowered ceiling did not crush the early death frame into zero-sized gibs")
	}
	check(false)
	// Arch-vile resurrection restores MF_SOLID without restoring ghost bounds.
	// If that ghost dies again, its early death frames are solid once more.
	g.thingState[0], g.thingStatePhase[0] = monsterStateDeath, 0
	check(true)
}

func TestCrusherBloodStopsOnSolidActorsDespiteNoBlockmapFlag(t *testing.T) {
	for _, obstacle := range []string{"victim", "player", "crushed-victim"} {
		t.Run(obstacle, func(t *testing.T) {
			g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}},
				Things: []mapdata.Thing{{Type: 58, Flags: 7}}},
				thingCollected: []bool{false}, thingHP: []int{150}, thingDead: []bool{false},
				thingGibbed: []bool{false}, opts: Options{SkillLevel: 3}, stats: playerStats{Health: 100}}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p.x = 1000 * fracUnit
			if obstacle == "player" {
				g.setThingPosFixed(0, 1000*fracUnit, 0)
				g.p.x = 0
			} else if obstacle == "crushed-victim" {
				g.thingHP[0], g.thingDead[0], g.thingGibbed[0] = -10, true, true
				g.thingState[0] = monsterStateGibs
			}
			doomrand.Clear()
			g.spawnCrusherBlood(0, 0, 28*fracUnit)
			p := &g.hitscanPuffs[0]
			dx, dy := p.momx, p.momy
			_, before := doomrand.State()
			if dx == 0 || dy == 0 || !g.tickHitscanPuff(p) {
				t.Fatal("fixture did not produce a surviving blood effect with XY movement")
			}
			if obstacle == "crushed-victim" {
				if p.x != dx || p.y != dy || p.momx != dx || p.momy != dy {
					t.Fatal("non-solid crushed corpse blocked the blood")
				}
			} else if p.x != 0 || p.y != 0 || p.momx != 0 || p.momy != 0 {
				t.Fatal("blood moved through the solid victim/player instead of stopping")
			}
			if _, after := doomrand.State(); after != before {
				t.Fatal("blood collision consumed gameplay RNG")
			}
		})
	}
}

func TestRevenantFistWindupAndImpactEachFaceInvisibleTargetOnce(t *testing.T) {
	for _, phase := range []int{1, 2} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}},
				Things: []mapdata.Thing{{Type: 66, X: 200, Flags: 7}}},
				thingCollected: []bool{false}, thingHP: []int{300}, thingDead: []bool{false},
				stats: playerStats{Health: 100}, inventory: playerInventory{InvisTics: 100}}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.setMonsterTargetPlayer(0)
			g.thingAttackPhase[0] = phase
			doomrand.Clear()
			g.runMonsterAttackPhaseEntry(0, 66, phase, 200*fracUnit, 0, 0, 0, 200*fracUnit)
			// Original A_FaceTarget uses (8-109)<<21 on the due-west angle.
			if _, rng := doomrand.State(); rng != 2 || g.thingAngleState[0] != 1935671295 || g.stats.Health != 100 {
				t.Fatalf("missed fist phase %d must face once without rolling damage: rng=%d angle=%d health=%d", phase, rng, g.thingAngleState[0], g.stats.Health)
			}
		})
	}
}

func TestBloodEffectsPreserveMovementAndThinkerOrderInBinarySnapshots(t *testing.T) {
	want := []hitscanPuff{{x: fracUnit, y: 2 * fracUnit, z: 3 * fracUnit,
		momx: 4 * fracUnit, momy: -5 * fracUnit, momz: fracUnit,
		floorz: -16 * fracUnit, ceilz: 128 * fracUnit, lastLook: 3,
		tics: 6, state: 90, totalTic: 8, kind: hitscanFxBlood, hidden: true, order: 17}}
	for _, tc := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		blob, err := encodeSnapshot(tc.magic, saveFile{Version: tc.version,
			Game: gameSaveState{HitscanPuffs: captureHitscanPuffs(want)}})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(blob, tc.magic)
		if err != nil {
			t.Fatal(err)
		}
		if got := restoreHitscanPuffs(file.Game.HitscanPuffs); !reflect.DeepEqual(got, want) {
			t.Fatalf("snapshot lost blood effect state: got=%+v want=%+v", got, want)
		}
	}
}

func TestChainsawHitForcesOnlyTheNextMovementCommand(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 3001, X: 48}},
	}, thingHP: []int{60}, thingCollected: []bool{false}, stats: playerStats{Health: 100},
		inventory: playerInventory{ReadyWeapon: weaponChainsaw}}
	g.initPhysics()
	g.ensureMonsterAIState()
	doomrand.Clear()
	if !g.fireChainsaw() || !g.p.justAttacked || g.demoTracePlayerMobjFlags()&demoTraceFlagJustAtk == 0 {
		t.Fatal("a chainsaw hit must set the player's JUSTATTACKED latch")
	}
	angle := g.p.angle
	g.setThingPosFixed(0, 1000*fracUnit, 0)
	g.runGameplayTic(moveCmd{forward: -50, side: 40, turnRaw: int64(doomAng90), turn: 1}, false, false)
	if g.p.justAttacked || g.p.angle != angle || g.currentMoveCmd.forward != 100 ||
		g.currentMoveCmd.side != 0 || g.currentMoveCmd.turnRaw != 0 || g.currentMoveCmd.turn != 0 {
		t.Fatal("the next player think did not consume the hit and replace movement with forced forward thrust")
	}
	if g.p.momx <= 50*2048 {
		t.Fatal("chainsaw lunge used ordinary running speed instead of forwardmove=100")
	}
	g.runGameplayTic(moveCmd{turnRaw: int64(doomAng90)}, false, false)
	if g.p.angle != angle+doomAng90 || g.currentMoveCmd.forward != 0 {
		t.Fatal("chainsaw lunge lasted beyond one movement command")
	}
	g.p.x, g.p.y, g.p.angle = -1000*fracUnit, 0, 0
	if g.fireChainsaw() || g.p.justAttacked {
		t.Fatal("a chainsaw miss must not force the next movement command")
	}
}

func TestChainsawLungeSurvivesBinarySnapshotsAndExpiresDuringTeleportDelay(t *testing.T) {
	for _, tc := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		original := player{justAttacked: true, reactionTime: 2, ceilz: 128 * fracUnit}
		blob, err := encodeSnapshot(tc.magic, saveFile{Version: tc.version,
			Game: gameSaveState{Player: capturePlayerSaveState(original)}})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(blob, tc.magic)
		if err != nil {
			t.Fatal(err)
		}
		g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}}, stats: playerStats{Health: 100}}
		g.initPhysics()
		g.p = restorePlayerSaveState(file.Game.Player)
		if !g.p.justAttacked {
			t.Fatal("snapshot lost the pending chainsaw lunge")
		}
		g.runGameplayTic(moveCmd{side: 40, turnRaw: int64(doomAng90)}, false, false)
		if g.p.justAttacked || g.p.angle != 0 || g.p.momx != 0 || g.p.momy != 0 || g.p.reactionTime != 1 {
			t.Fatal("teleport delay must consume the latch without applying movement")
		}
	}
}

func TestChainsawRangeAddsOneFixedQuantum(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 3004, X: 64, Y: 1}},
	}, thingHP: []int{20}, thingCollected: []bool{false}}
	g.initPhysics()
	g.ensureMonsterAIState()
	// Cancel the deterministic saw spread, leaving its attack ray at angle 0.
	doomrand.Clear()
	_ = doomrand.PRandom()
	g.p.angle = 0 - addDoomAngleSpread(0, doomGunSpreadShift)
	doomrand.Clear()
	if g.fireChainsaw() || g.thingHP[0] != 20 || g.p.justAttacked {
		t.Fatal("saw reached a target beyond MELEERANGE+1 fixed quantum")
	}
	// The same geometry is hittable with the old erroneous 65-unit range.
	if hit, _, _ := g.fireMeleeAtAngle(0, 65*fracUnit, 1); !hit {
		t.Fatal("fixture must distinguish a full map unit from a fixed quantum")
	}
}

func TestBSPUsesNodeRoundingAtPartitionBoundary(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Nodes:      []mapdata.Node{{DX: 3, DY: 5, ChildID: [2]uint16{0x8000, 0x8001}}},
			SubSectors: []mapdata.SubSector{{}, {}},
			Sectors:    []mapdata.Sector{{}, {}},
		},
		subSectorSec: []int{0, 1},
	}
	x, y := int64(fracUnit), int64(109227)
	if got := g.subSectorAtFixed(x, y); got != 1 {
		t.Fatalf("boundary subsector=%d want=1", got)
	}
	if got := g.sectorAt(x, y); got != 1 {
		t.Fatalf("boundary sector=%d want=1", got)
	}
	if got := doomPointOnDivlineSide(x, y, divline{dx: 3 * fracUnit, dy: 5 * fracUnit}); got != 0 {
		t.Fatalf("path traversal must retain its distinct rounding: side=%d", got)
	}
}

func TestSlideRayUsesNudgedOriginAtBlockBoundary(t *testing.T) {
	// E4M6 tic 4992: the trailing corner starts on line 1305 at y=-64,
	// a block boundary. P_PathTraverse moves it to y=-63 before collecting
	// intercepts, so this northward ray never hits that horizontal wall.
	cells := make([][]int16, 35*23)
	cells[6*35+23] = []int16{1305}
	g := &game{
		m:           &mapdata.Map{BlockMap: &mapdata.BlockMap{Cells: cells}},
		bmapOriginX: -1072 * fracUnit, bmapOriginY: -832 * fracUnit,
		bmapWidth: 35, bmapHeight: 23,
		lines: []physLine{{idx: 1305, x1: 1920 * fracUnit, y1: -64 * fracUnit,
			x2: 1760 * fracUnit, y2: -64 * fracUnit, dx: -160 * fracUnit,
			slope: slopeHorizontal, sideNum0: 1882, sideNum1: -1}},
		physForLine: make([]int, 1306),
		p:           player{x: 123731968, y: -3145728},
	}
	if frac, line, hit := g.firstBlockingIntercept(122683392, -4194304, 122583001, -4109936); hit {
		t.Fatalf("slide ray hit line=%d frac=%d; original finds no blocking line", line, frac)
	}
}

func TestDiagonalSlideUsesOriginalLineSideRounding(t *testing.T) {
	// MAP13 line 455: the original P_HitSlideLine points southwest here.
	// The divline test instead reports the opposite side and points northeast.
	line := physLine{x1: -1152 * fracUnit, y1: -2296 * fracUnit,
		dx: -32 * fracUnit, dy: -32 * fracUnit, slope: slopePositive}
	g := &game{p: player{x: -74448122, y: -149419943}}
	if g.pointOnLineSide(g.p.x, g.p.y, line) != 0 ||
		doomPointOnDivlineSide(g.p.x, g.p.y, divline{x: line.x1, y: line.y1, dx: line.dx, dy: line.dy}) != 1 {
		t.Fatal("fixture must distinguish line-side and divline rounding")
	}
	x, y := g.hitSlideLine(line, 0, -29954)
	if x != -14977 || y != -14966 {
		t.Fatalf("slide vector=(%d,%d) want=(-14977,-14966), observed in original Doom", x, y)
	}
}

func TestMissilesDoNotExpireAfterFormerFlightTimeout(t *testing.T) {
	for _, kind := range []projectileKind{projectileFireball, projectilePlasmaBall,
		projectileBaronBall, projectileFatShot, projectileRocket,
		projectilePlayerPlasma, projectileBFGBall} {
		for _, legacyTTL := range []int{0, 1, 8 * doomTicsPerSecond, 10 * doomTicsPerSecond} {
			t.Run(fmt.Sprintf("kind%d/ttl%d", kind, legacyTTL), func(t *testing.T) {
				g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}}}
				g.initPhysics()
				p := projectile{x: 100 * fracUnit, z: 32 * fracUnit,
					vx: 20 * fracUnit, radius: 6 * fracUnit, height: 8 * fracUnit,
					floorz: 0, ceilz: 128 * fracUnit, kind: kind, ttl: legacyTTL,
					sourcePlayer: true, sourceThing: -1}
				doomrand.Clear()
				const flightTics = 400
				for tic := 0; tic < flightTics; tic++ {
					next, keep := g.advanceProjectile(p)
					if !keep {
						t.Fatalf("unobstructed missile expired after %d tics", tic+1)
					}
					p = next
				}
				if p.x != (100+20*flightTics)*fracUnit || len(g.projectileImpacts) != 0 {
					t.Fatal("missile stopped moving or spawned a timeout explosion")
				}
				if _, rng := doomrand.State(); rng != 0 {
					t.Fatalf("unobstructed flight consumed gameplay RNG: %d", rng)
				}
			})
		}
	}
}

func TestMissileCoordinatesWrapBeforeSupportLookup(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Nodes:      []mapdata.Node{{DY: 1, ChildID: [2]uint16{0x8000, 0x8001}}},
		SubSectors: []mapdata.SubSector{{}, {}},
		Sectors:    []mapdata.Sector{{CeilingHeight: 256}, {CeilingHeight: 128}},
	}}
	g.initPhysics()
	g.subSectorSec = []int{0, 1}
	// MAP13's long-traveling Mancubus shot at tic 3,805 wraps from negative
	// X to positive X. Original Doom then relinks and clips at the new point.
	p := projectile{x: -2147337765, y: -182993635, z: 32 * fracUnit,
		vx: -1309980, vy: -43720, radius: 6 * fracUnit, height: 8 * fracUnit,
		floorz: 0, ceilz: 128 * fracUnit, kind: projectileFatShot, sourceThing: -1}
	next, keep := g.advanceProjectile(p)
	if !keep || next.x != 2146319551 || next.y != -183037355 {
		t.Fatalf("wrapped missile=(%d,%d) keep=%v, want original Doom's (2146319551,-183037355)", next.x, next.y, keep)
	}
	if next.ceilz != 256*fracUnit || next.subsector != 1 {
		t.Fatalf("support lookup used the unwrapped point: ceiling=%d subsector=%d", next.ceilz, next.subsector)
	}
}

func TestTracerPuffsUseTheMostRecentAimOrShotRange(t *testing.T) {
	g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}}}
	g.initPhysics()
	for _, tc := range []struct {
		name     string
		aim      bool
		rangeVal int64
		state    int
	}{{"punch_aim", true, 64 * fracUnit, 95},
		{"bullet_shot", false, 2048 * fracUnit, 93},
		{"saw_aim", true, 64*fracUnit + 1, 93},
		{"punch_shot", false, 64 * fracUnit, 95}} {
		t.Run(tc.name, func(t *testing.T) {
			actor := g.playerLineAttackActor()
			if tc.aim {
				g.aimLineAttack(actor, 0, tc.rangeVal)
			} else {
				g.lineAttackTrace(actor, 0, tc.rangeVal, 0, true)
			}
			g.hitscanPuffs = nil
			doomrand.Clear()
			g.spawnTracerSmokeTrail(100*fracUnit, 0, 32*fracUnit, fracUnit, 0)
			puff := g.hitscanPuffs[0]
			if puff.state != tc.state || (tc.state == 95 && puff.tics != 4) {
				t.Fatalf("tracer puff state=%d tics=%d want state=%d", puff.state, puff.tics, tc.state)
			}
			if _, rng := doomrand.State(); rng != 6 {
				t.Fatalf("puff override changed tracer spawn RNG: %d want=6", rng)
			}
			flightTics := puff.tics + 12
			if tc.state == 95 {
				flightTics = 8
			}
			for tic := 1; tic <= flightTics; tic++ {
				if keep := g.tickHitscanPuff(&puff); keep != (tic < flightTics) {
					t.Fatalf("puff lifetime differs at tic %d, expected expiry at %d", tic, flightTics)
				}
			}
		})
	}
}

func TestLastAttackRangeSurvivesSnapshotsAndLevelConstruction(t *testing.T) {
	m := &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}}
	g := &game{m: m, lastAttackRange: 64 * fracUnit}
	g.initPhysics()
	for _, tc := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		blob, err := encodeSnapshot(tc.magic, saveFile{Version: tc.version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(blob, tc.magic)
		if err != nil {
			t.Fatal(err)
		}
		restored := &game{m: m}
		restored.initPhysics()
		restoreGameSaveState(restored, file.Game)
		restored.spawnTracerSmokeTrail(100*fracUnit, 0, 32*fracUnit, fracUnit, 0)
		if restored.lastAttackRange != 64*fracUnit || restored.hitscanPuffs[0].state != 95 {
			t.Fatal("snapshot lost the punch range used by the next tracer puff")
		}
	}
	opts := Options{DemoScript: &DemoScript{}, DemoQuitOnComplete: true}
	sg := &sessionGame{g: g, opts: opts,
		gameFactory: func(m *mapdata.Map, opts Options) *game { return &game{m: m, opts: opts} }}
	if next := sg.buildGame(m, opts); next.lastAttackRange != g.lastAttackRange {
		t.Fatal("level construction cleared original Doom's shared attack range")
	}
	next := &game{}
	next.inheritDemoPlayback(g)
	if next.lastAttackRange != g.lastAttackRange {
		t.Fatal("demo respawn lost the shared attack range")
	}
}

func TestRevenantZeroTicAttackFacesInvisibleTargetBeforeWindup(t *testing.T) {
	for _, missile := range []bool{false, true} {
		for _, invisible := range []bool{false, true} {
			t.Run(fmt.Sprintf("missile%v/invisible%v", missile, invisible), func(t *testing.T) {
				g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 66, X: 64}}},
					thingHP: []int{300}, thingCollected: []bool{false}, stats: playerStats{Health: 100}}
				g.initPhysics()
				g.ensureMonsterAIState()
				g.setMonsterTargetPlayer(0)
				if invisible {
					g.inventory.InvisTics = 100
				}
				doomrand.Clear()
				if !g.startMonsterAttackState(0, 66, missile) {
					t.Fatal("revenant did not start its attack")
				}
				wantRNG, wantAngle := 0, uint32(2147483647)
				if invisible {
					// Both entry actions face the target; the second random
					// pair (220,222) determines the final angle.
					wantRNG, wantAngle = 4, 2143289343
				}
				if _, rng := doomrand.State(); rng != wantRNG || g.thingAngleState[0] != wantAngle {
					t.Fatalf("attack entry RNG=%d angle=%d want=%d/%d", rng, g.thingAngleState[0], wantRNG, wantAngle)
				}
				wantState, wantTics := 336, 6
				if missile {
					wantState, wantTics = 340, 10
				}
				state, _ := demoTraceMonsterAttackState(66, g.thingAttackPhase[0])
				if state != wantState || g.thingStateTics[0] != wantTics {
					t.Fatalf("visible windup state=%d tics=%d want=%d/%d", state, g.thingStateTics[0], wantState, wantTics)
				}
			})
		}
	}
}

func TestUseRayNudgesBlockBoundaryOrigin(t *testing.T) {
	g := &game{
		m:         &mapdata.Map{BlockMap: &mapdata.BlockMap{Cells: [][]int16{{0}}}},
		bmapWidth: 1, bmapHeight: 1, physForLine: []int{0},
		lines: []physLine{{idx: 0, x1: 32 * fracUnit, y1: -16 * fracUnit,
			x2: 32 * fracUnit, y2: 16 * fracUnit, dy: 32 * fracUnit}},
	}
	intercepts := g.collectUseLineIntercepts(0, 0, 64*fracUnit, 0)
	if len(intercepts) != 1 {
		t.Fatalf("intercepts=%v want one", intercepts)
	}
	// The ray starts at (1,1), retaining its original endpoint (64,0).
	if got, want := intercepts[0].frac, fixedDiv(31*fracUnit, 63*fracUnit); got != want {
		t.Fatalf("intercept fraction=%d want=%d", got, want)
	}
}

func TestWalkCrossesAllSpecialsInReverseCollectionOrder(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Special: 36, Tag: 7}, {Special: 36, Tag: 8}},
			Sectors:  []mapdata.Sector{{Tag: 7, CeilingHeight: 128}, {Tag: 8, CeilingHeight: 128}},
		},
		lineSpecial: []uint16{36, 36}, sectorFloor: []int64{0, 0},
		sectorCeil: []int64{128 * fracUnit, 128 * fracUnit}, physForLine: []int{0, 1},
	}
	for i := 0; i < 2; i++ {
		g.lines = append(g.lines, physLine{idx: i, x1: 0, y1: 64 * fracUnit, x2: 0, y2: -64 * fracUnit,
			dy: -128 * fracUnit, slope: slopeVertical, bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 0, 0}})
	}
	g.checkWalkSpecialLinesForActorWithCandidates(-8*fracUnit, 0, 8*fracUnit, 0, -1, true, []int{0, 1})
	if g.lineSpecial[0] != 0 || g.lineSpecial[1] != 0 || len(g.floors) != 2 {
		t.Fatalf("missed crossed trigger: specials=%v floors=%v", g.lineSpecial, g.floors)
	}
	if g.floors[1].order >= g.floors[0].order {
		t.Fatal("crossed triggers were not activated in reverse collection order")
	}
}

func TestProjectileWalkSpecialRequiresDestinationBoxToOverlapLineBounds(t *testing.T) {
	for _, nearEndpoint := range []bool{false, true} {
		t.Run(fmt.Sprint(nearEndpoint), func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{
					Linedefs: []mapdata.Linedef{{Special: 88, Tag: 18}},
					Sectors:  []mapdata.Sector{{Tag: 18, FloorHeight: 128, CeilingHeight: 192}},
				},
				lineSpecial: []uint16{88}, sectorFloor: []int64{128 * fracUnit},
				sectorCeil: []int64{192 * fracUnit}, physForLine: []int{0},
				lines: []physLine{{idx: 0, x1: 816 * fracUnit, y1: -1056 * fracUnit,
					x2: 864 * fracUnit, y2: -1104 * fracUnit, dx: 48 * fracUnit,
					dy: -48 * fracUnit, slope: slopeNegative,
					bbox: [4]int64{-1056 * fracUnit, -1104 * fracUnit, 864 * fracUnit, 816 * fracUnit}}},
			}
			prevX, prevY, curX, curY := int64(58514652), int64(-73089083), int64(56926452), int64(-73491433)
			if nearEndpoint {
				prevX -= 24 * fracUnit
				curX -= 24 * fracUnit
				prevY += 24 * fracUnit
				curY += 24 * fracUnit
			}
			g.checkProjectileWalkSpecialLines(prevX, prevY, curX, curY,
				projectile{kind: projectileFatShot, radius: 13 * fracUnit})
			if got := len(g.plats) != 0; got != nearEndpoint {
				t.Fatalf("platform activated=%t want=%t", got, nearEndpoint)
			}
		})
	}
}

func TestTeleportAttemptClearsRemainingCrossedSpecials(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprint(blocked), func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{
					Linedefs: []mapdata.Linedef{{Special: 39, Tag: 7}, {Special: 39, Tag: 7}},
					Sectors:  []mapdata.Sector{{Tag: 7, CeilingHeight: 128}},
					Things:   []mapdata.Thing{{Type: 14, X: 64, Y: 64}},
				},
				lineSpecial: []uint16{39, 39}, sectorFloor: []int64{0},
				sectorCeil: []int64{128 * fracUnit}, physForLine: []int{0, 1},
				p: player{x: 8 * fracUnit, ceilz: 128 * fracUnit},
			}
			for i := 0; i < 2; i++ {
				g.lines = append(g.lines, physLine{idx: i, y1: 64 * fracUnit, y2: -64 * fracUnit,
					dy: -128 * fracUnit, slope: slopeVertical, bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 0, 0}})
			}
			actorIdx, isPlayer := -1, true
			if blocked {
				actorIdx, isPlayer = 1, false
				g.m.Things = append(g.m.Things, mapdata.Thing{Type: 3004, X: 8, Flags: 7})
				g.ensureMonsterAIState()
				g.p.x, g.p.y = 64*fracUnit, 64*fracUnit
				g.stats.Health = 100
			}
			g.checkWalkSpecialLinesForActorWithCandidates(-8*fracUnit, 0, 8*fracUnit, 0, actorIdx, isPlayer, []int{0, 1})
			if g.lineSpecial[0] != 39 || g.lineSpecial[1] != 0 {
				t.Fatalf("teleport did not clear remaining spechit: %v", g.lineSpecial)
			}
			wantFog := 2
			if blocked {
				wantFog = 0
			}
			if len(g.hitscanPuffs) != wantFog {
				t.Fatalf("fog count=%d want=%d", len(g.hitscanPuffs), wantFog)
			}
		})
	}
}

func TestNoBlockmapDecorationsKeepTheirSpawnHeight(t *testing.T) {
	for _, typ := range []int16{14, 87, 89, 79, 80, 81} {
		t.Run(fmt.Sprint(typ), func(t *testing.T) {
			g := &game{
				m:           &mapdata.Map{Sectors: []mapdata.Sector{{}}, Things: []mapdata.Thing{{Type: typ, X: 32, Y: 32}}},
				sectorFloor: []int64{-76 * fracUnit}, sectorCeil: []int64{128 * fracUnit},
				thingX: []int64{32 * fracUnit}, thingY: []int64{32 * fracUnit},
				thingZState: []int64{-72 * fracUnit}, thingFloorState: []int64{-72 * fracUnit},
				thingCeilState: []int64{128 * fracUnit}, thingSupportValid: []bool{true},
				bmapWidth: 1, bmapHeight: 1, thingBlockCells: make([][]int, 1), thingBlockCell: []int{-1},
			}
			g.rebuildThingBlockmap()
			if g.thingBlockCell[0] != -1 || len(g.thingBlockCells[0]) != 0 {
				t.Fatal("MF_NOBLOCKMAP decoration entered the blockmap")
			}
			g.heightClipThing(0, g.m.Things[0])
			if g.thingZState[0] != -72*fracUnit || g.thingFloorState[0] != -72*fracUnit {
				t.Fatal("moving sector changed an unlinked decoration's support")
			}
		})
	}
}

func TestHeightClipRetainsOpeningBeforeBlockingLine(t *testing.T) {
	g := newDoorTimingGame(0)
	g.sectorFloor[0], g.sectorCeil[0] = 24*fracUnit, 26*fracUnit
	g.p.x = 1000 * fracUnit
	g.m.Things = []mapdata.Thing{{Type: 3004, X: 4}}
	g.ensureMonsterAIState()
	g.setThingSupportState(0, 0, 0, 128*fracUnit)
	g.lines = append(g.lines, physLine{idx: 1, x1: 8 * fracUnit, y1: -64 * fracUnit,
		x2: 8 * fracUnit, y2: 64 * fracUnit, dy: 128 * fracUnit,
		sideNum0: 0, sideNum1: -1, slope: slopeVertical,
		bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 8 * fracUnit, 8 * fracUnit}})
	if g.heightClipThing(0, g.m.Things[0]) {
		t.Fatal("monster fit in a two-unit opening")
	}
	if z, floor, ceil := g.thingSupportState(0, g.m.Things[0]); z != 24*fracUnit || floor != 24*fracUnit || ceil != 26*fracUnit {
		t.Fatalf("partial opening lost: z/floor/ceil=%d/%d/%d", z, floor, ceil)
	}
	g.m.Things = nil
	g.p.x, g.p.z, g.p.floorz, g.p.ceilz = 4*fracUnit, 0, 0, 128*fracUnit
	if g.heightClipPlayer(0) || g.p.floorz != 24*fracUnit || g.p.ceilz != 26*fracUnit || g.p.z != 24*fracUnit {
		t.Fatalf("player lost partial opening: %+v", g.p)
	}
}

func TestMissileSkyOpeningSurvivesLaterSolidWall(t *testing.T) {
	for _, sky := range []bool{false, true} {
		t.Run(fmt.Sprint(sky), func(t *testing.T) {
			g := newDoorTimingGame(0)
			g.sectorCeil[0] = 56 * fracUnit
			if sky {
				g.m.Sectors[0].CeilingPic = "F_SKY1"
			}
			g.lines = append(g.lines, physLine{idx: 1, x1: 8 * fracUnit, y1: -64 * fracUnit,
				x2: 8 * fracUnit, y2: 64 * fracUnit, dy: 128 * fracUnit,
				sideNum0: 0, sideNum1: -1, slope: slopeVertical,
				bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 8 * fracUnit, 8 * fracUnit}})
			p := projectile{x: 22 * fracUnit, z: 24 * fracUnit, vx: -18 * fracUnit,
				radius: 6 * fracUnit, height: 8 * fracUnit, ttl: 10, sourcePlayer: true, sourceThing: -1,
				frameTics: 4, kind: projectileFireball}
			doomrand.Clear()
			_, keep := g.advanceProjectile(p)
			_, rng := doomrand.State()
			want := 1
			if sky {
				want = 0
			}
			if keep || len(g.projectileImpacts) != want || rng != want {
				t.Fatalf("keep=%t impacts=%d RNG=%d want wall impact=%d", keep, len(g.projectileImpacts), rng, want)
			}
		})
	}
}

func TestRaiseToTextureUsesBothSidesAndDoesNotClampDestination(t *testing.T) {
	for _, tagged := range []bool{false, true} {
		t.Run(fmt.Sprint(tagged), func(t *testing.T) {
			g := newDoorTimingGame(0)
			g.m.Linedefs[0].Tag = 7
			g.m.Sectors[0].Tag = 7
			g.m.Sidedefs[0].Bottom, g.m.Sidedefs[1].Bottom = "TALL", "SHORT"
			g.opts.WallTextureHeights = map[string]int{"TALL": 128, "SHORT": 64}
			g.sectorFloor[0], g.sectorCeil[0] = 232*fracUnit, 256*fracUnit
			if tagged {
				g.activateTaggedFloor(7, mapdata.FloorRaiseToTexture)
			} else {
				g.activateFloorLine(0, mapdata.FloorInfo{Action: mapdata.FloorRaiseToTexture})
			}
			if got := g.floors[0].destHeight; got != 296*fracUnit {
				t.Fatalf("destination=%d want=%d", got, 296*fracUnit)
			}
		})
	}
}

func TestRaiseAndChangePlatsFinishWithoutWaiting(t *testing.T) {
	for _, tc := range []struct {
		action mapdata.PlatAction
		height int64
	}{{mapdata.PlatRaiseAndChange24, 24}, {mapdata.PlatRaiseAndChange32, 32}} {
		t.Run(string(tc.action), func(t *testing.T) {
			g := newDoorTimingGame(0)
			g.p.x = 1000 * fracUnit
			g.m.Linedefs[0].Tag, g.m.Sectors[0].Tag = 7, 7
			g.activatePlatLine(0, mapdata.PlatInfo{Action: tc.action})
			if g.plats[0].wait != 0 {
				t.Fatal("raise-and-change platform has a wait timer")
			}
			for tic := int64(0); tic < tc.height*2; tic++ {
				g.tickPlats()
			}
			if g.sectorFloor[0] != tc.height*fracUnit || g.plats[0] == nil {
				t.Fatal("platform completed before stepping past the destination")
			}
			g.tickPlats()
			if g.plats[0] != nil {
				t.Fatal("raise-and-change platform waited at its destination")
			}
		})
	}
}

func TestBlockedFloorRollsBackAndClipsSupportAgain(t *testing.T) {
	g := newDoorTimingGame(1)
	g.p.x = -1000 * fracUnit
	g.sectorFloor[1], g.sectorCeil[1] = 0, 56*fracUnit
	g.m.Things = []mapdata.Thing{{Type: 3004, X: 32, Flags: 7}, {Type: 5, X: 48, Flags: 7}}
	g.thingCollected, g.thingHP = []bool{false, false}, []int{20, 1000}
	g.ensureMonsterAIState()
	for i := range g.m.Things {
		g.setThingSupportState(i, 0, 0, 56*fracUnit)
	}
	ft := &floorThinker{direction: 1, speed: fracUnit, destHeight: 24 * fracUnit}
	g.floors = map[int]*floorThinker{1: ft}
	g.tickFloor(1, ft)
	if g.sectorFloor[1] != 0 || g.floors[1] != ft {
		t.Fatal("blocked floor advanced or completed")
	}
	for i := range g.m.Things {
		if z, floor, _ := g.thingSupportState(i, g.m.Things[i]); z != 0 || floor != 0 {
			t.Fatalf("thing %d retained rejected floor support: z/floor=%d/%d", i, z, floor)
		}
	}
}

func TestLowerToLowestDoesNotRaiseFloorToHigherNeighbor(t *testing.T) {
	g := newDoorTimingGame(0)
	g.sectorFloor[0], g.sectorFloor[1] = 0, 48*fracUnit
	if got := g.findLowestFloorSurrounding(0); got != 0 {
		t.Fatalf("lowest floor=%d want=0", got)
	}
}

func TestWolfSSRefireRunsOnEntryAndLoopsToAttack2(t *testing.T) {
	doomrand.Clear()
	g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 84, X: 64}}},
		thingHP: []int{50}, thingAttackTics: []int{1}, thingAttackPhase: []int{4},
		thingState: []monsterThinkState{monsterStateAttack}, thingStateTics: []int{1}}
	g.ensureMonsterAIState()
	if !g.tickMonsterAttackState(0, 84, 64*fracUnit, 0, 0, 0, 64*fracUnit) {
		t.Fatal("low refire roll should retain attack even without a target")
	}
	_, play := doomrand.State()
	if g.thingAttackPhase[0] != 5 || play != 1 {
		t.Fatal("SS did not execute A_CPosRefire on entering ATK6")
	}
	if !g.tickMonsterAttackState(0, 84, 64*fracUnit, 0, 0, 0, 64*fracUnit) || g.thingAttackPhase[0] != 1 {
		t.Fatal("SS did not loop to ATK2")
	}
}

func TestClose30ThenOpenReturnsToOriginalCeiling(t *testing.T) {
	g := newDoorTimingGame(1)
	g.sectorCeil[1] = 120 * fracUnit
	g.activateDoorSectors([]int{1}, mapdata.DoorClose30ThenOpen)
	if got := g.doors[1].topHeight; got != 120*fracUnit {
		t.Fatalf("reopen height=%d want=%d", got, 120*fracUnit)
	}
}

func TestDeadLostSoulMapThingExpiresAtSpawnCountdown(t *testing.T) {
	g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 23}}},
		thingCollected: []bool{false}, thingStateTics: []int{2}}
	g.ensureMonsterAIState()
	g.runOrderedWorldThinkers()
	if g.thingCollected[0] {
		t.Fatal("corpse removed before its countdown expired")
	}
	g.runOrderedWorldThinkers()
	if !g.thingCollected[0] {
		t.Fatal("S_SKULL_DIE6 did not remove the corpse on entering S_NULL")
	}
}

func TestLightFlashDoesNotRaiseItsMinimumToBrighterNeighbors(t *testing.T) {
	g := newDoorTimingGame(0)
	g.m.Sectors[0].Light, g.m.Sectors[1].Light = 128, 192
	g.sectorLightFx = make([]sectorLightEffect, 2)
	g.spawnSectorLightFlash(0)
	if got := g.sectorLightFx[0].minLight; got != 128 {
		t.Fatalf("minimum light=%d want=128", got)
	}
	g.sectorLightFx[0].count = 1
	g.tickSectorLightEffect(0)
	if g.m.Sectors[0].Light != 128 {
		t.Fatal("flashing light brightened to its neighbor's level")
	}
	g.spawnSectorFireFlicker(0)
	if got := g.sectorLightFx[0].minLight; got != 144 {
		t.Fatalf("fire flicker minimum=%d want=144", got)
	}
}

func TestBurntTreeAndCommanderKeenBlockActorMovement(t *testing.T) {
	for _, typ := range []int16{43, 72} {
		t.Run(fmt.Sprint(typ), func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{Things: []mapdata.Thing{{Type: typ, Flags: 7}}},
				p: player{x: 1000 * fracUnit}, opts: Options{SkillLevel: 4},
			}
			radius := thingTypeRadius(typ)
			if !g.actorBlockedByThings(radius+playerRadius-1, 0, playerRadius, -1, true) {
				t.Fatal("MF_SOLID map thing did not block an overlapping actor")
			}
			if g.actorBlockedByThings(radius+playerRadius, 0, playerRadius, -1, true) {
				t.Fatal("actor touching the bbox boundary was blocked")
			}
		})
	}
}

func TestCrusherSlowsAndDamagesOnFourTicPulses(t *testing.T) {
	doomrand.Clear()
	g := newDoorTimingGame(1)
	g.p.x = -1000 * fracUnit
	g.sectorFloor[1], g.sectorCeil[1] = 16*fracUnit, 72*fracUnit
	g.m.Things = []mapdata.Thing{{Type: 3004, X: 32, Flags: 7}}
	g.thingCollected, g.thingHP = []bool{false}, []int{20}
	g.ensureMonsterAIState()
	g.setThingSupportState(0, 16*fracUnit, 16*fracUnit, 72*fracUnit)
	g.worldTic = 1
	ct := &ceilingThinker{action: mapdata.CeilingCrushRaise, crush: true,
		direction: -1, speed: fracUnit, bottomHeight: 24 * fracUnit, topHeight: 72 * fracUnit}
	g.tickCeiling(1, ct)
	if ct.speed != fracUnit/8 || g.sectorCeil[1] != 71*fracUnit || g.thingHP[0] != 10 {
		t.Fatalf("crusher speed/height/health=%d/%d/%d", ct.speed, g.sectorCeil[1], g.thingHP[0])
	}
	if len(g.hitscanPuffs) != 1 || g.hitscanPuffs[0].momz != 0 || g.hitscanPuffs[0].tics != 8 {
		t.Fatal("crusher did not spawn its unjittered eight-tic blood state")
	}
	if _, play := doomrand.State(); play != 6 {
		t.Fatalf("crusher RNG=%d want one pain roll and five blood-spawn rolls", play)
	}
	g.worldTic = 2
	g.tickCeiling(1, ct)
	if g.thingHP[0] != 10 || len(g.hitscanPuffs) != 1 {
		t.Fatal("crusher damaged the actor between four-tic pulses")
	}
	fx := g.hitscanPuffs[0]
	restored := restoreHitscanPuffs(captureHitscanPuffs([]hitscanPuff{fx}))[0]
	if restored.momx != fx.momx || restored.momy != fx.momy {
		t.Fatal("crusher blood lost horizontal momentum across save/load")
	}
}

func TestLowerAndCrushCeilingRollsBackWithoutCrusherDamage(t *testing.T) {
	g := newDoorTimingGame(1)
	g.p.x = -1000 * fracUnit
	g.m.Linedefs[0].Tag, g.m.Sectors[1].Tag = 7, 7
	g.sectorFloor[1], g.sectorCeil[1] = 16*fracUnit, 72*fracUnit
	g.m.Things = []mapdata.Thing{{Type: 3004, X: 32, Flags: 7}}
	g.thingCollected, g.thingHP = []bool{false}, []int{20}
	g.ensureMonsterAIState()
	g.setThingSupportState(0, 16*fracUnit, 16*fracUnit, 72*fracUnit)
	g.worldTic = 1
	g.activateCeilingLine(0, mapdata.CeilingInfo{Action: mapdata.CeilingLowerAndCrush})
	ct := g.ceilings[1]
	if ct == nil || ct.crush {
		t.Fatal("lowerAndCrush did not retain EV_DoCeiling's crush=false")
	}
	g.tickCeiling(1, ct)
	if ct.speed != fracUnit/8 || g.sectorCeil[1] != 72*fracUnit || g.thingHP[0] != 20 || len(g.hitscanPuffs) != 0 {
		t.Fatal("blocked lowerAndCrush failed to restore the ceiling or dealt crusher damage")
	}
}

func TestPlatformStopAndRestartOperateOnOccupiedSectors(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Tag: 7}},
			Sectors:  []mapdata.Sector{{Tag: 7, CeilingHeight: 128}},
		},
		sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
		plats: map[int]*platThinker{0: {sector: 0, typ: platTypePerpetualRaise,
			status: platStatusUp, speed: fracUnit, low: 0, high: 32 * fracUnit}},
	}
	pt := g.plats[0]
	if !g.activatePlatLine(0, mapdata.PlatInfo{Action: mapdata.PlatStop, UsesTag: true}) {
		t.Fatal("stop skipped an occupied sector")
	}
	g.tickPlat(0, pt)
	if pt.status != platStatusInStasis || pt.oldStatus != platStatusUp || g.sectorFloor[0] != 0 {
		t.Fatalf("platform moved in stasis: %+v floor=%d", pt, g.sectorFloor[0])
	}
	_, before := doomrand.State()
	if !g.activatePlatLine(0, mapdata.PlatInfo{Action: mapdata.PlatPerpetualRaise, UsesTag: true}) {
		t.Fatal("restart skipped an occupied sector")
	}
	_, after := doomrand.State()
	if g.plats[0] != pt || pt.status != platStatusUp || before != after {
		t.Fatal("restart must restore the existing thinker without spawning or drawing RNG")
	}
	g.tickPlat(0, pt)
	if g.sectorFloor[0] != fracUnit {
		t.Fatal("restarted platform did not resume its old direction")
	}
}

func TestMovingFloorDoesNotRetouchPickupAfterUpdatingItemHeight(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{FloorHeight: -9, CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 2012, Flags: skillMask}},
	}, thingCollected: []bool{false}, opts: Options{SkillLevel: 3}}
	g.initPhysics()
	g.initPlayerState()
	g.stats.Health = 75
	// The item starts nine units below a player on the adjacent floor. The
	// player's height clip checks it before the item rises into touch range.
	g.p = player{z: 0, floorz: 0, ceilz: 128 * fracUnit}
	g.setThingSupportState(0, -9*fracUnit, -9*fracUnit, 128*fracUnit)
	g.setSectorFloorHeight(0, -8*fracUnit)
	if g.thingCollected[0] || g.stats.Health != 75 {
		t.Fatal("floor movement retouched the item after changing its height")
	}
	g.checkPositionForWithPickupTouch(g.p.x, g.p.y, false, true)
	if !g.thingCollected[0] || g.stats.Health != 100 {
		t.Fatal("next position check did not touch the raised item")
	}
}

func TestEmptyWeaponReadyWaitsForFireAttempt(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	g.inventory.ReadyWeapon = weaponRocketLauncher
	g.weaponState = weaponInfo(weaponRocketLauncher).readystate
	g.inventory.Weapons[2003] = true
	g.stats.Rockets = 0
	g.weaponAttackDown = true
	g.weaponActionReady(weaponInfo(weaponRocketLauncher).readystate)
	if g.inventory.PendingWeapon != 0 || g.weaponAttackDown {
		t.Fatal("releasing an empty ready weapon must only clear attackdown")
	}
	g.statusAttackDown = true
	g.weaponActionReady(weaponInfo(weaponRocketLauncher).readystate)
	if g.inventory.PendingWeapon != weaponPistol {
		t.Fatal("attempting to fire an empty weapon must select the ammo fallback")
	}
}

func TestNightmareRespawnUsesOriginalSpawnPointAndGatesRNG(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprint(blocked), func(t *testing.T) {
			spawn := mapdata.Thing{Type: 66, X: 64, Y: 32, Angle: 100, Flags: skillMask | thingFlagAmbush}
			g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}, Things: []mapdata.Thing{spawn}},
				opts:           Options{RespawnMonsters: true, SkillLevel: 4},
				thingCollected: []bool{false}, thingDropped: []bool{false},
				thingHP: []int{-1}, thingSpawnPoint: []mapdata.Thing{spawn},
				thingThinkerOrder: []int64{1}, nextThinkerOrder: 2,
			}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p.x = 1000 * fracUnit
			if blocked {
				g.p.x, g.p.y = 64*fracUnit, 32*fracUnit
			}
			g.thingDead[0] = true
			g.thingState[0] = monsterStateDeath
			g.thingStateTics[0] = -1
			g.thingStatePhase[0] = 5
			g.setThingPosFixed(0, 96*fracUnit, 32*fracUnit)
			g.setThingSupportState(0, 0, 0, 128*fracUnit)
			doomrand.SetState(0, 65) // next P_Random is 4: an eligible respawn.
			g.worldTic = 321         // leveltime 320, divisible by 32.
			g.thingMoveCount[0] = 418
			g.tickNightmareRespawn(0, spawn)
			_, rng := doomrand.State()
			if rng != 65 || g.thingMoveCount[0] != 419 {
				t.Fatal("respawn drew RNG before twelve seconds")
			}
			g.worldTic = 322
			g.tickNightmareRespawn(0, spawn)
			_, rng = doomrand.State()
			if rng != 65 {
				t.Fatal("respawn drew RNG outside the 32-tic pulse")
			}
			g.worldTic = 353
			g.tickNightmareRespawn(0, spawn)
			_, rng = doomrand.State()
			if blocked {
				if len(g.m.Things) != 1 || len(g.hitscanPuffs) != 0 || rng != 66 || g.thingCollected[0] {
					t.Fatal("blocked respawn spawned effects or removed the corpse")
				}
				return
			}
			if len(g.m.Things) != 2 || !g.thingCollected[0] || len(g.hitscanPuffs) != 2 || rng != 69 {
				t.Fatal("respawn did not create two fogs and one monster in RNG order")
			}
			if g.thingX[1] != 64*fracUnit || g.thingY[1] != 32*fracUnit || g.thingAngleState[1] != degToAngle(90) || g.thingReactionTics[1] != 18 || g.thingHP[1] != 300 || g.thingSpawnPoint[1] != spawn {
				t.Fatal("respawn lost the original map spawn attributes")
			}
			if g.hitscanPuffs[0].x != 96*fracUnit || g.hitscanPuffs[1].x != 64*fracUnit {
				t.Fatal("respawn fogs used the wrong positions")
			}
		})
	}
}

func TestMonsterSpawnPointsSurviveBinarySnapshots(t *testing.T) {
	want := []mapdata.Thing{{Type: 66, X: -32, Y: 64, Angle: 100, Flags: 15}, {}}
	for _, tc := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		blob, err := encodeSnapshot(tc.magic, saveFile{Version: tc.version, Game: gameSaveState{ThingSpawnPoint: want}})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(blob, tc.magic)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(file.Game.ThingSpawnPoint, want) {
			t.Fatal("binary snapshot lost respawn provenance")
		}
	}
}

func TestTaggedDoorSwitchClearsOnlyOnSuccessfulActivation(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Linedefs: []mapdata.Linedef{{Special: 103, Tag: 7, SideNum: [2]int16{0, -1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}}, Sectors: []mapdata.Sector{{Tag: 7, CeilingHeight: 128}},
	}, lineSpecial: []uint16{103}, sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
		doors: map[int]*doorThinker{0: {sector: 0, direction: 1}},
	}
	if g.useSpecialLineForActor(0, 0, true) || g.lineSpecial[0] != 103 {
		t.Fatal("busy door must preserve its switch trigger")
	}
	delete(g.doors, 0)
	if !g.useSpecialLineForActor(0, 0, true) || g.lineSpecial[0] != 0 {
		t.Fatal("successful tagged door switch must clear its one-shot trigger")
	}
}

func TestRefirePreservesHeldAttackThroughRocketWeaponSwitch(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	g.inventory.Weapons[2003] = true
	g.stats.Rockets = 5
	g.inventory.PendingWeapon = weaponRocketLauncher
	g.statusAttackDown, g.weaponAttackDown = true, true
	g.weaponActionRefire(weaponStatePistolAtk4)
	if !g.weaponAttackDown {
		t.Fatal("A_ReFire cleared attackdown during the pending switch")
	}
	g.inventory.ReadyWeapon, g.inventory.PendingWeapon = weaponRocketLauncher, 0
	g.weaponState = weaponStateRocketReady
	g.weaponActionReady(weaponStateRocketReady)
	if g.weaponState != weaponStateRocketReady {
		t.Fatal("raising a rocket launcher fired without a fresh attack press")
	}
}

func TestBaronDamageFrameCanMeleeItsDeadTarget(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 3003}, {Type: 3001, X: 48}},
	}, thingHP: []int{1000, -5}, thingCollected: []bool{false, false}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.thingDead[1] = true
	g.setMonsterTargetThing(0, 1)
	doomrand.Clear()
	if !g.monsterAttack(0, 3003, 48*fracUnit) {
		t.Fatal("retained target did not reach the baron's damage frame")
	}
	_, rng := doomrand.State()
	if rng != 1 || len(g.projectiles) != 0 || g.thingHP[1] != -5 {
		t.Fatalf("dead target must receive a harmless melee swing: rng=%d projectiles=%d hp=%d", rng, len(g.projectiles), g.thingHP[1])
	}
}

func TestDemoIntermissionUsesButtonEdgesWithoutStartupDelay(t *testing.T) {
	g := &game{demoIntermissionActive: true, useButtonDown: true, weaponAttackDown: true}
	script := &DemoScript{Tics: []DemoTic{
		{Buttons: demoButtonUse | demoButtonAttack}, {},
		{Buttons: demoButtonUse}, {Buttons: demoButtonUse}, {},
		{Buttons: demoButtonAttack},
	}}
	sg := &sessionGame{g: g, intermission: sessionIntermission{state: intermissionState{
		Active: true, Commercial: true, SPState: 1, PauseTics: 35,
	}}}
	for i := range script.Tics {
		if err := g.updateDemoIntermission(script); err != nil {
			t.Fatal(err)
		}
		if sg.tickIntermission() {
			t.Fatal("demo must defer map loading to the next tic")
		}
		switch i {
		case 0, 1:
			if sg.intermission.state.SPState != 1 {
				t.Fatal("held exit input skipped the stats")
			}
		case 2, 3, 4:
			if sg.intermission.state.SPState != 10 || sg.intermission.state.Screen != intermissionScreenStats {
				t.Fatal("first fresh press must reveal stats without repeating while held")
			}
		case 5:
			if sg.intermission.state.Screen != intermissionScreenNoState {
				t.Fatal("second fresh press must start the map-load countdown")
			}
		}
	}
	for i := 0; i < 10; i++ {
		sg.tickIntermission()
		if g.demoWorldDone != (i == 9) {
			t.Fatalf("world-done action queued on countdown tic %d", i)
		}
	}
}

func TestKeenPainAndLastDeathOpenTaggedDoor(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128, Tag: 666}},
		Things:  []mapdata.Thing{{Type: 72}, {Type: 72, X: 64}},
	}, thingHP: []int{100, 100}, thingCollected: []bool{false, false}, thingDead: []bool{false, false}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.thingStateTics[0], g.thingStateTics[1] = -1, -1
	g.setThingSupportState(0, 56*fracUnit, 0, 128*fracUnit)
	g.setThingSupportState(1, 56*fracUnit, 0, 128*fracUnit)
	doomrand.Clear()
	g.tickThingThinker(0, g.m.Things[0])
	if z, _, _ := g.thingSupportState(0, g.m.Things[0]); z != 56*fracUnit || g.thingMomZ[0] != 0 {
		t.Fatal("idle Keen must hang from the ceiling without gravity")
	}
	g.damageShootableThing(0, 3)
	_, rng := doomrand.State()
	if g.thingHP[0] != 97 || rng != 1 || g.thingStateTics[0] != 4 {
		t.Fatal("Keen must be shootable and enter its four-tic pain frame")
	}
	for i := 0; i < 12; i++ {
		g.tickThingThinker(0, g.m.Things[0])
	}
	if g.thingState[0] != monsterStateSpawn || g.thingStateTics[0] != -1 || g.thingMomZ[0] != 0 {
		t.Fatal("Keen pain must return to its infinite hanging state")
	}
	for target := 0; target < 2; target++ {
		g.damageShootableThing(target, 1000)
		if g.thingCurrentHeight(target, g.m.Things[target]) != 18*fracUnit {
			t.Fatal("Keen death must quarter the actor height")
		}
		for g.thingStatePhase[target] < 10 {
			g.tickThingThinker(target, g.m.Things[target])
			if g.thingStatePhase[target] < 10 && len(g.doors) != 0 {
				t.Fatal("Keen door opened before S_COMMKEEN11")
			}
		}
		if g.monsterCorpseStillSolid(target) {
			t.Fatal("A_KeenDie must clear corpse solidity")
		}
		if (len(g.doors) == 1) != (target == 1) {
			t.Fatal("tag 666 must open only after the last living Keen dies")
		}
	}
}

func TestTinyCorpseThrustStillSnapsToRaisedFloor(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{FloorHeight: 24, CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 72}},
	}, thingHP: []int{-20}, thingCollected: []bool{false}, thingDead: []bool{false}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.thingDead[0], g.thingState[0], g.thingStateTics[0] = true, monsterStateDeath, 5
	g.setThingSupportState(0, 16*fracUnit, 16*fracUnit, 128*fracUnit)
	g.setThingMomentum(0, -4, 8, 0)
	g.p.x = 1000 * fracUnit
	g.tickThingThinker(0, g.m.Things[0])
	z, floor, _ := g.thingSupportState(0, g.m.Things[0])
	if z != 24*fracUnit || floor != z || g.thingMomX[0] != 0 || g.thingMomY[0] != 0 {
		t.Fatal("stopping tiny thrust skipped the new floor's vertical adjustment")
	}
}

func TestDamageWakeChecksMeleeBeforeMissileRNG(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 3001}, {Type: 3006, X: 32}},
	}, thingHP: []int{60, 100}, thingCollected: []bool{false, false}, thingDead: []bool{false, false}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.thingState[0], g.thingStatePhase[0] = monsterStateSpawn, 0
	doomrand.SetState(0, 8) // 248 fails the imp's pain roll.
	g.damageMonsterFrom(0, 3, false, 1, 32*fracUnit, 0, true)
	_, rng := doomrand.State()
	if rng != 9 || g.thingState[0] != monsterStateAttack {
		t.Fatalf("damage wake must enter melee without drawing missile RNG: rng=%d state=%d", rng, g.thingState[0])
	}
}

func TestPlatCompletionPreservesHazard(t *testing.T) {
	for _, action := range []mapdata.PlatAction{mapdata.PlatRaiseAndChange24, mapdata.PlatRaiseAndChange32, mapdata.PlatDownWaitUpStay} {
		g := &game{m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{{Tag: 7, SideNum: [2]int16{0, -1}}},
			Sidedefs: []mapdata.Sidedef{{Sector: 1}},
			Sectors: []mapdata.Sector{{Tag: 7, Special: 7, CeilingHeight: 128, FloorPic: "SLIME"},
				{CeilingHeight: 128, FloorPic: "FLOOR"}},
		}, sectorFloor: []int64{0, 0}, sectorCeil: []int64{128 * fracUnit, 128 * fracUnit}}
		if !g.activatePlatLine(0, mapdata.PlatInfo{Action: action}) {
			t.Fatal("platform did not activate")
		}
		if g.m.Sectors[0].Special != 7 || (action != mapdata.PlatDownWaitUpStay && g.m.Sectors[0].FloorPic != "FLOOR") {
			t.Fatal("EV_DoPlat must copy the texture immediately while retaining the hazard")
		}
		pt := g.plats[0]
		pt.status, pt.high = platStatusUp, g.sectorFloor[0]
		g.tickPlat(0, pt)
		if len(g.plats) != 0 || g.m.Sectors[0].Special != 7 {
			t.Fatal("platform completion cleared the sector hazard")
		}
	}
}

func TestMovingFloorTouchesPickupInBlockmapOrder(t *testing.T) {
	for _, itemFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(itemFirst), func(t *testing.T) {
			g := &game{m: &mapdata.Map{
				Sectors: []mapdata.Sector{{FloorHeight: -9, CeilingHeight: 128}},
				Things:  []mapdata.Thing{{X: 32, Y: 32, Type: 2012, Flags: skillMask}},
			}, thingCollected: []bool{false}, opts: Options{SkillLevel: 3}}
			g.initPhysics()
			g.initPlayerState()
			g.stats.Health = 75
			g.p = player{x: 32 * fracUnit, y: 32 * fracUnit, z: 0, floorz: 0, ceilz: 128 * fracUnit}
			g.bmapWidth, g.bmapHeight = 1, 1
			g.thingBlockCells = [][]int{{0}}
			g.thingBlockCell = []int{0}
			g.thingBlockOrder = []int64{1}
			g.playerBlockOrder = 2
			if itemFirst {
				g.thingBlockOrder[0], g.playerBlockOrder = 2, 1
			}
			g.sectorBBox = []worldBBox{{minX: 0, minY: 0, maxX: 128, maxY: 128}}
			g.setThingSupportState(0, -9*fracUnit, -9*fracUnit, 128*fracUnit)
			g.setSectorFloorHeight(0, -8*fracUnit)
			if g.thingCollected[0] != itemFirst {
				t.Fatalf("pickup=%v, want item-first=%v", g.thingCollected[0], itemFirst)
			}
		})
	}
}

func TestEnvironmentalDamageDoesNotInventMonsterTarget(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 69, Angle: 180}},
	}, thingHP: []int{500}, thingCollected: []bool{false}, thingDead: []bool{false},
		thingAggro: []bool{false}, opts: Options{SkillLevel: 3}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.p = player{x: 128 * fracUnit, ceilz: 128 * fracUnit}
	g.lines = []physLine{{x1: 64 * fracUnit, y1: -128 * fracUnit,
		x2: 64 * fracUnit, y2: 128 * fracUnit, dy: 256 * fracUnit}}
	g.setThingWorldAngle(0, degToAngle(180))
	g.thingMoveDir[0] = monsterDirEast
	doomrand.Clear()
	g.damageMonsterFrom(0, 5, false, -1, 0, 0, false)
	if g.thingAggro[0] || g.monsterHasTarget(0) || g.thingState[0] != monsterStatePain {
		t.Fatal("environmental damage invented a player target")
	}
	for i := 0; i < 4; i++ {
		g.tickThingThinker(0, g.m.Things[0])
	}
	_, rng := doomrand.State()
	if g.thingState[0] != monsterStateSpawn || g.thingStateTics[0] != 10 || rng != 1 {
		t.Fatalf("targetless pain recovery must stop in A_Look: state=%d tics=%d rng=%d", g.thingState[0], g.thingStateTics[0], rng)
	}
	// The signed ANG180 delta wraps negative, so Doom turns through ANG225.
	if angle := g.thingWorldAngle(0, g.m.Things[0]); angle != degToAngle(225) {
		t.Fatalf("pain recovery skipped A_Chase's turn: angle=%d", angle)
	}
}

func TestMeleeImpactOpensShootableDoor(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Vertexes: []mapdata.Vertex{{X: 32, Y: -64}, {X: 32, Y: 64}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Flags: mlTwoSided,
			Special: 46, Tag: 7, SideNum: [2]int16{0, 1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 1}, {Sector: 0}},
		Sectors:  []mapdata.Sector{{Tag: 7}, {CeilingHeight: 128}},
	}}
	g.initPhysics()
	g.fireMeleeAtAngle(0, 64*fracUnit, 1)
	if len(g.doors) != 1 || g.doors[0].direction != 1 || g.lineSpecial[0] != 46 {
		t.Fatal("a fist/saw trace must activate the repeatable impact door")
	}
	if len(g.hitscanPuffs) != 1 || g.hitscanPuffs[0].state != 95 || g.hitscanPuffs[0].tics != 4 {
		t.Fatal("a punch impact must start in S_PUFF3 with its full countdown")
	}
}

func TestArchvileDamageWakeSearchesCorpseBeforeOrdinaryChase(t *testing.T) {
	g := newArchvileRaiseTestGame(49, 37)
	g.thingState[0], g.thingStatePhase[0] = monsterStateSpawn, 0
	g.thingAggro[0], g.thingTargetPlayer[0], g.thingTargetIdx[0] = false, false, -1
	g.thingThreshold[0], g.thingMoveCount[0] = 0, 6
	g.playerMobjHealth = 100
	doomrand.SetState(0, 8) // 248 fails the Arch-vile's pain roll.
	g.damageMonsterFrom(0, 1, true, -1, g.p.x, g.p.y, true)
	if g.thingState[0] != monsterStateHeal || g.thingState[1] != monsterStateRaise || g.thingDead[1] {
		t.Fatal("damage wake did not execute the initial A_VileChase corpse search")
	}
	if g.thingThreshold[0] != monsterBaseThreshold || g.thingMoveCount[0] != 6 {
		t.Fatal("healing damage wake also executed ordinary chase bookkeeping")
	}
}

func TestDamageWakeRunsGenericMonsterChaseImmediately(t *testing.T) {
	for _, typ := range []int16{84, 16, 7, 64, 66, 67, 69} {
		t.Run(fmt.Sprint(typ), func(t *testing.T) {
			sourceType := typ
			if typ == 64 {
				// P_DamageMobj never retargets damage from an Arch-vile.
				sourceType = 3001
			}
			g := &game{m: &mapdata.Map{
				Sectors: []mapdata.Sector{{CeilingHeight: 128}},
				Things:  []mapdata.Thing{{Type: typ}, {Type: sourceType, X: 256}},
			}, thingHP: []int{monsterSpawnHealth(typ), monsterSpawnHealth(sourceType)}, thingCollected: []bool{false, false}, thingDead: []bool{false, false},
				thingAggro: []bool{false, false}, opts: Options{SkillLevel: 3}}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p.x, g.p.y = -256*fracUnit, -256*fracUnit
			g.thingState[0], g.thingStatePhase[0] = monsterStateSpawn, 0
			g.thingMoveDir[0] = monsterDirEast
			g.thingReactionTics[0] = 8
			g.thingJustAtk[0] = true
			// A wall hides the source, so the initial A_Chase must choose a walk.
			g.lines = []physLine{{x1: 128 * fracUnit, y1: -128 * fracUnit,
				x2: 128 * fracUnit, y2: 128 * fracUnit, dy: 256 * fracUnit}}
			doomrand.SetState(0, 8) // 248 fails the pain roll.
			g.damageMonsterFrom(0, 3, false, 1, 256*fracUnit, 0, true)
			x, _ := g.thingPosFixed(0, g.m.Things[0])
			// The Mastermind's larger radius blocks the eastward step here;
			// its new direction can be west and its random move count can be zero.
			moved := x > 0 && g.thingMoveCount[0] > 0
			if typ == 7 {
				moved = x != 0 && g.thingMoveCount[0] >= 0
			}
			if g.thingState[0] != monsterStateSee || !moved || g.thingThreshold[0] != monsterBaseThreshold-1 {
				t.Fatalf("damage wake skipped the run action: state=%d x=%d count=%d threshold=%d", g.thingState[0], x, g.thingMoveCount[0], g.thingThreshold[0])
			}
			if g.thingJustAtk[0] {
				t.Fatal("damage wake must consume JUSTATTACKED before testing another attack")
			}
		})
	}
}

func TestMissilePlayerAndSolidCorpseFollowBlockmapOrder(t *testing.T) {
	for _, playerFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(playerFirst), func(t *testing.T) {
			g := &game{m: &mapdata.Map{
				Sectors: []mapdata.Sector{{CeilingHeight: 128}},
				Things:  []mapdata.Thing{{Type: 3001, X: 32, Y: 32}},
			}, thingHP: []int{0}, thingCollected: []bool{false}, thingDead: []bool{true}}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p = player{x: 32 * fracUnit, y: 32 * fracUnit}
			g.stats.Health = 100
			g.bmapWidth, g.bmapHeight = 1, 1
			g.thingBlockCells, g.thingBlockCell = [][]int{{0}}, []int{0}
			g.thingBlockOrder, g.playerBlockOrder = []int64{2}, 1
			if playerFirst {
				g.thingBlockOrder[0], g.playerBlockOrder = 1, 2
			}
			p := projectile{sourceThing: -1, radius: 6 * fracUnit, height: 8 * fracUnit}
			hit, ok := g.projectileThingHitAtPosition(p, g.p.x, g.p.y, 8*fracUnit)
			if !ok || hit.isPlayer != playerFirst || hit.damage != playerFirst {
				t.Fatalf("missile hit %+v, player-first=%v", hit, playerFirst)
			}
		})
	}
}

func TestFloorLowerAndChangeInheritsModelOnCompletion(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Linedefs: []mapdata.Linedef{{Tag: 7, SideNum: [2]int16{0, -1}},
			{Flags: mlTwoSided, SideNum: [2]int16{1, 2}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 2}, {Sector: 0}, {Sector: 1}},
		Sectors: []mapdata.Sector{{Tag: 7, CeilingHeight: 128, FloorPic: "FLOOR"},
			{FloorHeight: -24, CeilingHeight: 128, FloorPic: "SLIME", Special: 7},
			{CeilingHeight: 128, FloorPic: "SWITCH"}},
	}, sectorFloor: []int64{0, -24 * fracUnit, 0}, sectorCeil: []int64{128 * fracUnit, 128 * fracUnit, 128 * fracUnit}}
	if !g.activateFloorLine(0, mapdata.FloorInfo{Action: mapdata.FloorLowerAndChange}) {
		t.Fatal("floor did not activate")
	}
	if g.m.Sectors[0].Special != 0 || g.m.Sectors[0].FloorPic != "FLOOR" {
		t.Fatal("lower-and-change applied its model before reaching the destination")
	}
	ft := g.floors[0]
	for i := 0; i < 25; i++ {
		g.tickFloor(0, ft)
	}
	if len(g.floors) != 0 || g.m.Sectors[0].Special != 7 || g.m.Sectors[0].FloorPic != "SLIME" {
		t.Fatalf("floor did not inherit its adjoining model: %+v", g.m.Sectors[0])
	}
}

func TestFloorRaise24AndChangeCopiesSpecialImmediately(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Linedefs: []mapdata.Linedef{{Tag: 7, SideNum: [2]int16{0, -1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 1}},
		Sectors: []mapdata.Sector{{Tag: 7, CeilingHeight: 128, FloorPic: "FLOOR"},
			{CeilingHeight: 128, FloorPic: "SECRET", Special: 9}},
	}, sectorFloor: []int64{0, 0}, sectorCeil: []int64{128 * fracUnit, 128 * fracUnit}}
	if !g.activateFloorLine(0, mapdata.FloorInfo{Action: mapdata.FloorRaise24AndChange}) {
		t.Fatal("floor did not activate")
	}
	if g.m.Sectors[0].Special != 9 || g.m.Sectors[0].FloorPic != "SECRET" {
		t.Fatal("raise-and-change did not copy the front sector immediately")
	}
	// Discovering the secret while it moves must not be undone at completion.
	g.m.Sectors[0].Special = 0
	ft := g.floors[0]
	for i := 0; i < 25; i++ {
		g.tickFloor(0, ft)
	}
	if len(g.floors) != 0 || g.m.Sectors[0].Special != 0 {
		t.Fatal("floor completion restored an already discovered secret")
	}
}

func TestDemoCommercialFinaleRetainsPendingMapAndCarryover(t *testing.T) {
	g := &game{demoIntermissionActive: true, demoTick: 1, weaponAttackDown: true, useButtonDown: true}
	g.opts.DemoScript = &DemoScript{Tics: make([]DemoTic, 53)}
	for i := range g.opts.DemoScript.Tics {
		// WI/F_Ticker receives raw commands. Weapon-change buttons alone
		// advance commercial text even though they are neither attack nor use.
		g.opts.DemoScript.Tics[i].Buttons = demoButtonChange
	}
	next := &mapdata.Map{Name: "MAP07"}
	carry := &playerLevelCarryover{}
	sg := &sessionGame{g: g, current: "MAP06", levelCarryover: carry,
		intermission: sessionIntermission{nextMap: next, state: intermissionState{
			Active: true, Commercial: true, Screen: intermissionScreenNoState, Cnt: 1,
		}}}
	sg.tickIntermission()
	if g.demoWorldDone || !g.demoFinaleActive || !g.demoFinaleCommercial || g.demoIntermissionActive ||
		!sg.finale.Active || sg.finale.Tic != 0 || sg.intermission.state.Active {
		t.Fatal("G_WorldDone did not start commercial finale on the intermission's last tic")
	}
	if sg.intermission.nextMap != next || sg.levelCarryover != carry {
		t.Fatal("finale discarded pending level or carryover")
	}
	for i := 0; i < 52; i++ {
		if err := g.updateDemoIntermission(g.opts.DemoScript); err != nil {
			t.Fatal(err)
		}
		sg.tickFinale()
		if g.demoWorldDone != (i == 51) {
			t.Fatalf("worlddone=%t on finale command %d", g.demoWorldDone, i+1)
		}
	}
	if !g.weaponAttackDown || !g.useButtonDown {
		t.Fatal("F_Ticker must not update the player button latches")
	}
	if sg.intermission.nextMap != next || sg.levelCarryover != carry {
		t.Fatal("finale completion discarded pending level or carryover")
	}
}
