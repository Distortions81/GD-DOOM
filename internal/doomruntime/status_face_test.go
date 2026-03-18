package doomruntime

import "testing"

func TestStatusFaceInvulnerableUsesGodFace(t *testing.T) {
	g := &game{
		stats: playerStats{Health: 100},
	}
	g.initPlayerState()
	g.inventory.InvulnTics = 1
	g.initStatusFaceState()

	g.statusUpdateFaceWidget()

	if g.statusFaceIndex != statusGodFace {
		t.Fatalf("face=%d want=%d", g.statusFaceIndex, statusGodFace)
	}
}

func TestStatusFaceWeaponPickupTriggersEvilGrin(t *testing.T) {
	g := &game{
		stats: playerStats{Health: 100},
	}
	g.initPlayerState()
	g.initStatusFaceState()
	g.statusBonusCount = 6
	g.inventory.Weapons[2001] = true

	g.statusUpdateFaceWidget()

	want := statusEvilGrinOffset
	if g.statusFaceIndex != want {
		t.Fatalf("face=%d want=%d", g.statusFaceIndex, want)
	}
}

func TestStatusFaceAttackerHeadOnUsesRampageFace(t *testing.T) {
	g := &game{
		stats: playerStats{Health: 100},
		p:     player{x: 0, y: 0, angle: 0},
	}
	g.initPlayerState()
	g.initStatusFaceState()
	g.statusDamageCount = 8
	g.statusOldHealth = 100
	g.statusHasAttacker = true
	g.statusAttackerX = 128 * fracUnit
	g.statusAttackerY = 0

	g.statusUpdateFaceWidget()

	want := statusRampageOffset
	if g.statusFaceIndex != want {
		t.Fatalf("face=%d want=%d", g.statusFaceIndex, want)
	}
}
