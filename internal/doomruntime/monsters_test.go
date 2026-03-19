package doomruntime

import (
	"fmt"
	"reflect"
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
	if !hasSoundEvent(g.soundQueue, soundEventMonsterSeePosit1) &&
		!hasSoundEvent(g.soundQueue, soundEventMonsterSeePosit2) &&
		!hasSoundEvent(g.soundQueue, soundEventMonsterSeePosit3) {
		t.Fatalf("wake should emit seesound, queue=%v", g.soundQueue)
	}
	tx, ty := g.thingPosFixed(0, g.m.Things[0])
	if tx != int64(g.m.Things[0].X)<<fracBits || ty != int64(g.m.Things[0].Y)<<fracBits {
		t.Fatal("monster should not move on the same tic it wakes")
	}
	if g.thingState[0] != monsterStateSee && g.thingState[0] != monsterStateAttack {
		t.Fatalf("monster state=%d want see-or-attack", g.thingState[0])
	}
	if g.thingStateTics[0] <= 0 {
		t.Fatalf("monster state tics=%d want > 0", g.thingStateTics[0])
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

func TestTickMonstersAmbushNoiseSetsTargetWithoutWakeWhenLOSBlocked(t *testing.T) {
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
		thingTargetPlayer: []bool{false},
		thingTargetIdx:    []int{-1},
		thingCooldown:     []int{0},
		sectorSoundTarget: []bool{true},
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0},
	}
	g.initPhysics()
	g.tickMonsters()
	if g.thingAggro[0] {
		t.Fatal("ambush monster should not wake from blocked sound target")
	}
	if !g.thingTargetPlayer[0] {
		t.Fatal("ambush monster should retain player target from sound target even when it stays asleep")
	}
}

func TestMonsterMoveStepMatchesDoomSpeedTable(t *testing.T) {
	tests := []struct {
		typ  int16
		want int64
	}{
		{3004, 8 * fracUnit},
		{9, 8 * fracUnit},
		{3001, 8 * fracUnit},
		{3002, 10 * fracUnit},
		{58, 10 * fracUnit},
		{3005, 8 * fracUnit},
		{3003, 8 * fracUnit},
		{69, 8 * fracUnit},
		{66, 10 * fracUnit},
		{16, 16 * fracUnit},
		{7, 12 * fracUnit},
		{68, 12 * fracUnit},
		{67, 8 * fracUnit},
		{64, 15 * fracUnit},
		{71, 8 * fracUnit},
		{3006, 8 * fracUnit},
		{84, 8 * fracUnit},
		{65, 8 * fracUnit},
	}
	for _, tt := range tests {
		if got := monsterMoveStep(tt.typ, false); got != tt.want {
			t.Fatalf("type %d speed=%d want=%d", tt.typ, got, tt.want)
		}
	}
}

func TestMonsterAttack_ChaingunnerUsesShotgunSoundLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 65, X: 0, Y: 0},
			},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{70},
		thingTargetPlayer: []bool{true},
		thingTargetIdx:    []int{-1},
		soundQueue:        make([]soundEvent, 0, 4),
		p:                 player{x: 128 * fracUnit, y: 0},
		stats:             playerStats{Health: 100},
	}
	if !g.monsterAttack(0, 65, 128*fracUnit) {
		t.Fatal("chaingunner attack should resolve")
	}
	if !hasSoundEvent(g.soundQueue, soundEventShootShotgun) {
		t.Fatalf("soundQueue=%v missing chaingunner shotgun attack sound", g.soundQueue)
	}
}

func TestMonsterAttackFrameTablesMatchDoomStateTables(t *testing.T) {
	tests := []struct {
		typ      int16
		wantSeq  []byte
		wantTics []int
	}{
		{3004, []byte{'E', 'F', 'E'}, []int{10, 8, 8}},
		{9, []byte{'E', 'F', 'E'}, []int{10, 10, 10}},
		{3001, []byte{'E', 'F', 'G'}, []int{8, 8, 6}},
		{3002, []byte{'E', 'F', 'G'}, []int{8, 8, 8}},
		{58, []byte{'E', 'F', 'G'}, []int{8, 8, 8}},
		{3005, []byte{'B', 'C', 'D'}, []int{5, 5, 5}},
		{3003, []byte{'E', 'F', 'G'}, []int{8, 8, 8}},
		{69, []byte{'E', 'F', 'G'}, []int{8, 8, 8}},
		{64, []byte{'G', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P'}, []int{0, 10, 8, 8, 8, 8, 8, 8, 8, 8, 20}},
		{66, []byte{'H', 'H', 'K', 'K'}, []int{0, 10, 10, 10}},
		{67, []byte{'G', 'H', 'I', 'G', 'H', 'I', 'G', 'H', 'I', 'G'}, []int{20, 10, 5, 5, 10, 5, 5, 10, 5, 5}},
		{68, []byte{'A', 'G', 'H', 'H'}, []int{20, 4, 4, 1}},
		{16, []byte{'E', 'F', 'E', 'F', 'E', 'F'}, []int{6, 12, 12, 12, 12, 12}},
		{71, []byte{'D', 'E', 'F', 'F'}, []int{5, 5, 5, 0}},
		{7, []byte{'A', 'G', 'H', 'H'}, []int{20, 4, 4, 1}},
		{84, []byte{'E', 'F', 'G', 'F', 'G', 'F'}, []int{10, 10, 4, 6, 4, 1}},
	}
	for _, tt := range tests {
		if got := monsterAttackFrameSeq(tt.typ); !reflect.DeepEqual(got, tt.wantSeq) {
			t.Fatalf("type %d attack seq=%v want=%v", tt.typ, got, tt.wantSeq)
		}
		if got := monsterAttackFrameTics(tt.typ); !reflect.DeepEqual(got, tt.wantTics) {
			t.Fatalf("type %d attack tics=%v want=%v", tt.typ, got, tt.wantTics)
		}
	}
}

func TestMonsterSpawnAndSeeFrameTablesMatchDoomStateTables(t *testing.T) {
	spawnTests := []struct {
		typ      int16
		wantSeq  []byte
		wantTics []int
	}{
		{3004, []byte{'A', 'B'}, []int{10, 10}},
		{9, []byte{'A', 'B'}, []int{10, 10}},
		{3001, []byte{'A', 'B'}, []int{10, 10}},
		{3002, []byte{'A', 'B'}, []int{10, 10}},
		{58, []byte{'A', 'B'}, []int{10, 10}},
		{3005, []byte{'A'}, []int{10}},
		{3003, []byte{'A', 'B'}, []int{10, 10}},
		{69, []byte{'A', 'B'}, []int{10, 10}},
		{64, []byte{'A', 'B'}, []int{10, 10}},
		{66, []byte{'A', 'B'}, []int{10, 10}},
		{67, []byte{'A', 'B'}, []int{15, 15}},
		{68, []byte{'A', 'B'}, []int{10, 10}},
		{16, []byte{'A', 'B'}, []int{10, 10}},
		{71, []byte{'A'}, []int{10}},
		{7, []byte{'A', 'B'}, []int{10, 10}},
		{84, []byte{'A', 'B'}, []int{10, 10}},
	}
	for _, tt := range spawnTests {
		if got := monsterSpawnFrameSeq(tt.typ); !reflect.DeepEqual(got, tt.wantSeq) {
			t.Fatalf("type %d spawn seq=%v want=%v", tt.typ, got, tt.wantSeq)
		}
		if got := monsterSpawnFrameTics(tt.typ); !reflect.DeepEqual(got, tt.wantTics) {
			t.Fatalf("type %d spawn tics=%v want=%v", tt.typ, got, tt.wantTics)
		}
	}

	seeTests := []struct {
		typ      int16
		fast     bool
		wantSeq  []byte
		wantTics []int
	}{
		{3004, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{4, 4, 4, 4, 4, 4, 4, 4}},
		{3004, true, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{2, 2, 2, 2, 2, 2, 2, 2}},
		{9, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{3, 3, 3, 3, 3, 3, 3, 3}},
		{9, true, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{2, 2, 2, 2, 2, 2, 2, 2}},
		{3001, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{3, 3, 3, 3, 3, 3, 3, 3}},
		{3002, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{2, 2, 2, 2, 2, 2, 2, 2}},
		{58, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{2, 2, 2, 2, 2, 2, 2, 2}},
		{3005, false, []byte{'A'}, []int{3}},
		{3003, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{3, 3, 3, 3, 3, 3, 3, 3}},
		{69, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{3, 3, 3, 3, 3, 3, 3, 3}},
		{64, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D', 'E', 'E', 'F', 'F'}, []int{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}},
		{66, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D', 'E', 'E', 'F', 'F'}, []int{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}},
		{67, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D', 'E', 'E', 'F', 'F'}, []int{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}},
		{68, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D', 'E', 'E', 'F', 'F'}, []int{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}},
		{16, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{3, 3, 3, 3, 3, 3, 3, 3}},
		{71, false, []byte{'A', 'A', 'B', 'B', 'C', 'C'}, []int{3, 3, 3, 3, 3, 3}},
		{7, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{3, 3, 3, 3, 3, 3, 3, 3}},
		{84, false, []byte{'A', 'A', 'B', 'B', 'C', 'C', 'D', 'D'}, []int{3, 3, 3, 3, 3, 3, 3, 3}},
	}
	for _, tt := range seeTests {
		if got := monsterSeeFrameSeq(tt.typ); !reflect.DeepEqual(got, tt.wantSeq) {
			t.Fatalf("type %d see seq=%v want=%v", tt.typ, got, tt.wantSeq)
		}
		if got := monsterSeeFrameTics(tt.typ, tt.fast); !reflect.DeepEqual(got, tt.wantTics) {
			t.Fatalf("type %d fast=%t see tics=%v want=%v", tt.typ, tt.fast, got, tt.wantTics)
		}
	}
}

func TestMonsterPainFrameTablesMatchDoomStateTables(t *testing.T) {
	tests := []struct {
		typ       int16
		wantSeq   []byte
		wantTics  []int
		wantTotal int
	}{
		{3004, []byte{'G', 'G'}, []int{3, 3}, 6},
		{9, []byte{'G', 'G'}, []int{3, 3}, 6},
		{3001, []byte{'H', 'H'}, []int{2, 2}, 4},
		{3002, []byte{'H', 'H'}, []int{2, 2}, 4},
		{58, []byte{'H', 'H'}, []int{2, 2}, 4},
		{3006, []byte{'E', 'E'}, []int{3, 3}, 6},
		{3005, []byte{'E', 'E', 'F'}, []int{3, 3, 6}, 12},
		{3003, []byte{'H', 'H'}, []int{2, 2}, 4},
		{69, []byte{'H', 'H'}, []int{2, 2}, 4},
		{64, []byte{'Q', 'Q'}, []int{5, 5}, 10},
		{66, []byte{'L', 'L'}, []int{5, 5}, 10},
		{67, []byte{'J', 'J'}, []int{3, 3}, 6},
		{16, []byte{'G'}, []int{10}, 10},
		{7, []byte{'I', 'I'}, []int{3, 3}, 6},
		{68, []byte{'I', 'I'}, []int{3, 3}, 6},
		{71, []byte{'G', 'G'}, []int{6, 6}, 12},
		{84, []byte{'H', 'H'}, []int{3, 3}, 6},
	}
	for _, tt := range tests {
		if got := monsterPainFrameSeq(tt.typ); !reflect.DeepEqual(got, tt.wantSeq) {
			t.Fatalf("type %d pain seq=%v want=%v", tt.typ, got, tt.wantSeq)
		}
		if got := monsterPainFrameTics(tt.typ); !reflect.DeepEqual(got, tt.wantTics) {
			t.Fatalf("type %d pain tics=%v want=%v", tt.typ, got, tt.wantTics)
		}
		if got := monsterPainDurationTics(tt.typ); got != tt.wantTotal {
			t.Fatalf("type %d pain total=%d want=%d", tt.typ, got, tt.wantTotal)
		}
	}
}

func TestMonsterMetadataMatchesVanillaRoster(t *testing.T) {
	tests := []struct {
		typ        int16
		reaction   int
		painChance int
		radius     int64
		height     int64
	}{
		{3004, 8, 200, 20 * fracUnit, 56 * fracUnit},
		{9, 8, 170, 20 * fracUnit, 56 * fracUnit},
		{3001, 8, 200, 20 * fracUnit, 56 * fracUnit},
		{3002, 8, 180, 30 * fracUnit, 56 * fracUnit},
		{58, 8, 180, 30 * fracUnit, 56 * fracUnit},
		{3005, 8, 128, 31 * fracUnit, 56 * fracUnit},
		{3003, 8, 50, 24 * fracUnit, 64 * fracUnit},
		{69, 8, 50, 24 * fracUnit, 64 * fracUnit},
		{3006, 8, 256, 16 * fracUnit, 56 * fracUnit},
		{64, 8, 10, 20 * fracUnit, 56 * fracUnit},
		{66, 8, 100, 20 * fracUnit, 56 * fracUnit},
		{67, 8, 80, 48 * fracUnit, 64 * fracUnit},
		{68, 8, 128, 64 * fracUnit, 64 * fracUnit},
		{16, 8, 20, 40 * fracUnit, 110 * fracUnit},
		{71, 8, 128, 31 * fracUnit, 56 * fracUnit},
		{7, 8, 40, 128 * fracUnit, 100 * fracUnit},
		{84, 8, 170, 20 * fracUnit, 56 * fracUnit},
	}
	for _, tt := range tests {
		if got := monsterReactionTimeTics(tt.typ); got != tt.reaction {
			t.Fatalf("type %d reactiontime=%d want=%d", tt.typ, got, tt.reaction)
		}
		if got := monsterPainChance(tt.typ); got != tt.painChance {
			t.Fatalf("type %d painchance=%d want=%d", tt.typ, got, tt.painChance)
		}
		if got := monsterRadius(tt.typ); got != tt.radius {
			t.Fatalf("type %d radius=%d want=%d", tt.typ, got, tt.radius)
		}
		if got := monsterHeight(tt.typ); got != tt.height {
			t.Fatalf("type %d height=%d want=%d", tt.typ, got, tt.height)
		}
	}
}

func TestPainElementalAttackSpawnsLostSoul(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things:  []mapdata.Thing{{Type: 71, X: 64, Y: 0}},
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		thingCollected:      []bool{false},
		thingDropped:        []bool{false},
		thingHP:             []int{400},
		thingAggro:          []bool{true},
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
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		thingStatePhase:     []int{0},
		thingWorldAnimRef:   []thingAnimRefState{{}},
		thingX:              []int64{64 * fracUnit},
		thingY:              []int64{0},
		thingAngleState:     []uint32{0},
		thingZState:         []int64{0},
		thingFloorState:     []int64{0},
		thingCeilState:      []int64{128 * fracUnit},
		thingSupportValid:   []bool{true},
		thingBlockCell:      []int{-1},
		thingBlockNext:      []int{-1},
		thingSectorCache:    []int{0},
		sectorFloor:         []int64{0},
		sectorCeil:          []int64{128 * fracUnit},
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	if !g.monsterAttack(0, 71, 256*fracUnit) {
		t.Fatal("pain elemental attack should spawn a lost soul")
	}
	if got := len(g.m.Things); got != 2 {
		t.Fatalf("thing count=%d want=2", got)
	}
	if g.m.Things[1].Type != 3006 {
		t.Fatalf("spawned thing type=%d want 3006", g.m.Things[1].Type)
	}
	if !g.thingAggro[1] {
		t.Fatal("spawned lost soul should be active")
	}
}

func TestPainElementalAttackRespectsLostSoulCap(t *testing.T) {
	things := make([]mapdata.Thing, 22)
	collected := make([]bool, 22)
	hp := make([]int, 22)
	aggro := make([]bool, 22)
	angles := make([]uint32, 22)
	thingX := make([]int64, 22)
	thingY := make([]int64, 22)
	thingZ := make([]int64, 22)
	floor := make([]int64, 22)
	ceil := make([]int64, 22)
	support := make([]bool, 22)
	sectors := make([]int, 22)
	for i := range things {
		things[i] = mapdata.Thing{Type: 3006, X: int16(i * 8), Y: 0}
		hp[i] = 100
		aggro[i] = true
		thingX[i] = int64(i*8) * fracUnit
		ceil[i] = 128 * fracUnit
		support[i] = true
	}
	things[0] = mapdata.Thing{Type: 71, X: 0, Y: 0}
	hp[0] = 400
	g := &game{
		m: &mapdata.Map{
			Things:  things,
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		thingCollected:    collected,
		thingHP:           hp,
		thingAggro:        aggro,
		thingAngleState:   angles,
		thingX:            thingX,
		thingY:            thingY,
		thingZState:       thingZ,
		thingFloorState:   floor,
		thingCeilState:    ceil,
		thingSupportValid: support,
		thingSectorCache:  sectors,
		sectorFloor:       []int64{0},
		sectorCeil:        []int64{128 * fracUnit},
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0, z: 0},
	}
	if g.monsterAttack(0, 71, 256*fracUnit) {
		t.Fatal("pain elemental should not spawn above the lost soul cap")
	}
}

func TestPainElementalDeathSpawnsThreeLostSouls(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things:  []mapdata.Thing{{Type: 71, X: 64, Y: 0}},
			Sectors: []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
		},
		thingCollected:      []bool{false},
		thingDropped:        []bool{false},
		thingHP:             []int{10},
		thingAggro:          []bool{true},
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
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		thingStatePhase:     []int{0},
		thingWorldAnimRef:   []thingAnimRefState{{}},
		thingX:              []int64{64 * fracUnit},
		thingY:              []int64{0},
		thingAngleState:     []uint32{0},
		thingZState:         []int64{0},
		thingFloorState:     []int64{0},
		thingCeilState:      []int64{128 * fracUnit},
		thingSupportValid:   []bool{true},
		thingBlockCell:      []int{-1},
		thingBlockNext:      []int{-1},
		thingSectorCache:    []int{0},
		sectorFloor:         []int64{0},
		sectorCeil:          []int64{128 * fracUnit},
		stats:               playerStats{Health: 100},
		p:                   player{x: 0, y: 0, z: 0},
	}
	g.damageMonster(0, 20)
	if got := len(g.m.Things); got != 4 {
		t.Fatalf("thing count=%d want=4 after pain elemental death", got)
	}
	for i := 1; i < 4; i++ {
		if g.m.Things[i].Type != 3006 {
			t.Fatalf("spawned thing %d type=%d want 3006", i, g.m.Things[i].Type)
		}
	}
}

func TestArchvileAttackDamagesAndLaunchesPlayer(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 64, X: 64, Y: 0}},
		},
		stats: playerStats{Health: 100},
		p:     player{x: 0, y: 0, z: 0},
	}
	if !g.monsterAttack(0, 64, 128*fracUnit) {
		t.Fatal("arch-vile attack should succeed with a live target")
	}
	if g.stats.Health != 80 {
		t.Fatalf("health=%d want=80", g.stats.Health)
	}
	if g.p.momz != 10*fracUnit {
		t.Fatalf("momz=%d want=%d", g.p.momz, 10*fracUnit)
	}
}

func TestArchvileRaisesNearbyCorpse(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 64, X: 0, Y: 0},
				{Type: 3004, X: 32, Y: 0},
			},
		},
		thingCollected:      []bool{false, false},
		thingHP:             []int{700, 0},
		thingAggro:          []bool{true, false},
		thingDead:           []bool{false, true},
		thingDeathTics:      []int{0, 12},
		thingPainTics:       []int{0, 3},
		thingAttackTics:     []int{0, 5},
		thingAttackPhase:    []int{0, 0},
		thingAttackFireTics: []int{-1, 2},
		thingReactionTics:   []int{0, 4},
		thingJustAtk:        []bool{false, true},
		thingJustHit:        []bool{false, true},
		thingState:          []monsterThinkState{monsterStateSee, monsterStateDeath},
		thingStateTics:      []int{0, 12},
		thingStatePhase:     []int{0, 0},
	}
	if !g.archvileTryRaiseCorpse(0) {
		t.Fatal("arch-vile should raise a nearby corpse")
	}
	if g.thingDead[1] {
		t.Fatal("corpse should be revived")
	}
	if g.thingHP[1] != monsterSpawnHealth(3004) {
		t.Fatalf("revived hp=%d want=%d", g.thingHP[1], monsterSpawnHealth(3004))
	}
	if g.thingState[1] != monsterStateSee {
		t.Fatalf("state=%d want see", g.thingState[1])
	}
	if g.thingDeathTics[1] != 0 || g.thingPainTics[1] != 0 || g.thingAttackTics[1] != 0 || g.thingAttackFireTics[1] != -1 {
		t.Fatalf("revived state not cleared: death=%d pain=%d attack=%d fire=%d", g.thingDeathTics[1], g.thingPainTics[1], g.thingAttackTics[1], g.thingAttackFireTics[1])
	}
}

func TestArchvileDoesNotRaiseLostSoulOrBossCorpse(t *testing.T) {
	tests := []int16{3006, 16, 7}
	for _, corpseType := range tests {
		t.Run(fmt.Sprintf("corpse_%d", corpseType), func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{
					Things: []mapdata.Thing{
						{Type: 64, X: 0, Y: 0},
						{Type: corpseType, X: 32, Y: 0},
					},
				},
				thingCollected:      []bool{false, false},
				thingHP:             []int{700, 0},
				thingDead:           []bool{false, true},
				thingDeathTics:      []int{0, 10},
				thingAttackFireTics: []int{-1, -1},
				thingState:          []monsterThinkState{monsterStateSee, monsterStateDeath},
				thingStateTics:      []int{0, 10},
				thingStatePhase:     []int{0, 0},
			}
			if g.archvileTryRaiseCorpse(0) {
				t.Fatalf("arch-vile should not raise corpse type %d", corpseType)
			}
			if !g.thingDead[1] {
				t.Fatalf("corpse type %d should remain dead", corpseType)
			}
		})
	}
}

func TestDemoTraceMonsterAttackStateMatchesDoomStateNumbers(t *testing.T) {
	tests := []struct {
		typ  int16
		base int
	}{
		{3004, 184},
		{9, 217},
		{3001, 452},
		{3002, 485},
		{58, 485},
		{3005, 504},
		{3003, 537},
		{69, 566},
	}
	for _, tt := range tests {
		for phase := 0; phase < 3; phase++ {
			got, ok := demoTraceMonsterAttackState(tt.typ, phase)
			if !ok {
				t.Fatalf("type %d phase %d returned no state", tt.typ, phase)
			}
			if want := tt.base + phase; got != want {
				t.Fatalf("type %d phase %d state=%d want=%d", tt.typ, phase, got, want)
			}
		}
	}
}

func TestDemoTraceMonsterSpawnAndSeeStatesMatchDoomStateNumbers(t *testing.T) {
	spawnTests := []struct {
		typ  int16
		base int
		len  int
	}{
		{3004, 174, 2},
		{9, 207, 2},
		{3001, 442, 2},
		{3002, 475, 2},
		{58, 475, 2},
		{3005, 502, 1},
		{3003, 527, 2},
		{69, 556, 2},
	}
	for _, tt := range spawnTests {
		for phase := 0; phase < tt.len; phase++ {
			got, ok := demoTraceMonsterSpawnState(tt.typ, phase)
			if !ok {
				t.Fatalf("type %d spawn phase %d returned no state", tt.typ, phase)
			}
			if want := tt.base + phase; got != want {
				t.Fatalf("type %d spawn phase %d state=%d want=%d", tt.typ, phase, got, want)
			}
		}
	}

	seeTests := []struct {
		typ  int16
		base int
		len  int
	}{
		{3004, 176, 8},
		{9, 209, 8},
		{3001, 444, 8},
		{3002, 477, 8},
		{58, 477, 8},
		{3005, 503, 1},
		{3003, 529, 8},
		{69, 558, 8},
	}
	for _, tt := range seeTests {
		for phase := 0; phase < tt.len; phase++ {
			got, ok := demoTraceMonsterSeeState(tt.typ, phase)
			if !ok {
				t.Fatalf("type %d see phase %d returned no state", tt.typ, phase)
			}
			if want := tt.base + phase; got != want {
				t.Fatalf("type %d see phase %d state=%d want=%d", tt.typ, phase, got, want)
			}
		}
	}
}

func TestDemoTraceMonsterPainStateMatchesDoomStateNumbers(t *testing.T) {
	tests := []struct {
		typ       int16
		remaining int
		want      int
	}{
		{3004, 6, 187},
		{3004, 3, 188},
		{9, 4, 220},
		{9, 2, 221},
		{3001, 4, 455},
		{3001, 2, 456},
		{3002, 4, 488},
		{3005, 12, 507},
		{3005, 9, 508},
		{3005, 6, 509},
		{3003, 4, 540},
		{69, 4, 569},
		{3006, 6, 593},
		{7, 6, 619},
		{16, 10, 690},
	}
	for _, tt := range tests {
		got, ok := demoTraceMonsterPainState(tt.typ, tt.remaining)
		if !ok {
			t.Fatalf("type %d remaining %d returned no state", tt.typ, tt.remaining)
		}
		if got != tt.want {
			t.Fatalf("type %d remaining %d state=%d want=%d", tt.typ, tt.remaining, got, tt.want)
		}
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
		{3002, -1},
		{58, -1},
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

func TestMonsterAttackStateEntrySoundEvent(t *testing.T) {
	tests := []struct {
		typ  int16
		want soundEvent
	}{
		{3002, soundEventMonsterAttackSgt},
		{58, soundEventMonsterAttackSgt},
		{64, soundEventMonsterAttackArchvile},
		{67, soundEventMonsterAttackMancubus},
		{3001, -1},
		{3006, -1},
	}
	for _, tc := range tests {
		if got := monsterAttackStateEntrySoundEvent(tc.typ); got != tc.want {
			t.Fatalf("type=%d entry sound=%v want=%v", tc.typ, got, tc.want)
		}
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
	if g.thingAggro[0] {
		t.Fatal("monster aggro should clear when player is dead")
	}
}

func TestMonsterAttackReturnsFalseWhenPlayerDead(t *testing.T) {
	g := &game{
		stats:  playerStats{Health: 100},
		p:      player{x: 0, y: 0},
		isDead: true,
	}
	if g.monsterAttack(0, 3004, 64*fracUnit) {
		t.Fatal("monster attack should fail when player is dead")
	}
}

func TestMonsterCheckMissileRangeReturnsFalseWhenPlayerDead(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:             playerStats{Health: 100},
		p:                 player{x: 0, y: 0},
		isDead:            true,
		thingJustHit:      []bool{true},
		thingReactionTics: []int{0},
	}
	if g.monsterCheckMissileRange(0, 3004, 128*fracUnit, 128*fracUnit, 0, 0, 0) {
		t.Fatal("missile range should fail when player is dead")
	}
	if !g.thingJustHit[0] {
		t.Fatal("dead-target early return should not consume just-hit state")
	}
}

func TestMonsterCanMeleeReturnsFalseWhenPlayerDead(t *testing.T) {
	g := &game{
		stats:  playerStats{Health: 100},
		p:      player{x: 0, y: 0},
		isDead: true,
	}
	if g.monsterCanMelee(3002, 32*fracUnit, 32*fracUnit, 0, 0, 0) {
		t.Fatal("melee check should fail when player is dead")
	}
}

func TestTickMonstersClearsStaleTargetFlagsWhenPlayerDead(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 32, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{true},
		thingJustAtk:   []bool{true},
		thingJustHit:   []bool{true},
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0},
		isDead:         true,
	}
	g.tickMonsters()
	if g.thingAggro[0] || g.thingJustAtk[0] || g.thingJustHit[0] {
		t.Fatalf("stale target flags not cleared: aggro=%v justAtk=%v justHit=%v", g.thingAggro[0], g.thingJustAtk[0], g.thingJustHit[0])
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
	if _, _, ok := g.tryMoveProbeMonster(0, 3004, 16*fracUnit, 0); ok {
		t.Fatal("monster move should be blocked by another solid monster")
	}
}

func TestDoomSolidMapThingTypes_PlayerBlocked(t *testing.T) {
	for typ := range doomSolidMapThingTypes {
		t.Run(fmt.Sprintf("type_%d", typ), func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{
					Things: []mapdata.Thing{
						{Type: typ, X: 32, Y: 0},
					},
					Sectors: []mapdata.Sector{
						{FloorHeight: 0, CeilingHeight: 128},
					},
				},
				thingCollected: []bool{false},
				thingHP:        []int{20},
				thingDead:      []bool{false},
				p:              player{x: 0, y: 0},
			}
			g.initPhysics()
			if g.tryMove(16*fracUnit, 0) {
				t.Fatalf("player move should be blocked by solid thing type %d", typ)
			}
		})
	}
}

func TestTryMove_PlayerNotBlockedByFilteredSolidThing(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2035, X: 32, Y: 0, Flags: thingFlagNotSingle},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		opts:           Options{SkillLevel: 3, GameMode: gameModeSingle},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p:              player{x: 0, y: 0},
	}
	g.initPhysics()
	if !g.tryMove(16*fracUnit, 0) {
		t.Fatal("player move should ignore filtered solid thing")
	}
}

func TestTryMove_PlayerNotBlockedByUndrawnSolidThing(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 43, X: 32, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p:              player{x: 0, y: 0},
	}
	g.initPhysics()
	if !g.tryMove(16*fracUnit, 0) {
		t.Fatal("player move should ignore solid thing that is not drawable")
	}
}

func TestTryMove_PlayerNotBlockedByDeadBarrel(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: barrelThingType, X: 32, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{0},
		thingDead:      []bool{true},
		p:              player{x: 0, y: 0},
	}
	g.initPhysics()
	if !g.tryMove(16*fracUnit, 0) {
		t.Fatal("player move should pass through dead barrel")
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
	if _, _, ok := g.tryMoveProbeMonster(0, 3004, 8*fracUnit, 0); ok {
		t.Fatal("monster move should be blocked by a step higher than 24 units")
	}
}

func TestDoomSolidMapThingTypes_MonsterBlocked(t *testing.T) {
	for typ := range doomSolidMapThingTypes {
		t.Run(fmt.Sprintf("type_%d", typ), func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{
					Things: []mapdata.Thing{
						{Type: 3002, X: -64, Y: 256},
						{Type: typ, X: -128, Y: 256},
					},
					Sectors: []mapdata.Sector{
						{FloorHeight: 0, CeilingHeight: 128},
					},
				},
				thingCollected: []bool{false, false},
				thingHP:        []int{150, 20},
				thingDead:      []bool{false, false},
			}
			g.initPhysics()
			if _, _, ok := g.tryMoveProbeMonster(0, 3002, -93*fracUnit, 227*fracUnit); ok {
				t.Fatalf("monster move should be blocked by solid thing type %d", typ)
			}
		})
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

func TestMonsterMoveInDir_UsesManualDoorWhenOpeningIsTooShortToFit(t *testing.T) {
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
				{FloorHeight: 0, CeilingHeight: 6},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p:              player{x: -128 * fracUnit, y: 0},
	}
	g.initPhysics()
	beforeX, beforeY := g.thingPosFixed(0, g.m.Things[0])
	if !g.monsterMoveInDir(0, 3004, monsterDirEast) {
		t.Fatal("monster move should succeed by using the manual door")
	}
	if len(g.doors) == 0 {
		t.Fatal("manual door should have been activated")
	}
	afterX, afterY := g.thingPosFixed(0, g.m.Things[0])
	if afterX != beforeX || afterY != beforeY {
		t.Fatalf("monster should stay put while opening door: before=(%d,%d) after=(%d,%d)", beforeX, beforeY, afterX, afterY)
	}
}

func TestMonsterMoveInDir_DoesNotCloseActiveManualDoorForMonster(t *testing.T) {
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
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingDead:      []bool{false},
		p: player{x: -128 * fracUnit, y: 0},
	}
	g.initPhysics()
	g.doors[1] = &doorThinker{
		sector:    1,
		typ:       doorNormal,
		direction: 1,
		speed:     vDoorSpeed,
		topWait:   vDoorWaitTic,
		topHeight: 124 * fracUnit,
	}
	if !g.monsterMoveInDir(0, 3004, monsterDirEast) {
		t.Fatal("monster use of active manual door should still count as success")
	}
	if got := g.doors[1].direction; got != 1 {
		t.Fatalf("monster should not close active manual door, direction=%d", got)
	}
}

func TestMonsterMoveInDir_OrdinaryBlockedMovePreservesMoveDir(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 58, X: -24, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: 0, Y: -64},
				{X: 0, Y: 64},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, Flags: 0, SideNum: [2]int16{0, 1}},
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
		thingHP:        []int{150},
		thingDead:      []bool{false},
		thingMoveDir:   []monsterMoveDir{monsterDirNorthEast},
	}
	g.initPhysics()
	g.thingMoveDir[0] = monsterDirNorthEast
	if g.monsterMoveInDir(0, 58, monsterDirNorthEast) {
		t.Fatal("ordinary blocked move should fail")
	}
	if got := g.thingMoveDir[0]; got != monsterDirNorthEast {
		t.Fatalf("blocked move should preserve old movedir, got %d", got)
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

func TestTickMonstersWakeUpRecomputesTargetPositionBeforeChase(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: -96, Y: -32},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected:    []bool{false},
		thingHP:           []int{20},
		thingAggro:        []bool{false},
		thingMoveDir:      []monsterMoveDir{monsterDirEast},
		thingMoveCount:    []int{0},
		thingReactionTics: []int{0},
		thingThreshold:    []int{0},
		thingState:        []monsterThinkState{monsterStateSpawn},
		thingStateTics:    []int{0},
		thingStatePhase:   []int{0},
		thingLastLook:     []int{0},
		thingX:            []int64{-96 * fracUnit},
		thingY:            []int64{-32 * fracUnit},
		sectorFloor:       []int64{0},
		sectorCeil:        []int64{128 * fracUnit},
		sectorSoundTarget: []bool{true},
		p: player{
			x:      -256 * fracUnit,
			y:      -256 * fracUnit,
			z:      0,
			floorz: 0,
			ceilz:  128 * fracUnit,
		},
		stats: playerStats{Health: 100},
	}

	g.tickMonsters()

	if !g.monsterHasTarget(0) {
		t.Fatal("monster did not acquire player target on wake-up tic")
	}
	if got := g.thingMoveDir[0]; got != monsterDirSouthWest {
		t.Fatalf("movedir=%v want south-west", got)
	}
	x, y := g.thingPosFixed(0, g.m.Things[0])
	if x >= -96*fracUnit || y >= -32*fracUnit {
		t.Fatalf("monster moved to (%d,%d), want movement toward south-west from wake-up tic", x, y)
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

func TestTickMonstersPainExpiryResumesChaseStateSameTic(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 9, X: 0, Y: 0},
			},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{30},
		thingAggro:          []bool{true},
		thingTargetPlayer:   []bool{true},
		thingTargetIdx:      []int{-1},
		thingPainTics:       []int{1},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingReactionTics:   []int{0},
		thingMoveDir:        []monsterMoveDir{monsterDirEast},
		thingMoveCount:      []int{1},
		thingAngleState:     []uint32{degToAngle(0)},
		thingZState:         []int64{0},
		thingFloorState:     []int64{0},
		thingCeilState:      []int64{128 * fracUnit},
		thingSupportValid:   []bool{true},
		thingState:          []monsterThinkState{monsterStatePain},
		thingStateTics:      []int{1},
		thingStatePhase:     []int{0},
		p:                   player{x: -128 * fracUnit, y: 0, z: 0, floorz: 0, ceilz: 128 * fracUnit},
	}

	g.tickMonsters()

	if g.thingPainTics[0] != 0 {
		t.Fatalf("pain tics=%d want 0", g.thingPainTics[0])
	}
	if g.thingState[0] != monsterStateSee {
		t.Fatalf("state=%d want see", g.thingState[0])
	}
	if g.thingStateTics[0] <= 0 {
		t.Fatalf("state tics=%d want > 0", g.thingStateTics[0])
	}
}

func TestTickMonstersDeadMonsterStillSlidesLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 9, X: 0, Y: 0},
			},
			Vertexes: []mapdata.Vertex{
				{X: -128, Y: -128},
				{X: 128, Y: -128},
				{X: 128, Y: 128},
				{X: -128, Y: 128},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, SideNum: [2]int16{0, -1}},
				{V1: 1, V2: 2, SideNum: [2]int16{0, -1}},
				{V1: 2, V2: 3, SideNum: [2]int16{0, -1}},
				{V1: 3, V2: 0, SideNum: [2]int16{0, -1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{-5},
		thingDead:      []bool{true},
		thingDeathTics: []int{3},
		thingMomX:      []int64{2 * fracUnit},
		thingMomY:      []int64{0},
		thingMomZ:      []int64{0},
		thingX:         []int64{0},
		thingY:         []int64{0},
		thingAngleState: []uint32{0},
	}
	g.ensureMonsterAIState()
	g.initPhysics()
	g.tickMonsters()
	if got := g.thingDeathTics[0]; got != 2 {
		t.Fatalf("death tics=%d want=2", got)
	}
	if got := g.thingMomX[0]; got != 0 {
		t.Fatalf("momx=%d want=0 when blocked by test fixture bounds", got)
	}
}

func TestTickMonstersJustAttackedStillTurnsTowardMoveDirLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 9, X: 0, Y: 0},
			},
		},
		thingCollected:      []bool{false},
		thingHP:             []int{30},
		thingAggro:          []bool{true},
		thingTargetPlayer:   []bool{true},
		thingTargetIdx:      []int{-1},
		thingPainTics:       []int{0},
		thingAttackTics:     []int{0},
		thingAttackFireTics: []int{-1},
		thingReactionTics:   []int{0},
		thingMoveDir:        []monsterMoveDir{monsterDirWest},
		thingMoveCount:      []int{0},
		thingJustAtk:        []bool{true},
		thingAngleState:     []uint32{2502785088},
		thingZState:         []int64{0},
		thingFloorState:     []int64{0},
		thingCeilState:      []int64{128 * fracUnit},
		thingSupportValid:   []bool{true},
		thingState:          []monsterThinkState{monsterStateSee},
		thingStateTics:      []int{0},
		thingStatePhase:     []int{0},
		p:                   player{x: -128 * fracUnit, y: 0, z: 0, floorz: 0, ceilz: 128 * fracUnit},
	}

	g.tickMonsters()

	if got := g.thingAngleState[0]; got != 2147483648 {
		t.Fatalf("angle=%d want %d", got, 2147483648)
	}
	if g.thingJustAtk[0] {
		t.Fatal("thingJustAtk should clear after the Doom just-attacked chase gate")
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
		thingCollected:      []bool{false},
		thingHP:             []int{60},
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

	if !g.startMonsterAttackState(0, 3001, true) {
		t.Fatal("expected attack state to start")
	}
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

func TestMonsterHasLOSPlayerUsesRuntimeSupportZ(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3001, X: -64, Y: 0},
			},
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 56, CeilingHeight: 128},
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
		thingX:            []int64{-64 * fracUnit},
		thingY:            []int64{0},
		thingZState:       []int64{64 * fracUnit},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{128 * fracUnit},
		thingSupportValid: []bool{true},
		sectorFloor:       []int64{0, 56 * fracUnit},
		sectorCeil:        []int64{128 * fracUnit, 128 * fracUnit},
		p:                 player{x: 64 * fracUnit, y: 0, z: 0},
	}

	if !g.monsterHasLOSPlayer(3001, g.thingX[0], g.thingY[0]) {
		t.Fatal("LOS should use the monster runtime z rather than the probed floor height")
	}
}
