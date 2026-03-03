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
