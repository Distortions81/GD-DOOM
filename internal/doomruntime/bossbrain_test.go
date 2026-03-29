package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestBossBrainSpitCyclesTargets(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 89, X: 0, Y: 0},
				{Type: 87, X: 64, Y: 0},
				{Type: 87, X: 128, Y: 0},
			},
		},
		opts:           Options{SkillLevel: 3},
		thingCollected: []bool{false, false, false},
	}
	if !g.bossBrainSpit(0) {
		t.Fatal("first boss brain spit should succeed")
	}
	if len(g.bossSpawnCubes) != 1 || g.bossSpawnCubes[0].targetIdx != 1 {
		t.Fatalf("first cube target=%v want 1", g.bossSpawnCubes)
	}
	if len(g.soundQueue) != 2 || g.soundQueue[0] != soundEventBossBrainSpit || g.soundQueue[1] != soundEventBossBrainCube {
		t.Fatalf("first spit sounds=%v want [%v %v]", g.soundQueue, soundEventBossBrainSpit, soundEventBossBrainCube)
	}
	if !g.bossBrainSpit(0) {
		t.Fatal("second boss brain spit should succeed")
	}
	if len(g.bossSpawnCubes) != 2 || g.bossSpawnCubes[1].targetIdx != 2 {
		t.Fatalf("second cube target=%v want 2", g.bossSpawnCubes)
	}
}

func TestBossBrainSpitAlternatesOnEasySkill(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 89, X: 0, Y: 0},
				{Type: 87, X: 64, Y: 0},
			},
		},
		opts:           Options{SkillLevel: 1},
		thingCollected: []bool{false, false},
	}
	if !g.bossBrainSpit(0) {
		t.Fatal("first easy-skill spit should fire")
	}
	if g.bossBrainSpit(0) {
		t.Fatal("second easy-skill spit should be skipped")
	}
	if len(g.bossSpawnCubes) != 1 {
		t.Fatalf("cube count=%d want=1", len(g.bossSpawnCubes))
	}
	if !g.bossBrainSpit(0) {
		t.Fatal("third easy-skill spit should fire again")
	}
	if len(g.bossSpawnCubes) != 2 {
		t.Fatalf("cube count=%d want=2", len(g.bossSpawnCubes))
	}
}

func TestBossBrainSpawnTypeMatchesVanillaBuckets(t *testing.T) {
	tests := []struct {
		r    int
		want int16
	}{
		{0, 3001},
		{49, 3001},
		{50, 9},
		{89, 9},
		{90, 58},
		{119, 58},
		{120, 71},
		{129, 71},
		{130, 3005},
		{159, 3005},
		{160, 64},
		{161, 64},
		{162, 66},
		{171, 66},
		{172, 68},
		{191, 68},
		{192, 67},
		{221, 67},
		{222, 69},
		{245, 69},
		{246, 3003},
		{255, 3003},
	}
	for _, tt := range tests {
		if got := bossBrainSpawnType(tt.r); got != tt.want {
			t.Fatalf("random=%d type=%d want=%d", tt.r, got, tt.want)
		}
	}
}

func TestBossCubeResolvesIntoSpawnedMonster(t *testing.T) {
	doomrand.Clear()
	want := bossBrainSpawnType(doomrand.PRandom())
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 87, X: 96, Y: 0},
			},
		},
		thingCollected:      []bool{false},
		thingDropped:        []bool{false},
		thingX:              []int64{96 * fracUnit},
		thingY:              []int64{0},
		thingAngleState:     []uint32{0},
		thingZState:         []int64{0},
		thingFloorState:     []int64{0},
		thingCeilState:      []int64{128 * fracUnit},
		thingSupportValid:   []bool{true},
		thingBlockCell:      []int{-1},
		thingHP:             []int{1000},
		thingAggro:          []bool{false},
		thingCooldown:       []int{0},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{false},
		thingJustHit:        []bool{false},
		thingReactionTics:   []int{0},
		thingWakeTics:       []int{0},
		thingLastLook:       []int{0},
		thingDead:           []bool{false},
		thingDeathTics:      []int{0},
		thingAttackTics:     []int{0},
		thingAttackPhase:    []int{0},
		thingAttackFireTics: []int{-1},
		thingPainTics:       []int{0},
		thingThinkWait:      []int{0},
		thingState:          []monsterThinkState{monsterStateSpawn},
		thingStateTics:      []int{0},
		thingStatePhase:     []int{0},
		thingWorldAnimRef:   []thingAnimRefState{{}},
		thingSectorCache:    []int{0},
		sectorFloor:         []int64{0},
		sectorCeil:          []int64{128 * fracUnit},
	}
	g.resolveBossCube(bossSpawnCube{targetIdx: 0})
	if len(g.m.Things) != 2 {
		t.Fatalf("thing count=%d want=2", len(g.m.Things))
	}
	if len(g.bossSpawnFires) != 1 {
		t.Fatalf("spawn fire count=%d want=1", len(g.bossSpawnFires))
	}
	if len(g.soundQueue) != 1 || g.soundQueue[0] != soundEventTeleport {
		t.Fatalf("sound queue=%v want [%v]", g.soundQueue, soundEventTeleport)
	}
	if g.m.Things[1].Type != want {
		t.Fatalf("spawned type=%d want=%d", g.m.Things[1].Type, want)
	}
	if !g.thingAggro[1] {
		t.Fatal("spawned monster should be active")
	}
}

func TestBossCubeSpawnUsesDoomSpawnActionCadence(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 89, X: 0, Y: 0},
				{Type: 87, X: 0, Y: 120},
			},
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		opts:                Options{SkillLevel: 3},
		thingCollected:      []bool{false, false},
		thingX:              []int64{0, 0},
		thingY:              []int64{0, 120 * fracUnit},
		thingZState:         []int64{0, 0},
		thingFloorState:     []int64{0, 0},
		thingCeilState:      []int64{128 * fracUnit, 128 * fracUnit},
		thingSupportValid:   []bool{true, true},
		thingSectorCache:    []int{0, 0},
		thingBlockCell:      []int{-1, -1},
		thingHP:             []int{1000, 1000},
		thingAggro:          []bool{false, false},
		thingCooldown:       []int{0, 0},
		thingMoveDir:        []monsterMoveDir{monsterDirNoDir, monsterDirNoDir},
		thingMoveCount:      []int{0, 0},
		thingJustAtk:        []bool{false, false},
		thingJustHit:        []bool{false, false},
		thingReactionTics:   []int{0, 0},
		thingWakeTics:       []int{0, 0},
		thingLastLook:       []int{0, 0},
		thingDead:           []bool{false, false},
		thingDeathTics:      []int{0, 0},
		thingAttackTics:     []int{0, 0},
		thingAttackPhase:    []int{0, 0},
		thingAttackFireTics: []int{-1, -1},
		thingPainTics:       []int{0, 0},
		thingThinkWait:      []int{0, 0},
		thingState:          []monsterThinkState{monsterStateSpawn, monsterStateSpawn},
		thingStateTics:      []int{0, 0},
		thingStatePhase:     []int{0, 0},
		thingWorldAnimRef:   []thingAnimRefState{{}, {}},
		sectorFloor:         []int64{0},
		sectorCeil:          []int64{128 * fracUnit},
	}

	if !g.spawnBossCube(0, 1) {
		t.Fatal("spawnBossCube should succeed")
	}
	if got := len(g.bossSpawnCubes); got != 1 {
		t.Fatalf("cube count=%d want=1", got)
	}
	if got := g.bossSpawnCubes[0].reaction; got != 3 {
		t.Fatalf("reaction after initial A_SpawnSound=%d want=3", got)
	}

	for tick := 0; tick < 8; tick++ {
		g.tickBossSpawnCubes()
	}
	if got := len(g.bossSpawnCubes); got != 1 {
		t.Fatalf("cube count after 8 ticks=%d want=1", got)
	}
	if got := len(g.m.Things); got != 2 {
		t.Fatalf("thing count after 8 ticks=%d want=2", got)
	}

	g.tickBossSpawnCubes()
	if got := len(g.bossSpawnCubes); got != 0 {
		t.Fatalf("cube count after 9 ticks=%d want=0", got)
	}
	if got := len(g.m.Things); got != 3 {
		t.Fatalf("thing count after resolve=%d want=3", got)
	}
}

func TestBossBrainDeathRequestsExit(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Name:   "MAP30",
			Things: []mapdata.Thing{{Type: 88, X: 0, Y: 0}},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{1},
		thingAggro:          []bool{false},
		thingJustHit:        []bool{false},
		thingDead:           []bool{false},
		thingDeathTics:      []int{0},
		thingPainTics:       []int{0},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingState:          []monsterThinkState{monsterStateSpawn},
		thingStateTics:      []int{0},
		thingStatePhase:     []int{0},
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0},
	}
	g.damageMonster(0, 10)
	if !g.levelExitRequested {
		t.Fatal("boss brain death should request a level exit")
	}
	if len(g.soundQueue) != 1 || g.soundQueue[0] != soundEventBossBrainDeath {
		t.Fatalf("death sound=%v want [%v]", g.soundQueue, soundEventBossBrainDeath)
	}
}
