package doomruntime

import (
	"reflect"
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
	if len(g.soundQueue) != 1 || g.soundQueue[0] != soundEventBossBrainSpit {
		t.Fatalf("first spit sounds=%v want [%v]", g.soundQueue, soundEventBossBrainSpit)
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
		{50, 3002},
		{89, 3002},
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
	_ = doomrand.PRandom() // P_SpawnMobj for the teleport fog.
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
	for skill := 1; skill <= 5; skill++ {
		g.opts.SkillLevel = skill
		if !g.thingActiveInSession(1) || !g.thingBlocksInSession(1) {
			t.Fatalf("runtime monster excluded from queries at skill %d", skill)
		}
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
	if got := g.bossSpawnCubes[0].reaction; got != 4 {
		t.Fatalf("reaction before first state action=%d want=4", got)
	}

	for tick := 0; tick < 10; tick++ {
		g.tickBossSpawnCubes()
	}
	if got := len(g.bossSpawnCubes); got != 1 {
		t.Fatalf("cube count after 10 ticks=%d want=1", got)
	}
	if got := len(g.m.Things); got != 2 {
		t.Fatalf("thing count after 10 ticks=%d want=2", got)
	}

	g.tickBossSpawnCubes()
	if got := len(g.bossSpawnCubes); got != 0 {
		t.Fatalf("cube count after 11 ticks=%d want=0", got)
	}
	if got := len(g.m.Things); got != 3 {
		t.Fatalf("thing count after resolve=%d want=3", got)
	}
}

func TestBossBrainDeathWaitsForStateChainBeforeExit(t *testing.T) {
	doomrand.Clear()
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
	if g.levelExitRequested {
		t.Fatal("boss brain death must wait for the death state chain")
	}
	if len(g.soundQueue) != 1 || g.soundQueue[0] != soundEventBossBrainDeath {
		t.Fatalf("death sound=%v want [%v]", g.soundQueue, soundEventBossBrainDeath)
	}
	if len(g.projectiles) != 65 {
		t.Fatalf("brain scream spawned %d rockets, want 65", len(g.projectiles))
	}
	if _, index := doomrand.State(); index != 5 {
		t.Fatalf("brain death RNG index=%d want=5 (65*4+1 draws)", index)
	}
	remaining := g.thingStateTics[0] + 20
	for tic := 1; tic < remaining; tic++ {
		g.tickBossBrain(0, g.m.Things[0])
		if g.levelExitRequested {
			t.Fatalf("brain exited early after %d tics", tic)
		}
	}
	g.tickBossBrain(0, g.m.Things[0])
	if !g.levelExitRequested {
		t.Fatal("S_BRAIN_DIE4 should request the level exit")
	}
}

func TestBossCubeNoClipClampsWithoutExploding(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{FloorHeight: 32, CeilingHeight: 64}}},
		sectorFloor: []int64{32 * fracUnit}, sectorCeil: []int64{64 * fracUnit},
	}
	for _, start := range []struct{ z, vz int64 }{
		{8 * fracUnit, -fracUnit}, {100 * fracUnit, fracUnit},
	} {
		cube := bossSpawnCube{z: start.z, vz: start.vz, stateTics: 3, reaction: 10}
		if !g.advanceBossSpawnCube(&cube) {
			t.Fatal("MF_NOCLIP cube disappeared at a sector plane")
		}
		if cube.z != 32*fracUnit || cube.vz != 0 {
			t.Fatalf("clamped cube z=%d momz=%d", cube.z, cube.vz)
		}
	}
}

func TestBossBrainLooksBeforeLaterPlayerMovementThinker(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things:  []mapdata.Thing{{Type: 89, X: 70, Flags: 7}, {Type: 1}},
			Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		},
		localPlayerThingIndex: 1, localSlot: 1,
		p:     player{momx: 30 * fracUnit, ceilz: 128 * fracUnit},
		stats: playerStats{Health: 100}, playerMobjHealth: 100,
		sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
		thingCollected: []bool{false, false}, thingHP: []int{1000, 100},
		thingStateTics: []int{1, -1}, thingStatePhase: []int{0, 0},
	}
	g.tickThinkers()
	if g.p.x != 30*fracUnit {
		t.Fatalf("player x=%d want=30 units", g.p.x)
	}
	if g.thingStatePhase[0] != 0 {
		t.Fatal("brain saw the player's later movement during its earlier thinker")
	}
	if !g.monsterLookForPlayer(0, false, 70*fracUnit, 0) {
		t.Fatal("fixture must become visible after the player's movement")
	}
}

func TestBossBrainSnapshotRetainsOrderedActors(t *testing.T) {
	cube := bossSpawnCube{order: 105, floorz: 128 * fracUnit, ceilz: 640 * fracUnit,
		x: 2880 * fracUnit, y: 1400 * fracUnit, z: 384 * fracUnit,
		vx: -212950, vy: -619790, vz: -45343, angle: 2995198239,
		lastLook: 2, targetIdx: 8, stateTics: 2, stateStep: 3, reaction: 17}
	fire := bossSpawnFire{order: 106, floorz: 128 * fracUnit, ceilz: 640 * fracUnit,
		x: 2400 * fracUnit, y: 160 * fracUnit, z: 256 * fracUnit, lastLook: 1, tics: 27}
	rocket := projectile{kind: projectileBrainExplosion, order: 107, sourceThing: -1,
		z: 300 * fracUnit, vz: 51200, floorz: 128 * fracUnit, ceilz: 640 * fracUnit,
		frame: 1, frameTics: 6, lastLook: 3, ttl: 1 << 30}
	file := saveFile{Version: saveGameVersion, Game: gameSaveState{
		BossSpawnCubes: captureBossSpawnCubes([]bossSpawnCube{cube}),
		BossSpawnFires: captureBossSpawnFires([]bossSpawnFire{fire}),
		Projectiles:    captureProjectiles([]projectile{rocket}),
	}}
	data, err := encodeSnapshot(saveGameMagic, file)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	if got := restoreBossSpawnCubes(decoded.Game.BossSpawnCubes); !reflect.DeepEqual(got, []bossSpawnCube{cube}) {
		t.Fatalf("cube changed across snapshot: %+v", got)
	}
	if got := restoreBossSpawnFires(decoded.Game.BossSpawnFires); !reflect.DeepEqual(got, []bossSpawnFire{fire}) {
		t.Fatalf("fire changed across snapshot: %+v", got)
	}
	if got := restoreProjectiles(decoded.Game.Projectiles); !reflect.DeepEqual(got, []projectile{rocket}) {
		t.Fatalf("brain rocket changed across snapshot: %+v", got)
	}
}
