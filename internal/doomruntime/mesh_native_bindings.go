package doomruntime

import (
	"gddoom/internal/runtimecfg"
	"math"
)

type NativeBindingDefinition struct {
	ID    int
	Label string
	Value runtimecfg.KeyBinding
}

func NativeBindingDefinitions(bindings runtimecfg.InputBindings) []NativeBindingDefinition {
	out := make([]NativeBindingDefinition, 0, len(bindingActionDefs))
	for _, def := range bindingActionDefs {
		out = append(out, NativeBindingDefinition{int(def.action), def.label, bindingValue(bindings, def.action)})
	}
	return out
}
func SetNativeBinding(bindings *runtimecfg.InputBindings, action, slot int, name string) {
	if action < 0 || action >= int(bindingActionCount) {
		return
	}
	setBindingSlot(bindings, bindingAction(action), slot, name)
}
func NativeBindingConflict(bindings runtimecfg.InputBindings, action, slot int) string {
	return bindingConflictMessage(bindings, bindingAction(action), slot)
}
func NativeBindingNames() []string {
	out := make([]string, 0, len(supportedBindingKeys)+len(supportedBindingMouseButtons))
	for _, key := range supportedBindingKeys {
		out = append(out, key.name)
	}
	for _, button := range supportedBindingMouseButtons {
		out = append(out, button.name)
	}
	return out
}
func (c *NativeCampaign) SetInputPreferences(bindings runtimecfg.InputBindings, mouseLook, mouseInvert, autoWeaponSwitch bool, keyboardSpeed float64) {
	for _, opts := range []*Options{&c.session.opts, &c.Game.g.opts} {
		opts.InputBindings = runtimecfg.NormalizeInputBindings(bindings)
		opts.MouseLook, opts.MouseInvert, opts.AutoWeaponSwitch = mouseLook, mouseInvert, autoWeaponSwitch
		opts.KeyboardTurnSpeed = normalizeKeyboardTurnSpeed(keyboardSpeed)
	}
	c.Game.g.autoWeaponSwitch = autoWeaponSwitch
	if c.Game.g.opts.DemoScript != nil {
		c.Game.g.opts.AutoWeaponSwitch, c.Game.g.autoWeaponSwitch = true, true
	}
}

// SetCameraSmoothing retains the renderer preference across campaign reloads.
func (c *NativeCampaign) SetCameraSmoothing(enabled bool) {
	c.session.opts.SmoothCameraYaw = enabled
	c.Game.g.opts.SmoothCameraYaw = enabled
}

// MouseTurn uses the same source-port angular units per pixel as the main host.
func (n *NativeMeshGame) MouseTurn(dx int) float64 {
	if !n.g.opts.MouseLook {
		return 0
	}
	raw := mouseLookTurnRawScaled(dx, n.g.opts.MouseLookSpeed, 1, n.g.opts.MouseInvert)
	return float64(raw) * (2 * math.Pi / 4294967296)
}
