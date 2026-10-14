//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/runtimecfg"
	rl "github.com/gen2brain/raylib-go/raylib"
	"strconv"
	"strings"
)

var nativeBindingKeys = func() map[string]int32 {
	keys := map[string]int32{
		"UP": rl.KeyUp, "DOWN": rl.KeyDown, "LEFT": rl.KeyLeft, "RIGHT": rl.KeyRight, "SPACE": rl.KeySpace, "TAB": rl.KeyTab,
		"ENTER": rl.KeyEnter, "KPENTER": rl.KeyKpEnter, "ESCAPE": rl.KeyEscape, "LSHIFT": rl.KeyLeftShift, "RSHIFT": rl.KeyRightShift,
		"LCTRL": rl.KeyLeftControl, "RCTRL": rl.KeyRightControl, "LALT": rl.KeyLeftAlt, "RALT": rl.KeyRightAlt, "CAPSLOCK": rl.KeyCapsLock,
		"PAGEUP": rl.KeyPageUp, "PAGEDOWN": rl.KeyPageDown, "[": rl.KeyLeftBracket, "]": rl.KeyRightBracket, "\\": rl.KeyBackSlash,
		"-": rl.KeyMinus, "=": rl.KeyEqual, ",": rl.KeyComma, ".": rl.KeyPeriod, "/": rl.KeySlash, "BACKSPACE": rl.KeyBackspace,
	}
	for key := rl.KeyA; key <= rl.KeyZ; key++ {
		keys[string(rune(key))] = int32(key)
	}
	for key := rl.KeyZero; key <= rl.KeyNine; key++ {
		keys[string(rune(key))] = int32(key)
	}
	for i := 0; i < 12; i++ {
		keys["F"+strconv.Itoa(i+1)] = rl.KeyF1 + int32(i)
	}
	return keys
}()
var nativeBindingMouse = map[string]rl.MouseButton{"MB1": rl.MouseButtonLeft, "MB2": rl.MouseButtonRight, "MB3": rl.MouseButtonMiddle, "MB4": rl.MouseButtonSide, "MB5": rl.MouseButtonExtra}
var nativeBindingNames = doomruntime.NativeBindingNames()

func nativeBindingNameHeld(name string) bool {
	if key, ok := nativeBindingKeys[name]; ok {
		return rl.IsKeyDown(key)
	}
	if button, ok := nativeBindingMouse[name]; ok {
		return rl.IsMouseButtonDown(button)
	}
	return false
}
func nativeBindingNamePressed(name string) bool {
	if key, ok := nativeBindingKeys[name]; ok {
		return rl.IsKeyPressed(key)
	}
	if button, ok := nativeBindingMouse[name]; ok {
		return rl.IsMouseButtonPressed(button)
	}
	return false
}
func nativeBindingHeld(binding runtimecfg.KeyBinding) bool {
	return bindingMatches(binding, nativeBindingNameHeld)
}
func nativeBindingPressed(binding runtimecfg.KeyBinding) bool {
	return bindingMatches(binding, nativeBindingNamePressed)
}
func bindingMatches(binding runtimecfg.KeyBinding, match func(string) bool) bool {
	for _, name := range binding {
		if name != "" && match(strings.ToUpper(strings.TrimSpace(name))) {
			return true
		}
	}
	return false
}
func nativeBindingRows(bindings runtimecfg.InputBindings) []doomruntime.NativeBindingDefinition {
	rows := doomruntime.NativeBindingDefinitions(bindings)
	out := rows[:0]
	for _, row := range rows {
		// Voice is intentionally outside the native parity work.
		if row.Label == "PUSH TO TALK" {
			continue
		}
		out = append(out, row)
	}
	return out
}

// A pure sampler keeps gameplay input independent of a window backend and makes
// remapped keyboard/mouse commands comparable to the main engine's commands.
func sampleNativeMovement(bindings runtimecfg.InputBindings, alwaysRun, automap, cheatTyping bool, held, pressed func(string) bool) doomruntime.NativeMeshInput {
	down := func(binding runtimecfg.KeyBinding) bool {
		return bindingMatches(binding, func(name string) bool {
			if automap && (name == "UP" || name == "DOWN" || name == "LEFT" || name == "RIGHT" || name == "E") {
				return false
			}
			return held(name)
		})
	}
	axis := func(positive, negative bool) int {
		value := 0
		if positive {
			value++
		}
		if negative {
			value--
		}
		return value
	}
	input := doomruntime.NativeMeshInput{
		Forward: axis(down(bindings.MoveForward), down(bindings.MoveBackward)),
		Side:    axis(down(bindings.StrafeRight), down(bindings.StrafeLeft)),
		Turn:    axis(down(bindings.TurnLeft), down(bindings.TurnRight)),
		Run:     alwaysRun != bindingMatches(bindings.RunModifier, held), Fire: bindingMatches(bindings.Fire, held), Use: down(bindings.Use),
	}
	if automap {
		input.Turn += axis(held("Q"), held("E"))
	}
	if bindingMatches(bindings.StrafeModifier, held) {
		input.Side -= input.Turn
		input.Turn = 0
	}
	if !cheatTyping {
		weapons := []runtimecfg.KeyBinding{bindings.Weapon1, bindings.Weapon2, bindings.Weapon3, bindings.Weapon4, bindings.Weapon5, bindings.Weapon6, bindings.Weapon7}
		for i, binding := range weapons {
			if bindingMatches(binding, pressed) {
				input.WeaponSlot = i + 1
			}
		}
		// Ctrl+brackets resize the HUD and must not also cycle a weapon.
		ctrl := held("LCTRL") || held("RCTRL")
		hudChord := func(binding runtimecfg.KeyBinding, bracket string) bool {
			if !ctrl || !pressed(bracket) {
				return false
			}
			for _, name := range binding {
				if name == bracket {
					return true
				}
			}
			return false
		}
		if bindingMatches(bindings.WeaponNext, pressed) && !hudChord(bindings.WeaponNext, "]") {
			input.WeaponCycle = 1
		}
		if bindingMatches(bindings.WeaponPrev, pressed) && !hudChord(bindings.WeaponPrev, "[") {
			input.WeaponCycle = -1
		}
	}
	return input
}

func nativeKeyBound(bindings runtimecfg.InputBindings, name string) bool {
	for _, row := range doomruntime.NativeBindingDefinitions(bindings) {
		for _, bound := range row.Value {
			if bound == name {
				return true
			}
		}
	}
	return false
}
