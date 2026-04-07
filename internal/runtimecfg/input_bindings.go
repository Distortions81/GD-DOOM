package runtimecfg

import "strings"

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
	out.Fire = KeyBinding{"LCTRL", "RCTRL"}
	out.Use = KeyBinding{"SPACE", "E"}
	out.Automap = KeyBinding{"TAB", ""}
	out.WeaponPrev = KeyBinding{"PAGEUP", "["}
	out.WeaponNext = KeyBinding{"PAGEDOWN", "]"}
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
