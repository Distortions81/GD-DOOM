package doomruntime

import (
	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"testing"
)

func TestBoomEmptyWeaponWaitsForCommand(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true, BoomGameOptions: &demo.GameOptions{Seed: 1993, Insurance: true}})
	g.stats.Bullets = 0
	g.stats.Shells = 2
	g.inventory.Weapons[82] = true
	g.weaponState = weaponStatePistolReady
	g.weaponStateTics = 1
	g.weaponPSpriteY = weaponTopY
	before := doomrand.Capture()
	g.fireWeaponStateSequence()
	if g.inventory.PendingWeapon != 0 || g.inventory.ReadyWeapon != weaponPistol || g.weaponState != weaponStatePistolReady || doomrand.Capture() != before {
		t.Fatal("empty Boom weapon invented a switch or an attack")
	}
	g.weaponAttackDown = true
	tc := g.buildOutgoingDemoTic(moveCmd{}, false, true)
	if tc.Buttons != demoButtonAttack|demoButtonChange|byte(8<<3) {
		t.Fatalf("fallback command %#x does not encode direct SSG", tc.Buttons)
	}
	cmd, _, _ := demoTicCommand(tc)
	g.selectCommandWeapon(cmd.weaponSlot)
	if g.inventory.PendingWeapon != weaponSuperShotgun {
		t.Fatal("encoded fallback did not select SSG")
	}
}
func TestBoomCommandSelectionsAreExact(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g := newGame(predictionTestMap(), Options{Headless: true})
		g.boomMomentumDropoff = boom
		g.inventory.Weapons[2001] = true
		g.inventory.Weapons[82] = true
		g.inventory.Weapons[2005] = true
		g.selectCommandWeapon(3)
		want := weaponSuperShotgun
		if boom {
			want = weaponShotgun
		}
		if g.inventory.PendingWeapon != want {
			t.Fatalf("boom=%t shotgun command selected %v", boom, g.inventory.PendingWeapon)
		}
		g.inventory.PendingWeapon = 0
		g.selectCommandWeapon(1)
		want = weaponChainsaw
		if boom {
			want = weaponFist
		}
		if g.inventory.PendingWeapon != want {
			t.Fatal("fist command used the wrong toggle policy")
		}
	}
}
func TestBoomFallbackThresholds(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true})
	g.inventory.ReadyWeapon = weaponRocketLauncher
	g.inventory.Weapons[82] = true
	g.stats.Bullets = 0
	g.stats.Shells = 2
	if g.fallbackWeapon(true) != weaponSuperShotgun || g.fallbackWeapon(false) != weaponFist {
		t.Fatal("SSG compatibility threshold differs")
	}
	g.stats.Shells = 0
	g.stats.Cells = 40
	g.inventory.Weapons[2006] = true
	if g.fallbackWeapon(true) != weaponBFG || g.fallbackWeapon(false) != weaponFist {
		t.Fatal("BFG compatibility threshold differs")
	}
}
func TestBoomAutomaticSwitchReleaseAndPending(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true, BoomGameOptions: &demo.GameOptions{Seed: 1993}})
	g.inventory.ReadyWeapon = weaponPistol
	g.stats.Bullets = 0
	g.weaponAttackDown = true
	if slot := g.localWeaponCommand(0, false); slot != 1 {
		t.Fatal("first out-of-ammo input failed to select fallback after release")
	}
	g.inventory.PendingWeapon = weaponFist
	if slot := g.localWeaponCommand(0, true); slot != 0 {
		t.Fatal("pending fallback was emitted repeatedly")
	}
}
