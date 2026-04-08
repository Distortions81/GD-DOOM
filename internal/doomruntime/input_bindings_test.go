package doomruntime

import (
	"testing"

	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestSetBindingSlotPreservesSecondaryWhenPrimaryCleared(t *testing.T) {
	binds := runtimecfg.InputBindings{
		Use: runtimecfg.KeyBinding{"SPACE", "E"},
	}
	setBindingSlot(&binds, bindingUse, 0, "")
	if got := bindingValue(binds, bindingUse); got[0] != "E" || got[1] != "" {
		t.Fatalf("use binding=%v want [E \"\"]", got)
	}
}

func TestBindingHeldUsesConfiguredActionKeys(t *testing.T) {
	g := &game{
		opts: Options{
			InputBindings: runtimecfg.NormalizeInputBindings(runtimecfg.InputBindings{
				MoveForward: runtimecfg.KeyBinding{"I", ""},
			}),
		},
		input: gameInputSnapshot{
			pressedKeys: map[ebiten.Key]struct{}{
				ebiten.KeyI: {},
			},
		},
	}
	if !g.bindingHeld(bindingMoveForward) {
		t.Fatal("expected custom move-forward binding to register as held")
	}
	if g.bindingHeld(bindingMoveBackward) {
		t.Fatal("did not expect unrelated binding to register as held")
	}
}

func TestBindingHeldUsesConfiguredMouseButtons(t *testing.T) {
	g := &game{
		opts: Options{
			InputBindings: runtimecfg.NormalizeInputBindings(runtimecfg.InputBindings{
				Fire: runtimecfg.KeyBinding{"MB1", ""},
			}),
		},
		input: gameInputSnapshot{
			pressedMouseButtons: map[ebiten.MouseButton]struct{}{
				ebiten.MouseButtonLeft: {},
			},
		},
	}
	if !g.bindingHeld(bindingFire) {
		t.Fatal("expected mouse fire binding to register as held")
	}
}

func TestBindingConflictMessageReportsSelectedConflict(t *testing.T) {
	binds := runtimecfg.NormalizeInputBindings(runtimecfg.InputBindings{
		MoveForward: runtimecfg.KeyBinding{"W", ""},
		Use:         runtimecfg.KeyBinding{"W", ""},
	})
	msg := bindingConflictMessage(binds, bindingUse, 0)
	if msg != "CONFLICT: MOVE FORWARD PRIMARY" {
		t.Fatalf("bindingConflictMessage()=%q", msg)
	}
}
