package doomruntime

import (
	"image/color"
	"math"
	"strings"

	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (sg *sessionGame) openFrontendKeybindMenu() {
	if sg == nil {
		return
	}
	sg.frontend.Mode = frontendModeKeybinds
	sg.frontendKeybindRow = clampKeybindRow(sg.frontendKeybindRow)
	sg.frontendKeybindSlot = min(max(sg.frontendKeybindSlot, 0), 1)
	sg.frontendKeybindCapture = false
}

func (sg *sessionGame) tickFrontendKeybindMenu() error {
	if sg == nil {
		return nil
	}
	if sg.handleFrontendKeybindPointerPress() {
		return nil
	}
	if sg.frontendKeybindCapture {
		if sg.keyJustPressed(ebiten.KeyEscape) {
			sg.frontendKeybindCapture = false
			sg.playMenuBackSound()
			return nil
		}
		if key, ok := firstSupportedSessionPressedKey(sg.input.justPressedKeys); ok {
			sg.setFrontendBinding(bindingAction(sg.frontendKeybindRow), sg.frontendKeybindSlot, bindingKeyName(key))
			delete(sg.input.justPressedKeys, key)
			sg.frontendKeybindCapture = false
			sg.playMenuConfirmSound()
			return nil
		}
		if button, ok := firstSupportedSessionPressedMouseButton(sg.input.justPressedMouseButtons); ok {
			sg.setFrontendBinding(bindingAction(sg.frontendKeybindRow), sg.frontendKeybindSlot, bindingMouseButtonName(button))
			delete(sg.input.justPressedMouseButtons, button)
			sg.frontendKeybindCapture = false
			sg.playMenuConfirmSound()
		}
		return nil
	}
	if sg.keyJustPressed(ebiten.KeyEscape) {
		sg.frontend.Mode = frontendModeOptions
		sg.frontend.OptionsOn = frontendOptionsRowKeybinds
		sg.playMenuBackSound()
		return nil
	}
	if sg.keyJustPressed(ebiten.KeyArrowUp) {
		sg.frontendKeybindRow = clampKeybindRow(sg.frontendKeybindRow - 1)
		sg.playMenuMoveSound()
	}
	if sg.keyJustPressed(ebiten.KeyArrowDown) {
		sg.frontendKeybindRow = clampKeybindRow(sg.frontendKeybindRow + 1)
		sg.playMenuMoveSound()
	}
	if sg.keyJustPressed(ebiten.KeyArrowLeft) {
		sg.frontendKeybindSlot = max(0, sg.frontendKeybindSlot-1)
		sg.playMenuMoveSound()
	}
	if sg.keyJustPressed(ebiten.KeyArrowRight) {
		sg.frontendKeybindSlot = min(1, sg.frontendKeybindSlot+1)
		sg.playMenuMoveSound()
	}
	if sg.keyJustPressed(ebiten.KeyBackspace) {
		sg.setFrontendBinding(bindingAction(sg.frontendKeybindRow), sg.frontendKeybindSlot, "")
		sg.playMenuBackSound()
	}
	if sg.keyJustPressed(ebiten.KeyF5) {
		sg.opts.InputBindings = runtimecfg.DefaultInputBindings()
		if sg.g != nil {
			sg.g.opts.InputBindings = sg.opts.InputBindings
		}
		if sg.opts.OnInputBindingsChanged != nil {
			sg.opts.OnInputBindingsChanged(sg.opts.InputBindings)
		}
		sg.frontendStatus("DEFAULT BINDINGS RESTORED", doomTicsPerSecond*2)
		sg.playMenuConfirmSound()
	}
	if sg.keyJustPressed(ebiten.KeyEnter) || sg.keyJustPressed(ebiten.KeyKPEnter) {
		sg.frontendKeybindCapture = true
		sg.playMenuConfirmSound()
	}
	return nil
}

func (sg *sessionGame) handleFrontendKeybindPointerPress() bool {
	if sg == nil || !sg.frontend.Active || sg.frontend.Mode != frontendModeKeybinds {
		return false
	}
	sw := max(sg.touch.screenW, 1)
	sh := max(sg.touch.screenH, 1)
	transform := newTouchLayoutTransform(sw, sh, 320, 200)
	handlePoint := func(screenX, screenY int) bool {
		localX, localY, ok := transform.screenToLocal(screenX, screenY)
		if !ok {
			return false
		}
		backRect := sg.frontendBackRect(1.0, 18)
		if frontendPointInRect(localX, localY, backRect) {
			if sg.frontendKeybindCapture {
				sg.frontendKeybindCapture = false
			} else {
				sg.frontend.Mode = frontendModeOptions
				sg.frontend.OptionsOn = frontendOptionsRowKeybinds
			}
			sg.playMenuBackSound()
			return true
		}
		const menuY = 40
		const lineHeight = 16
		start := keybindMenuStartRow(sg.frontendKeybindRow)
		for i := 0; i < keybindMenuVisibleRows; i++ {
			row := start + i
			if row >= int(bindingActionCount) {
				break
			}
			y := menuY + i*lineHeight - 2
			for slot := 0; slot < 2; slot++ {
				x := 184
				if slot == 1 {
					x = 254
				}
				rect := frontendRowRect(x, y, 54, lineHeight)
				if !frontendPointInRect(localX, localY, rect) {
					continue
				}
				if sg.frontendKeybindRow == row && sg.frontendKeybindSlot == slot {
					sg.frontendKeybindCapture = true
					sg.playMenuConfirmSound()
				} else {
					sg.frontendKeybindRow = row
					sg.frontendKeybindSlot = slot
					sg.playMenuMoveSound()
				}
				return true
			}
		}
		return false
	}
	if sg.mouseJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if handlePoint(x, y) {
			return true
		}
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		if handlePoint(x, y) {
			return true
		}
	}
	return false
}

func (sg *sessionGame) drawFrontendKeybindMenu(screen *ebiten.Image, scale, ox, oy float64) {
	if sg == nil {
		return
	}
	const menuX = 16
	const menuY = 40
	const lineHeight = 16
	start := keybindMenuStartRow(sg.frontendKeybindRow)
	backLabel := "BACK: ESC"
	backX := 320 - 8 - int(math.Ceil(float64(sg.intermissionTextWidth(backLabel))*1.0))
	sg.rt.sessionDrawHUTextAt(screen, "KEY BINDINGS", ox+float64(menuX)*scale, oy+float64(18)*scale, scale*1.4, scale*1.4)
	sg.rt.sessionDrawHUTextAt(screen, backLabel, ox+float64(backX)*scale, oy+float64(18)*scale, scale*1.0, scale*1.0)
	sg.rt.sessionDrawHUTextAt(screen, "PRIMARY", ox+float64(188)*scale, oy+float64(28)*scale, scale*1.0, scale*1.0)
	sg.rt.sessionDrawHUTextAt(screen, "ALT", ox+float64(258)*scale, oy+float64(28)*scale, scale*1.0, scale*1.0)
	for i := 0; i < keybindMenuVisibleRows; i++ {
		row := start + i
		if row >= int(bindingActionCount) {
			break
		}
		action := bindingAction(row)
		y := menuY + i*lineHeight + 2
		sg.rt.sessionDrawHUTextAt(screen, bindingActionLabel(action), ox+float64(menuX)*scale, oy+float64(y)*scale, scale*1.0, scale*1.0)
		value := bindingValue(sg.opts.InputBindings, action)
		for slot := 0; slot < 2; slot++ {
			x := 188
			if slot == 1 {
				x = 258
			}
			label := bindingSlotLabel(value[slot])
			if row == sg.frontendKeybindRow && slot == sg.frontendKeybindSlot {
				label = "[" + label + "]"
			}
			sg.rt.sessionDrawHUTextAt(screen, label, ox+float64(x)*scale, oy+float64(y)*scale, scale*1.0, scale*1.0)
		}
	}
	footer := "ARROWS MOVE  ENTER REBIND  BKSP CLEAR  F5 DEFAULTS"
	if msg := bindingConflictMessage(sg.opts.InputBindings, bindingAction(sg.frontendKeybindRow), sg.frontendKeybindSlot); msg != "" {
		footer = msg
	}
	if sg.frontendKeybindCapture {
		footer = "PRESS KEY OR MOUSE  ESC CANCEL"
		ebitenutil.DrawRect(screen, 0, 0, float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy()), color.RGBA{A: 20})
	}
	sg.rt.sessionDrawHUTextAt(screen, footer, ox+float64(menuX)*scale, oy+float64(182)*scale, scale*0.9, scale*0.9)
}

func (g *game) tickPauseKeybindMenu() {
	if g == nil {
		return
	}
	if g.pauseMenuKeybindCapture {
		if g.keyJustPressed(ebiten.KeyEscape) {
			g.pauseMenuKeybindCapture = false
			return
		}
		if key, ok := firstSupportedPressedKey(g.input.justPressedKeys); ok {
			setBindingSlot(&g.opts.InputBindings, bindingAction(g.pauseMenuKeybindRow), g.pauseMenuKeybindSlot, bindingKeyName(key))
			delete(g.input.justPressedKeys, key)
			g.pauseMenuKeybindCapture = false
			g.publishInputBindingsChanged()
			return
		}
		if button, ok := firstSupportedPressedMouseButton(g.input.justPressedMouseButtons); ok {
			setBindingSlot(&g.opts.InputBindings, bindingAction(g.pauseMenuKeybindRow), g.pauseMenuKeybindSlot, bindingMouseButtonName(button))
			delete(g.input.justPressedMouseButtons, button)
			g.pauseMenuKeybindCapture = false
			g.publishInputBindingsChanged()
		}
		return
	}
	if g.keyJustPressed(ebiten.KeyEscape) {
		g.pauseMenuMode = pauseMenuModeOptions
		g.pauseMenuOptionsOn = frontendOptionsRowKeybinds
		return
	}
	if g.keyJustPressed(ebiten.KeyArrowUp) {
		g.pauseMenuKeybindRow = clampKeybindRow(g.pauseMenuKeybindRow - 1)
	}
	if g.keyJustPressed(ebiten.KeyArrowDown) {
		g.pauseMenuKeybindRow = clampKeybindRow(g.pauseMenuKeybindRow + 1)
	}
	if g.keyJustPressed(ebiten.KeyArrowLeft) {
		g.pauseMenuKeybindSlot = max(0, g.pauseMenuKeybindSlot-1)
	}
	if g.keyJustPressed(ebiten.KeyArrowRight) {
		g.pauseMenuKeybindSlot = min(1, g.pauseMenuKeybindSlot+1)
	}
	if g.keyJustPressed(ebiten.KeyBackspace) {
		setBindingSlot(&g.opts.InputBindings, bindingAction(g.pauseMenuKeybindRow), g.pauseMenuKeybindSlot, "")
		g.publishInputBindingsChanged()
	}
	if g.keyJustPressed(ebiten.KeyF5) {
		g.opts.InputBindings = runtimecfg.DefaultInputBindings()
		g.publishInputBindingsChanged()
		g.pauseMenuStatus = "DEFAULT BINDINGS RESTORED"
		g.pauseMenuStatusTics = doomTicsPerSecond * 2
	}
	if g.keyJustPressed(ebiten.KeyEnter) || g.keyJustPressed(ebiten.KeyKPEnter) {
		g.pauseMenuKeybindCapture = true
	}
}

func (g *game) drawPauseKeybindMenu(screen *ebiten.Image, drawText func(string, float64, float64, float64), drawSkull func(int, int)) {
	if g == nil {
		return
	}
	const menuX = 16
	const menuY = 40
	const lineHeight = 16
	start := keybindMenuStartRow(g.pauseMenuKeybindRow)
	drawText("KEY BINDINGS", menuX, 18, 1.4)
	drawText("BACK: ESC", 246, 18, 1.0)
	drawText("PRIMARY", 188, 28, 1.0)
	drawText("ALT", 258, 28, 1.0)
	drawSkull(0, menuY+(g.pauseMenuKeybindRow-start)*lineHeight)
	for i := 0; i < keybindMenuVisibleRows; i++ {
		row := start + i
		if row >= int(bindingActionCount) {
			break
		}
		action := bindingAction(row)
		y := menuY + i*lineHeight + 2
		drawText(bindingActionLabel(action), menuX, float64(y), 1.0)
		value := bindingValue(g.opts.InputBindings, action)
		for slot := 0; slot < 2; slot++ {
			x := 188
			if slot == 1 {
				x = 258
			}
			label := bindingSlotLabel(value[slot])
			if row == g.pauseMenuKeybindRow && slot == g.pauseMenuKeybindSlot {
				label = "[" + label + "]"
			}
			drawText(label, float64(x), float64(y), 1.0)
		}
	}
	footer := "ARROWS MOVE  ENTER REBIND  BKSP CLEAR  F5 DEFAULTS"
	if msg := bindingConflictMessage(g.opts.InputBindings, bindingAction(g.pauseMenuKeybindRow), g.pauseMenuKeybindSlot); msg != "" {
		footer = msg
	}
	if g.pauseMenuKeybindCapture {
		footer = "PRESS KEY OR MOUSE  ESC CANCEL"
	}
	drawText(strings.ToUpper(footer), menuX, 182, 0.9)
}
