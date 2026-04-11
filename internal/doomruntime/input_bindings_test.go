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

func TestBindingHeldUsesTouchActions(t *testing.T) {
	g := &game{
		input: gameInputSnapshot{
			touchHeldActions: touchActionUp | touchActionStrafeLeft | touchActionTurnRight | touchActionFire,
		},
	}
	if !g.bindingHeld(bindingMoveForward) {
		t.Fatal("expected touch up action to register as move-forward held")
	}
	if !g.bindingHeld(bindingStrafeLeft) {
		t.Fatal("expected touch strafe-left action to register as strafe-left held")
	}
	if !g.bindingHeld(bindingTurnRight) {
		t.Fatal("expected touch turn-right action to register as turn-right held")
	}
	if !g.bindingHeld(bindingFire) {
		t.Fatal("expected touch fire action to register as fire held")
	}
	if g.bindingHeld(bindingUse) {
		t.Fatal("did not expect unrelated touch action to register as use held")
	}
}

func TestGameplayPadActionsLeftPadUsesForwardBackAndStrafe(t *testing.T) {
	left := touchPad{cx: 100, cy: 100, radius: 50}
	right := touchPad{cx: 300, cy: 100, radius: 50}

	mask := gameplayPadActions(left, right, 70, 60)
	if mask&touchActionStrafeLeft == 0 {
		t.Fatal("expected left pad to trigger strafe left")
	}
	if mask&touchActionUp == 0 {
		t.Fatal("expected left pad to trigger move forward")
	}
	if mask&(touchActionTurnLeft|touchActionFire|touchActionUseEnter) != 0 {
		t.Fatal("did not expect left pad to trigger right-pad actions")
	}
}

func TestGameplayPadActionsRightPadUsesTurnFireAndUse(t *testing.T) {
	left := touchPad{cx: 100, cy: 100, radius: 50}
	right := touchPad{cx: 300, cy: 100, radius: 50}

	mask := gameplayPadActions(left, right, 335, 135)
	if mask&touchActionTurnRight == 0 {
		t.Fatal("expected right pad to trigger turn right")
	}
	if mask&touchActionUseEnter == 0 {
		t.Fatal("expected right pad to trigger use on down axis")
	}
	if mask&(touchActionStrafeRight|touchActionUp) != 0 {
		t.Fatal("did not expect right pad to trigger left-pad movement actions")
	}
}

func TestGameplayPadActionsUsesGraceAreaOutsideVisibleCircle(t *testing.T) {
	left := touchPad{cx: 100, cy: 100, radius: 50}
	right := touchPad{cx: 300, cy: 100, radius: 50}

	mask := gameplayPadActions(left, right, 39, 100)
	if mask&touchActionStrafeLeft == 0 {
		t.Fatal("expected grace area touch to still trigger strafe left")
	}
}

func TestBindingJustPressedUsesTouchActions(t *testing.T) {
	g := &game{
		input: gameInputSnapshot{
			touchJustPressedActions: touchActionUseEnter,
		},
	}
	if !g.bindingJustPressed(bindingUse) {
		t.Fatal("expected touch use action to register as just pressed")
	}
	if !g.enterJustPressed() {
		t.Fatal("expected touch use action to count as enter just pressed")
	}
}

func TestTouchJustPressedIsLatchedUntilConsumed(t *testing.T) {
	sg := &sessionGame{
		touch: touchControllerState{
			latchedJustPressed: touchActionUseEnter | touchActionBack,
		},
	}
	if !sg.touchJustPressed(touchActionUseEnter) {
		t.Fatal("expected latched touch use action to be consumed once")
	}
	if sg.touchJustPressed(touchActionUseEnter) {
		t.Fatal("did not expect consumed touch use action to trigger twice")
	}
	if !sg.touchJustPressed(touchActionBack) {
		t.Fatal("expected other latched touch actions to remain available")
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
