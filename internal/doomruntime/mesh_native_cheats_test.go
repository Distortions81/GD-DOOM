package doomruntime

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"testing"
)

func TestNativeTypedCheatsAndWarpRequests(t *testing.T) {
	n := loadNativeCombatGame(t)
	for _, r := range "IDDQD" {
		if !n.TypeCheats([]rune{r}) {
			t.Fatal("cheat characters were allowed to trigger host shortcuts")
		}
	}
	if !n.g.invulnerable {
		t.Fatal("IDDQD did not use shared god-mode logic")
	}
	n.TypeCheats([]rune("idkfa"))
	if !n.g.inventory.BlueKey || !n.g.inventory.Weapons[2001] || n.g.stats.Shells <= 0 {
		t.Fatal("IDKFA inventory missing")
	}
	n.TypeCheats([]rune("idclip"))
	if !n.g.noClip {
		t.Fatal("IDCLIP did not enable noclip")
	}
	n.TypeCheats([]rune("idbeholdr"))
	if n.g.inventory.RadSuitTics <= 0 {
		t.Fatal("powerup cheat missing")
	}
	var gotMap, gotCode string
	n.g.opts.PlayCheatMusic = func(name, code string) (bool, error) { gotMap, gotCode = name, code; return true, nil }
	n.TypeCheats([]rune("idmus12"))
	if gotMap != "E1M1" || gotCode != "12" {
		t.Fatal("IDMUS callback missing")
	}
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	n.g.opts.NewGameLoader = func(name string) (*mapdata.Map, error) { return mapdata.LoadMap(wf, mapdata.MapName(name)) }
	n.TypeCheats([]rune("idclev12"))
	m, skill, ok := n.TakeNewGameRequest()
	if !ok || m.Name != "E1M2" || skill != 3 {
		t.Fatal("IDCLEV validated map request missing")
	}
	if _, _, ok := n.TakeNewGameRequest(); ok {
		t.Fatal("warp request was not consumed")
	}
	n.TypeCheats([]rune("idclev99"))
	if _, _, ok := n.TakeNewGameRequest(); ok {
		t.Fatal("invalid warp produced a map request")
	}
	n.ClearCheatInput()
	if n.TypeCheats([]rune("p")) {
		t.Fatal("ordinary P shortcut was treated as cheat typing")
	}
}
