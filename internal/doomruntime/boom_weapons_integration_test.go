//go:build integration

package doomruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"os"
	"testing"
)

// Doom's info.h state identifiers, independently shared by the reference.
// Runtime psprite enums omit interleaved flash states; compare canonical IDs.
func weaponDoomStateID(state weaponPspriteState) int {
	switch {
	case state == weaponStateNone:
		return 0
	case state >= weaponStateFistReady && state <= weaponStateFistAtk5:
		return int(state-weaponStateFistReady) + 2
	case state >= weaponStatePistolReady && state <= weaponStatePistolAtk4:
		return int(state-weaponStatePistolReady) + 10
	case state >= weaponStateShotgunReady && state <= weaponStateShotgunAtk9:
		return int(state-weaponStateShotgunReady) + 18
	case state >= weaponStateSuperShotgunReady && state <= weaponStateSuperShotgunAtk10:
		return int(state-weaponStateSuperShotgunReady) + 32
	case state >= weaponStateChaingunReady && state <= weaponStateChaingunAtk3:
		return int(state-weaponStateChaingunReady) + 49
	case state >= weaponStateRocketReady && state <= weaponStateRocketAtk3:
		return int(state-weaponStateRocketReady) + 57
	case state >= weaponStateSawReady && state <= weaponStateSawAtk3:
		return int(state-weaponStateSawReady) + 67
	case state >= weaponStatePlasmaReady && state <= weaponStatePlasmaAtk2:
		return int(state-weaponStatePlasmaReady) + 74
	case state >= weaponStateBFGReady && state <= weaponStateBFGAtk4:
		return int(state-weaponStateBFGReady) + 81
	case state == weaponStatePistolFlash:
		return 17
	case state >= weaponStateShotgunFlash1 && state <= weaponStateShotgunFlash2:
		return int(state-weaponStateShotgunFlash1) + 30
	case state >= weaponStateSuperShotgunFlash1 && state <= weaponStateSuperShotgunFlash2:
		return int(state-weaponStateSuperShotgunFlash1) + 47
	case state >= weaponStateChaingunFlash1 && state <= weaponStateChaingunFlash2:
		return int(state-weaponStateChaingunFlash1) + 55
	case state >= weaponStateRocketFlash1 && state <= weaponStateRocketFlash4:
		return int(state-weaponStateRocketFlash1) + 63
	case state >= weaponStatePlasmaFlash1 && state <= weaponStatePlasmaFlash2:
		return int(state-weaponStatePlasmaFlash1) + 79
	case state >= weaponStateBFGFlash1 && state <= weaponStateBFGFlash2:
		return int(state-weaponStateBFGFlash1) + 88
	}
	panic("unknown weapon state")
}
func TestBoomWeaponsCapture(t *testing.T) {
	path := os.Getenv("GD_WEAPON_CASES")
	if path == "" {
		t.Skip("run verify_boom_weapons.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name                                                                                           string
		Boom, Automatic, Ready, Owned, Bullets, Shells, Cells, Rockets, Pattern, Slot, Tics, Insurance int
		Seed                                                                                           uint32
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_WEAPON_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(os.Getenv("GD_WEAPON_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprintln(out, "name,tic,buttons,ready,pending,attackdown,refire,bullets,shells,cells,rockets,state,tics,y,flash,ftics,fallback,playindex,seeds")
	for _, c := range cases {
		m, err := mapdata.LoadMap(wf, "MAP01")
		if err != nil {
			t.Fatal(err)
		}
		h := DemoHeader{Version: 109, Skill: 2, NoMonsters: true, PlayerInGame: [4]bool{true}}
		if c.Boom != 0 {
			h.Version = 202
			settings := demo.GameOptions{Seed: c.Seed, Insurance: c.Insurance != 0, NoMonsters: true, MonstersRemember: true, VariableFriction: true, AllowPushers: true, PlayerBobbing: true}
			h.BoomOptions = settings.Bytes()
		}
		g := newGame(m, Options{Headless: true, DemoScript: &DemoScript{Header: h}})

		for id := 0; id < 9; id++ {
			w := commandWeapon(id + 1)
			switch w {
			case weaponShotgun:
				g.inventory.Weapons[2001] = c.Owned&(1<<id) != 0
			case weaponSuperShotgun:
				g.inventory.Weapons[82] = c.Owned&(1<<id) != 0
			case weaponChaingun:
				g.inventory.Weapons[2002] = c.Owned&(1<<id) != 0
			case weaponRocketLauncher:
				g.inventory.Weapons[2003] = c.Owned&(1<<id) != 0
			case weaponPlasma:
				g.inventory.Weapons[2004] = c.Owned&(1<<id) != 0
			case weaponBFG:
				g.inventory.Weapons[2006] = c.Owned&(1<<id) != 0
			case weaponChainsaw:
				g.inventory.Weapons[2005] = c.Owned&(1<<id) != 0
			}
		}
		g.stats.Bullets = c.Bullets
		g.stats.Shells = c.Shells
		g.stats.Cells = c.Cells
		g.stats.Rockets = c.Rockets
		g.weaponAttackDown, g.weaponRefire = false, false
		g.inventory.ReadyWeapon = commandWeapon(c.Ready + 1)
		g.inventory.PendingWeapon = g.inventory.ReadyWeapon
		g.weaponState, g.weaponFlashState = weaponStateNone, weaponStateNone
		g.weaponStateTics, g.weaponFlashTics = 0, 0
		g.bringUpWeapon()
		for tic := 0; tic < c.Tics; tic++ {
			fire := c.Pattern == 0 || c.Pattern == 1 && tic < 30 || c.Pattern == 2 && (tic < 30 || tic >= 80) || c.Pattern == 3 && (tic < 55 || tic >= 110)
			buttons := byte(0)
			if fire {
				buttons = demoButtonAttack
			}
			if c.Pattern == 3 && tic == 40 {
				buttons |= demoButtonChange | byte(c.Slot<<3)
			}
			if c.Automatic != 0 && c.Boom != 0 {
				g.demoWeaponSlot = 0
				tc := g.buildOutgoingDemoTic(moveCmd{}, false, fire)
				if tc.Buttons&demoButtonChange != 0 {
					buttons = tc.Buttons
				}
			}
			g.stepGameplayFromDemoTic(DemoTic{Buttons: buttons})
			pending := 10
			if g.inventory.PendingWeapon != 0 {
				pending = demoTraceWeaponID(g.inventory.PendingWeapon)
			}
			s := doomrand.Capture()
			fmt.Fprintf(out, "%s,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,", c.Name, tic, buttons, demoTraceWeaponID(g.inventory.ReadyWeapon), pending, boolToInt(g.weaponAttackDown), boolToInt(g.weaponRefire), g.stats.Bullets, g.stats.Shells, g.stats.Cells, g.stats.Rockets, weaponDoomStateID(g.weaponState), g.weaponStateTics, g.weaponPSpriteY*fracUnit, weaponDoomStateID(g.weaponFlashState), g.weaponFlashTics, demoTraceWeaponID(g.fallbackWeapon(c.Boom != 0)), s.PlayIndex)
			for cls, seed := range s.Seeds {
				if c.Boom != 0 && doomrand.Class(cls) != doomrand.Misc {
					fmt.Fprintf(out, "%d;", seed)
				}
			}
			fmt.Fprintln(out)
		}
	}
}
