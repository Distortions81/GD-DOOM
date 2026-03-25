package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

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
	if got := g.projectiles[0].x; got != g.thingX[0] {
		t.Fatalf("projectile x=%d want runtime x=%d", got, g.thingX[0])
	}
	if got := g.projectiles[0].y; got != g.thingY[0] {
		t.Fatalf("projectile y=%d want runtime y=%d", got, g.thingY[0])
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
		worldTic: 4,
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
	blocked, _, _, _, _ := g.projectileBlockedAt(p, p.x, p.y, p.z, p.x+p.vx, p.y+p.vy, p.z+p.vz)
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
	_, hit := g.projectileHitsShootableThingAlongPath(p, p.x, p.y, p.z, p.x+p.vx, p.y+p.vy, p.z+p.vz)
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

	hit, ok := g.projectileHitsShootableThingAlongPath(p, p.x, p.y, p.z, p.x+p.vx, p.y+p.vy, p.z+p.vz)
	if !ok {
		t.Fatal("expected projectile to hit player at destination")
	}
	if !hit.isPlayer {
		t.Fatalf("hit target=%+v want player", hit)
	}
	if hit.frac != 1 {
		t.Fatalf("hit frac=%f want 1 for destination collision", hit.frac)
	}
	if hit.x != p.x+p.vx || hit.y != p.y+p.vy || hit.z != p.z+p.vz {
		t.Fatalf("impact=(%d,%d,%d) want destination (%d,%d,%d)", hit.x, hit.y, hit.z, p.x+p.vx, p.y+p.vy, p.z+p.vz)
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
		x:      -83 * fracUnit,
		y:      -77 * fracUnit,
		z:      120 * fracUnit,
		vx:     -10 * fracUnit,
		vy:     -1 * fracUnit,
		vz:     0,
		radius: monsterProjectileRadius(3001),
		height: monsterProjectileHeight(3001),
		sourceThing: -1,
		kind:   projectileFireball,
	}

	hit, ok := g.projectileHitsShootableThingAlongPath(p, p.x, p.y, p.z, p.x+p.vx, p.y+p.vy, p.z+p.vz)
	if !ok {
		t.Fatal("expected projectile to hit thing at destination overlap")
	}
	if hit.idx != 0 || hit.isPlayer {
		t.Fatalf("hit=%+v want thing 0", hit)
	}
	if hit.x != p.x || hit.y != p.y || hit.z != p.z {
		t.Fatalf("impact=(%d,%d,%d) want old position (%d,%d,%d)", hit.x, hit.y, hit.z, p.x, p.y, p.z)
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
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		thingHP:     []int{60, 60},
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
				vz:           0,
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
