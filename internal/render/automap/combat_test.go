package automap

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestPickHitscanMonsterTarget(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 64, Y: 0},
				{Type: 3004, X: 96, Y: 32},
			},
		},
		thingCollected: []bool{false, false},
		thingHP:        []int{20, 20},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
	}
	idx, ok := g.pickHitscanMonsterTarget()
	if !ok {
		t.Fatal("expected a target")
	}
	if idx != 0 {
		t.Fatalf("target idx=%d want=0", idx)
	}
}

func TestHandleFireConsumesAmmoAndDamages(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, X: 64, Y: 0}},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
		stats:          playerStats{Bullets: 10, Health: 100},
		inventory:      playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.handleFire()
	if g.stats.Bullets != 9 {
		t.Fatalf("bullets=%d want=9", g.stats.Bullets)
	}
	if g.thingHP[0] >= 20 {
		t.Fatalf("monster hp=%d want < 20", g.thingHP[0])
	}
}

func TestHandleFireNoAmmoFallsBackToFist(t *testing.T) {
	g := &game{
		stats:     playerStats{Bullets: 0},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.handleFire()
	if g.inventory.ReadyWeapon != weaponFist {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponFist)
	}
}

func TestShotgunConsumesShellAndDealsPelletDamage(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, X: 64, Y: 0}},
		},
		thingCollected: []bool{false},
		thingHP:        []int{40},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
		stats:          playerStats{Health: 100, Shells: 2},
		inventory: playerInventory{
			ReadyWeapon: weaponShotgun,
			Weapons:     map[int16]bool{2001: true},
		},
	}
	g.handleFire()
	if g.stats.Shells != 1 {
		t.Fatalf("shells=%d want=1", g.stats.Shells)
	}
	if g.thingHP[0] >= 40 {
		t.Fatalf("monster hp=%d want < 40", g.thingHP[0])
	}
}

func TestNoAmmoAutoSwitchesToFist(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons:     map[int16]bool{},
		},
	}
	g.ensureWeaponHasAmmo()
	if g.inventory.ReadyWeapon != weaponFist {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponFist)
	}
}

func TestPistolFirstShotConsumesSingleDamageRoll(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:     playerStats{Bullets: 1},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	_, p0 := doomrand.State()
	g.handleFire()
	_, p1 := doomrand.State()
	if d := prandDelta(p0, p1); d != 1 {
		t.Fatalf("p-random calls=%d want=1 for accurate first pistol shot", d)
	}
}

func TestPistolRefireConsumesSpreadAndDamageRolls(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:        playerStats{Bullets: 1},
		inventory:    playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
		weaponRefire: true,
	}
	_, p0 := doomrand.State()
	g.handleFire()
	_, p1 := doomrand.State()
	if d := prandDelta(p0, p1); d != 3 {
		t.Fatalf("p-random calls=%d want=3 for refire pistol shot", d)
	}
}

func TestShotgunConsumesSevenPelletRandomRolls(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Shells: 1},
		inventory: playerInventory{
			ReadyWeapon: weaponShotgun,
			Weapons:     map[int16]bool{2001: true},
		},
	}
	_, p0 := doomrand.State()
	g.handleFire()
	_, p1 := doomrand.State()
	if d := prandDelta(p0, p1); d != 21 {
		t.Fatalf("p-random calls=%d want=21 for 7-pellet shotgun shot", d)
	}
}

func prandDelta(before, after int) int {
	d := after - before
	if d < 0 {
		d += 256
	}
	return d
}

func TestTickWeaponFirePistolCadence(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:     playerStats{Bullets: 3},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.setAttackHeld(true)
	g.tickWeaponFire()
	if g.stats.Bullets != 2 {
		t.Fatalf("bullets=%d want=2 after first shot", g.stats.Bullets)
	}
	for i := 0; i < 13; i++ {
		g.tickWeaponFire()
		if g.stats.Bullets != 2 {
			t.Fatalf("shot fired too early at tic %d: bullets=%d", i+1, g.stats.Bullets)
		}
	}
	g.tickWeaponFire()
	if g.stats.Bullets != 1 {
		t.Fatalf("bullets=%d want=1 at tic 14 refire cadence", g.stats.Bullets)
	}
}

func TestPistolAutoaimSlopeHitsLowerTarget(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 64, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		p: player{
			x:      0,
			y:      0,
			z:      44 * fracUnit,
			floorz: 44 * fracUnit,
			ceilz:  128 * fracUnit,
			angle:  degToAngle(0),
		},
		stats:     playerStats{Bullets: 1},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.handleFire()
	if g.thingHP[0] >= 20 {
		t.Fatalf("monster hp=%d want < 20 (autoaim slope should hit lower target)", g.thingHP[0])
	}
}

func TestCycleWeaponSkipsUnowned(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons:     map[int16]bool{2002: true},
		},
	}
	g.cycleWeapon(1)
	if g.inventory.ReadyWeapon != weaponChaingun {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponChaingun)
	}
}

func TestCycleWeaponWrapsBackward(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponFist,
			Weapons:     map[int16]bool{2006: true},
		},
	}
	g.cycleWeapon(-1)
	if g.inventory.ReadyWeapon != weaponBFG {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponBFG)
	}
}

func TestFireGunShotSpawnsHitscanPuffOnWallImpact(t *testing.T) {
	doomrand.Clear()
	g := &game{
		lines: []physLine{
			{
				x1:       64 * fracUnit,
				y1:       -32 * fracUnit,
				x2:       64 * fracUnit,
				y2:       32 * fracUnit,
				flags:    0, // one-sided blocker
				sideNum1: -1,
			},
		},
		p: player{x: 0, y: 0, z: 0, angle: degToAngle(0)},
	}
	if g.fireGunShot(g.p.angle, pistolRange, 0, true) {
		t.Fatal("expected no monster hit")
	}
	if len(g.hitscanPuffs) == 0 {
		t.Fatal("expected wall-impact hitscan puff to spawn")
	}
	if g.hitscanPuffs[0].kind != hitscanFxPuff {
		t.Fatalf("effect kind=%d want puff", g.hitscanPuffs[0].kind)
	}
}

func TestFireGunShotSpawnsHitscanBloodOnMonsterHit(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, X: 64, Y: 0}},
		},
		thingCollected: make([]bool, 1),
		thingHP:        []int{20},
		p:              player{x: 0, y: 0, z: 0, angle: degToAngle(0)},
	}
	slope := g.bulletSlopeForAim(g.p.angle, pistolRange)
	if !g.fireGunShot(g.p.angle, pistolRange, slope, true) {
		t.Fatal("expected monster hit")
	}
	if len(g.hitscanPuffs) == 0 {
		t.Fatal("expected blood effect to spawn")
	}
	if g.hitscanPuffs[0].kind != hitscanFxBlood {
		t.Fatalf("effect kind=%d want blood", g.hitscanPuffs[0].kind)
	}
}

func TestHitscanPuffsExpire(t *testing.T) {
	g := &game{}
	g.spawnHitscanPuff(0, 0, 0)
	if len(g.hitscanPuffs) != 1 {
		t.Fatalf("puffs=%d want=1", len(g.hitscanPuffs))
	}
	for i := 0; i < 16; i++ {
		g.tickHitscanPuffs()
	}
	if len(g.hitscanPuffs) != 0 {
		t.Fatalf("puffs=%d want=0 after expiry", len(g.hitscanPuffs))
	}
}
