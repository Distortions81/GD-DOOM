package doomruntime

import (
	"gddoom/internal/mapdata"
	"testing"
)

func TestPowerupColormapPriority(t *testing.T) {
	oldRGBA := doomColormapRGBA
	doomColormapRGBA = make([]uint32, 33*256)
	t.Cleanup(func() { doomColormapRGBA = oldRGBA })
	old := doomColormapRows
	doomColormapRows = 33
	t.Cleanup(func() { doomColormapRows = old })
	g := &game{opts: Options{DoomColorMap: make([]byte, 33*256)}, viewW: 320, viewH: 200, screenBlocks: 11}
	for _, tc := range []struct {
		inv, amp, row int
		active        bool
	}{{0, 129, 1, true}, {0, 128, 0, false}, {0, 8, 1, true}, {129, 129, 32, true}, {128, 129, 0, false}, {7, 129, 0, false}, {8, 129, 32, true}} {
		g.inventory.InvulnTics, g.inventory.LightAmpTics = tc.inv, tc.amp
		row, active := g.playerFixedColormapRow()
		if row != tc.row || active != tc.active {
			t.Fatalf("inv=%d infrared=%d: row=%d active=%v", tc.inv, tc.amp, row, active)
		}
		if active && g.weaponLightRow(true) != tc.row {
			t.Fatal("muzzle flash escaped fixed colormap")
		}
	}
}

func TestSkullKeysBelongToPlayerAndSurviveSnapshots(t *testing.T) {
	g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 40}}}, thingCollected: []bool{true}, inventory: playerInventory{BlueKey: true}}
	if g.statusBarState().keySkull[0] {
		t.Fatal("another player's collected skull changed this player's key")
	}
	if _, _, ok := g.applyPickup(40, false); !ok {
		t.Fatal("skull pickup failed")
	}
	state := capturePlayerInventorySaveState(g.inventory)
	if !restorePlayerInventorySaveState(state).SkullKeys[0] {
		t.Fatal("replica lost skull artwork")
	}
	file := saveFile{Version: saveGameVersion, Game: gameSaveState{Inventory: state}}
	data, err := encodeSnapshot([]byte("test"), file)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeSnapshot(data, []byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Game.Inventory.SkullKeys != state.SkullKeys {
		t.Fatal("save lost skull artwork")
	}
	g.finishLevelForCarryover()
	if g.inventory.SkullKeys != [3]bool{} {
		t.Fatal("keys leaked into next map")
	}
	g.grantIDKFA()
	if g.inventory.SkullKeys != [3]bool{true, true, true} {
		t.Fatal("all-keys cheat omitted skulls")
	}
}

func TestCoopKeyVariantsShareAndDeathmatchGrantsSkulls(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeCoop)
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	a.players[1].inventory.BlueKey = true
	a.players[1].inventory.SkullKeys[0] = true
	a.shareAuthoritativeCoopKeys()
	for _, p := range a.players {
		if p != nil && !p.inventory.SkullKeys[0] {
			t.Fatal("shared key lost its variant")
		}
	}
	if !a.g.authorityRules.TeamSkullKeys[0] {
		t.Fatal("joining players would lose shared skull variant")
	}
	d := newAuthorityRulesTest(t, gameModeDeathmatch)
	for _, id := range []byte{1, 2} {
		if err := d.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range d.players {
		if p != nil && p.inventory.SkullKeys != [3]bool{true, true, true} {
			t.Fatal("deathmatch spawn omitted skull keys")
		}
	}
}

func TestMultiplayerStatusCacheChangesWithViewerAndScores(t *testing.T) {
	g := &game{opts: Options{GameMode: gameModeDeathmatch}, localSlot: 2, authorityRules: &authorityRulesState{}}
	g.authorityRules.Scores[2].Frags = -3
	state := g.statusBarState()
	if !state.deathmatch || !state.multiplayer || state.playerColor != 1 || state.frags != -3 {
		t.Fatalf("wrong HUD state: %+v", state)
	}
	g.authorityRules.Scores[2].Frags++
	if g.statusBarState() == state {
		t.Fatal("frag score would remain cached")
	}
}
