package doomruntime

import (
	"image/color"

	"gddoom/internal/platformcfg"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type touchActionMask uint32

const (
	touchActionUp touchActionMask = 1 << iota
	touchActionDown
	touchActionLeft
	touchActionRight
	touchActionFire
	touchActionUseEnter
	touchActionBack
)

type touchControllerState struct {
	seen               bool
	held               touchActionMask
	justPressed        touchActionMask
	latchedJustPressed touchActionMask
	screenW            int
	screenH            int
}

type touchControlButton struct {
	action touchActionMask
	label  string
	x      float64
	y      float64
	w      float64
	h      float64
}

func touchActionForBinding(action bindingAction) touchActionMask {
	switch action {
	case bindingMoveForward:
		return touchActionUp
	case bindingMoveBackward:
		return touchActionDown
	case bindingTurnLeft:
		return touchActionLeft
	case bindingTurnRight:
		return touchActionRight
	case bindingFire:
		return touchActionFire
	case bindingUse:
		return touchActionUseEnter
	default:
		return 0
	}
}

func (sg *sessionGame) sampleTouchController() {
	if sg == nil {
		return
	}
	sw, sh := sg.touch.screenW, sg.touch.screenH
	if sw <= 0 || sh <= 0 {
		sw, sh = ebiten.WindowSize()
	}
	sw = max(sw, 1)
	sh = max(sh, 1)

	justPressedIDs := inpututil.AppendJustPressedTouchIDs(nil)
	ids := append([]ebiten.TouchID(nil), justPressedIDs...)
	for _, id := range ebiten.AppendTouchIDs(nil) {
		found := false
		for _, existing := range ids {
			if existing == id {
				found = true
				break
			}
		}
		if !found {
			ids = append(ids, id)
		}
	}
	if len(justPressedIDs) > 0 || len(ids) > 0 {
		sg.touch.seen = true
	}
	buttons := sg.touchButtons(sw, sh)
	held := touchActionMask(0)
	for _, id := range ids {
		x, y := ebiten.TouchPosition(id)
		for _, button := range buttons {
			if touchButtonContains(button, float64(x), float64(y)) {
				held |= button.action
				break
			}
		}
	}
	sg.touch.justPressed = held &^ sg.touch.held
	sg.touch.latchedJustPressed |= sg.touch.justPressed
	sg.touch.held = held
	if sg.g != nil {
		sg.g.input.touchHeldActions = held
		sg.g.input.touchJustPressedActions = sg.touch.latchedJustPressed
		sg.g.input.touchSeen = sg.touch.seen
	}
}

func (sg *sessionGame) touchJustPressed(action touchActionMask) bool {
	if sg == nil || sg.touch.latchedJustPressed&action == 0 {
		return false
	}
	sg.touch.latchedJustPressed &^= action
	return true
}

func (sg *sessionGame) shouldDrawTouchControls() bool {
	return sg != nil && (sg.touch.seen || platformcfg.IsWASMBuild())
}

func (sg *sessionGame) touchButtons(sw, sh int) []touchControlButton {
	size := float64(minInt(sw, sh)) * 0.14
	if size < 56 {
		size = 56
	}
	if size > 110 {
		size = 110
	}
	margin := size * 0.35
	gap := size * 0.18
	fireW := size * 1.15
	fireH := size * 0.9

	if sg == nil || sg.finale.Active || sg.intermission.state.Active || sg.quitPrompt.Active || sg.transitionActive() {
		return []touchControlButton{
			{
				action: touchActionUseEnter,
				label:  "ENTER",
				x:      float64(sw) - margin - fireW,
				y:      float64(sh) - margin - fireH,
				w:      fireW,
				h:      fireH,
			},
		}
	}
	if sg.frontend.Active {
		leftX := margin
		baseY := float64(sh) - margin - size
		centerX := leftX + size + gap
		rightX := float64(sw) - margin - fireW
		backY := float64(sh) - margin - fireH
		enterY := backY - fireH - gap
		return []touchControlButton{
			{action: touchActionUp, label: "UP", x: centerX, y: baseY - size - gap, w: size, h: size},
			{action: touchActionLeft, label: "LEFT", x: leftX, y: baseY, w: size, h: size},
			{action: touchActionDown, label: "DOWN", x: centerX, y: baseY, w: size, h: size},
			{action: touchActionRight, label: "RIGHT", x: centerX + size + gap, y: baseY, w: size, h: size},
			{action: touchActionUseEnter, label: "ENTER", x: rightX, y: enterY, w: fireW, h: fireH},
			{action: touchActionBack, label: "BACK", x: rightX, y: backY, w: fireW, h: fireH},
		}
	}

	leftX := margin
	baseY := float64(sh) - margin - size
	centerX := leftX + size + gap

	rightX := float64(sw) - margin - fireW
	useY := float64(sh) - margin - fireH
	fireY := useY - fireH - gap

	return []touchControlButton{
		{action: touchActionUp, label: "UP", x: centerX, y: baseY - size - gap, w: size, h: size},
		{action: touchActionLeft, label: "LEFT", x: leftX, y: baseY, w: size, h: size},
		{action: touchActionDown, label: "DOWN", x: centerX, y: baseY, w: size, h: size},
		{action: touchActionRight, label: "RIGHT", x: centerX + size + gap, y: baseY, w: size, h: size},
		{action: touchActionFire, label: "FIRE", x: rightX, y: fireY, w: fireW, h: fireH},
		{action: touchActionUseEnter, label: "USE/ENTER", x: rightX, y: useY, w: fireW, h: fireH},
	}
}

func touchButtonContains(button touchControlButton, x, y float64) bool {
	return x >= button.x && x < button.x+button.w && y >= button.y && y < button.y+button.h
}

func (sg *sessionGame) drawTouchControls(screen *ebiten.Image) {
	if screen == nil || !sg.shouldDrawTouchControls() {
		return
	}
	buttons := sg.touchButtons(max(screen.Bounds().Dx(), 1), max(screen.Bounds().Dy(), 1))
	for _, button := range buttons {
		fill := color.RGBA{R: 20, G: 20, B: 20, A: 112}
		border := color.RGBA{R: 96, G: 96, B: 96, A: 132}
		if sg.touch.held&button.action != 0 {
			fill = color.RGBA{R: 34, G: 34, B: 34, A: 148}
			border = color.RGBA{R: 140, G: 140, B: 140, A: 172}
		}
		ebitenutil.DrawRect(screen, button.x, button.y, button.w, button.h, fill)
		ebitenutil.DrawRect(screen, button.x, button.y, button.w, 2, border)
		ebitenutil.DrawRect(screen, button.x, button.y+button.h-2, button.w, 2, border)
		ebitenutil.DrawRect(screen, button.x, button.y, 2, button.h, border)
		ebitenutil.DrawRect(screen, button.x+button.w-2, button.y, 2, button.h, border)

		labelX := int(button.x + button.w*0.5)
		labelY := int(button.y + button.h*0.5 - 4)
		if sg.g != nil {
			sg.drawIntermissionText(screen, button.label, labelX, labelY, 1, 0, 0, true)
		} else {
			ebitenutil.DebugPrintAt(screen, button.label, int(button.x+8), int(button.y+button.h*0.5-4))
		}
	}
	drawTouchSeenStatus(screen, sg.touch.seen)
	drawActiveTouchPoints(screen)
}

func drawTouchSeenStatus(screen *ebiten.Image, seen bool) {
	if screen == nil {
		return
	}
	label := "TOUCH: none"
	fill := color.RGBA{R: 24, G: 24, B: 24, A: 160}
	border := color.RGBA{R: 88, G: 88, B: 88, A: 180}
	if seen {
		label = "TOUCH: seen"
		fill = color.RGBA{R: 24, G: 40, B: 24, A: 170}
		border = color.RGBA{R: 96, G: 160, B: 96, A: 190}
	}
	x, y, w, h := 12.0, 12.0, 120.0, 24.0
	ebitenutil.DrawRect(screen, x, y, w, h, fill)
	ebitenutil.DrawRect(screen, x, y, w, 2, border)
	ebitenutil.DrawRect(screen, x, y+h-2, w, 2, border)
	ebitenutil.DrawRect(screen, x, y, 2, h, border)
	ebitenutil.DrawRect(screen, x+w-2, y, 2, h, border)
	ebitenutil.DebugPrintAt(screen, label, int(x+8), int(y+6))
}

func drawActiveTouchPoints(screen *ebiten.Image) {
	if screen == nil {
		return
	}
	for _, id := range ebiten.AppendTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		vector.DrawFilledCircle(screen, float32(x), float32(y), 12, color.RGBA{R: 255, A: 220}, true)
		vector.DrawFilledCircle(screen, float32(x), float32(y), 5, color.RGBA{R: 255, G: 255, B: 255, A: 220}, true)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
