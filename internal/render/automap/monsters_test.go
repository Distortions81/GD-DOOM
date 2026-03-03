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
				{Type: 3004, X: 32, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		thingAggro:     []bool{true},
		thingCooldown:  []int{0},
		stats:          playerStats{Health: 100},
		p:              player{x: 0, y: 0},
	}
	g.tickMonsters()
	if g.stats.Health >= 100 {
		t.Fatalf("health=%d want < 100", g.stats.Health)
	}
	if g.thingCooldown[0] == 0 {
		t.Fatal("monster attack should set cooldown")
	}
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
	g.moveMonsterToward(0, 0, 0, 128*fracUnit, 0, 8*fracUnit)
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
