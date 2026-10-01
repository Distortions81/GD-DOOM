package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestDoomProjectileShouldSplitMove_MatchesVanillaSignedComparison(t *testing.T) {
	if !doomProjectileShouldSplitMove(doomMaxMove/2+1, 0) {
		t.Fatal("positive x move above half max should split")
	}
	if !doomProjectileShouldSplitMove(0, doomMaxMove/2+1) {
		t.Fatal("positive y move above half max should split")
	}
	if doomProjectileShouldSplitMove(0, -(doomMaxMove/2 + 1)) {
		t.Fatal("large negative y move should not split in vanilla")
	}
	if doomProjectileShouldSplitMove(-(doomMaxMove/2 + 1), 0) {
		t.Fatal("large negative x move should not split in vanilla")
	}
}

func TestSplitMissileDeathContinuesRemainingStepWithOrdinaryCollisions(t *testing.T) {
	for _, health := range []int{1, 100} {
		t.Run(fmt.Sprint(health), func(t *testing.T) {
			g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}},
				p: player{x: 16 * fracUnit, ceilz: 128 * fracUnit}, stats: playerStats{Health: health}, playerMobjHealth: health,
				sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit}}
			p := projectile{z: 32 * fracUnit, vx: 20 * fracUnit,
				radius: 6 * fracUnit, height: 8 * fracUnit, kind: projectileFatShot,
				sourceType: 67, sourceThing: -1, order: 42, frameTics: 4, ceilz: 128 * fracUnit}
			doomrand.Clear()
			defer doomrand.Clear()
			if _, keep := g.advanceProjectile(p); keep || len(g.projectileImpacts) != 1 {
				t.Fatal("missile should explode on the first split-step collision")
			}
			wantX := int64(0)
			if health == 1 {
				wantX = 10 * fracUnit
				if !g.isDead {
					t.Fatal("fixture missile should kill the player and clear player solidity")
				}
			}
			fx := g.projectileImpacts[0]
			if fx.x != wantX || fx.y != 0 || fx.z != 32*fracUnit {
				t.Fatalf("impact=(%d,%d,%d), want (%d,0,32 units)", fx.x, fx.y, fx.z, wantX)
			}
		})
	}
}

func TestMissileSpawnChecksHalfStepHeightAndDefersFloorImpact(t *testing.T) {
	for _, tc := range []struct {
		name      string
		z, vz     int64
		wantSpawn bool
	}{
		{"below-floor", fracUnit, -4 * fracUnit, true},
		{"at-floor", fracUnit, -2 * fracUnit, true},
		{"ceiling-after-half-step", 120 * fracUnit, 4 * fracUnit, false},
		{"ceiling-cleared-by-half-step", 122 * fracUnit, -4 * fracUnit, true},
		{"step-too-high-after-half-step", -23 * fracUnit, -4 * fracUnit, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}},
				p:           player{x: 256 * fracUnit, y: 256 * fracUnit},
				sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit}}
			p := projectile{x: 32 * fracUnit, y: 32 * fracUnit, z: tc.z,
				vx: 8 * fracUnit, vz: tc.vz, radius: 6 * fracUnit, height: 8 * fracUnit,
				kind: projectileFireball, sourceType: 3001, sourceThing: -1, frameTics: 4}
			doomrand.Clear()
			defer doomrand.Clear()
			if got := g.finishProjectileSpawn(&p, true); got != tc.wantSpawn {
				t.Fatalf("spawn=%v, want %v", got, tc.wantSpawn)
			}
			if tc.wantSpawn {
				_, rng := doomrand.State()
				if p.z != tc.z+(tc.vz>>1) || len(g.projectileImpacts) != 0 || rng != 0 {
					t.Fatalf("spawn changed height or exploded early: z=%d impacts=%d rng=%d", p.z, len(g.projectileImpacts), rng)
				}
				if tc.name == "below-floor" || tc.name == "at-floor" {
					if _, keep := g.advanceProjectile(p); keep || len(g.projectileImpacts) != 1 {
						t.Fatal("normal missile thinker should move horizontally, then explode at the floor")
					}
					fx := g.projectileImpacts[0]
					if fx.x != 44*fracUnit || fx.z != 0 {
						t.Fatalf("impact=(%d,%d), want (44 units, floor)", fx.x, fx.z)
					}
				}
			}
		})
	}
}

func TestImpAttackSpawnsProjectile(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: 128, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{60},
		thingAggro:     []bool{true},
		thingCooldown:  []int{0},
		soundQueue:     make([]soundEvent, 0, 2),
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0, z: 0},
		projectiles:    make([]projectile, 0, 2),
	}
	if !g.monsterAttack(0, 3001, 256*fracUnit) {
		t.Fatal("imp attack should spawn a projectile")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100 (projectiles should not be instant hit)", g.stats.Health)
	}
	if !hasSoundEvent(g.soundQueue, soundEventShootFireball) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventShootFireball)
	}
}

func TestImpAttackUsesMissileOutsideVanillaMeleeRange(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: 61, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{60},
		thingAggro:     []bool{true},
		thingCooldown:  []int{0},
		soundQueue:     make([]soundEvent, 0, 2),
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0, z: 0},
		projectiles:    make([]projectile, 0, 2),
	}
	dist := doomApproxDistance(g.p.x-(61*fracUnit), g.p.y)
	if !g.monsterAttack(0, 3001, dist) {
		t.Fatal("imp attack should spawn a projectile just outside vanilla melee range")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100 (imp should not melee at 61 units)", g.stats.Health)
	}
}

func TestHellKnightAttackSpawnsBaronProjectile(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 69, X: 128, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{500},
		thingAggro:     []bool{true},
		soundQueue:     make([]soundEvent, 0, 2),
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0, z: 0},
		projectiles:    make([]projectile, 0, 2),
	}
	if !g.monsterAttack(0, 69, 256*fracUnit) {
		t.Fatal("hell knight attack should spawn a projectile")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	if got := g.projectiles[0].kind; got != projectileBaronBall {
		t.Fatalf("projectile kind=%v want=%v", got, projectileBaronBall)
	}
	if !hasSoundEvent(g.soundQueue, soundEventShootFireball) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventShootFireball)
	}
}

func TestBaronProjectileSpeedMatchesDoomSourceFastMode(t *testing.T) {
	if got := monsterProjectileSpeed(3003, false); got != 15*fracUnit {
		t.Fatalf("normal speed=%d want=%d", got, 15*fracUnit)
	}
	if got := monsterProjectileSpeed(3003, true); got != 20*fracUnit {
		t.Fatalf("fast speed=%d want=%d", got, 20*fracUnit)
	}
	if got := monsterProjectileSpeed(69, true); got != 20*fracUnit {
		t.Fatalf("knight fast speed=%d want=%d", got, 20*fracUnit)
	}
}

func TestRevenantTracerSpeedMatchesDoomSource(t *testing.T) {
	if got := monsterProjectileSpeed(66, false); got != 10*fracUnit {
		t.Fatalf("normal tracer speed=%d want=%d", got, 10*fracUnit)
	}
	if got := monsterProjectileSpeed(66, true); got != 10*fracUnit {
		t.Fatalf("fast tracer speed=%d want=%d", got, 10*fracUnit)
	}
}

func TestFastProjectileSpeedsChangeOnlyOriginalThreeTypes(t *testing.T) {
	for _, tc := range []struct {
		typ          int16
		normal, fast int64
	}{
		{3001, 10, 20}, // Imp: MT_TROOPSHOT
		{3005, 10, 20}, // Cacodemon: MT_HEADSHOT
		{3003, 15, 20}, // Baron: MT_BRUISERSHOT
		{69, 15, 20},   // Hell Knight: MT_BRUISERSHOT
		{66, 10, 10},   // Revenant: MT_TRACER
		{67, 20, 20},   // Mancubus: MT_FATSHOT
		{16, 20, 20},   // Cyberdemon: MT_ROCKET
		{68, 25, 25},   // Arachnotron: MT_ARACHPLAZ
	} {
		if got := monsterProjectileSpeed(tc.typ, false); got != tc.normal*fracUnit {
			t.Fatalf("type=%d normal speed=%d want=%d", tc.typ, got, tc.normal*fracUnit)
		}
		if got := monsterProjectileSpeed(tc.typ, true); got != tc.fast*fracUnit {
			t.Fatalf("type=%d fast speed=%d want=%d", tc.typ, got, tc.fast*fracUnit)
		}
	}
}

func TestImpProjectileSpawnsFromRuntimePosition(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: 128, Y: 0},
			},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{60},
		thingAggro:        []bool{true},
		thingX:            []int64{320 * fracUnit},
		thingY:            []int64{64 * fracUnit},
		thingZState:       []int64{0},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{128 * fracUnit},
		thingSupportValid: []bool{true},
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0, z: 0},
		projectiles:       make([]projectile, 0, 1),
	}
	if !g.monsterAttack(0, 3001, doomApproxDistance(g.p.x-g.thingX[0], g.p.y-g.thingY[0])) {
		t.Fatal("imp attack should spawn a projectile")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	p := g.projectiles[0]
	if got := p.sourceX; got != g.thingX[0] {
		t.Fatalf("projectile sourceX=%d want runtime x=%d", got, g.thingX[0])
	}
	if got := p.sourceY; got != g.thingY[0] {
		t.Fatalf("projectile sourceY=%d want runtime y=%d", got, g.thingY[0])
	}
	if got := p.x; got != g.thingX[0]+(p.vx>>1) {
		t.Fatalf("projectile x=%d want half-step from runtime x=%d", got, g.thingX[0]+(p.vx>>1))
	}
	if got := p.y; got != g.thingY[0]+(p.vy>>1) {
		t.Fatalf("projectile y=%d want half-step from runtime y=%d", got, g.thingY[0]+(p.vy>>1))
	}
}

func TestArachnotronAttackSpawnsProjectileAfterWindup(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 68, X: 96, Y: 0}},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{500},
		thingAggro:          []bool{true},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{false},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		projectiles:         make([]projectile, 0, 2),
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	if !g.startMonsterAttackState(0, 68, true) {
		t.Fatal("expected arachnotron attack state to start")
	}
	for i := 0; i < 23; i++ {
		g.tickMonsters()
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectiles=%d want=1 after arachnotron windup", got)
	}
	if g.projectiles[0].kind != projectilePlasmaBall {
		t.Fatalf("projectile kind=%v want=%v", g.projectiles[0].kind, projectilePlasmaBall)
	}
}

func TestMonsterProjectileRenderStartsAtSpawnPointOnFirstFrame(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3005, X: 96, Y: 0}},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{400},
		thingAggro:        []bool{true},
		thingTargetPlayer: []bool{true},
		thingTargetIdx:    []int{-1},
		thingX:            []int64{96 * fracUnit},
		thingY:            []int64{0},
		thingZState:       []int64{0},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{128 * fracUnit},
		thingSupportValid: []bool{true},
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0, z: 0},
		projectiles:       make([]projectile, 0, 1),
	}

	if !g.spawnMonsterProjectile(0, 3005) {
		t.Fatal("expected cacodemon projectile to spawn")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	p := g.projectiles[0]
	if p.prevX == p.x && p.prevY == p.y && p.prevZ == p.z {
		t.Fatalf("spawn prev unexpectedly matches current=(%d,%d,%d)", p.x, p.y, p.z)
	}
	if rx, ry, rz := g.projectileRenderPosFixed(p, 0.25); rx == p.x && ry == p.y && rz == p.z {
		t.Fatalf("render pos unexpectedly matches current=(%d,%d,%d)", rx, ry, rz)
	}
}

func TestBaronProjectileRenderStartsFromVisualMuzzleOffset(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3003, X: 96, Y: 0}},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{1000},
		thingAggro:        []bool{true},
		thingTargetPlayer: []bool{true},
		thingTargetIdx:    []int{-1},
		thingX:            []int64{96 * fracUnit},
		thingY:            []int64{0},
		thingZState:       []int64{0},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{128 * fracUnit},
		thingSupportValid: []bool{true},
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0, z: 0},
		projectiles:       make([]projectile, 0, 1),
	}
	if !g.spawnMonsterProjectile(0, 3003) {
		t.Fatal("expected baron projectile to spawn")
	}
	p := g.projectiles[0]
	if p.prevX == p.x && p.prevY == p.y {
		t.Fatalf("baron muzzle offset missing prev=(%d,%d) cur=(%d,%d)", p.prevX, p.prevY, p.x, p.y)
	}
}

func TestArachnotronAttackRefiresLikeDoomSource(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 68, X: 96, Y: 0}},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{500},
		thingAggro:          []bool{true},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{false},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		projectiles:         make([]projectile, 0, 4),
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	if !g.startMonsterAttackState(0, 68, true) {
		t.Fatal("expected arachnotron attack state to start")
	}
	for i := 0; i < 29; i++ {
		g.tickMonsters()
	}
	if got := len(g.projectiles); got != 2 {
		t.Fatalf("projectiles=%d want=2 after first arachnotron cycle", got)
	}
	for i := 0; i < 9; i++ {
		g.tickMonsters()
	}
	if got := len(g.projectiles); got != 3 {
		t.Fatalf("projectiles=%d want=3 after arachnotron refire", got)
	}
}

func TestCyberdemonAttackSpawnsThreeRockets(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 16, X: 128, Y: 0}},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{4000},
		thingAggro:          []bool{true},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{false},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		projectiles:         make([]projectile, 0, 4),
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	if !g.startMonsterAttackState(0, 16, true) {
		t.Fatal("expected cyberdemon attack state to start")
	}
	for i := 0; i < 66; i++ {
		g.tickMonsters()
	}
	if got := len(g.projectiles); got != 3 {
		t.Fatalf("projectiles=%d want=3 after cyberdemon volley", got)
	}
	for i, p := range g.projectiles {
		if p.kind != projectileRocket {
			t.Fatalf("projectile %d kind=%v want=%v", i, p.kind, projectileRocket)
		}
	}
}

func TestSpiderMastermindAttackRefiresLikeDoomSource(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 7, X: 128, Y: 0}},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{3000},
		thingAggro:          []bool{true},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{false},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		projectiles:         make([]projectile, 0, 8),
		soundQueue:          make([]soundEvent, 0, 8),
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	if !g.startMonsterAttackState(0, 7, true) {
		t.Fatal("expected spider mastermind attack state to start")
	}
	for i := 0; i < 29; i++ {
		g.tickMonsters()
	}
	shots := 0
	for _, ev := range g.soundQueue {
		if ev == soundEventShootShotgun {
			shots++
		}
	}
	if shots != 3 {
		t.Fatalf("shot sounds=%d want=3 after first spider cycle plus refire", shots)
	}
	if got := g.thingState[0]; got != monsterStateAttack {
		t.Fatalf("state=%v want attack after spider refire cycle", got)
	}
	for i := 0; i < 9; i++ {
		g.tickMonsters()
	}
	shots = 0
	for _, ev := range g.soundQueue {
		if ev == soundEventShootShotgun {
			shots++
		}
	}
	if shots != 5 {
		t.Fatalf("shot sounds=%d want=5 after spider refire", shots)
	}
}

func TestRevenantAttackSpawnsTracerProjectile(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 66, X: 96, Y: 0}},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{300},
		thingAggro:          []bool{true},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{false},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		projectiles:         make([]projectile, 0, 2),
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	if !g.startMonsterAttackState(0, 66, true) {
		t.Fatal("expected revenant attack state to start")
	}
	for i := 0; i < 20; i++ {
		g.tickMonsters()
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectiles=%d want=1 after revenant attack", got)
	}
	if g.projectiles[0].kind != projectileTracer {
		t.Fatalf("projectile kind=%v want=%v", g.projectiles[0].kind, projectileTracer)
	}
}

func TestRevenantTracerHomesTowardPlayer(t *testing.T) {
	g := &game{
		p:        player{x: 128 * fracUnit, y: 128 * fracUnit, z: 0},
		demoTick: 5, // A_Tracer runs on gametic 4.
	}
	p := projectile{
		x:            0,
		y:            0,
		z:            32 * fracUnit,
		vx:           20 * fracUnit,
		vy:           0,
		vz:           0,
		kind:         projectileTracer,
		tracerPlayer: true,
	}
	oldVy := p.vy
	g.tickProjectileSpecial(&p)
	if p.vy == oldVy {
		t.Fatal("tracer should steer toward player")
	}
}

func TestMancubusAttackSpawnsSixProjectilesAcrossThreeVolleys(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 67, X: 128, Y: 0}},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{600},
		thingAggro:          []bool{true},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{false},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		soundQueue:          make([]soundEvent, 0, 8),
		projectiles:         make([]projectile, 0, 8),
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	if !g.startMonsterAttackState(0, 67, true) {
		t.Fatal("expected mancubus attack state to start")
	}
	for i := 0; i < 80; i++ {
		g.tickMonsters()
	}
	if got := len(g.projectiles); got != 6 {
		t.Fatalf("projectiles=%d want=6 after mancubus volleys", got)
	}
	for i, p := range g.projectiles {
		if p.kind != projectileFatShot {
			t.Fatalf("projectile %d kind=%v want=%v", i, p.kind, projectileFatShot)
		}
	}
	if got := countSoundEvent(g.soundQueue, soundEventShootFireball); got != 6 {
		t.Fatalf("fireball launch sound count=%d want=6 queue=%v", got, g.soundQueue)
	}
}

func TestMancubusAttackPhaseFacesTargetOnVolleyFrames(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 67, X: 128, Y: 0}},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{600},
		thingAggro:        []bool{true},
		thingTargetPlayer: []bool{true},
		thingTargetIdx:    []int{-1},
		thingAttackPhase:  []int{1},
		thingAngleState: []uint32{
			degToAngle(180),
		},
		thingX:      []int64{128 * fracUnit},
		thingY:      []int64{0},
		projectiles: make([]projectile, 0, 2),
		soundQueue:  make([]soundEvent, 0, 2),
		stats:       playerStats{Health: 100},
		p:           player{x: 0, y: 64 * fracUnit, z: 0},
	}

	tx, ty := g.thingPosFixed(0, g.m.Things[0])
	dist := doomApproxDistance(g.p.x-tx, g.p.y-ty)
	g.runMonsterAttackPhaseEntry(0, 67, 1, tx, ty, g.p.x, g.p.y, dist)

	want := doomPointToAngle2(tx, ty, g.p.x, g.p.y) + 0x08000000
	if got := g.thingWorldAngle(0, g.m.Things[0]); got != want {
		t.Fatalf("angle=%d want=%d", got, want)
	}
	if got := len(g.projectiles); got != 2 {
		t.Fatalf("projectiles=%d want=2", got)
	}
}

func TestMancubusAttackPhase4SecondProjectileUsesDoubleNegativeSpread(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 67, X: 128, Y: 0}},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{600},
		thingAggro:        []bool{true},
		thingTargetPlayer: []bool{true},
		thingTargetIdx:    []int{-1},
		thingAttackPhase:  []int{4},
		thingAngleState:   []uint32{degToAngle(180)},
		thingX:            []int64{128 * fracUnit},
		thingY:            []int64{0},
		projectiles:       make([]projectile, 0, 2),
		soundQueue:        make([]soundEvent, 0, 2),
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 64 * fracUnit, z: 0},
	}

	tx, ty := g.thingPosFixed(0, g.m.Things[0])
	dist := doomApproxDistance(g.p.x-tx, g.p.y-ty)
	g.runMonsterAttackPhaseEntry(0, 67, 4, tx, ty, g.p.x, g.p.y, dist)

	if got := len(g.projectiles); got != 2 {
		t.Fatalf("projectiles=%d want=2", got)
	}
	base := doomPointToAngle2(tx, ty, g.p.x, g.p.y)
	if got := g.projectiles[0].angle; got != base {
		t.Fatalf("projectile[0] angle=%d want=%d", got, base)
	}
	if got, want := g.projectiles[1].angle, base-2*0x08000000; got != want {
		t.Fatalf("projectile[1] angle=%d want=%d", got, want)
	}
}

func TestProjectileImpactSoundEventMatchesVanillaProjectileDeathsounds(t *testing.T) {
	if got := projectileImpactSoundEvent(projectileRocket); got != soundEventBarrelExplode {
		t.Fatalf("rocket impact sound=%v want %v", got, soundEventBarrelExplode)
	}
	if got := projectileImpactSoundEvent(projectileBFGBall); got != soundEventImpactRocket {
		t.Fatalf("bfg impact sound=%v want %v", got, soundEventImpactRocket)
	}
	if got := projectileImpactSoundEvent(projectilePlayerPlasma); got != soundEventImpactFire {
		t.Fatalf("player plasma impact sound=%v want %v", got, soundEventImpactFire)
	}
}

func TestImpProjectileExplodesOnImpWithoutDamage(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: -32, Y: 0},
				{Type: 3001, X: 32, Y: 0},
			},
		},
		thingCollected: []bool{false, false},
		thingHP:        []int{60, 60},
		projectiles: []projectile{{
			x:           -32 * fracUnit,
			y:           0,
			z:           40 * fracUnit,
			vx:          64 * fracUnit,
			vy:          0,
			vz:          0,
			radius:      6 * fracUnit,
			height:      8 * fracUnit,
			ttl:         4,
			sourceThing: 0,
			sourceType:  3001,
			kind:        projectileFireball,
		}},
	}

	g.tickProjectiles()

	if got := g.thingHP[1]; got != 60 {
		t.Fatalf("same-species projectile hp=%d want=60", got)
	}
	if got := len(g.projectiles); got != 0 {
		t.Fatalf("projectiles remaining=%d want=0", got)
	}
	if got := len(g.projectileImpacts); got != 1 {
		t.Fatalf("impacts=%d want=1", got)
	}
}

func TestBaronProjectileExplodesOnHellKnightWithoutDamage(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3003, X: -32, Y: 0},
				{Type: 69, X: 32, Y: 0},
			},
		},
		thingCollected: []bool{false, false},
		thingHP:        []int{1000, 500},
		projectiles: []projectile{{
			x:           -32 * fracUnit,
			y:           0,
			z:           56 * fracUnit,
			vx:          64 * fracUnit,
			vy:          0,
			vz:          0,
			radius:      6 * fracUnit,
			height:      8 * fracUnit,
			ttl:         4,
			sourceThing: 0,
			sourceType:  3003,
			kind:        projectileBaronBall,
		}},
	}

	g.tickProjectiles()

	if got := g.thingHP[1]; got != 500 {
		t.Fatalf("baron-vs-knight hp=%d want=500", got)
	}
	if got := len(g.projectileImpacts); got != 1 {
		t.Fatalf("impacts=%d want=1", got)
	}
}

func TestProjectileHitsPlayer(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Health: 100},
		p: player{
			x:      0,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		projectiles: []projectile{
			{
				x:          -30 * fracUnit,
				y:          0,
				z:          32 * fracUnit,
				vx:         12 * fracUnit,
				vy:         0,
				radius:     6 * fracUnit,
				height:     8 * fracUnit,
				ttl:        5,
				sourceType: 3001,
				kind:       projectileFireball,
			},
		},
	}
	g.tickProjectiles()
	if g.stats.Health >= 100 {
		t.Fatalf("health=%d want < 100 after projectile impact", g.stats.Health)
	}
	if got := len(g.projectiles); got != 0 {
		t.Fatalf("projectiles remaining=%d want=0", got)
	}
	if got := len(g.projectileImpacts); got != 1 {
		t.Fatalf("impact count=%d want=1", got)
	}
	if !hasSoundEvent(g.soundQueue, soundEventImpactFire) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventImpactFire)
	}
}

func TestAdvanceProjectileImpactTicTransitionsPhase(t *testing.T) {
	g := &game{}
	fx := projectileImpact{
		kind:      projectileRocket,
		tics:      11,
		totalTics: 11,
		phase:     0,
		phaseTics: 1,
	}

	if ok := g.advanceProjectileImpactTic(&fx); !ok {
		t.Fatal("advanceProjectileImpactTic returned false")
	}
	if got := fx.tics; got != 10 {
		t.Fatalf("tics=%d want=10", got)
	}
	if got := fx.phase; got != 1 {
		t.Fatalf("phase=%d want=1", got)
	}
	if got := fx.phaseTics; got != 6 {
		t.Fatalf("phaseTics=%d want=6", got)
	}
}

func TestDeferredRocketImpactRandomizesAfterSplashPoint(t *testing.T) {
	doomrand.Clear()
	g := &game{}
	p := projectile{kind: projectileRocket, angle: 123, order: 7}

	_, p0 := doomrand.State()
	idx := g.spawnProjectileImpactFromDeferredRandom(p, 10, 20, 30)
	_, p1 := doomrand.State()
	if got := prandDelta(p0, p1); got != 0 {
		t.Fatalf("pre-finalize p-random calls=%d want=0", got)
	}
	if idx != 0 || len(g.projectileImpacts) != 1 {
		t.Fatalf("impact idx=%d count=%d want idx=0 count=1", idx, len(g.projectileImpacts))
	}

	g.finalizeDeferredProjectileImpact(idx)
	_, p2 := doomrand.State()
	if got := prandDelta(p1, p2); got != 1 {
		t.Fatalf("finalize p-random calls=%d want=1", got)
	}
	if got := g.projectileImpacts[0].order; got != 7 {
		t.Fatalf("impact order=%d want=7", got)
	}
	if got := g.projectileImpacts[0].phase; got != 0 {
		t.Fatalf("impact phase=%d want=0", got)
	}
	if got := g.projectileImpacts[0].phaseTics; got < 5 || got > 8 {
		t.Fatalf("impact phaseTics=%d want in [5,8] before first impact tic advances", got)
	}
}

func TestSpawnProjectileImpactFrom_RocketKeepsFirstImpactTic(t *testing.T) {
	doomrand.Clear()
	g := &game{}
	p := projectile{kind: projectileRocket, angle: 123, order: 7}

	g.spawnProjectileImpactFrom(p, 10, 20, 30)

	if got := len(g.projectileImpacts); got != 1 {
		t.Fatalf("impact count=%d want=1", got)
	}
	fx := g.projectileImpacts[0]
	if got := fx.phase; got != 0 {
		t.Fatalf("impact phase=%d want=0", got)
	}
	if got := fx.phaseTics; got < 5 || got > 8 {
		t.Fatalf("impact phaseTics=%d want in [5,8]", got)
	}
	if got := fx.tics; got < 15 || got > 18 {
		t.Fatalf("impact total tics=%d want in [15,18]", got)
	}
}

func TestRocketExplosionExpiresAfterDoomThinkerLifetime(t *testing.T) {
	doomrand.Clear()
	g := &game{worldTic: 1}
	p := projectile{kind: projectileRocket, order: 7, sourceThing: -1}
	g.explodeProjectileInThinker(p, 10, 20, 30)
	// The first P_Random is 8, so the death frames last 8+6+4 tics.
	// The collision's own P_MobjThinker consumes the first of those 18.
	if fx := g.projectileImpacts[0]; fx.phaseTics != 7 || fx.tics != 17 {
		t.Fatalf("collision impact=(%d,%d) want=(7,17)", fx.phaseTics, fx.tics)
	}
	for i := 0; i < 16; i++ {
		g.tickProjectileImpactByOrder(p.order)
	}
	if len(g.projectileImpacts) != 1 {
		t.Fatal("rocket disappeared before its final death-state tic")
	}
	g.tickProjectileImpactByOrder(p.order)
	if len(g.projectileImpacts) != 0 {
		t.Fatal("rocket survived its final death-state tic")
	}
}

func TestSpawnCheckPlasmaExplosionWaitsForFirstThinker(t *testing.T) {
	doomrand.Clear()
	g := &game{}
	p := projectile{kind: projectilePlayerPlasma, order: 7}
	g.spawnProjectileImpactFrom(p, 10, 20, 30)
	if fx := g.projectileImpacts[0]; fx.phaseTics != 4 || fx.tics != 20 {
		t.Fatalf("spawn impact=(%d,%d) want=(4,20)", fx.phaseTics, fx.tics)
	}
	g.runOrderedWorldThinkers()
	if fx := g.projectileImpacts[0]; fx.phaseTics != 3 || fx.tics != 19 {
		t.Fatalf("first thinker impact=(%d,%d) want=(3,19)", fx.phaseTics, fx.tics)
	}
}

func TestArachnotronExplosionUsesFiveFiveTicStates(t *testing.T) {
	doomrand.Clear()
	g := &game{}
	p := projectile{kind: projectilePlasmaBall, sourceType: 68, order: 7}
	g.spawnProjectileImpactFrom(p, 10, 20, 30)
	if fx := g.projectileImpacts[0]; fx.phaseTics != 5 || fx.tics != 25 {
		t.Fatalf("arachnotron impact=(%d,%d) want=(5,25)", fx.phaseTics, fx.tics)
	}
}

func TestProjectilePassesThroughTwoSidedWindow(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{}, {}},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
		},
		lines: []physLine{
			{
				x1:       0,
				y1:       -64 * fracUnit,
				x2:       0,
				y2:       64 * fracUnit,
				flags:    mlTwoSided,
				sideNum0: 0,
				sideNum1: 1,
			},
		},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
	}
	p := projectile{
		x:      -16 * fracUnit,
		y:      0,
		z:      32 * fracUnit,
		vx:     32 * fracUnit,
		vy:     0,
		vz:     0,
		height: 8 * fracUnit,
	}
	blocked, _, _, _, _, _ := g.projectileBlockedAt(p, p.x, p.y, p.z, p.x+p.vx, p.y+p.vy, p.z+p.vz)
	if blocked {
		t.Fatal("projectile should pass through open two-sided line/window")
	}
}

func TestProjectileDoesNotHitSourceImp(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: 128, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{60},
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0, z: 0, floorz: 0, ceilz: 128 * fracUnit},
		projectiles:    make([]projectile, 0, 2),
	}
	if !g.spawnMonsterProjectile(0, 3001) {
		t.Fatal("expected imp projectile to spawn")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	p := g.projectiles[0]
	_, hit := g.projectileThingHitAtPosition(p, p.x+p.vx, p.y+p.vy, p.z)
	if hit {
		t.Fatal("projectile should not select the source imp as a hit target")
	}
}

func TestProjectileSelectsPlayerAtDestinationDuringThingPass(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m:     &mapdata.Map{},
		stats: playerStats{Health: 100},
		p: player{
			x:      16 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}
	p := projectile{
		x:      -32 * fracUnit,
		y:      0,
		z:      32 * fracUnit,
		vx:     32 * fracUnit,
		vy:     0,
		vz:     0,
		radius: monsterProjectileRadius(3001),
		height: monsterProjectileHeight(3001),
		kind:   projectileFireball,
	}

	hit, ok := g.projectileThingHitAtPosition(p, p.x+p.vx, p.y+p.vy, p.z)
	if !ok {
		t.Fatal("expected projectile to hit player at destination")
	}
	if !hit.isPlayer {
		t.Fatalf("hit target=%+v want player", hit)
	}
	if hit.frac != 1 {
		t.Fatalf("hit frac=%f want 1 for destination collision", hit.frac)
	}
	if hit.x != p.x+p.vx || hit.y != p.y+p.vy || hit.z != p.z {
		t.Fatalf("impact=(%d,%d,%d) want destination/current-z (%d,%d,%d)", hit.x, hit.y, hit.z, p.x+p.vx, p.y+p.vy, p.z)
	}
}

func TestProjectileHitsThingOnDestinationBoxOverlapLikeDoomTryMove(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: -86, Y: -75},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
	}
	g.initPhysics()
	g.setThingSupportState(0, 88*fracUnit, 0, 128*fracUnit)

	p := projectile{
		x:           -83 * fracUnit,
		y:           -77 * fracUnit,
		z:           120 * fracUnit,
		vx:          -10 * fracUnit,
		vy:          -1 * fracUnit,
		vz:          0,
		radius:      monsterProjectileRadius(3001),
		height:      monsterProjectileHeight(3001),
		sourceThing: -1,
		kind:        projectileFireball,
	}

	hit, ok := g.projectileThingHitAtPosition(p, p.x+p.vx, p.y+p.vy, p.z)
	if !ok {
		t.Fatal("expected projectile to hit thing at destination overlap")
	}
	if hit.idx != 0 || hit.isPlayer {
		t.Fatalf("hit=%+v want thing 0", hit)
	}
	if hit.x != p.x+p.vx || hit.y != p.y+p.vy || hit.z != p.z {
		t.Fatalf("impact=(%d,%d,%d) want destination/current-z (%d,%d,%d)", hit.x, hit.y, hit.z, p.x+p.vx, p.y+p.vy, p.z)
	}
}

func TestProjectileThingHitAtPositionUsesCurrentZLikeDoomXYMove(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m:     &mapdata.Map{},
		stats: playerStats{Health: 100},
		p: player{
			x:      20 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
	}
	p := projectile{
		x:      0,
		y:      0,
		z:      65 * fracUnit,
		vx:     25 * fracUnit,
		vy:     0,
		vz:     -40 * fracUnit,
		radius: monsterProjectileRadius(3005),
		height: monsterProjectileHeight(3005),
		kind:   projectilePlasmaBall,
	}
	if _, ok := g.projectileThingHitAtPosition(p, p.x+p.vx, p.y+p.vy, p.z); ok {
		t.Fatal("projectile XY collision should use current z, not swept z")
	}
}

func TestPlayerRocketHitsBarrelOnDestinationSquareOverlap(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2035, X: -96, Y: 1200},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
	}
	g.initPhysics()
	g.setThingSupportState(0, 0, 0, 128*fracUnit)

	p := projectile{
		x:            -75 * fracUnit,
		y:            1170 * fracUnit,
		z:            32 * fracUnit,
		vx:           -64800,
		vy:           1309100,
		vz:           0,
		radius:       11 * fracUnit,
		height:       8 * fracUnit,
		sourceThing:  -1,
		sourcePlayer: true,
		kind:         projectileRocket,
	}

	hit, ok := g.projectileThingHitAtPosition(p, p.x+p.vx, p.y+p.vy, p.z)
	if !ok {
		t.Fatal("expected player rocket to hit barrel on destination square overlap")
	}
	if hit.idx != 0 || hit.isPlayer {
		t.Fatalf("hit=%+v want barrel 0", hit)
	}
	if hit.frac != 1 {
		t.Fatalf("hit frac=%f want 1 for destination overlap", hit.frac)
	}
}

func TestPlayerRocketSpawnsProjectile(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Health: 100, Rockets: 3},
		p: player{
			x:      0,
			y:      0,
			z:      0,
			angle:  0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		inventory:   playerInventory{ReadyWeapon: weaponRocketLauncher},
		soundQueue:  make([]soundEvent, 0, 2),
		projectiles: make([]projectile, 0, 1),
	}
	if !g.fireSelectedWeapon() {
		t.Fatal("rocket launcher should spawn a projectile")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	p := g.projectiles[0]
	if p.kind != projectileRocket {
		t.Fatalf("projectile kind=%v want=%v", p.kind, projectileRocket)
	}
	if p.sourceX != g.p.x || p.sourceY != g.p.y {
		t.Fatalf("projectile source=(%d,%d) want player origin (%d,%d)", p.sourceX, p.sourceY, g.p.x, g.p.y)
	}
	if p.x != g.p.x+(p.vx>>1) || p.y != g.p.y+(p.vy>>1) {
		t.Fatalf("projectile position=(%d,%d) want half-step (%d,%d)", p.x, p.y, g.p.x+(p.vx>>1), g.p.y+(p.vy>>1))
	}
	if want := g.p.z + 32*fracUnit + (p.vz >> 1); p.z != want {
		t.Fatalf("projectile z=%d want=%d", p.z, want)
	}
	if !p.sourcePlayer {
		t.Fatal("player rocket should be marked as player-sourced")
	}
	if !hasSoundEvent(g.soundQueue, soundEventShootRocket) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventShootRocket)
	}
}

func TestPlayerRocketUsesDoomFineAngleMomentum(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Health: 100, Rockets: 3},
		p: player{
			x:      16130837,
			y:      190469,
			z:      0,
			angle:  1778384896,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		inventory:   playerInventory{ReadyWeapon: weaponRocketLauncher},
		projectiles: make([]projectile, 0, 1),
	}
	if !g.fireSelectedWeapon() {
		t.Fatal("rocket launcher should spawn a projectile")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	p := g.projectiles[0]
	if p.vx != -1124500 || p.vy != 673400 {
		t.Fatalf("rocket momentum=(%d,%d) want=(-1124500,673400)", p.vx, p.vy)
	}
}

func TestMonsterProjectileSpawnPreservesHalfStepBeforeDeferredAdvance(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: 0, Y: 0},
				{Type: 3004, X: 128, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		sectorFloor:       []int64{0},
		sectorCeil:        []int64{128 * fracUnit},
		thingHP:           []int{60, 100},
		thingTargetIdx:    []int{0, 0},
		thingTargetPlayer: []bool{false, false},
		stats:             playerStats{Health: 100},
		p: player{
			x:      128 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		projectiles: make([]projectile, 0, 1),
	}
	g.initPhysics()
	g.setThingSupportState(0, 0, 0, 128*fracUnit)
	g.setThingSupportState(1, 0, 0, 128*fracUnit)
	g.thingTargetIdx[0] = 1
	g.thingTargetPlayer[0] = false

	if !g.spawnMonsterProjectile(0, 3001) {
		t.Fatal("expected imp projectile to spawn")
	}
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count=%d want=1", got)
	}
	p := g.projectiles[0]
	if !p.deferredTick {
		t.Fatal("monster projectile should defer its thinker advance until later in the tic")
	}
	if p.x != p.sourceX+(p.vx>>1) || p.y != p.sourceY+(p.vy>>1) {
		t.Fatalf("projectile position=(%d,%d) want half-step (%d,%d)", p.x, p.y, p.sourceX+(p.vx>>1), p.sourceY+(p.vy>>1))
	}
	wantZ := 32*fracUnit + (p.vz >> 1)
	if p.z != wantZ {
		t.Fatalf("projectile z=%d want=%d", p.z, wantZ)
	}

	g.tickDeferredProjectiles()

	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectile count after deferred tick=%d want=1", got)
	}
	p = g.projectiles[0]
	if p.deferredTick {
		t.Fatal("projectile should no longer be deferred after deferred tick")
	}
	if p.x != p.sourceX+(p.vx>>1)+p.vx || p.y != p.sourceY+(p.vy>>1)+p.vy {
		t.Fatalf("projectile position after deferred tick=(%d,%d) want (%d,%d)", p.x, p.y, p.sourceX+(p.vx>>1)+p.vx, p.sourceY+(p.vy>>1)+p.vy)
	}
}

func TestPlayerRocketSpawnConsumesLastLookAndCheckMissileSpawnPRandom(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Health: 100, Rockets: 3},
		p: player{
			x:      0,
			y:      0,
			z:      0,
			angle:  0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		inventory:   playerInventory{ReadyWeapon: weaponRocketLauncher},
		projectiles: make([]projectile, 0, 1),
	}
	if !g.fireSelectedWeapon() {
		t.Fatal("rocket launcher should spawn a projectile")
	}
	_, prnd := doomrand.State()
	if prnd != 2 {
		t.Fatalf("prnd=%d want=2 after lastlook + checkmissilespawn tics consume", prnd)
	}
}

func TestSameSpeciesMissileExplosionDoesNotConsumeDamageRandom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: 0, Y: 0},
				{Type: 3001, X: 32, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		sectorFloor:    []int64{0},
		sectorCeil:     []int64{128 * fracUnit},
		thingHP:        []int{60, 60},
		thingCollected: []bool{false, false},
	}
	g.initPhysics()
	g.setThingSupportState(0, 0, 0, 128*fracUnit)
	g.setThingSupportState(1, 0, 0, 128*fracUnit)

	p := projectile{
		x:           0,
		y:           0,
		z:           32 * fracUnit,
		vx:          8 * fracUnit,
		vy:          0,
		vz:          0,
		radius:      monsterProjectileRadius(3001),
		height:      monsterProjectileHeight(3001),
		ttl:         1,
		sourceThing: 0,
		sourceType:  3001,
		kind:        projectileFireball,
	}
	p.floorz, p.ceilz = 0, 128*fracUnit

	doomrand.Clear()
	wantNext := doomrand.PRandomOffset(1)
	_, keep := g.advanceProjectile(p)
	if keep {
		t.Fatal("projectile should explode on same-species contact")
	}
	if g.thingHP[1] != 60 {
		t.Fatal("same-species contact must not inflict missile damage")
	}
	if got := doomrand.PRandom(); got != wantNext {
		t.Fatalf("same-species explosion consumed wrong number of PRandom calls: got next=%d want=%d (after exactly one in-impact random, not two)", got, wantNext)
	}
}

func TestPlayerRocketSplashDamagesNearbyBarrel(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: barrelThingType, X: 0, Y: 0},
				{Type: barrelThingType, X: 32, Y: 0},
			},
		},
		thingCollected:  []bool{false, false},
		thingHP:         []int{20, 20},
		thingDead:       []bool{false, false},
		thingState:      []monsterThinkState{monsterStateSpawn, monsterStateSpawn},
		thingStateTics:  []int{6, 6},
		thingStatePhase: []int{0, 0},
		thingDeathTics:  []int{0, 0},
		projectiles: []projectile{
			{
				x:            -30 * fracUnit,
				y:            0,
				z:            20 * fracUnit,
				vx:           24 * fracUnit,
				vy:           0,
				vz:           0,
				radius:       11 * fracUnit,
				height:       8 * fracUnit,
				ttl:          5,
				sourceType:   16,
				sourceThing:  -1,
				sourcePlayer: true,
				kind:         projectileRocket,
			},
		},
	}

	g.tickProjectiles()

	if !g.thingDead[0] {
		t.Fatal("direct-hit barrel should die")
	}
	if !g.thingDead[1] {
		t.Fatal("nearby barrel should die from rocket splash")
	}
}

func TestPlayerRocketSplashCanDamagePlayer(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Health: 100},
		p: player{
			x:      32 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		projectiles: []projectile{
			{
				x:            0,
				y:            0,
				z:            20 * fracUnit,
				vx:           0,
				vy:           0,
				vz:           -32 * fracUnit,
				radius:       11 * fracUnit,
				height:       8 * fracUnit,
				ttl:          1,
				sourceType:   16,
				sourceThing:  -1,
				sourcePlayer: true,
				kind:         projectileRocket,
			},
		},
	}

	g.tickProjectiles()

	if g.stats.Health >= 100 {
		t.Fatalf("health=%d want < 100 after rocket splash", g.stats.Health)
	}
	if got := len(g.projectiles); got != 0 {
		t.Fatalf("projectiles remaining=%d want=0", got)
	}
}
