package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func lockInventory(mask int) playerInventory {
	var inv playerInventory
	for i := 0; i < 3; i++ {
		inv.CardKeys[i] = mask&(1<<i) != 0
		inv.SkullKeys[i] = mask&(1<<(i+3)) != 0
	}
	inv.BlueKey = inv.CardKeys[0] || inv.SkullKeys[0]
	inv.YellowKey = inv.CardKeys[1] || inv.SkullKeys[1]
	inv.RedKey = inv.CardKeys[2] || inv.SkullKeys[2]
	return inv
}

func TestBoomDistinctLocksExhaustive(t *testing.T) {
	for mode := 0; mode < 16; mode++ {
		for mask := 0; mask < 64; mask++ {
			special := uint16(0x3800 | mode<<6 | 6)
			d := mapdata.LookupLineSpecial(special).Door
			if d == nil {
				t.Fatalf("missing lock %x", special)
			}
			key := mode & 7
			equivalent := mode&8 != 0
			want := false
			switch key {
			case 0:
				want = mask != 0
			case 7:
				if equivalent {
					want = (mask|mask>>3)&7 == 7
				} else {
					want = mask == 63
				}
			default:
				bit := []int{2, 0, 1, 5, 3, 4}[key-1]
				want = mask&(1<<bit) != 0
				if equivalent {
					want = (mask|mask>>3)&(1<<(bit%3)) != 0
				}
			}
			inv := lockInventory(mask)
			if got := d.CanActivate(inv.keys()); got != want {
				t.Fatalf("special=%x mask=%06b got=%v want=%v", special, mask, got, want)
			}
		}
	}
}

func TestBoomKeyPickupsAndSnapshotVariants(t *testing.T) {
	for mask := 0; mask < 64; mask++ {
		g := boomCeilingGame(0x39c6)
		for bit, typ := range []int16{5, 6, 13, 40, 39, 38} {
			if mask&(1<<bit) != 0 {
				g.applyPickup(typ, false)
			}
		}
		want := lockInventory(mask)
		if g.inventory.keys() != want.keys() {
			t.Fatalf("pickup mask %d: %+v", mask, g.inventory.keys())
		}
		for _, version := range []int{saveGameVersion, keyframeVersion} {
			file := saveFile{Version: version, Game: captureGameSaveState(g)}
			data, err := encodeSnapshot(saveGameMagic, file)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeSnapshot(data, saveGameMagic)
			if err != nil {
				t.Fatal(err)
			}
			restored := restorePlayerInventorySaveState(decoded.Game.Inventory)
			if restored.keys() != want.keys() {
				t.Fatalf("roundtrip version %d mask %d: %+v", version, mask, restored.keys())
			}
		}
	}
}

func TestBoomDistinctKeysShareAndRespawn(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeCoop)
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	// A blue skull alone must not grant the blue card during team propagation.
	a.players[1].inventory = lockInventory(8)
	a.shareAuthoritativeCoopKeys()
	for _, p := range a.players {
		if p != nil && p.inventory.keys() != lockInventory(8).keys() {
			t.Fatal("skull sharing synthesized a card")
		}
	}
	// A different teammate brings the blue card; both variants must now survive.
	a.players[2].inventory = lockInventory(1)
	a.shareAuthoritativeCoopKeys()
	for _, p := range a.players {
		if p != nil && p.inventory.keys() != lockInventory(9).keys() {
			t.Fatal("team did not retain both variants")
		}
	}
	killAuthorityTestPlayer(a, 1, 0)
	for range 3 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	if a.players[1].inventory.keys() != lockInventory(9).keys() {
		t.Fatal("respawn lost distinct keys")
	}
}

func TestBoomPreviousSnapshotKeepsFrictionExtension(t *testing.T) {
	for _, version := range []int{31, 18} {
		g := boomCeilingGame(0x39c6)
		g.p.friction = 1234
		g.p.movefactor = 5678
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		if file.Game.Player.Friction != 1234 || file.Game.Player.MoveFactor != 5678 {
			t.Fatal("old friction extension lost")
		}
	}
}

func TestBoomLegacySnapshotReconstructsCollectedCards(t *testing.T) {
	base := &mapdata.Map{Name: "MAP01", Sectors: []mapdata.Sector{{CeilingHeight: 128}}, Things: []mapdata.Thing{{Type: 1}, {Type: 5, Flags: 7}, {Type: 6, Flags: 7}, {Type: 13, Flags: 7}, {Type: 40, Flags: 7}, {Type: 39, Flags: 7}, {Type: 38, Flags: 7}}}
	for _, version := range []int{31, 18} {
		for _, mask := range []int{0, 1, 8, 9, 7, 56, 63} {
			g := newGame(cloneMapForRestart(base), Options{SkillLevel: 3})
			for bit, typ := range []int16{5, 6, 13, 40, 39, 38} {
				if mask&(1<<bit) != 0 {
					g.applyPickup(typ, false)
					g.thingCollected[bit+1] = true
				}
			}
			data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Current: base.Name, Game: captureGameSaveState(g)})
			if err != nil {
				t.Fatal(err)
			}
			file, err := decodeSnapshot(data, saveGameMagic)
			if err != nil {
				t.Fatal(err)
			}
			loaded := &sessionGame{opts: Options{NewGameLoader: func(string) (*mapdata.Map, error) { return cloneMapForRestart(base), nil }}}
			if err := loaded.applyLoadedSnapshot(file, ""); err != nil {
				t.Fatal(err)
			}
			if loaded.g.inventory.keys() != lockInventory(mask).keys() {
				t.Fatalf("legacy version %d mask %d: %+v", version, mask, loaded.g.inventory.keys())
			}
		}
	}
}

func TestBoomLegacyCardsIgnoreExcludedPickups(t *testing.T) {
	g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 5, Flags: 1}, {Type: 6, Flags: 2}}}, opts: Options{SkillLevel: 3}, thingCollected: []bool{true, true}, inventory: lockInventory(8 | 16)}
	// Skill-filtered objects are marked collected too; only the eligible yellow
	// card is evidence of previously owning both variants.
	if got := g.collectedCardKeys(); got != [3]bool{false, true, false} {
		t.Fatalf("filtered pickup granted a card: %v", got)
	}
}

func TestBoomKeyPickupOrder(t *testing.T) {
	for _, pair := range [][2]int16{{5, 40}, {6, 39}, {13, 38}} {
		for reverse := 0; reverse < 2; reverse++ {
			g := &game{}
			g.applyPickup(pair[reverse], false)
			g.applyPickup(pair[1-reverse], false)
			inv := g.inventory
			color := 0
			if pair[0] == 6 {
				color = 1
			}
			if pair[0] == 13 {
				color = 2
			}
			if !inv.CardKeys[color] || !inv.SkullKeys[color] {
				t.Fatalf("order %v/%d lost a key: %+v", pair, reverse, inv)
			}
		}
	}
}
