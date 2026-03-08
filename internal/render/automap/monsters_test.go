package automap

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestTickMonstersDamagesPlayer(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3002, X: 32, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{150},
		thingAggro:     []bool{true},
		thingCooldown:  []int{0},
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0},
	}
	g.tickMonsters()
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100 before melee windup resolves", g.stats.Health)
	}
	for i := 0; i < 20; i++ {
		g.tickMonsters()
		if g.stats.Health < 100 {
			return
		}
	}
	t.Fatalf("health=%d want < 100 after melee windup", g.stats.Health)
}

func TestTickMonstersWakesWhenPlayerInFrontAndVisible(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 256, Y: 0, Angle: 180},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{false},
		thingCooldown:  []int{0},
		soundQueue:     make([]soundEvent, 0, 4),
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0},
	}
	g.tickMonsters()
	if !g.thingAggro[0] {
		t.Fatal("monster should wake when player is in range and visible")
	}
	if !hasSoundEvent(g.soundQueue, soundEventMonsterSeePosit) {
		t.Fatalf("wake should emit seesound, queue=%v", g.soundQueue)
	}
	tx, ty := g.thingPosFixed(0, g.m.Things[0])
	if tx != int64(g.m.Things[0].X)<<fracBits || ty != int64(g.m.Things[0].Y)<<fracBits {
		t.Fatal("monster should not move on the same tic it wakes")
	}
}

func TestTickMonstersDoesNotWakeWhenPlayerBehindAndFar(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 256, Y: 0, Angle: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{false},
		thingCooldown:  []int{0},
		soundQueue:     make([]soundEvent, 0, 4),
		stats:          playerStats{Health: 100},
		p:              player{x: -256 * fracUnit, y: 0},
	}
	g.tickMonsters()
	if g.thingAggro[0] {
		t.Fatal("monster should not wake when player is behind and outside melee range")
	}
	if len(g.soundQueue) != 0 {
		t.Fatalf("behind wake should not emit seesound, queue=%v", g.soundQueue)
	}
}

func TestTickMonstersWakesWhenPlayerBehindButClose(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 32, Y: 0, Angle: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{false},
		thingCooldown:  []int{0},
		soundQueue:     make([]soundEvent, 0, 4),
		stats:          playerStats{Health: 100},
		p:              player{x: -16 * fracUnit, y: 0},
	}
	g.tickMonsters()
	if !g.thingAggro[0] {
		t.Fatal("monster should wake when player is behind but within melee range")
	}
}

func TestTickMonstersWakesByNoiseWithoutLOSForNonAmbush(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 2048, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: 1024, Y: -64},
				{X: 1024, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Flags: mlBlocking, SideNum: [2]int16{0, -1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{20},
		thingAggro:        []bool{false},
		thingCooldown:     []int{0},
		sectorSoundTarget: []bool{true},
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0},
	}
	g.initPhysics()
	g.tickMonsters()
	if !g.thingAggro[0] {
		t.Fatal("non-ambush monster should wake from sector sound target without direct LOS")
	}
}

func TestTickMonstersAmbushDoesNotWakeFromNoiseWithoutLOS(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 2048, Y: 0, Flags: thingFlagAmbush},
			},
			Vertexes: []mapdata.Vertex{
				{X: 1024, Y: -64},
				{X: 1024, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Flags: mlBlocking, SideNum: [2]int16{0, -1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{20},
		thingAggro:        []bool{false},
		thingCooldown:     []int{0},
		sectorSoundTarget: []bool{true},
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0},
	}
	g.initPhysics()
	g.tickMonsters()
	if g.thingAggro[0] {
		t.Fatal("ambush monster should not wake from noise without direct LOS")
	}
}

func TestShouldEmitMonsterActiveSound_DoomChance(t *testing.T) {
	if !shouldEmitMonsterActiveSound(0) {
		t.Fatal("0 should emit")
	}
	if !shouldEmitMonsterActiveSound(2) {
		t.Fatal("2 should emit")
	}
	if shouldEmitMonsterActiveSound(3) {
		t.Fatal("3 should not emit")
	}
}

func TestMonsterMeleeAttackSoundEvent(t *testing.T) {
	tests := []struct {
		typ  int16
		want soundEvent
	}{
		{3001, soundEventMonsterAttackClaw},
		{3003, soundEventMonsterAttackClaw},
		{69, soundEventMonsterAttackClaw},
		{3002, soundEventMonsterAttackSgt},
		{58, soundEventMonsterAttackSgt},
		{3006, soundEventMonsterAttackSkull},
	}
	for _, tc := range tests {
		if got := monsterMeleeAttackSoundEvent(tc.typ); got != tc.want {
			t.Fatalf("type=%d melee sound=%v want=%v", tc.typ, got, tc.want)
		}
	}
	if got := monsterMeleeAttackSoundEvent(66); got != -1 {
		t.Fatalf("revenant melee sound=%v want none", got)
	}
}

func TestPropagateSectorNoise_StopsAfterSecondSoundBlock(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: -64},
				{X: 0, Y: 64},
				{X: 128, Y: -64},
				{X: 128, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Flags: mlTwoSided | lineSoundBlock, SideNum: [2]int16{0, 1}},
				{V1: 2, V2: 3, Flags: mlTwoSided | lineSoundBlock, SideNum: [2]int16{2, 3}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0}, {Sector: 1}, {Sector: 1}, {Sector: 2},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		sectorSoundTarget: make([]bool, 3),
	}
	g.initPhysics()
	best := []int{-1, -1, -1}
	g.propagateSectorNoise(0, 0, best)
	if !g.sectorSoundTarget[0] || !g.sectorSoundTarget[1] {
		t.Fatal("noise should cross the first sound-block line")
	}
	if g.sectorSoundTarget[2] {
		t.Fatal("noise should not cross a second sound-block line")
	}
}

func TestTickMonstersNoActionWhenPlayerDead(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 32, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{true},
		thingCooldown:  []int{0},
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0},
		isDead:         true,
	}
	g.tickMonsters()
	if g.stats.Health != 100 {
		t.Fatalf("dead player health changed to %d", g.stats.Health)
	}
}

func TestMoveMonsterTowardDoesNotMovePlayer(t *testing.T) {
	g := &game{
		p: player{
			x:      100 * fracUnit,
			y:      200 * fracUnit,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}
	px0, py0 := g.p.x, g.p.y
	g.moveMonsterToward(0, 3004, 0, 0, 128*fracUnit, 0, 8*fracUnit)
	if g.p.x != px0 || g.p.y != py0 {
		t.Fatalf("player moved by monster path probe: (%d,%d) -> (%d,%d)", px0, py0, g.p.x, g.p.y)
	}
}

func TestMonsterTryMoveProbe_RespectsBlockMonstersFlag(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: 64},
				{X: 0, Y: -64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Flags: mlTwoSided | mlBlockMonsters, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		p: player{x: -32 * fracUnit, y: 0, z: 0, floorz: 0, ceilz: 128 * fracUnit},
	}
	g.initPhysics()
	if !g.tryMove(-8*fracUnit, 0) {
		t.Fatal("player should not be blocked by block-monsters line")
	}
	if g.tryMoveProbe(8*fracUnit, 0) {
		t.Fatal("monster probe should be blocked by block-monsters line")
	}
}

func TestTryMove_PlayerBlockedByMonster(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3002, X: 32, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{150},
		thingDead:      []bool{false},
		p:              player{x: 0, y: 0},
	}
	g.initPhysics()
	if g.tryMove(16*fracUnit, 0) {
		t.Fatal("player move should be blocked by solid monster")
	}
}

func TestTryMoveProbeMonster_BlockedByOtherMonster(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 0, Y: 0},
				{Type: 3004, X: 32, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false, false},
		thingHP:        []int{20, 20},
		thingDead:      []bool{false, false},
	}
	g.initPhysics()
	if g.tryMoveProbeMonster(0, 3004, 16*fracUnit, 0) {
		t.Fatal("monster move should be blocked by another solid monster")
	}
}

func TestTryMoveProbeMonster_BlockedByHighStep(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: -24, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: 64},
				{X: 0, Y: -64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Flags: mlTwoSided, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 32, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p:              player{x: -128 * fracUnit, y: 0},
	}
	g.initPhysics()
	if g.tryMoveProbeMonster(0, 3004, 8*fracUnit, 0) {
		t.Fatal("monster move should be blocked by a step higher than 24 units")
	}
}

func TestMonsterMoveInDir_UsesManualDoor(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: -24, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: -64},
				{X: 0, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Special: 1, Flags: mlTwoSided, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p:              player{x: -128 * fracUnit, y: 0},
	}
	g.initPhysics()
	if !g.monsterMoveInDir(0, 3004, monsterDirEast) {
		t.Fatal("monster move should succeed by opening manual door")
	}
	if len(g.doors) == 0 {
		t.Fatal("manual door should have been activated")
	}
}

func TestMonsterMoveInDir_DoesNotUseSecretDoor(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: -32, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: -64},
				{X: 0, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Special: 1, Flags: mlTwoSided | mlSecret, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
	}
	g.initPhysics()
	if g.monsterMoveInDir(0, 3004, monsterDirEast) {
		t.Fatal("monster move should not use secret manual door")
	}
	if len(g.doors) != 0 {
		t.Fatal("secret door should not have been activated")
	}
}

func TestMonsterMoveInDir_TriggersWalkDoorRaise(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: -4, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: 64},
				{X: 0, Y: -64},
				{X: 128, Y: 64},
				{X: 128, Y: -64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Special: 4, Flags: mlTwoSided, Tag: 7, SideNum: [2]int16{0, 1}},
				{V1: 2, V2: 3, Flags: mlTwoSided, SideNum: [2]int16{2, 3}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
				{Sector: 2},
				{Sector: 3},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 0, Tag: 7},
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p:              player{x: -128 * fracUnit, y: 0},
	}
	g.initPhysics()
	if !g.monsterMoveInDir(0, 3004, monsterDirEast) {
		t.Fatal("monster should move across walk door line")
	}
	if len(g.doors) == 0 {
		t.Fatal("walk door raise should have activated")
	}
}

func TestMonsterMoveInDir_DoesNotTriggerWalkStairs(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: -4, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: 64},
				{X: 0, Y: -64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Special: 8, Flags: mlTwoSided, Tag: 7, SideNum: [2]int16{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 128, Tag: 7, FloorPic: "STEP1"},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p:              player{x: -128 * fracUnit, y: 0},
	}
	g.initPhysics()
	if !g.monsterMoveInDir(0, 3004, monsterDirEast) {
		t.Fatal("monster should move across non-blocking walk line")
	}
	if len(g.floors) != 0 {
		t.Fatal("monster should not trigger walk stairs special")
	}
}

func TestActorHasLOS_BlockedByHighWindow(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 96, CeilingHeight: 128},
			},
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
		sectorFloor: []int64{0, 96 * fracUnit},
		sectorCeil:  []int64{128 * fracUnit, 128 * fracUnit},
	}
	if g.actorHasLOS(-64*fracUnit, 0, 0, 56*fracUnit, 64*fracUnit, 0, 0, 56*fracUnit) {
		t.Fatal("LOS should be blocked when only a high window is open above both actors")
	}
}

func TestMeleeOnlyMonsterDoesNotRangedAttack(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Health: 100},
	}
	// Farther than melee range, demon should not perform ranged attacks.
	if g.monsterAttack(0, 3002, 400*fracUnit) {
		t.Fatal("demon should not perform ranged attack")
	}
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100", g.stats.Health)
	}
}

func TestMonsterCheckMissileRangeUsesDoomDistanceChance(t *testing.T) {
	doomrand.Clear()
	g := &game{}
	tx := int64(300 * fracUnit)
	ty := int64(0)
	px := int64(0)
	py := int64(0)
	dist := int64(300 * fracUnit)

	// First random byte is 8, which is below the computed threshold here.
	if g.monsterCheckMissileRange(0, 3004, dist, tx, ty, px, py) {
		t.Fatal("first far-range missile check should fail by random chance")
	}
	// Second random byte is 109, which passes the same threshold.
	if !g.monsterCheckMissileRange(0, 3004, dist, tx, ty, px, py) {
		t.Fatal("second far-range missile check should pass by random chance")
	}
}

func TestMonsterPickNewChaseDirMovesTowardTarget(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 0, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingMoveDir:   []monsterMoveDir{monsterDirNoDir},
		thingMoveCount: []int{0},
		sectorFloor:    []int64{0},
		sectorCeil:     []int64{128 * fracUnit},
		p: player{
			x:      128 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
	}

	g.monsterPickNewChaseDir(0, 3004, g.p.x, g.p.y)
	if g.thingMoveDir[0] != monsterDirEast {
		t.Fatalf("movedir=%v want east", g.thingMoveDir[0])
	}
	if g.m.Things[0].X <= 0 {
		t.Fatalf("monster did not move east, x=%d", g.m.Things[0].X)
	}
	if g.thingMoveCount[0] < 0 || g.thingMoveCount[0] > 15 {
		t.Fatalf("movecount=%d want [0,15]", g.thingMoveCount[0])
	}
}

func TestTickMonstersJustAttackedSkipsAttackTic(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 64, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{true},
		thingCooldown:  []int{0},
		thingMoveDir:   []monsterMoveDir{monsterDirNoDir},
		thingMoveCount: []int{0},
		thingJustAtk:   []bool{true},
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0},
	}
	g.tickMonsters()
	if g.thingJustAtk[0] {
		t.Fatal("just-attacked flag should clear after skip tic")
	}
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100 on post-attack skip tic", g.stats.Health)
	}
}

func TestZombiemanChaseCadenceMatchesRunTics(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 0, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{true},
		thingCooldown:  []int{200},
		thingMoveDir:   []monsterMoveDir{monsterDirNoDir},
		thingMoveCount: []int{0},
		thingJustAtk:   []bool{false},
		thingThinkWait: []int{0},
		sectorFloor:    []int64{0},
		sectorCeil:     []int64{128 * fracUnit},
		p: player{
			x:      256 * fracUnit,
			y:      0,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		stats: playerStats{Health: 100},
	}

	g.tickMonsters()
	x1 := g.m.Things[0].X
	if x1 <= 0 {
		t.Fatalf("expected first chase tic to move, x=%d", x1)
	}

	for i := 0; i < 3; i++ {
		g.tickMonsters()
		if g.m.Things[0].X != x1 {
			t.Fatalf("zombieman moved during wait tic %d: got x=%d want %d", i+1, g.m.Things[0].X, x1)
		}
	}

	g.tickMonsters()
	if g.m.Things[0].X <= x1 {
		t.Fatalf("expected move again on 4th tic, x=%d start=%d", g.m.Things[0].X, x1)
	}
}

func TestDamageMonsterTriggersPainStateForAlwaysPainMonster(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3006, X: 0, Y: 0}, // lost soul (pain chance 256)
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{100},
		thingAggro:     []bool{false},
		thingPainTics:  []int{0},
	}
	g.damageMonster(0, 1)
	if g.thingHP[0] != 99 {
		t.Fatalf("hp=%d want=99", g.thingHP[0])
	}
	if g.thingPainTics[0] <= 0 {
		t.Fatalf("pain tics=%d want > 0", g.thingPainTics[0])
	}
}

func TestTickMonstersPainStatePausesThinker(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 0, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{true},
		thingCooldown:  []int{0},
		thingMoveDir:   []monsterMoveDir{monsterDirEast},
		thingMoveCount: []int{0},
		thingPainTics:  []int{3},
		sectorFloor:    []int64{0},
		sectorCeil:     []int64{128 * fracUnit},
		p: player{
			x: 256 * fracUnit,
			y: 0,
		},
		stats: playerStats{Health: 100},
	}
	x0 := g.m.Things[0].X
	g.tickMonsters()
	if g.thingPainTics[0] != 2 {
		t.Fatalf("pain tics=%d want=2", g.thingPainTics[0])
	}
	if g.m.Things[0].X != x0 {
		t.Fatalf("monster moved during pain state: x=%d start=%d", g.m.Things[0].X, x0)
	}
	if g.stats.Health != 100 {
		t.Fatalf("monster attacked during pain state: health=%d", g.stats.Health)
	}
}

func TestImpProjectileAttackHasDoomWindup(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: 72, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{60},
		thingAggro:     []bool{true},
		thingMoveDir:   []monsterMoveDir{monsterDirNoDir},
		thingMoveCount: []int{0},
		thingJustAtk:   []bool{false},
		projectiles:    make([]projectile, 0, 2),
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0, z: 0},
	}

	g.tickMonsters()
	if got := len(g.projectiles); got != 0 {
		t.Fatalf("projectiles=%d want=0 before imp windup resolves", got)
	}
	for i := 0; i < 15; i++ {
		g.tickMonsters()
	}
	if got := len(g.projectiles); got != 0 {
		t.Fatalf("projectiles=%d want=0 before fire tic", got)
	}
	g.tickMonsters()
	if got := len(g.projectiles); got != 1 {
		t.Fatalf("projectiles=%d want=1 after windup", got)
	}
}

func TestMonsterCanTryMissileNow_BlocksNegativeMoveCount(t *testing.T) {
	g := &game{
		thingMoveCount: []int{-1},
	}
	if g.monsterCanTryMissileNow(0) {
		t.Fatal("missile attempt should be blocked when movecount is negative")
	}
}

func TestMonsterCheckMissileRange_RespectsReactionTime(t *testing.T) {
	doomrand.Clear()
	g := &game{
		thingReactionTics: []int{2},
	}
	tx := int64(300 * fracUnit)
	ty := int64(0)
	px := int64(0)
	py := int64(0)
	dist := int64(300 * fracUnit)
	if g.monsterCheckMissileRange(0, 3001, dist, tx, ty, px, py) {
		t.Fatal("missile check should fail while reactiontime > 0")
	}
}
