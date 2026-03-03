package automap

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
}
