package doomruntime

import (
	"strings"

	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
)

type bindingAction int

const (
	bindingMoveForward bindingAction = iota
	bindingMoveBackward
	bindingStrafeLeft
	bindingStrafeRight
	bindingTurnLeft
	bindingTurnRight
	bindingStrafeModifier
	bindingRunModifier
	bindingFire
	bindingChat
	bindingVoice
	bindingUse
	bindingAutomap
	bindingWeaponPrev
	bindingWeaponNext
	bindingWeapon1
	bindingWeapon2
	bindingWeapon3
	bindingWeapon4
	bindingWeapon5
	bindingWeapon6
	bindingWeapon7
	bindingActionCount
)

type bindingActionDef struct {
	action bindingAction
	label  string
}

var bindingActionDefs = [...]bindingActionDef{
	{bindingMoveForward, "MOVE FORWARD"},
	{bindingMoveBackward, "MOVE BACKWARD"},
	{bindingStrafeLeft, "STRAFE LEFT"},
	{bindingStrafeRight, "STRAFE RIGHT"},
	{bindingTurnLeft, "TURN LEFT"},
	{bindingTurnRight, "TURN RIGHT"},
	{bindingStrafeModifier, "STRAFE MODIFIER"},
	{bindingRunModifier, "RUN MODIFIER"},
	{bindingFire, "FIRE"},
	{bindingChat, "CHAT"},
	{bindingVoice, "PUSH TO TALK"},
	{bindingUse, "USE / OPEN"},
	{bindingAutomap, "AUTOMAP"},
	{bindingWeaponPrev, "WEAPON PREV"},
	{bindingWeaponNext, "WEAPON NEXT"},
	{bindingWeapon1, "WEAPON 1"},
	{bindingWeapon2, "WEAPON 2"},
	{bindingWeapon3, "WEAPON 3"},
	{bindingWeapon4, "WEAPON 4"},
	{bindingWeapon5, "WEAPON 5"},
	{bindingWeapon6, "WEAPON 6"},
	{bindingWeapon7, "WEAPON 7"},
}

type bindingKeyDef struct {
	key   ebiten.Key
	name  string
	label string
}

type bindingMouseDef struct {
	button ebiten.MouseButton
	name   string
	label  string
}

var supportedBindingKeys = [...]bindingKeyDef{
	{ebiten.KeyA, "A", "A"}, {ebiten.KeyB, "B", "B"}, {ebiten.KeyC, "C", "C"}, {ebiten.KeyD, "D", "D"},
	{ebiten.KeyE, "E", "E"}, {ebiten.KeyF, "F", "F"}, {ebiten.KeyG, "G", "G"}, {ebiten.KeyH, "H", "H"},
	{ebiten.KeyI, "I", "I"}, {ebiten.KeyJ, "J", "J"}, {ebiten.KeyK, "K", "K"}, {ebiten.KeyL, "L", "L"},
	{ebiten.KeyM, "M", "M"}, {ebiten.KeyN, "N", "N"}, {ebiten.KeyO, "O", "O"}, {ebiten.KeyP, "P", "P"},
	{ebiten.KeyQ, "Q", "Q"}, {ebiten.KeyR, "R", "R"}, {ebiten.KeyS, "S", "S"}, {ebiten.KeyT, "T", "T"},
	{ebiten.KeyU, "U", "U"}, {ebiten.KeyV, "V", "V"}, {ebiten.KeyW, "W", "W"}, {ebiten.KeyX, "X", "X"},
	{ebiten.KeyY, "Y", "Y"}, {ebiten.KeyZ, "Z", "Z"},
	{ebiten.Key0, "0", "0"}, {ebiten.Key1, "1", "1"}, {ebiten.Key2, "2", "2"}, {ebiten.Key3, "3", "3"},
	{ebiten.Key4, "4", "4"}, {ebiten.Key5, "5", "5"}, {ebiten.Key6, "6", "6"}, {ebiten.Key7, "7", "7"},
	{ebiten.Key8, "8", "8"}, {ebiten.Key9, "9", "9"},
	{ebiten.KeyArrowUp, "UP", "UP"}, {ebiten.KeyArrowDown, "DOWN", "DOWN"},
	{ebiten.KeyArrowLeft, "LEFT", "LEFT"}, {ebiten.KeyArrowRight, "RIGHT", "RIGHT"},
	{ebiten.KeySpace, "SPACE", "SPACE"}, {ebiten.KeyTab, "TAB", "TAB"},
	{ebiten.KeyEnter, "ENTER", "ENTER"}, {ebiten.KeyKPEnter, "KPENTER", "KPENTER"},
	{ebiten.KeyEscape, "ESCAPE", "ESC"},
	{ebiten.KeyShiftLeft, "LSHIFT", "LSHIFT"}, {ebiten.KeyShiftRight, "RSHIFT", "RSHIFT"},
	{ebiten.KeyControlLeft, "LCTRL", "LCTRL"}, {ebiten.KeyControlRight, "RCTRL", "RCTRL"},
	{ebiten.KeyAltLeft, "LALT", "LALT"}, {ebiten.KeyAltRight, "RALT", "RALT"},
	{ebiten.KeyCapsLock, "CAPSLOCK", "CAPS"},
	{ebiten.KeyPageUp, "PAGEUP", "PGUP"}, {ebiten.KeyPageDown, "PAGEDOWN", "PGDN"},
	{ebiten.KeyBracketLeft, "[", "["}, {ebiten.KeyBracketRight, "]", "]"},
	{ebiten.KeyBackslash, "\\", "\\"},
	{ebiten.KeyMinus, "-", "-"}, {ebiten.KeyEqual, "=", "="},
	{ebiten.KeyComma, ",", ","}, {ebiten.KeyPeriod, ".", "."}, {ebiten.KeySlash, "/", "/"},
	{ebiten.KeyF1, "F1", "F1"}, {ebiten.KeyF2, "F2", "F2"}, {ebiten.KeyF3, "F3", "F3"}, {ebiten.KeyF4, "F4", "F4"},
	{ebiten.KeyF5, "F5", "F5"}, {ebiten.KeyF6, "F6", "F6"}, {ebiten.KeyF7, "F7", "F7"}, {ebiten.KeyF8, "F8", "F8"},
	{ebiten.KeyF9, "F9", "F9"}, {ebiten.KeyF10, "F10", "F10"}, {ebiten.KeyF11, "F11", "F11"}, {ebiten.KeyF12, "F12", "F12"},
	{ebiten.KeyBackspace, "BACKSPACE", "BKSP"},
}

var supportedBindingMouseButtons = [...]bindingMouseDef{
	{ebiten.MouseButtonLeft, "MB1", "MB1"},
	{ebiten.MouseButtonRight, "MB2", "MB2"},
	{ebiten.MouseButtonMiddle, "MB3", "MB3"},
	{ebiten.MouseButton3, "MB4", "MB4"},
	{ebiten.MouseButton4, "MB5", "MB5"},
}

const keybindMenuVisibleRows = 8

func clampKeybindRow(row int) int {
	if row < 0 {
		return 0
	}
	if row >= int(bindingActionCount) {
		return int(bindingActionCount) - 1
	}
	return row
}

func keybindMenuStartRow(selected int) int {
	selected = clampKeybindRow(selected)
	maxStart := int(bindingActionCount) - keybindMenuVisibleRows
	if maxStart < 0 {
		maxStart = 0
	}
	start := selected - keybindMenuVisibleRows/2
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}
	return start
}

func bindingActionLabel(action bindingAction) string {
	if int(action) < 0 || int(action) >= len(bindingActionDefs) {
		return ""
	}
	return bindingActionDefs[action].label
}

func bindingValue(bindings runtimecfg.InputBindings, action bindingAction) runtimecfg.KeyBinding {
	switch action {
	case bindingMoveForward:
		return bindings.MoveForward
	case bindingMoveBackward:
		return bindings.MoveBackward
	case bindingStrafeLeft:
		return bindings.StrafeLeft
	case bindingStrafeRight:
		return bindings.StrafeRight
	case bindingTurnLeft:
		return bindings.TurnLeft
	case bindingTurnRight:
		return bindings.TurnRight
	case bindingStrafeModifier:
		return bindings.StrafeModifier
	case bindingRunModifier:
		return bindings.RunModifier
	case bindingFire:
		return bindings.Fire
	case bindingChat:
		return bindings.Chat
	case bindingVoice:
		return bindings.Voice
	case bindingUse:
		return bindings.Use
	case bindingAutomap:
		return bindings.Automap
	case bindingWeaponPrev:
		return bindings.WeaponPrev
	case bindingWeaponNext:
		return bindings.WeaponNext
	case bindingWeapon1:
		return bindings.Weapon1
	case bindingWeapon2:
		return bindings.Weapon2
	case bindingWeapon3:
		return bindings.Weapon3
	case bindingWeapon4:
		return bindings.Weapon4
	case bindingWeapon5:
		return bindings.Weapon5
	case bindingWeapon6:
		return bindings.Weapon6
	case bindingWeapon7:
		return bindings.Weapon7
	default:
		return runtimecfg.KeyBinding{}
	}
}

func setBindingValue(bindings *runtimecfg.InputBindings, action bindingAction, value runtimecfg.KeyBinding) {
	if bindings == nil {
		return
	}
	switch action {
	case bindingMoveForward:
		bindings.MoveForward = value
	case bindingMoveBackward:
		bindings.MoveBackward = value
	case bindingStrafeLeft:
		bindings.StrafeLeft = value
	case bindingStrafeRight:
		bindings.StrafeRight = value
	case bindingTurnLeft:
		bindings.TurnLeft = value
	case bindingTurnRight:
		bindings.TurnRight = value
	case bindingStrafeModifier:
		bindings.StrafeModifier = value
	case bindingRunModifier:
		bindings.RunModifier = value
	case bindingFire:
		bindings.Fire = value
	case bindingChat:
		bindings.Chat = value
	case bindingVoice:
		bindings.Voice = value
	case bindingUse:
		bindings.Use = value
	case bindingAutomap:
		bindings.Automap = value
	case bindingWeaponPrev:
		bindings.WeaponPrev = value
	case bindingWeaponNext:
		bindings.WeaponNext = value
	case bindingWeapon1:
		bindings.Weapon1 = value
	case bindingWeapon2:
		bindings.Weapon2 = value
	case bindingWeapon3:
		bindings.Weapon3 = value
	case bindingWeapon4:
		bindings.Weapon4 = value
	case bindingWeapon5:
		bindings.Weapon5 = value
	case bindingWeapon6:
		bindings.Weapon6 = value
	case bindingWeapon7:
		bindings.Weapon7 = value
	}
}

func bindingSlotLabel(name string) string {
	name = strings.ToUpper(strings.TrimSpace(name))
	for _, def := range supportedBindingKeys {
		if def.name == name {
			return def.label
		}
	}
	for _, def := range supportedBindingMouseButtons {
		if def.name == name {
			return def.label
		}
	}
	if name == "" {
		return "-"
	}
	return name
}

func bindingKeyName(key ebiten.Key) string {
	for _, def := range supportedBindingKeys {
		if def.key == key {
			return def.name
		}
	}
	return ""
}

func bindingKeyFromName(name string) (ebiten.Key, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))
	for _, def := range supportedBindingKeys {
		if def.name == name {
			return def.key, true
		}
	}
	return ebiten.Key(0), false
}

func bindingMouseButtonFromName(name string) (ebiten.MouseButton, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))
	for _, def := range supportedBindingMouseButtons {
		if def.name == name {
			return def.button, true
		}
	}
	return ebiten.MouseButton(0), false
}

func bindingsContain(bindings runtimecfg.KeyBinding, key ebiten.Key) bool {
	for _, name := range bindings {
		if bound, ok := bindingKeyFromName(name); ok && bound == key {
			return true
		}
	}
	return false
}

func bindingSlotValue(bindings runtimecfg.InputBindings, action bindingAction, slot int) string {
	if slot < 0 || slot > 1 {
		return ""
	}
	return bindingValue(bindings, action)[slot]
}

func setBindingSlot(bindings *runtimecfg.InputBindings, action bindingAction, slot int, name string) {
	if bindings == nil || slot < 0 || slot > 1 {
		return
	}
	value := bindingValue(*bindings, action)
	value[slot] = strings.ToUpper(strings.TrimSpace(name))
	if value[0] == value[1] {
		value[1] = ""
	}
	if value[0] == "" && value[1] != "" {
		value[0], value[1] = value[1], ""
	}
	setBindingValue(bindings, action, value)
	*bindings = runtimecfg.NormalizeInputBindings(*bindings)
}

func bindingPressed(pressed map[ebiten.Key]struct{}, bindings runtimecfg.KeyBinding) bool {
	if len(pressed) == 0 {
		return false
	}
	for key := range pressed {
		if bindingsContain(bindings, key) {
			return true
		}
	}
	return false
}

func bindingPressedCounts(pressed map[ebiten.Key]int, bindings runtimecfg.KeyBinding) bool {
	if len(pressed) == 0 {
		return false
	}
	for key, n := range pressed {
		if n > 0 && bindingsContain(bindings, key) {
			return true
		}
	}
	return false
}

func bindingMousePressed(pressed map[ebiten.MouseButton]struct{}, bindings runtimecfg.KeyBinding) bool {
	if len(pressed) == 0 {
		return false
	}
	for button := range pressed {
		for _, name := range bindings {
			if bound, ok := bindingMouseButtonFromName(name); ok && bound == button {
				return true
			}
		}
	}
	return false
}

func bindingMousePressedCounts(pressed map[ebiten.MouseButton]int, bindings runtimecfg.KeyBinding) bool {
	if len(pressed) == 0 {
		return false
	}
	for button, n := range pressed {
		if n <= 0 {
			continue
		}
		for _, name := range bindings {
			if bound, ok := bindingMouseButtonFromName(name); ok && bound == button {
				return true
			}
		}
	}
	return false
}

func firstSupportedPressedKey(pressed map[ebiten.Key]struct{}) (ebiten.Key, bool) {
	if len(pressed) == 0 {
		return ebiten.Key(0), false
	}
	for _, def := range supportedBindingKeys {
		if _, ok := pressed[def.key]; ok {
			return def.key, true
		}
	}
	return ebiten.Key(0), false
}

func firstSupportedSessionPressedKey(pressed map[ebiten.Key]int) (ebiten.Key, bool) {
	if len(pressed) == 0 {
		return ebiten.Key(0), false
	}
	for _, def := range supportedBindingKeys {
		if pressed[def.key] > 0 {
			return def.key, true
		}
	}
	return ebiten.Key(0), false
}

func firstSupportedPressedMouseButton(pressed map[ebiten.MouseButton]struct{}) (ebiten.MouseButton, bool) {
	if len(pressed) == 0 {
		return ebiten.MouseButton(0), false
	}
	for _, def := range supportedBindingMouseButtons {
		if _, ok := pressed[def.button]; ok {
			return def.button, true
		}
	}
	return ebiten.MouseButton(0), false
}

func firstSupportedSessionPressedMouseButton(pressed map[ebiten.MouseButton]int) (ebiten.MouseButton, bool) {
	if len(pressed) == 0 {
		return ebiten.MouseButton(0), false
	}
	for _, def := range supportedBindingMouseButtons {
		if pressed[def.button] > 0 {
			return def.button, true
		}
	}
	return ebiten.MouseButton(0), false
}

func bindingMouseButtonName(button ebiten.MouseButton) string {
	for _, def := range supportedBindingMouseButtons {
		if def.button == button {
			return def.name
		}
	}
	return ""
}

func bindingConflict(bindings runtimecfg.InputBindings, action bindingAction, slot int) (bindingAction, int, bool) {
	name := strings.ToUpper(strings.TrimSpace(bindingSlotValue(bindings, action, slot)))
	if name == "" {
		return 0, 0, false
	}
	for other := bindingAction(0); other < bindingActionCount; other++ {
		if other == action {
			continue
		}
		value := bindingValue(bindings, other)
		for otherSlot, candidate := range value {
			if strings.EqualFold(strings.TrimSpace(candidate), name) {
				return other, otherSlot, true
			}
		}
	}
	return 0, 0, false
}

func bindingConflictMessage(bindings runtimecfg.InputBindings, action bindingAction, slot int) string {
	other, otherSlot, ok := bindingConflict(bindings, action, slot)
	if !ok {
		return ""
	}
	slotLabel := "PRIMARY"
	if otherSlot == 1 {
		slotLabel = "ALT"
	}
	return "CONFLICT: " + bindingActionLabel(other) + " " + slotLabel
}

func (sg *sessionGame) setFrontendBinding(action bindingAction, slot int, name string) {
	if sg == nil {
		return
	}
	setBindingSlot(&sg.opts.InputBindings, action, slot, name)
	if sg.g != nil {
		sg.g.opts.InputBindings = sg.opts.InputBindings
	}
	if sg.opts.OnInputBindingsChanged != nil {
		sg.opts.OnInputBindingsChanged(sg.opts.InputBindings)
	}
}
