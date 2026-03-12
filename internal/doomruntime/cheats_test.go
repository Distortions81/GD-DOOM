package doomruntime

import "testing"

func TestApplyCheatLevel2GrantsIDFA(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	g.stats.Bullets = 0
	g.stats.Shells = 0
	g.stats.Rockets = 0
	g.stats.Cells = 0
	g.stats.Armor = 0

	g.applyCheatLevel(2, false)

	if !g.inventory.Weapons[2001] || !g.inventory.Weapons[2002] || !g.inventory.Weapons[2003] || !g.inventory.Weapons[2004] || !g.inventory.Weapons[2005] || !g.inventory.Weapons[2006] {
		t.Fatal("idfa should grant all weapons")
	}
	if g.stats.Bullets != 200 || g.stats.Shells != 50 || g.stats.Rockets != 50 || g.stats.Cells != 300 {
		t.Fatalf("ammo not maxed: b=%d s=%d r=%d c=%d", g.stats.Bullets, g.stats.Shells, g.stats.Rockets, g.stats.Cells)
	}
	if g.stats.Armor != 200 {
		t.Fatalf("armor=%d want=200", g.stats.Armor)
	}
	if g.invulnerable {
		t.Fatal("cheat level 2 should not force invulnerability")
	}
}

func TestApplyCheatLevel3GrantsKeysAndInvuln(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	g.applyCheatLevel(3, false)
	if !g.inventory.BlueKey || !g.inventory.RedKey || !g.inventory.YellowKey {
		t.Fatal("idkfa should grant all keys")
	}
	if !g.invulnerable {
		t.Fatal("level 3 should enable invulnerability")
	}
}
