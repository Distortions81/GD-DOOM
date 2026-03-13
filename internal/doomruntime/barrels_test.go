package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestInitThingCombatStateInitializesBarrelHealthAndState(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: barrelThingType}},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{0},
		thingReactionTics: []int{0},
		thingLastLook:     []int{0},
		thingThinkWait:    []int{0},
		thingState:        []monsterThinkState{monsterStateSee},
		thingStateTics:    []int{0},
		thingStatePhase:   []int{0},
	}

	g.initThingCombatState()

	if got := g.thingHP[0]; got != 20 {
		t.Fatalf("barrel hp=%d want=20", got)
	}
	if got := g.thingState[0]; got != monsterStateSpawn {
		t.Fatalf("barrel state=%v want=%v", got, monsterStateSpawn)
	}
	if got := g.thingStateTics[0]; got < 1 || got > 6 {
		t.Fatalf("barrel spawn tics=%d want in [1,6]", got)
	}
}

func TestLineAttackTargetsBarrelAsShootableThing(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: barrelThingType, X: 64, Y: 0}},
		},
		p:              player{x: 0, y: 0, z: 0},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
	}

	outcome := g.lineAttackTrace(g.playerLineAttackActor(), 0, 128*fracUnit, 0, true)
	if outcome.target.kind != lineAttackTargetThing || outcome.target.idx != 0 {
		t.Fatalf("target=%+v want barrel idx 0", outcome.target)
	}
	if !outcome.spawnPuff || outcome.spawnBlood {
		t.Fatalf("barrel hit should spawn puff only, got puff=%v blood=%v", outcome.spawnPuff, outcome.spawnBlood)
	}
}

func TestBarrelExplosionChainsToNearbyBarrel(t *testing.T) {
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
	}

	g.damageBarrel(0, 20)
	if !g.thingDead[0] {
		t.Fatal("first barrel should be dead after lethal damage")
	}
	if got := g.thingStateTics[0]; got < 1 || got > 5 {
		t.Fatalf("initial barrel death tics=%d want in [1,5]", got)
	}

	for tic := 0; tic < 20 && !g.thingDead[1]; tic++ {
		g.tickMonsters()
	}
	if !g.thingDead[1] {
		t.Fatal("nearby barrel should die from first barrel explosion")
	}

	for tic := 0; tic < 64 && !g.thingCollected[0]; tic++ {
		g.tickMonsters()
	}
	if !g.thingCollected[0] {
		t.Fatal("barrel should be removed after BEXP5 completes")
	}
}

func TestBarrelExplosionDamagesNearbyPlayer(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: barrelThingType, X: 0, Y: 0}},
		},
		stats:          playerStats{Health: 100},
		p:              player{x: 32 * fracUnit, y: 0, z: 0, floorz: 0, ceilz: 128 * fracUnit},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		thingState:     []monsterThinkState{monsterStateSpawn},
		thingStateTics: []int{6},
		thingStatePhase: []int{
			0,
		},
		thingDeathTics: []int{0},
	}

	g.damageBarrel(0, 20)
	for tic := 0; tic < 20 && g.stats.Health == 100; tic++ {
		g.tickMonsters()
	}

	if g.stats.Health >= 100 {
		t.Fatalf("health=%d want < 100 after barrel explosion", g.stats.Health)
	}
}

func TestBarrelExplosionLongChain(t *testing.T) {
	doomrand.Clear()
	things := make([]mapdata.Thing, 6)
	hp := make([]int, 6)
	dead := make([]bool, 6)
	state := make([]monsterThinkState, 6)
	stateTics := make([]int, 6)
	phase := make([]int, 6)
	deathTics := make([]int, 6)
	for i := range things {
		things[i] = mapdata.Thing{Type: barrelThingType, X: int16(i * 32), Y: 0}
		hp[i] = 20
		state[i] = monsterStateSpawn
		stateTics[i] = 6
	}
	g := &game{
		m:               &mapdata.Map{Things: things},
		thingCollected:  make([]bool, len(things)),
		thingHP:         hp,
		thingDead:       dead,
		thingState:      state,
		thingStateTics:  stateTics,
		thingStatePhase: phase,
		thingDeathTics:  deathTics,
	}

	g.damageBarrel(0, 20)
	for tic := 0; tic < 200; tic++ {
		g.tickMonsters()
	}
	for i := range things {
		if !g.thingDead[i] {
			t.Fatalf("barrel %d alive hp=%d", i, g.thingHP[i])
		}
	}
}

func TestProjectileHitsBarrel(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: barrelThingType, X: 0, Y: 0}},
		},
		thingCollected:  []bool{false},
		thingHP:         []int{1},
		thingDead:       []bool{false},
		thingState:      []monsterThinkState{monsterStateSpawn},
		thingStateTics:  []int{6},
		thingStatePhase: []int{0},
		thingDeathTics:  []int{0},
		projectiles: []projectile{
			{
				x:           -30 * fracUnit,
				y:           0,
				z:           20 * fracUnit,
				vx:          24 * fracUnit,
				vy:          0,
				vz:          0,
				radius:      6 * fracUnit,
				height:      8 * fracUnit,
				ttl:         5,
				sourceThing: -1,
				sourceType:  3001,
				kind:        projectileFireball,
			},
		},
	}

	g.tickProjectiles()

	if got := len(g.projectiles); got != 0 {
		t.Fatalf("projectiles remaining=%d want=0", got)
	}
	if !g.thingDead[0] {
		t.Fatal("projectile hit should kill barrel")
	}
	if got := len(g.projectileImpacts); got != 1 {
		t.Fatalf("impact count=%d want=1", got)
	}
}

func TestBarrelRuntimeStateDrivesRenderAndTrace(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: barrelThingType}},
		},
		thingCollected:  []bool{false},
		thingDead:       []bool{true},
		thingStatePhase: []int{3},
		thingStateTics:  []int{10},
		thingDeathTics:  []int{10},
	}

	if got := g.runtimeWorldThingSpriteNameScaled(0, g.m.Things[0], 0, 1); got != "BEXPD0" {
		t.Fatalf("runtime barrel sprite=%q want BEXPD0", got)
	}
	if got := demoTraceThingState(g, 0, barrelThingType); got != barrelStateBEXP4 {
		t.Fatalf("trace barrel state=%d want=%d", got, barrelStateBEXP4)
	}
}
