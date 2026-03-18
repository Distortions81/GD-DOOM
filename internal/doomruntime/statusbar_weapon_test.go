package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestStatusOwnedWeaponsGatesCommercialWeapons(t *testing.T) {
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
	if owned[4] || owned[7] || owned[8] {
		t.Fatalf("commercial weapons should be hidden on non-commercial maps: owned=%v", owned)
	}
	if g.statusWeaponOwned(3) {
		t.Fatal("slot 3 should not report super shotgun owned on non-commercial maps")
	}
}
