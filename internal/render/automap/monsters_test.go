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

func TestTickMonstersWakesByRangeAndLOS(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 256, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{false},
		thingCooldown:  []int{0},
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0},
	}
	g.tickMonsters()
	if !g.thingAggro[0] {
		t.Fatal("monster should wake when player is in range and visible")
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
	if g.monsterCheckMissileRange(3004, dist, tx, ty, px, py) {
		t.Fatal("first far-range missile check should fail by random chance")
	}
	// Second random byte is 109, which passes the same threshold.
	if !g.monsterCheckMissileRange(3004, dist, tx, ty, px, py) {
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
