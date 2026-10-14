package doomruntime

import (
	"gddoom/internal/doomrand"
	"gddoom/internal/runtimecfg"
	"math"
	"testing"
)

func TestNativeInputPreferencesSurviveRestartAndMatchMainMouse(t *testing.T) {
	c := nativeSaveFixture(t)
	binds := runtimecfg.DefaultInputBindings()
	binds.Fire = runtimecfg.KeyBinding{"MB2", "LCTRL"}
	c.SetControls(true, 2)
	c.SetInputPreferences(binds, true, true, false, 1.5)
	fresh, err := c.session.opts.NewGameLoader("E1M1")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Restart(fresh); err != nil {
		t.Fatal(err)
	}
	g := c.Game.g
	if !g.alwaysRun || g.autoWeaponSwitch || g.opts.MouseLookSpeed != 2 || g.opts.KeyboardTurnSpeed != 1.5 || !g.opts.MouseInvert || g.opts.InputBindings.Fire != binds.Fire {
		t.Fatal("restart lost live controls")
	}
	for _, delta := range []int{-13, -1, 0, 1, 15} {
		want := float64(g.mouseLookTurnRaw(delta)) * (2 * math.Pi / 4294967296)
		if got := c.Game.MouseTurn(delta); got != want {
			t.Fatalf("mouse turn=%v want=%v", got, want)
		}
	}
	c.SetInputPreferences(binds, false, false, true, 1)
	if c.Game.MouseTurn(15) != 0 {
		t.Fatal("disabled aiming still turned")
	}
}
func TestNativeWeaponCycleCommandMatchesMain(t *testing.T) {
	native := loadNativeCombatGame(t)
	native.g.inventory.Weapons[2001] = true
	native.g.stats.Shells = 20
	reference := loadNativeCombatGame(t)
	reference.g.inventory.Weapons[2001] = true
	reference.g.stats.Shells = 20
	rnd, prnd := doomrand.State()
	native.Tick(NativeMeshInput{WeaponCycle: 1})
	doomrand.SetState(rnd, prnd)
	reference.g.cycleWeapon(1)
	reference.g.capturePrevState()
	reference.g.stepGameplayFromDemoTic(DemoTic{})
	if native.g.inventory.PendingWeapon != weaponShotgun || native.g.inventory.PendingWeapon != reference.g.inventory.PendingWeapon || native.g.SimChecksum() != reference.g.SimChecksum() {
		t.Fatal("native weapon cycle diverged from main command")
	}
}

func TestNativeCombinedStrafeModifierMatchesMainMovement(t *testing.T) {
	native := loadNativeCombatGame(t)
	reference := loadNativeCombatGame(t)
	for range 35 {
		rnd, prnd := doomrand.State()
		native.Tick(NativeMeshInput{Side: 2, Run: true})
		afterRNG, afterPlayRNG := doomrand.State()
		doomrand.SetState(rnd, prnd)
		reference.g.capturePrevState()
		reference.g.stepGameplayFromDemoTic(DemoTic{Side: int8(2 * sideMove[1])})
		gotRNG, gotPlayRNG := doomrand.State()
		if native.g.SimChecksum() != reference.g.SimChecksum() || gotRNG != afterRNG || gotPlayRNG != afterPlayRNG {
			t.Fatal("combined strafing differs from main command")
		}
	}
}
