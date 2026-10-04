//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/runtimecfg"
	"testing"
)

func TestNativeBindingNamesCoverSharedKeyboardAndMouse(t *testing.T) {
	for _, name := range nativeBindingNames {
		_, key := nativeBindingKeys[name]
		_, mouse := nativeBindingMouse[name]
		if !key && !mouse {
			t.Fatalf("shared binding %s has no Raylib mapping", name)
		}
	}
}
func TestNativeRemappedCommandsModifiersAndWeapons(t *testing.T) {
	binds := runtimecfg.DefaultInputBindings()
	binds.MoveForward = runtimecfg.KeyBinding{"I", ""}
	binds.Fire = runtimecfg.KeyBinding{"MB2", ""}
	binds.Use = runtimecfg.KeyBinding{"MB3", ""}
	binds.Weapon3 = runtimecfg.KeyBinding{"K", ""}
	keys := map[string]bool{"I": true, "MB2": true, "MB3": true, "K": true, "RSHIFT": true, "RIGHT": true}
	down := func(name string) bool { return keys[name] }
	input := sampleNativeMovement(binds, false, false, false, down, down)
	if input.Forward != 1 || input.Turn != -1 || !input.Run || !input.Fire || !input.Use || input.WeaponSlot != 3 {
		t.Fatalf("remapped commands=%+v", input)
	}
	keys["RALT"] = true
	input = sampleNativeMovement(binds, true, false, false, down, down)
	if input.Run || input.Turn != 0 || input.Side != 1 {
		t.Fatalf("always-run/strafe modifier=%+v", input)
	}
	keys = map[string]bool{"MB5": true}
	input = sampleNativeMovement(binds, false, false, false, down, down)
	if input.WeaponCycle != 1 {
		t.Fatal("extra mouse button did not select next weapon")
	}
	input = sampleNativeMovement(binds, false, false, true, down, down)
	if input.WeaponSlot != 0 || input.WeaponCycle != 0 {
		t.Fatal("cheat typing switched weapons")
	}
	keys = map[string]bool{"UP": true, "RIGHT": true, "E": true, "I": true}
	input = sampleNativeMovement(binds, false, true, false, down, down)
	if input.Forward != 1 || input.Turn != -1 || input.Use {
		t.Fatalf("automap command/key reservations=%+v", input)
	}
}

func TestNativeDefaultRunAndShiftWalking(t *testing.T) {
	bindings := runtimecfg.DefaultInputBindings()
	for _, modifier := range []string{"", "LSHIFT", "RSHIFT"} {
		held := func(name string) bool { return name == "W" || (modifier != "" && name == modifier) }
		input := sampleNativeMovement(bindings, runtimecfg.DefaultAlwaysRun, false, false, held, held)
		if input.Forward != 1 || input.Run != (modifier == "") {
			t.Fatalf("modifier=%q command=%+v", modifier, input)
		}
	}
}

func TestNativeHUDResizeChordDoesNotChangeWeapon(t *testing.T) {
	bindings := runtimecfg.DefaultInputBindings()
	bindings.WeaponNext = runtimecfg.KeyBinding{"]", "MB5"}
	bindings.WeaponPrev = runtimecfg.KeyBinding{"[", "MB4"}
	for _, tc := range []struct {
		ctrl, pressed string
		cycle         int
	}{
		{"", "]", 1}, {"", "[", -1}, {"LCTRL", "]", 0}, {"RCTRL", "[", 0},
		{"LCTRL", "MB5", 1}, {"RCTRL", "MB4", -1},
	} {
		held := func(name string) bool { return tc.ctrl != "" && name == tc.ctrl }
		pressed := func(name string) bool { return name == tc.pressed }
		in := sampleNativeMovement(bindings, true, false, false, held, pressed)
		if in.WeaponCycle != tc.cycle {
			t.Fatalf("ctrl=%s key=%s cycle=%d want=%d", tc.ctrl, tc.pressed, in.WeaponCycle, tc.cycle)
		}
	}
}
