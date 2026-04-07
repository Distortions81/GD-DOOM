package doomruntime

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
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
	if sg.keyJustPressed(ebiten.KeyEnter) || sg.keyJustPressed(ebiten.KeyKPEnter) {
		sg.frontendKeybindCapture = true
		sg.playMenuConfirmSound()
	}
	return nil
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
	footer := "ARROWS MOVE  ENTER REBIND  BKSP CLEAR"
	if sg.frontendKeybindCapture {
		footer = "PRESS A KEY  ESC CANCEL"
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
	footer := "ARROWS MOVE  ENTER REBIND  BKSP CLEAR"
	if g.pauseMenuKeybindCapture {
		footer = "PRESS A KEY  ESC CANCEL"
	}
	drawText(strings.ToUpper(footer), menuX, 182, 0.9)
}
