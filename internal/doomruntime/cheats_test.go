package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

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
	if _, ok := g.playerFixedColormapRow(); ok {
		t.Fatal("cheat invulnerability should not enable inverse colormap")
	}
}

func TestConsumeTypedCheatInput_IDDQDTogglesInvulnerability(t *testing.T) {
	g := &game{}
	g.initPlayerState()

	g.input.inputChars = []rune("iddqd")
	g.consumeTypedCheatInput()
	if !g.invulnerable {
		t.Fatal("iddqd should enable invulnerability")
	}

	g.input.inputChars = []rune("iddqd")
	g.consumeTypedCheatInput()
	if g.invulnerable {
		t.Fatal("second iddqd should disable invulnerability")
	}
}

func TestConsumeTypedCheatInput_IDFAGrantsWeaponsAndAmmo(t *testing.T) {
	g := &game{}
	g.initPlayerState()

	g.input.inputChars = []rune("idfa")
	g.consumeTypedCheatInput()

	if !g.inventory.Weapons[2001] || !g.inventory.Weapons[2002] || !g.inventory.Weapons[2003] || !g.inventory.Weapons[2004] || !g.inventory.Weapons[2005] || !g.inventory.Weapons[2006] {
		t.Fatal("idfa should grant all weapons")
	}
	if g.stats.Bullets != 200 || g.stats.Shells != 50 || g.stats.Rockets != 50 || g.stats.Cells != 300 {
		t.Fatalf("ammo not maxed: b=%d s=%d r=%d c=%d", g.stats.Bullets, g.stats.Shells, g.stats.Rockets, g.stats.Cells)
	}
}

func TestConsumeTypedCheatInput_IDKFAGrantsKeys(t *testing.T) {
	g := &game{}
	g.initPlayerState()

	g.input.inputChars = []rune("idkfa")
	g.consumeTypedCheatInput()

	if !g.inventory.BlueKey || !g.inventory.RedKey || !g.inventory.YellowKey {
		t.Fatal("idkfa should grant all keys")
	}
}

func TestConsumeTypedCheatInput_IDDTCyclesAutomapParity(t *testing.T) {
	g := &game{}

	for i, want := range []struct {
		reveal revealMode
		iddt   int
	}{
		{reveal: revealAllMap, iddt: 0},
		{reveal: revealAllMap, iddt: 1},
		{reveal: revealAllMap, iddt: 2},
		{reveal: revealNormal, iddt: 0},
	} {
		g.input.inputChars = []rune("iddt")
		g.consumeTypedCheatInput()
		if g.parity.reveal != want.reveal || g.parity.iddt != want.iddt {
			t.Fatalf("step %d got reveal=%d iddt=%d want reveal=%d iddt=%d", i, g.parity.reveal, g.parity.iddt, want.reveal, want.iddt)
		}
	}
}

func TestConsumeTypedCheatInput_IDCLIPTogglesNoClip(t *testing.T) {
	g := &game{}

	g.input.inputChars = []rune("idclip")
	g.consumeTypedCheatInput()
	if !g.noClip {
		t.Fatal("idclip should enable noclip")
	}

	g.input.inputChars = []rune("idspispopd")
	g.consumeTypedCheatInput()
	if g.noClip {
		t.Fatal("idspispopd should toggle noclip off")
	}
}

func TestConsumeTypedCheatInput_IDCLEVRequestsEpisodeWarp(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Name: "E1M1"},
		opts: Options{
			SkillLevel: 4,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapName != "E1M3" {
					t.Fatalf("mapName=%q want E1M3", mapName)
				}
				return &mapdata.Map{Name: mapdata.MapName(mapName)}, nil
			},
		},
	}

	g.input.inputChars = []rune("idclev13")
	g.consumeTypedCheatInput()

	if g.newGameRequestedMap == nil || g.newGameRequestedMap.Name != "E1M3" {
		t.Fatalf("newGameRequestedMap=%v want E1M3", g.newGameRequestedMap)
	}
	if g.newGameRequestedSkill != 4 {
		t.Fatalf("newGameRequestedSkill=%d want 4", g.newGameRequestedSkill)
	}
}

func TestConsumeTypedCheatInput_IDCLEVRequestsCommercialWarp(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Name: "MAP01"},
		opts: Options{
			SkillLevel: 3,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapName != "MAP15" {
					t.Fatalf("mapName=%q want MAP15", mapName)
				}
				return &mapdata.Map{Name: mapdata.MapName(mapName)}, nil
			},
		},
	}

	g.input.inputChars = []rune("idclev15")
	g.consumeTypedCheatInput()

	if g.newGameRequestedMap == nil || g.newGameRequestedMap.Name != "MAP15" {
		t.Fatalf("newGameRequestedMap=%v want MAP15", g.newGameRequestedMap)
	}
}

func TestConsumeTypedCheatInput_IDCLEVRejectsBadMap(t *testing.T) {
	g := &game{
		hudMessagesEnabled: true,
		m:                  &mapdata.Map{Name: "MAP01"},
		opts: Options{
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				t.Fatalf("unexpected map load %q", mapName)
				return nil, nil
			},
		},
	}

	g.input.inputChars = []rune("idclev00")
	g.consumeTypedCheatInput()

	if g.newGameRequestedMap != nil {
		t.Fatalf("newGameRequestedMap=%v want nil", g.newGameRequestedMap)
	}
	if g.useText != "IDCLEV BAD MAP" {
		t.Fatalf("useText=%q want IDCLEV BAD MAP", g.useText)
	}
}

func TestConsumeTypedCheatInput_IDMUSRequestsMusicChange(t *testing.T) {
	var gotMap string
	var gotCode string
	g := &game{
		hudMessagesEnabled: true,
		m:                  &mapdata.Map{Name: "MAP01"},
		opts: Options{
			PlayCheatMusic: func(currentMapName string, code string) (bool, error) {
				gotMap = currentMapName
				gotCode = code
				return true, nil
			},
		},
	}

	g.input.inputChars = []rune("idmus34")
	g.consumeTypedCheatInput()

	if gotMap != "MAP01" || gotCode != "34" {
		t.Fatalf("playCheatMusic got (%q,%q) want (MAP01,34)", gotMap, gotCode)
	}
	if g.useText != "Music Change" {
		t.Fatalf("useText=%q want Music Change", g.useText)
	}
}

func TestConsumeTypedCheatInput_IDMUSRejectsBadSelection(t *testing.T) {
	g := &game{
		hudMessagesEnabled: true,
		m:                  &mapdata.Map{Name: "MAP01"},
		opts: Options{
			PlayCheatMusic: func(currentMapName string, code string) (bool, error) {
				return false, nil
			},
		},
	}

	g.input.inputChars = []rune("idmus99")
	g.consumeTypedCheatInput()

	if g.useText != "IMPOSSIBLE SELECTION" {
		t.Fatalf("useText=%q want IMPOSSIBLE SELECTION", g.useText)
	}
}

func TestUpdateParityControls_TypedInputSuppressesSourcePortLetterHotkeys(t *testing.T) {
	g := &game{
		edgeInputPass:      true,
		hudMessagesEnabled: true,
		opts:               Options{SourcePortMode: true},
		input:              gameInputSnapshot{inputChars: []rune{'i'}, justPressedKeys: map[ebiten.Key]struct{}{ebiten.KeyI: {}}},
	}

	g.updateParityControls()

	if g.parity.iddt != 0 {
		t.Fatalf("iddt=%d want 0", g.parity.iddt)
	}
}

func TestConsumeTypedCheatInput_IDMYPOSReportsPosition(t *testing.T) {
	g := &game{
		hudMessagesEnabled: true,
		p: player{
			x:     0x123450,
			y:     0x6789ab,
			angle: 0x13579bdf,
		},
	}

	g.input.inputChars = []rune("idmypos")
	g.consumeTypedCheatInput()

	want := "ang=0x13579bdf;x,y=(0x123450,0x6789ab)"
	if g.useText != want {
		t.Fatalf("useText=%q want %q", g.useText, want)
	}
}

func TestConsumeTypedCheatInput_IDCHOPPERSGrantsChainsawAndCheatInvulnPulse(t *testing.T) {
	g := &game{hudMessagesEnabled: true}
	g.initPlayerState()

	g.input.inputChars = []rune("idchoppers")
	g.consumeTypedCheatInput()

	if !g.inventory.Weapons[2005] {
		t.Fatal("chainsaw should be granted")
	}
	if g.inventory.InvulnTics != 1 {
		t.Fatalf("invuln=%d want 1", g.inventory.InvulnTics)
	}
	if g.useText != "... doesn't suck - GM" {
		t.Fatalf("useText=%q want %q", g.useText, "... doesn't suck - GM")
	}
}

func TestConsumeTypedCheatInput_IDBEHOLDMenuMessage(t *testing.T) {
	g := &game{hudMessagesEnabled: true}

	g.input.inputChars = []rune("idbehold")
	g.consumeTypedCheatInput()

	if g.useText != "inVuln, Str, Inviso, Rad, Allmap, or Lite-amp" {
		t.Fatalf("useText=%q", g.useText)
	}
}

func TestConsumeTypedCheatInput_IDBEHOLDVariants(t *testing.T) {
	g := &game{hudMessagesEnabled: true}
	g.initPlayerState()

	g.input.inputChars = []rune("idbeholdv")
	g.consumeTypedCheatInput()
	if g.inventory.InvulnTics != 30*doomTicsPerSecond {
		t.Fatalf("invuln=%d want %d", g.inventory.InvulnTics, 30*doomTicsPerSecond)
	}

	g.input.inputChars = []rune("idbeholds")
	g.consumeTypedCheatInput()
	if !g.inventory.Strength || g.inventory.StrengthCount != 1 {
		t.Fatalf("strength=%v/%d want true/1", g.inventory.Strength, g.inventory.StrengthCount)
	}

	g.input.inputChars = []rune("idbeholdi")
	g.consumeTypedCheatInput()
	if g.inventory.InvisTics != 60*doomTicsPerSecond {
		t.Fatalf("invis=%d want %d", g.inventory.InvisTics, 60*doomTicsPerSecond)
	}

	g.input.inputChars = []rune("idbeholdr")
	g.consumeTypedCheatInput()
	if g.inventory.RadSuitTics != 60*doomTicsPerSecond {
		t.Fatalf("radsuit=%d want %d", g.inventory.RadSuitTics, 60*doomTicsPerSecond)
	}

	g.input.inputChars = []rune("idbeholda")
	g.consumeTypedCheatInput()
	if !g.inventory.AllMap {
		t.Fatal("allmap should be enabled")
	}

	g.input.inputChars = []rune("idbeholdl")
	g.consumeTypedCheatInput()
	if g.inventory.LightAmpTics != 120*doomTicsPerSecond {
		t.Fatalf("lightamp=%d want %d", g.inventory.LightAmpTics, 120*doomTicsPerSecond)
	}
}

func TestConsumeTypedCheatInput_IDBEHOLDStrengthTogglesOff(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	g.inventory.Strength = true
	g.inventory.StrengthCount = 10

	g.input.inputChars = []rune("idbeholds")
	g.consumeTypedCheatInput()

	if g.inventory.Strength || g.inventory.StrengthCount != 0 {
		t.Fatalf("strength=%v/%d want false/0", g.inventory.Strength, g.inventory.StrengthCount)
	}
}
