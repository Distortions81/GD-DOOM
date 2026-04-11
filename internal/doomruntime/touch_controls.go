package doomruntime

import (
	"image"
	"image/color"
	"math"

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
	touchActionStrafeLeft
	touchActionStrafeRight
	touchActionTurnLeft
	touchActionTurnRight
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
	// Analog joystick axes in [-1, 1]; left pad = move, right pad = look/action
	leftX  float64 // strafe
	leftY  float64 // forward/back
	rightX float64 // turn
	rightY float64 // fire (negative) / use (positive)
}

type touchControlButton struct {
	action touchActionMask
	label  string
	x      float64
	y      float64
	w      float64
	h      float64
}

type touchPad struct {
	cx     float64
	cy     float64
	radius float64
}

const touchPadGraceRatio = 0.22

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
		return touchActionTurnLeft
	case bindingTurnRight:
		return touchActionTurnRight
	case bindingStrafeLeft:
		return touchActionStrafeLeft
	case bindingStrafeRight:
		return touchActionStrafeRight
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
	held := touchActionMask(0)
	var leftX, leftY, rightX, rightY float64
	if sg.gameplayTouchUsesPads() {
		leftPad, rightPad := sg.gameplayTouchPads(localW, localH)
		leftScreen := touchPadToScreen(transform, leftPad)
		rightScreen := touchPadToScreen(transform, rightPad)
		for _, id := range ids {
			x, y := ebiten.TouchPosition(id)
			mask, lx, ly, rx, ry := gameplayPadActionsAnalog(leftScreen, rightScreen, float64(x), float64(y))
			held |= mask
			leftX += lx
			leftY += ly
			rightX += rx
			rightY += ry
		}
		leftX = clampFloat(leftX, -1, 1)
		leftY = clampFloat(leftY, -1, 1)
		rightX = clampFloat(rightX, -1, 1)
		rightY = clampFloat(rightY, -1, 1)
	} else {
		buttons := sg.touchButtons(localW, localH)
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
	}
	sg.touch.justPressed = held &^ sg.touch.held
	sg.touch.latchedJustPressed |= sg.touch.justPressed
	sg.touch.held = held
	sg.touch.leftX = leftX
	sg.touch.leftY = leftY
	sg.touch.rightX = rightX
	sg.touch.rightY = rightY
	if sg.g != nil {
		sg.g.input.touchHeldActions = held
		sg.g.input.touchJustPressedActions = sg.touch.latchedJustPressed
		sg.g.input.touchSeen = sg.touch.seen
		sg.g.input.touchLeftX = leftX
		sg.g.input.touchLeftY = leftY
		sg.g.input.touchRightX = rightX
		sg.g.input.touchRightY = rightY
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
	return sg != nil && sg.touch.seen
}

func (sg *sessionGame) gameplayTouchUsesPads() bool {
	return sg != nil &&
		sg.g != nil &&
		!sg.frontend.Active &&
		!sg.intermission.state.Active &&
		!sg.finale.Active &&
		!sg.quitPrompt.Active &&
		!sg.transitionActive()
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

func (sg *sessionGame) gameplayTouchPads(sw, sh int) (touchPad, touchPad) {
	radius := float64(minInt(sw, sh)) * 0.15
	if radius < 54 {
		radius = 54
	}
	if radius > 96 {
		radius = 96
	}
	margin := radius * 0.55
	bottom := float64(sh) - margin - radius
	left := touchPad{
		cx:     margin + radius,
		cy:     bottom,
		radius: radius,
	}
	right := touchPad{
		cx:     float64(sw) - margin - radius,
		cy:     bottom,
		radius: radius,
	}
	return left, right
}

func touchPadToScreen(transform touchLayoutTransform, pad touchPad) touchPad {
	if transform.identity() {
		return pad
	}
	sx := float64(transform.rw) / float64(transform.localW)
	sy := float64(transform.rh) / float64(transform.localH)
	return touchPad{
		cx:     float64(transform.ox) + pad.cx*sx,
		cy:     float64(transform.oy) + pad.cy*sy,
		radius: pad.radius * minFloat(sx, sy),
	}
}

// gameplayPadActionsAnalog returns the digital action mask plus analog axis values
// for both pads. Axes are normalized to [-1, 1] with deadzone applied.
func gameplayPadActionsAnalog(leftPad, rightPad touchPad, x, y float64) (touchActionMask, float64, float64, float64, float64) {
	if nx, ny, ok := analogPadSample(leftPad, x, y); ok {
		mask := touchActionMask(0)
		if nx < -0.1 {
			mask |= touchActionStrafeLeft
		} else if nx > 0.1 {
			mask |= touchActionStrafeRight
		}
		if ny < -0.1 {
			mask |= touchActionUp
		} else if ny > 0.1 {
			mask |= touchActionDown
		}
		return mask, nx, ny, 0, 0
	}
	if nx, ny, ok := analogPadSample(rightPad, x, y); ok {
		mask := touchActionMask(0)
		if nx < -0.1 {
			mask |= touchActionTurnLeft
		} else if nx > 0.1 {
			mask |= touchActionTurnRight
		}
		if ny < -0.65 {
			mask |= touchActionFire
		} else if ny > 0.65 {
			mask |= touchActionUseEnter
		}
		return mask, 0, 0, nx, ny
	}
	return 0, 0, 0, 0, 0
}

func gameplayPadActions(leftPad, rightPad touchPad, x, y float64) touchActionMask {
	if mask, ok := classifyPadTouch(leftPad, x, y, touchActionStrafeLeft, touchActionStrafeRight, touchActionUp, touchActionDown); ok {
		return mask
	}
	mask, _ := classifyPadTouch(rightPad, x, y, touchActionTurnLeft, touchActionTurnRight, touchActionFire, touchActionUseEnter)
	return mask
}

// analogPadSample returns normalized [-1,1] axes for a pad touch, with deadzone.
func analogPadSample(pad touchPad, x, y float64) (float64, float64, bool) {
	dx := x - pad.cx
	dy := y - pad.cy
	radius := maxFloat(pad.radius, 1)
	hitRadius := radius * (1 + touchPadGraceRatio)
	if dx*dx+dy*dy > hitRadius*hitRadius {
		return 0, 0, false
	}

	const deadzoneFrac = 0.18

	dist := math.Sqrt(dx*dx + dy*dy)
	var nx, ny float64
	if dist > 0 {
		scale := minFloat(dist, radius) / radius
		nx = (dx / dist) * scale
		ny = (dy / dist) * scale
	}

	applyDeadzone := func(v float64) float64 {
		abs := math.Abs(v)
		if abs < deadzoneFrac {
			return 0
		}
		scaled := (abs - deadzoneFrac) / (1 - deadzoneFrac)
		if v < 0 {
			return -scaled
		}
		return scaled
	}
	return applyDeadzone(nx), applyDeadzone(ny), true
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func classifyPadTouch(pad touchPad, x, y float64, leftAction, rightAction, upAction, downAction touchActionMask) (touchActionMask, bool) {
	dx := x - pad.cx
	dy := y - pad.cy
	radius := maxFloat(pad.radius, 1)
	hitRadius := radius * (1 + touchPadGraceRatio)
	distSq := dx*dx + dy*dy
	if distSq > hitRadius*hitRadius {
		return 0, false
	}
	deadzone := radius * 0.24
	mask := touchActionMask(0)
	if dx <= -deadzone {
		mask |= leftAction
	} else if dx >= deadzone {
		mask |= rightAction
	}
	if dy <= -deadzone {
		mask |= upAction
	} else if dy >= deadzone {
		mask |= downAction
	}
	return mask, true
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
	if sg.gameplayTouchUsesPads() {
		sg.drawGameplayTouchPads(screen, transform, localW, localH)
		drawActiveTouchPoints(screen)
		return
	}
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

func (sg *sessionGame) drawGameplayTouchPads(screen *ebiten.Image, transform touchLayoutTransform, localW, localH int) {
	leftPad, rightPad := sg.gameplayTouchPads(localW, localH)
	leftScreen := touchPadToScreen(transform, leftPad)
	rightScreen := touchPadToScreen(transform, rightPad)
	sg.drawMovementTouchPad(screen, leftScreen, sg.touch.held&(touchActionUp|touchActionDown|touchActionStrafeLeft|touchActionStrafeRight), sg.touch.leftX, sg.touch.leftY)
	sg.drawActionTouchPad(screen, rightScreen, sg.touch.held&(touchActionFire|touchActionUseEnter|touchActionTurnLeft|touchActionTurnRight), sg.touch.rightX, sg.touch.rightY)
}

func (sg *sessionGame) drawMovementTouchPad(screen *ebiten.Image, pad touchPad, held touchActionMask, axisX, axisY float64) {
	fill := color.RGBA{R: 20, G: 20, B: 20, A: 92}
	border := color.RGBA{R: 96, G: 96, B: 96, A: 132}
	if held != 0 {
		fill = color.RGBA{R: 34, G: 34, B: 34, A: 132}
		border = color.RGBA{R: 140, G: 140, B: 140, A: 172}
	}
	vector.DrawFilledCircle(screen, float32(pad.cx), float32(pad.cy), float32(pad.radius), fill, true)
	vector.StrokeCircle(screen, float32(pad.cx), float32(pad.cy), float32(pad.radius), 2, border, true)
	ebitenutil.DrawRect(screen, pad.cx-1, pad.cy-pad.radius, 2, pad.radius*2, border)
	ebitenutil.DrawRect(screen, pad.cx-pad.radius, pad.cy-1, pad.radius*2, 2, border)
	sg.drawPadChevron(screen, pad.cx, pad.cy-pad.radius*0.58, 0, border, held&touchActionUp != 0)
	sg.drawPadChevron(screen, pad.cx, pad.cy+pad.radius*0.58, math.Pi, border, held&touchActionDown != 0)
	sg.drawPadChevron(screen, pad.cx-pad.radius*0.58, pad.cy, -math.Pi/2, border, held&touchActionStrafeLeft != 0)
	sg.drawPadChevron(screen, pad.cx+pad.radius*0.58, pad.cy, math.Pi/2, border, held&touchActionStrafeRight != 0)
	drawPadThumb(screen, pad, axisX, axisY)
}

func (sg *sessionGame) drawActionTouchPad(screen *ebiten.Image, pad touchPad, held touchActionMask, axisX, axisY float64) {
	baseFill := color.RGBA{R: 20, G: 20, B: 20, A: 92}
	border := color.RGBA{R: 96, G: 96, B: 96, A: 132}
	if held != 0 {
		baseFill = color.RGBA{R: 34, G: 34, B: 34, A: 132}
		border = color.RGBA{R: 140, G: 140, B: 140, A: 172}
	}
	vector.DrawFilledCircle(screen, float32(pad.cx), float32(pad.cy), float32(pad.radius), baseFill, true)
	sg.drawPadCap(screen, pad, true, color.RGBA{R: 150, G: 28, B: 28, A: 120}, held&touchActionFire != 0)
	sg.drawPadCap(screen, pad, false, color.RGBA{R: 28, G: 130, B: 44, A: 120}, held&touchActionUseEnter != 0)
	vector.StrokeCircle(screen, float32(pad.cx), float32(pad.cy), float32(pad.radius), 2, border, true)
	ebitenutil.DrawRect(screen, pad.cx-pad.radius, pad.cy-1, pad.radius*2, 2, border)
	// Labels inside the cap zones
	sg.drawTouchPadLabel(screen, "FIRE", int(pad.cx), int(pad.cy-pad.radius*0.88), true)
	sg.drawTouchPadLabel(screen, "USE", int(pad.cx), int(pad.cy+pad.radius*0.82), true)
	sg.drawTouchPadLabel(screen, "L", int(pad.cx-pad.radius*0.62), int(pad.cy-4), true)
	sg.drawTouchPadLabel(screen, "R", int(pad.cx+pad.radius*0.62), int(pad.cy-4), true)
	drawPadThumb(screen, pad, axisX, axisY)
}

func drawPadThumb(screen *ebiten.Image, pad touchPad, axisX, axisY float64) {
	if axisX == 0 && axisY == 0 {
		return
	}
	thumbR := pad.radius * 0.28
	tx := pad.cx + axisX*pad.radius*0.72
	ty := pad.cy + axisY*pad.radius*0.72
	vector.DrawFilledCircle(screen, float32(tx), float32(ty), float32(thumbR), color.RGBA{R: 200, G: 200, B: 200, A: 160}, true)
	vector.StrokeCircle(screen, float32(tx), float32(ty), float32(thumbR), 2, color.RGBA{R: 255, G: 255, B: 255, A: 200}, true)
}

func (sg *sessionGame) drawPadCap(screen *ebiten.Image, pad touchPad, top bool, base color.RGBA, active bool) {
	fill := base
	if active {
		fill.A = 176
	}
	step := 3.0
	// Cap covers only the outer 25% of the radius (from 0.75r to r).
	capStart := pad.radius * 0.75
	startY := -pad.radius
	endY := -capStart
	if !top {
		startY = capStart
		endY = pad.radius
	}
	for dy := startY; dy < endY; dy += step {
		y := pad.cy + dy
		span := math.Sqrt(maxFloat(0, pad.radius*pad.radius-dy*dy))
		ebitenutil.DrawRect(screen, pad.cx-span, y, span*2, step+1, fill)
	}
}

func (sg *sessionGame) drawPadChevron(screen *ebiten.Image, cx, cy, angle float64, base color.RGBA, active bool) {
	clr := base
	if active {
		clr = color.RGBA{R: 220, G: 220, B: 220, A: 220}
	}
	size := 10.0
	x1, y1 := rotatePoint(-size, size*0.55, angle)
	x2, y2 := rotatePoint(0, -size*0.55, angle)
	x3, y3 := rotatePoint(size, size*0.55, angle)
	vector.StrokeLine(screen, float32(cx+x1), float32(cy+y1), float32(cx+x2), float32(cy+y2), 3, clr, true)
	vector.StrokeLine(screen, float32(cx+x2), float32(cy+y2), float32(cx+x3), float32(cy+y3), 3, clr, true)
}

func rotatePoint(x, y, angle float64) (float64, float64) {
	s, c := math.Sin(angle), math.Cos(angle)
	return x*c - y*s, x*s + y*c
}

func (sg *sessionGame) drawTouchPadLabel(screen *ebiten.Image, label string, x, y int, centered bool) {
	if sg != nil && sg.g != nil {
		sg.drawIntermissionText(screen, label, x, y, 1, 0, 0, centered)
		return
	}
	if centered {
		x -= len(label) * 4
	}
	ebitenutil.DebugPrintAt(screen, label, x, y)
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

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
