package runtimecfg

import "strings"

import "github.com/hajimehoshi/ebiten/v2"

type KeyBinding [2]string

type InputBindings struct {
	MoveForward    KeyBinding `toml:"move_forward"`
	MoveBackward   KeyBinding `toml:"move_backward"`
	StrafeLeft     KeyBinding `toml:"strafe_left"`
	StrafeRight    KeyBinding `toml:"strafe_right"`
	TurnLeft       KeyBinding `toml:"turn_left"`
	TurnRight      KeyBinding `toml:"turn_right"`
	StrafeModifier KeyBinding `toml:"strafe_modifier"`
	RunModifier    KeyBinding `toml:"run_modifier"`
	Fire           KeyBinding `toml:"fire"`
	Chat           KeyBinding `toml:"chat"`
	Voice          KeyBinding `toml:"voice"`
	Use            KeyBinding `toml:"use"`
	Automap        KeyBinding `toml:"automap"`
	WeaponPrev     KeyBinding `toml:"weapon_prev"`
	WeaponNext     KeyBinding `toml:"weapon_next"`
	Weapon1        KeyBinding `toml:"weapon_1"`
	Weapon2        KeyBinding `toml:"weapon_2"`
	Weapon3        KeyBinding `toml:"weapon_3"`
	Weapon4        KeyBinding `toml:"weapon_4"`
	Weapon5        KeyBinding `toml:"weapon_5"`
	Weapon6        KeyBinding `toml:"weapon_6"`
	Weapon7        KeyBinding `toml:"weapon_7"`
}

func DefaultInputBindings() InputBindings {
	var out InputBindings
	out.MoveForward = KeyBinding{"W", "UP"}
	out.MoveBackward = KeyBinding{"S", "DOWN"}
	out.StrafeLeft = KeyBinding{"A", ""}
	out.StrafeRight = KeyBinding{"D", ""}
	out.TurnLeft = KeyBinding{"LEFT", ""}
	out.TurnRight = KeyBinding{"RIGHT", ""}
	out.StrafeModifier = KeyBinding{"LALT", "RALT"}
	out.RunModifier = KeyBinding{"LSHIFT", "RSHIFT"}
	out.Fire = KeyBinding{"LCTRL", "MB1"}
	out.Chat = KeyBinding{"T", ""}
	out.Voice = KeyBinding{"CAPSLOCK", ""}
	out.Use = KeyBinding{"SPACE", "E"}
	out.Automap = KeyBinding{"TAB", ""}
	out.WeaponPrev = KeyBinding{"PAGEUP", "MB4"}
	out.WeaponNext = KeyBinding{"PAGEDOWN", "MB5"}
	out.Weapon1 = KeyBinding{"1", ""}
	out.Weapon2 = KeyBinding{"2", ""}
	out.Weapon3 = KeyBinding{"3", ""}
	out.Weapon4 = KeyBinding{"4", ""}
	out.Weapon5 = KeyBinding{"5", ""}
	out.Weapon6 = KeyBinding{"6", ""}
	out.Weapon7 = KeyBinding{"7", ""}
	return out
}

func NormalizeInputBindings(in InputBindings) InputBindings {
	out := in
	def := DefaultInputBindings()
	normalize := func(dst *KeyBinding, fallback KeyBinding) {
		for i := range *dst {
			dst[i] = strings.ToUpper(strings.TrimSpace(dst[i]))
		}
		if dst[0] == "" && dst[1] == "" {
			*dst = fallback
		}
		if dst[0] == dst[1] {
			dst[1] = ""
		}
	}
	normalize(&out.MoveForward, def.MoveForward)
	normalize(&out.MoveBackward, def.MoveBackward)
	normalize(&out.StrafeLeft, def.StrafeLeft)
	normalize(&out.StrafeRight, def.StrafeRight)
	normalize(&out.TurnLeft, def.TurnLeft)
	normalize(&out.TurnRight, def.TurnRight)
	normalize(&out.StrafeModifier, def.StrafeModifier)
	normalize(&out.RunModifier, def.RunModifier)
	normalize(&out.Fire, def.Fire)
	normalize(&out.Chat, def.Chat)
	normalize(&out.Voice, def.Voice)
	normalize(&out.Use, def.Use)
	normalize(&out.Automap, def.Automap)
	normalize(&out.WeaponPrev, def.WeaponPrev)
	normalize(&out.WeaponNext, def.WeaponNext)
	normalize(&out.Weapon1, def.Weapon1)
	normalize(&out.Weapon2, def.Weapon2)
	normalize(&out.Weapon3, def.Weapon3)
	normalize(&out.Weapon4, def.Weapon4)
	normalize(&out.Weapon5, def.Weapon5)
	normalize(&out.Weapon6, def.Weapon6)
	normalize(&out.Weapon7, def.Weapon7)
	return out
}

var bindingHeldKeyMap = map[string]ebiten.Key{
	"A":         ebiten.KeyA,
	"B":         ebiten.KeyB,
	"C":         ebiten.KeyC,
	"D":         ebiten.KeyD,
	"E":         ebiten.KeyE,
	"F":         ebiten.KeyF,
	"G":         ebiten.KeyG,
	"H":         ebiten.KeyH,
	"I":         ebiten.KeyI,
	"J":         ebiten.KeyJ,
	"K":         ebiten.KeyK,
	"L":         ebiten.KeyL,
	"M":         ebiten.KeyM,
	"N":         ebiten.KeyN,
	"O":         ebiten.KeyO,
	"P":         ebiten.KeyP,
	"Q":         ebiten.KeyQ,
	"R":         ebiten.KeyR,
	"S":         ebiten.KeyS,
	"T":         ebiten.KeyT,
	"U":         ebiten.KeyU,
	"V":         ebiten.KeyV,
	"W":         ebiten.KeyW,
	"X":         ebiten.KeyX,
	"Y":         ebiten.KeyY,
	"Z":         ebiten.KeyZ,
	"0":         ebiten.Key0,
	"1":         ebiten.Key1,
	"2":         ebiten.Key2,
	"3":         ebiten.Key3,
	"4":         ebiten.Key4,
	"5":         ebiten.Key5,
	"6":         ebiten.Key6,
	"7":         ebiten.Key7,
	"8":         ebiten.Key8,
	"9":         ebiten.Key9,
	"UP":        ebiten.KeyArrowUp,
	"DOWN":      ebiten.KeyArrowDown,
	"LEFT":      ebiten.KeyArrowLeft,
	"RIGHT":     ebiten.KeyArrowRight,
	"SPACE":     ebiten.KeySpace,
	"TAB":       ebiten.KeyTab,
	"ENTER":     ebiten.KeyEnter,
	"KPENTER":   ebiten.KeyKPEnter,
	"ESCAPE":    ebiten.KeyEscape,
	"LSHIFT":    ebiten.KeyShiftLeft,
	"RSHIFT":    ebiten.KeyShiftRight,
	"LCTRL":     ebiten.KeyControlLeft,
	"RCTRL":     ebiten.KeyControlRight,
	"LALT":      ebiten.KeyAltLeft,
	"RALT":      ebiten.KeyAltRight,
	"CAPSLOCK":  ebiten.KeyCapsLock,
	"PAGEUP":    ebiten.KeyPageUp,
	"PAGEDOWN":  ebiten.KeyPageDown,
	"[":         ebiten.KeyBracketLeft,
	"]":         ebiten.KeyBracketRight,
	"\\":        ebiten.KeyBackslash,
	"-":         ebiten.KeyMinus,
	"=":         ebiten.KeyEqual,
	",":         ebiten.KeyComma,
	".":         ebiten.KeyPeriod,
	"/":         ebiten.KeySlash,
	"F1":        ebiten.KeyF1,
	"F2":        ebiten.KeyF2,
	"F3":        ebiten.KeyF3,
	"F4":        ebiten.KeyF4,
	"F5":        ebiten.KeyF5,
	"F6":        ebiten.KeyF6,
	"F7":        ebiten.KeyF7,
	"F8":        ebiten.KeyF8,
	"F9":        ebiten.KeyF9,
	"F10":       ebiten.KeyF10,
	"F11":       ebiten.KeyF11,
	"F12":       ebiten.KeyF12,
	"BACKSPACE": ebiten.KeyBackspace,
}

var bindingHeldMouseMap = map[string]ebiten.MouseButton{
	"MB1": ebiten.MouseButtonLeft,
	"MB2": ebiten.MouseButtonRight,
	"MB3": ebiten.MouseButtonMiddle,
	"MB4": ebiten.MouseButton3,
	"MB5": ebiten.MouseButton4,
}

func BindingHeld(binding KeyBinding) bool {
	for _, name := range binding {
		if BindingNameHeld(name) {
			return true
		}
	}
	return false
}

func BindingNameHeld(name string) bool {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	if key, ok := bindingHeldKeyMap[name]; ok {
		return ebiten.IsKeyPressed(key)
	}
	if button, ok := bindingHeldMouseMap[name]; ok {
		return ebiten.IsMouseButtonPressed(button)
	}
	return false
}
