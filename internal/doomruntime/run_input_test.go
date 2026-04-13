package doomruntime

import (
	"testing"

	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/sessionflow"
	"github.com/hajimehoshi/ebiten/v2"
)

type demoActiveRuntime struct {
	layoutCountRuntime
	demoActive bool
}

func (r *demoActiveRuntime) sessionSignals() gameplay.SessionSignals {
	return gameplay.SessionSignals{DemoActive: r.demoActive}
}

func TestShouldSampleRuntimeInputDuringFrontendAttractDemo(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true},
		rt:       &demoActiveRuntime{demoActive: true},
	}
	if !sg.shouldSampleRuntimeInput() {
		t.Fatal("shouldSampleRuntimeInput() = false, want true during frontend attract demo")
	}
}

func TestShouldNotSampleRuntimeInputDuringFrontendMenuWithoutDemo(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true},
		rt:       &demoActiveRuntime{demoActive: false},
	}
	if sg.shouldSampleRuntimeInput() {
		t.Fatal("shouldSampleRuntimeInput() = true, want false for frontend without active demo")
	}
}

func TestShouldNotSampleRuntimeInputWhenAttractMenuIsOpen(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true, MenuActive: true},
		rt:       &demoActiveRuntime{demoActive: true},
	}
	if sg.shouldSampleRuntimeInput() {
		t.Fatal("shouldSampleRuntimeInput() = true, want false when attract menu is already open")
	}
}

func TestOpenFrontendMenuFromSignalUsesAttractMenuForDemo(t *testing.T) {
	sg := &sessionGame{}
	sg.openFrontendMenuFromSignal(gameplay.SessionSignals{DemoActive: true})
	if !sg.frontend.Active || !sg.frontend.MenuActive {
		t.Fatal("expected frontend attract menu to open")
	}
	if !sg.frontend.Attract {
		t.Fatal("expected attract mode for demo-triggered frontend menu")
	}
	if sg.frontend.InGame {
		t.Fatal("did not expect in-game pause menu for demo-triggered frontend menu")
	}
}

func TestOpenFrontendMenuFromSignalUsesPauseMenuForGameplay(t *testing.T) {
	sg := &sessionGame{}
	sg.openFrontendMenuFromSignal(gameplay.SessionSignals{DemoActive: false})
	if !sg.frontend.Active || !sg.frontend.MenuActive {
		t.Fatal("expected frontend menu to open")
	}
	if sg.frontend.Attract {
		t.Fatal("did not expect attract mode for gameplay frontend menu")
	}
	if !sg.frontend.InGame {
		t.Fatal("expected in-game pause menu for gameplay frontend menu")
	}
}

func TestOpenFrontendMenuFromSignalUsesFirstSelectableItemForWatch(t *testing.T) {
	sg := &sessionGame{
		opts: Options{LiveTicSource: &testLiveTicSource{}},
	}
	sg.openFrontendMenuFromSignal(gameplay.SessionSignals{DemoActive: false})
	if got := sg.frontend.ItemOn; got != frontendWatchMenuSelectableRows[0] {
		t.Fatalf("ItemOn=%d want=%d", got, frontendWatchMenuSelectableRows[0])
	}
}

func TestOpenFrontendSoundMenuFromSignalStartsAtSoundSubmenu(t *testing.T) {
	sg := &sessionGame{}
	sg.openFrontendSoundMenuFromSignal(gameplay.SessionSignals{DemoActive: false})
	if !sg.frontend.Active || !sg.frontend.MenuActive {
		t.Fatal("expected frontend sound menu to open")
	}
	if sg.frontend.Mode != frontendModeSound {
		t.Fatalf("mode=%d want=%d", sg.frontend.Mode, frontendModeSound)
	}
	if sg.frontend.SoundOn != frontendSoundMenuRowSFX {
		t.Fatalf("soundOn=%d want=%d", sg.frontend.SoundOn, frontendSoundMenuRowSFX)
	}
}

func TestOpenFrontendSaveLoadMenuFromSignalStartsAtSlotMenu(t *testing.T) {
	sg := &sessionGame{}
	sg.openFrontendSaveLoadMenuFromSignal(gameplay.SessionSignals{DemoActive: false}, true)
	if !sg.frontend.Active || !sg.frontend.MenuActive {
		t.Fatal("expected frontend save/load menu to open")
	}
	if sg.frontend.Mode != frontendModeSaveLoad {
		t.Fatalf("mode=%d want=%d", sg.frontend.Mode, frontendModeSaveLoad)
	}
	if !sg.frontend.SaveLoadSaving {
		t.Fatal("expected save mode")
	}
	if sg.frontend.SaveLoadOn != 0 {
		t.Fatalf("saveLoadOn=%d want 0", sg.frontend.SaveLoadOn)
	}
}

func TestWatchModeDisablesSaveLoadHotkeys(t *testing.T) {
	g := &game{
		opts: Options{LiveTicSource: &testLiveTicSource{}},
		input: gameInputSnapshot{
			justPressedKeys: map[ebiten.Key]struct{}{
				ebiten.KeyF2: {},
				ebiten.KeyF3: {},
			},
		},
	}

	g.edgeInputPass = true
	g.updateParityControls()

	if g.saveGameRequested {
		t.Fatal("saveGameRequested should stay false in watch mode")
	}
	if g.loadGameRequested {
		t.Fatal("loadGameRequested should stay false in watch mode")
	}
}

func TestFrontendShouldUpdateRuntimeForWatch(t *testing.T) {
	if !frontendShouldUpdateRuntime(gameplay.SessionSignals{WatchActive: true}) {
		t.Fatal("frontendShouldUpdateRuntime() = false, want true for watch mode")
	}
}

func TestFrontendShouldUpdateRuntimeForDemo(t *testing.T) {
	if !frontendShouldUpdateRuntime(gameplay.SessionSignals{DemoActive: true}) {
		t.Fatal("frontendShouldUpdateRuntime() = false, want true for demo mode")
	}
}

func TestFrontendShouldNotUpdateRuntimeForPlainGameplayMenu(t *testing.T) {
	if frontendShouldUpdateRuntime(gameplay.SessionSignals{}) {
		t.Fatal("frontendShouldUpdateRuntime() = true, want false for plain gameplay menu")
	}
}

func TestUpdateFinaleCompletionReturnsToFrontendInsteadOfTerminating(t *testing.T) {
	sg := &sessionGame{
		bootMap: &mapdata.Map{Name: "E1M1"},
		finale: sessionFinale{
			Active:  true,
			Stage:   sessionflow.FinaleStagePicture,
			WaitTic: 0,
		},
	}

	if err := sg.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if sg.finale.Active {
		t.Fatal("expected finale to complete")
	}
	if !sg.frontend.Active {
		t.Fatal("expected frontend to become active after finale completion")
	}
	if sg.frontend.InGame {
		t.Fatal("expected title frontend, not in-game frontend, after finale completion")
	}
}

func TestSkipInputTriggeredConsumesAnyKeyboardKey(t *testing.T) {
	sg := &sessionGame{
		input: sessionInputSnapshot{
			justPressedKeys: map[ebiten.Key]int{
				ebiten.KeyA: 1,
			},
		},
	}
	if !sg.skipInputTriggered() {
		t.Fatal("skipInputTriggered() = false, want true for arbitrary keyboard key")
	}
	if len(sg.input.justPressedKeys) != 0 {
		t.Fatal("expected arbitrary keyboard key skip to consume pending keypresses")
	}
}

func TestSkipInputTriggeredUsesTouchEnter(t *testing.T) {
	sg := &sessionGame{
		touch: touchControllerState{
			latchedJustPressed: touchActionUseEnter,
		},
	}
	if !sg.skipInputTriggered() {
		t.Fatal("skipInputTriggered() = false, want true for touch enter")
	}
}

func TestShouldDrawTouchControlsOnlyAfterTouchSeen(t *testing.T) {
	sg := &sessionGame{}
	if sg.shouldDrawTouchControls() {
		t.Fatal("shouldDrawTouchControls() = true, want false before touch is seen")
	}
	sg.touch.seen = true
	if !sg.shouldDrawTouchControls() {
		t.Fatal("shouldDrawTouchControls() = false, want true after touch is seen")
	}
}

func TestShouldDrawTouchControlsHidesFrontendAttractOverlayWhenMenuClosed(t *testing.T) {
	sg := &sessionGame{
		touch:    touchControllerState{seen: true},
		frontend: frontendState{Active: true, MenuActive: false},
	}
	if sg.shouldDrawTouchControls() {
		t.Fatal("shouldDrawTouchControls() = true, want false during attract when menu is closed")
	}

	sg.frontend.MenuActive = true
	if !sg.shouldDrawTouchControls() {
		t.Fatal("shouldDrawTouchControls() = false, want true when frontend menu is open")
	}
}

func TestShouldDrawTouchControlsShowsFrontendSubmenusWhenMenuActive(t *testing.T) {
	sg := &sessionGame{
		touch:    touchControllerState{seen: true},
		frontend: frontendState{Active: true, Mode: frontendModeMusicPlayer, MenuActive: true},
	}
	if !sg.shouldDrawTouchControls() {
		t.Fatal("shouldDrawTouchControls() = false, want true for active frontend submenu")
	}
}

func TestTouchButtonsUseWholeScreenEnterForClosedFrontend(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true, MenuActive: false},
	}
	buttons := sg.touchButtons(320, 200)
	if len(buttons) != 1 {
		t.Fatalf("buttons len=%d want 1", len(buttons))
	}
	if buttons[0].action != touchActionUseEnter {
		t.Fatalf("button action=%v want touchActionUseEnter", buttons[0].action)
	}
	if buttons[0].x != 0 || buttons[0].y != 0 || buttons[0].w != 320 || buttons[0].h != 200 {
		t.Fatalf("button rect=(%.0f,%.0f,%.0f,%.0f) want (0,0,320,200)", buttons[0].x, buttons[0].y, buttons[0].w, buttons[0].h)
	}
}

func TestFrontendTouchButtonsAnchorToBottomEdge(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true, MenuActive: true},
	}
	buttons := sg.touchButtons(320, 200)
	if len(buttons) != 6 {
		t.Fatalf("buttons len=%d want 6", len(buttons))
	}
	for _, button := range buttons {
		switch button.label {
		case "LEFT", "DOWN", "RIGHT", "BACK":
			if button.y+button.h >= 192 {
				continue
			}
			t.Fatalf("%s button bottom=%0.1f want near screen bottom", button.label, button.y+button.h)
		}
	}
}

func TestGameplayTouchButtonsPlaceEscOnTopRight(t *testing.T) {
	sg := &sessionGame{g: &game{}}
	buttons := sg.gameplayTouchButtons(320, 200)
	if len(buttons) != 1 {
		t.Fatalf("buttons len=%d want 1", len(buttons))
	}
	if buttons[0].action != touchActionBack {
		t.Fatalf("button action=%v want touchActionBack", buttons[0].action)
	}
	if buttons[0].label != "ESC" {
		t.Fatalf("button label=%q want ESC", buttons[0].label)
	}
	if buttons[0].x+buttons[0].w < 300 || buttons[0].y > 30 {
		t.Fatalf("button rect=(%.1f,%.1f,%.1f,%.1f) want top-right placement", buttons[0].x, buttons[0].y, buttons[0].w, buttons[0].h)
	}
}

func TestHandleGameplayTouchShortcutsOpensFrontendMenu(t *testing.T) {
	sg := &sessionGame{
		g: &game{},
		touch: touchControllerState{
			latchedJustPressed: touchActionBack,
		},
	}

	sg.handleGameplayTouchShortcuts()

	if !sg.g.frontendMenuRequested {
		t.Fatal("expected gameplay touch ESC to request frontend menu")
	}
	if !sg.touch.suppressUntilClear {
		t.Fatal("expected gameplay touch ESC to suppress touch until release")
	}
}

func TestSuppressTouchUntilReleaseClearsLatchedAndHeldState(t *testing.T) {
	sg := &sessionGame{
		g: &game{},
		touch: touchControllerState{
			held:               touchActionBack,
			justPressed:        touchActionBack,
			latchedJustPressed: touchActionBack | touchActionUseEnter,
		},
	}

	sg.suppressTouchUntilRelease()

	if !sg.touch.suppressUntilClear {
		t.Fatal("expected touch suppression to be enabled")
	}
	if sg.touch.held != 0 || sg.touch.justPressed != 0 || sg.touch.latchedJustPressed != 0 {
		t.Fatalf("touch state=(held=%v just=%v latched=%v) want all cleared", sg.touch.held, sg.touch.justPressed, sg.touch.latchedJustPressed)
	}
	if sg.g.input.touchHeldActions != 0 || sg.g.input.touchJustPressedActions != 0 {
		t.Fatalf("game touch state=(held=%v just=%v) want cleared", sg.g.input.touchHeldActions, sg.g.input.touchJustPressedActions)
	}
}
