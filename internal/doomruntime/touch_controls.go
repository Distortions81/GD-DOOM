package doomruntime

import (
	"image"
	"image/color"
	"math"

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

type touchLayoutTransform struct {
	screenW int
	screenH int
	localW  int
	localH  int
	rw      int
	rh      int
	ox      int
	oy      int
}

func newTouchLayoutTransform(screenW, screenH, localW, localH int) touchLayoutTransform {
	screenW = max(screenW, 1)
	screenH = max(screenH, 1)
	localW = max(localW, 1)
	localH = max(localH, 1)
	rw, rh, ox, oy := fitRect(screenW, screenH, localW, localH)
	return touchLayoutTransform{
		screenW: screenW,
		screenH: screenH,
		localW:  localW,
		localH:  localH,
		rw:      rw,
		rh:      rh,
		ox:      ox,
		oy:      oy,
	}
}

func (t touchLayoutTransform) identity() bool {
	return t.screenW == t.localW && t.screenH == t.localH
}

func (t touchLayoutTransform) localToScreenRect(r image.Rectangle) image.Rectangle {
	if t.identity() {
		return r
	}
	sx := float64(t.rw) / float64(t.localW)
	sy := float64(t.rh) / float64(t.localH)
	return image.Rect(
		t.ox+int(math.Round(float64(r.Min.X)*sx)),
		t.oy+int(math.Round(float64(r.Min.Y)*sy)),
		t.ox+int(math.Round(float64(r.Max.X)*sx)),
		t.oy+int(math.Round(float64(r.Max.Y)*sy)),
	)
}

func (t touchLayoutTransform) screenToLocal(x, y int) (float64, float64, bool) {
	if t.identity() {
		return float64(x), float64(y), true
	}
	if x < t.ox || y < t.oy || x >= t.ox+t.rw || y >= t.oy+t.rh {
		return 0, 0, false
	}
	localX := (float64(x-t.ox) * float64(t.localW)) / float64(t.rw)
	localY := (float64(y-t.oy) * float64(t.localH)) / float64(t.rh)
	return localX, localY, true
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
	localW, localH := sg.touchControlLayoutSize(sw, sh)
	transform := newTouchLayoutTransform(sw, sh, localW, localH)

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
	buttons := sg.touchButtons(localW, localH)
	held := touchActionMask(0)
	for _, id := range ids {
		x, y := ebiten.TouchPosition(id)
		for _, button := range buttons {
			screenRect := transform.localToScreenRect(image.Rect(
				int(math.Floor(button.x)),
				int(math.Floor(button.y)),
				int(math.Ceil(button.x+button.w)),
				int(math.Ceil(button.y+button.h)),
			))
			if pointInRect(screenRect, x, y) {
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

func (sg *sessionGame) touchControlLayoutSize(sw, sh int) (int, int) {
	if sg == nil {
		return sw, sh
	}
	if sg.frontend.Active || sg.intermission.state.Active || sg.finale.Active || sg.quitPrompt.Active || sg.transitionActive() {
		return 320, 200
	}
	if sg.opts.SourcePortMode || sg.g == nil {
		return sw, sh
	}
	return max(sg.g.viewW, 1), max(sg.g.viewH, 1)
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

func pointInRect(r image.Rectangle, x, y int) bool {
	return x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y
}

func (sg *sessionGame) drawTouchControls(screen *ebiten.Image) {
	if screen == nil || !sg.shouldDrawTouchControls() {
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	localW, localH := sg.touchControlLayoutSize(sw, sh)
	transform := newTouchLayoutTransform(sw, sh, localW, localH)
	buttons := sg.touchButtons(localW, localH)
	for _, button := range buttons {
		drawButton := transform.localToScreenRect(image.Rect(
			int(math.Floor(button.x)),
			int(math.Floor(button.y)),
			int(math.Ceil(button.x+button.w)),
			int(math.Ceil(button.y+button.h)),
		))
		fill := color.RGBA{R: 20, G: 20, B: 20, A: 112}
		border := color.RGBA{R: 96, G: 96, B: 96, A: 132}
		if sg.touch.held&button.action != 0 {
			fill = color.RGBA{R: 34, G: 34, B: 34, A: 148}
			border = color.RGBA{R: 140, G: 140, B: 140, A: 172}
		}
		ebitenutil.DrawRect(screen, float64(drawButton.Min.X), float64(drawButton.Min.Y), float64(drawButton.Dx()), float64(drawButton.Dy()), fill)
		ebitenutil.DrawRect(screen, float64(drawButton.Min.X), float64(drawButton.Min.Y), float64(drawButton.Dx()), 2, border)
		ebitenutil.DrawRect(screen, float64(drawButton.Min.X), float64(drawButton.Max.Y-2), float64(drawButton.Dx()), 2, border)
		ebitenutil.DrawRect(screen, float64(drawButton.Min.X), float64(drawButton.Min.Y), 2, float64(drawButton.Dy()), border)
		ebitenutil.DrawRect(screen, float64(drawButton.Max.X-2), float64(drawButton.Min.Y), 2, float64(drawButton.Dy()), border)

		labelX := int(float64(drawButton.Min.X) + float64(drawButton.Dx())*0.5)
		labelY := int(float64(drawButton.Min.Y) + float64(drawButton.Dy())*0.5 - 4)
		if sg.g != nil {
			sg.drawIntermissionText(screen, button.label, labelX, labelY, 1, 0, 0, true)
		} else {
			ebitenutil.DebugPrintAt(screen, button.label, int(float64(drawButton.Min.X)+8), int(float64(drawButton.Min.Y)+float64(drawButton.Dy())*0.5-4))
		}
	}
	drawActiveTouchPoints(screen)
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
