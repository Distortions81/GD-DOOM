package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestRadiusAttackInterleavesPlayerAndLostSoulDeathRNG(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	for _, tc := range []struct {
		name                   string
		playerOrder, soulOrder int64
		wantDeathTics          int
	}{
		{"soul before player", 1, 2, 6},
		{"player before soul", 2, 1, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{
					Things:   []mapdata.Thing{{Type: 3006, X: 32}},
					Sectors:  []mapdata.Sector{{CeilingHeight: 128}},
					BlockMap: &mapdata.BlockMap{Width: 1, Height: 1},
				},
				p:                player{x: 64 * fracUnit, ceilz: 128 * fracUnit},
				stats:            playerStats{Health: 200},
				playerMobjHealth: 200,
				thingHP:          []int{1},
				thingCollected:   []bool{false},
				thingBlockOrder:  []int64{tc.soulOrder},
				bmapWidth:        1, bmapHeight: 1,
			}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.thingHP[0] = 1
			g.playerBlockOrder = tc.playerOrder
			g.thingBlockOrder[0] = tc.soulOrder
			g.rebuildThingBlockmap()
			doomrand.Clear()
			g.radiusAttackAt(0, 0, 0, 8*fracUnit, -1, 128, "Explosion", true, -1)
			if !g.thingDead[0] || g.thingStateTics[0] != tc.wantDeathTics {
				t.Fatalf("skull dead=%t first death-frame tics=%d, want true/%d", g.thingDead[0], g.thingStateTics[0], tc.wantDeathTics)
			}
			if g.stats.Health != 120 {
				t.Fatalf("player health=%d, want 120", g.stats.Health)
			}
			if _, prnd := doomrand.State(); prnd != 2 {
				t.Fatalf("RNG=%d, want two damage rolls", prnd)
			}
		})
	}
}

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

func TestCrushedExplodingBarrelKeepsTerminalGibsState(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	g := &game{
		m: &mapdata.Map{
			Things:  []mapdata.Thing{{Type: barrelThingType}},
			Sectors: []mapdata.Sector{{CeilingHeight: 8}},
		},
		thingCollected: []bool{false},
		thingDead:      []bool{false},
		thingHP:        []int{20},
		thingGibbed:    []bool{false},
	}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.thingDead[0], g.thingHP[0] = true, -55
	g.thingState[0], g.thingStatePhase[0], g.thingStateTics[0] = monsterStateDeath, 3, 1
	g.setThingSupportState(0, 0, 0, 8*fracUnit)
	if !g.heightClipThing(0, g.m.Things[0]) || !g.thingGibbed[0] {
		t.Fatal("crusher did not replace the barrel explosion with gibs")
	}
	doomrand.Clear()
	for tic := 0; tic < 30; tic++ {
		g.tickBarrel(0, g.m.Things[0])
	}
	if g.thingCollected[0] || g.thingState[0] != monsterStateGibs || g.thingStateTics[0] != -1 {
		t.Fatalf("crushed barrel collected=%t state=%v tics=%d; want persistent terminal gibs", g.thingCollected[0], g.thingState[0], g.thingStateTics[0])
	}
	if _, prnd := doomrand.State(); prnd != 0 {
		t.Fatalf("crushed barrel resumed its explosion, RNG=%d", prnd)
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

func TestDamageBarrelPreservesNegativeHealthLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: barrelThingType, X: 0, Y: 0}},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{20},
		thingReactionTics: []int{8},
		thingDead:         []bool{false},
		thingState:        []monsterThinkState{monsterStateSpawn},
		thingStateTics:    []int{6},
		thingStatePhase:   []int{0},
		thingDeathTics:    []int{0},
	}

	g.damageBarrel(0, 25)

	if got := g.thingHP[0]; got != -5 {
		t.Fatalf("hp=%d want=-5", got)
	}
	if !g.thingDead[0] {
		t.Fatal("barrel should be dead after lethal damage")
	}
	if got := g.thingReactionTics[0]; got != 8 {
		t.Fatalf("reactiontime=%d want=8", got)
	}
}

func TestTallGreenPillarDoesNotAttractAutoaimOrTakeDamage(t *testing.T) {
	g := &game{
		m:              &mapdata.Map{Things: []mapdata.Thing{{Type: 30, X: 64}, {Type: 3004, X: 128}}},
		thingCollected: []bool{false, false}, thingHP: []int{1000, 20}, thingDead: []bool{false, false},
	}
	_, target, ok := g.aimLineAttackTarget(g.playerLineAttackActor(), 0, doomBulletSlopeRange)
	if !ok || target.kind != lineAttackTargetThing || target.idx != 1 {
		t.Fatalf("autoaim target=%+v found=%t want monster behind pillar", target, ok)
	}
	doomrand.Clear()
	g.damageShootableThingFrom(0, 2000, true, -1, 0, 0, false)
	_, rng := doomrand.State()
	if g.thingHP[0] != 1000 || g.thingDead[0] || rng != 0 {
		t.Fatal("solid pillar took damage or consumed barrel death RNG")
	}
}

func TestDamageShootableThingFrom_PreservesNegativeBarrelHealth(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: barrelThingType, X: 0, Y: 0}},
		},
		thingCollected:  []bool{false},
		thingHP:         []int{20},
		thingDead:       []bool{false},
		thingState:      []monsterThinkState{monsterStateSpawn},
		thingStateTics:  []int{6},
		thingStatePhase: []int{0},
		thingDeathTics:  []int{0},
	}

	g.damageShootableThingFrom(0, 25, true, -1, 0, 0, false)

	if got := g.thingHP[0]; got != -5 {
		t.Fatalf("hp=%d want=-5", got)
	}
	if !g.thingDead[0] {
		t.Fatal("barrel should be dead after lethal damage")
	}
}

func TestThingBlockmapRebuildPreservesNewestFirstOrder(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: barrelThingType, X: 0, Y: 0},
				{Type: barrelThingType, X: 0, Y: 0},
				{Type: barrelThingType, X: 0, Y: 0},
			},
		},
		thingX:          []int64{0, 0, 0},
		thingY:          []int64{0, 0, 0},
		thingBlockOrder: []int64{1, 2, 3},
		thingBlockCell:  []int{-1, -1, -1},
		thingSectorCache: []int{
			-1, -1, -1,
		},
		bmapWidth:       1,
		bmapHeight:      1,
		thingBlockCells: make([][]int, 1),
	}

	g.rebuildThingBlockmap()
	got := g.thingBlockCells[0]
	want := []int{2, 1, 0}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("initial block order=%v want=%v", got, want)
		}
	}

	g.setThingPosFixed(0, 0, 0)
	got = g.thingBlockCells[0]
	want = []int{0, 2, 1}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("relinked block order=%v want=%v", got, want)
		}
	}
}

func TestDeadBarrelRemainsSolidUntilRemoved(t *testing.T) {
	g := &game{
		m:         &mapdata.Map{Things: []mapdata.Thing{{Type: barrelThingType, Flags: 7}}},
		thingDead: []bool{true},
	}
	if !g.thingBlocksInSession(0) {
		t.Fatal("dead barrel must remain solid during its BEXP states")
	}
	g.thingCollected = []bool{true}
	if g.thingBlocksInSession(0) {
		t.Fatal("removed barrel must no longer block")
	}
}

func TestThingBlockmapMovePreservesNewestFirstOrderAcrossCells(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: barrelThingType, X: 0, Y: 0},
				{Type: barrelThingType, X: 0, Y: 0},
				{Type: barrelThingType, X: 160, Y: 0},
			},
		},
		thingX:          []int64{0, 0, 160 * fracUnit},
		thingY:          []int64{0, 0, 0},
		thingBlockOrder: []int64{1, 2, 3},
		thingBlockCell:  []int{-1, -1, -1},
		thingSectorCache: []int{
			-1, -1, -1,
		},
		bmapOriginX:     0,
		bmapOriginY:     0,
		bmapWidth:       2,
		bmapHeight:      1,
		thingBlockCells: make([][]int, 2),
	}

	g.rebuildThingBlockmap()
	if got, want := g.thingBlockCells[0], []int{1, 0}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("cell0 initial order=%v want=%v", got, want)
	}
	if got, want := g.thingBlockCells[1], []int{2}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("cell1 initial order=%v want=%v", got, want)
	}

	g.setThingPosFixed(0, 160*fracUnit, 0)

	if got, want := g.thingBlockCells[0], []int{1}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("cell0 moved order=%v want=%v", got, want)
	}
	if got, want := g.thingBlockCells[1], []int{0, 2}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("cell1 moved order=%v want=%v", got, want)
	}
}

func TestRadiusAttackDoesNotSkipLaterThingsWhenDropRebuildsCell(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 0, Y: 0},
				{Type: 3004, X: 0, Y: 32},
			},
			BlockMap: &mapdata.BlockMap{OriginX: 0, OriginY: 0, Width: 1, Height: 1},
		},
		thingCollected: make([]bool, 2),
		thingDropped:   make([]bool, 2),
		thingHP:        []int{20, 20},
		thingDead:      make([]bool, 2),
		thingDeathTics: make([]int, 2),
		thingState: []monsterThinkState{
			monsterStateSpawn,
			monsterStateSpawn,
		},
		thingStateTics:   []int{10, 10},
		thingStatePhase:  make([]int, 2),
		thingBlockOrder:  []int64{1, 2},
		thingBlockCell:   []int{-1, -1},
		thingBlockCells:  make([][]int, 1),
		thingSectorCache: []int{-1, -1},
		bmapWidth:        1,
		bmapHeight:       1,
	}
	g.rebuildThingBlockmap()

	g.radiusAttackAt(0, 0, 0, 8*fracUnit, -1, 128, "Explosion", true, -1)

	if !g.thingDead[0] || !g.thingDead[1] {
		t.Fatalf("thingDead=%v want both monsters dead", g.thingDead[:2])
	}
	if got := len(g.m.Things); got != 4 {
		t.Fatalf("thing count=%d want=4 after two clip drops", got)
	}
	if !g.thingDropped[2] || !g.thingDropped[3] {
		t.Fatalf("drop flags=%v want dropped clips for both kills", g.thingDropped)
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
			Things:  []mapdata.Thing{{Type: barrelThingType, X: 0, Y: 0}},
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		thingCollected:  []bool{false},
		thingHP:         []int{1},
		thingDead:       []bool{false},
		thingState:      []monsterThinkState{monsterStateSpawn},
		thingStateTics:  []int{6},
		thingStatePhase: []int{0},
		thingDeathTics:  []int{0},
		thingSectorCache: []int{
			0,
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
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
