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
	if !p.sourcePlayer {
		t.Fatal("player rocket should be marked as player-sourced")
	}
	if !hasSoundEvent(g.soundQueue, soundEventShootRocket) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventShootRocket)
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
