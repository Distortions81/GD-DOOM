package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestStatusOwnedWeaponsAllowsRegisteredDoomEnergyWeapons(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Name: "E1M1"},
		inventory: playerInventory{
			Weapons: map[int16]bool{
				82:   true,
				2004: true,
				2006: true,
			},
		},
	}

	owned := g.statusOwnedWeapons()
	if owned[4] || !owned[7] || !owned[8] {
		t.Fatalf("Doom I must allow plasma/BFG while excluding SSG: owned=%v", owned)
	}
	if g.statusWeaponOwned(3) {
		t.Fatal("slot 3 should not report super shotgun owned on non-commercial maps")
	}
	g.selectWeaponSlot(6)
	if g.inventory.PendingWeapon != weaponPlasma {
		t.Fatal("registered Doom I refused plasma selection")
	}
	g.selectWeaponSlot(7)
	if g.inventory.PendingWeapon != weaponBFG {
		t.Fatal("registered Doom I refused BFG selection")
	}
	g.opts.Shareware = true
	owned = g.statusOwnedWeapons()
	if owned[4] || owned[7] || owned[8] {
		t.Fatalf("shareware allowed unavailable weapons: %v", owned)
	}
}
