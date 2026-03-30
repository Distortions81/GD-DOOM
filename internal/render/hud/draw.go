package hud

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	doomLogicalW = 320
	doomLogicalH = 200
	statusBarY   = 168.0
)

type PatchDrawer func(screen *ebiten.Image, name string, x, y, sx, sy float64) bool
type TextDrawer func(screen *ebiten.Image, text string, x, y, sx, sy float64)
type TextWidthFunc func(text string) int

func Transform(viewW, viewH int, sourcePort bool, hudScale float64) (sx, sy, ox, oy float64) {
	if hudScale <= 0 {
		hudScale = 1
	}
	sx = float64(max(viewW, 1)) / doomLogicalW
	sy = float64(max(viewH, 1)) / doomLogicalH
	if !sourcePort {
		return sx, sy, 0, 0
	}
	sx = hudScale
	sy = hudScale
	fitScale := math.Floor(math.Min(float64(max(viewW, 1))/doomLogicalW, float64(max(viewH, 1))/doomLogicalH))
	if fitScale < 1 {
		fitScale = 1
	}
	if sx > fitScale {
		sx = fitScale
		sy = fitScale
	}
	ox = (float64(viewW) - doomLogicalW*sx) * 0.5
	oy = float64(viewH) - doomLogicalH*sy
	if ox < 0 {
		ox = 0
	}
	if oy < 0 {
		oy = 0
	}
	return sx, sy, ox, oy
}

func StatusBarTop(viewW, viewH int, sourcePort bool, hudScale float64) float64 {
	_, sy, _, oy := Transform(viewW, viewH, sourcePort, hudScale)
	return oy + statusBarY*sy
}

type StatusBarInputs struct {
	ViewW        int
	ViewH        int
	SourcePort   bool
	HUDScale     float64
	Health       int
	Armor        int
	ReadyAmmo    int
	HasReadyAmmo bool
	WeaponOwned  [6]bool
	KeyOn        [3]bool
	AmmoCur      [4]int
	AmmoMax      [4]int
	FacePatch    string
}

func DrawStatusBar(screen *ebiten.Image, in StatusBarInputs, drawPatch PatchDrawer, drawTallNum func(*ebiten.Image, int, int, float64, float64, float64, float64), drawShortNum func(*ebiten.Image, int, int, float64, float64, float64, float64), drawPercent func(*ebiten.Image, int, float64, float64, float64, float64)) {
	if drawPatch == nil || drawTallNum == nil || drawShortNum == nil || drawPercent == nil {
		return
	}
	sx, sy, ox, oy := Transform(in.ViewW, in.ViewH, in.SourcePort, in.HUDScale)
	drawPatch(screen, "STBAR", ox, oy+statusBarY*sy, sx, sy)
	drawPatch(screen, "STARMS", ox+104*sx, oy+168*sy, sx, sy)

	if in.HasReadyAmmo {
		drawTallNum(screen, in.ReadyAmmo, 3, ox+44*sx, oy+171*sy, sx, sy)
	}
	drawPercent(screen, in.Health, ox+90*sx, oy+171*sy, sx, sy)
	drawPercent(screen, in.Armor, ox+221*sx, oy+171*sy, sx, sy)

	for i := 0; i < 6; i++ {
		slot := i + 2
		x := ox + float64(111+(i%3)*12)*sx
		y := oy + float64(172+(i/3)*10)*sy
		name := "STGNUM" + string(rune('0'+slot))
		if in.WeaponOwned[i] {
			name = "STYSNUM" + string(rune('0'+slot))
		}
		drawPatch(screen, name, x, y, sx, sy)
	}

	keyNames := [3]string{"STKEYS0", "STKEYS2", "STKEYS1"}
	keyY := [3]float64{171, 181, 191}
	for i := 0; i < 3; i++ {
		if in.KeyOn[i] {
			drawPatch(screen, keyNames[i], ox+239*sx, oy+keyY[i]*sy, sx, sy)
		}
	}

	curPos := [4][2]float64{{288, 173}, {288, 179}, {288, 191}, {288, 185}}
	maxPos := [4][2]float64{{314, 173}, {314, 179}, {314, 191}, {314, 185}}
	for i := 0; i < 4; i++ {
		drawShortNum(screen, in.AmmoCur[i], 3, ox+curPos[i][0]*sx, oy+curPos[i][1]*sy, sx, sy)
		drawShortNum(screen, in.AmmoMax[i], 3, ox+maxPos[i][0]*sx, oy+maxPos[i][1]*sy, sx, sy)
	}

	drawPatch(screen, in.FacePatch, ox+143*sx, oy+168*sy, sx, sy)
}

type MessageInputs struct {
	ViewW      int
	ViewH      int
	SourcePort bool
	HUDScale   float64
	Message    string
	X          float64
	Y          float64
}

func DrawHUDMessage(screen *ebiten.Image, in MessageInputs, drawText TextDrawer) {
	if drawText == nil || strings.TrimSpace(in.Message) == "" {
		return
	}
	lines := strings.Split(in.Message, "\n")
	if in.SourcePort {
		scale := in.HUDScale
		if scale <= 0 {
			scale = 1
		}
		lineStep := 9.0 * scale
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			drawText(screen, line, in.X*scale, in.Y*scale+float64(i)*lineStep, scale, scale)
		}
		return
	}
	sx, sy, _, _ := Transform(in.ViewW, in.ViewH, in.SourcePort, in.HUDScale)
	lineStep := 9.0 * sy
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		drawText(screen, line, in.X*sx, in.Y*sy+float64(i)*lineStep, sx, sy)
	}
}

type DeathOverlayInputs struct {
	ViewW int
	ViewH int
}

func DrawDeathOverlay(screen *ebiten.Image, in DeathOverlayInputs, textWidth TextWidthFunc, drawText TextDrawer) {
	if textWidth == nil || drawText == nil {
		return
	}
	ebitenutil.DrawRect(screen, 0, 0, float64(in.ViewW), float64(in.ViewH), color.RGBA{R: 25, G: 0, B: 0, A: 130})
	msg1 := "YOU DIED"
	msg2 := "PRESS ENTER TO RESTART"
	sx := 2.0
	sy := 2.0
	w1 := float64(textWidth(msg1)) * sx
	w2 := float64(textWidth(msg2)) * sx
	y := float64(in.ViewH / 2)
	x1 := (float64(in.ViewW) - w1) * 0.5
	x2 := (float64(in.ViewW) - w2) * 0.5
	drawText(screen, msg1, x1, y, sx, sy)
	drawText(screen, msg2, x2, y+22*sy, sx, sy)
}

func DrawFlashOverlay(screen *ebiten.Image, viewW, viewH, damageCount, bonusCount, strengthCount, radSuitTics int) {
	stage, clr := flashOverlayState(damageCount, bonusCount, strengthCount, radSuitTics)
	if stage <= 0 {
		return
	}
	a := flashOverlayAlpha(stage, clr)
	if a == 0 {
		return
	}
	ebitenutil.DrawRect(screen, 0, 0, float64(viewW), float64(viewH), color.RGBA{
		R: clr.R,
		G: clr.G,
		B: clr.B,
		A: a,
	})
}

func flashOverlayState(damageCount, bonusCount, strengthCount, radSuitTics int) (int, color.RGBA) {
	cnt := damageCount
	if strengthCount > 0 {
		berserk := 12 - (strengthCount >> 6)
		if berserk > cnt {
			cnt = berserk
		}
	}
	if cnt > 0 {
		stage := (cnt + 7) >> 3
		if stage > 8 {
			stage = 8
		}
		return stage, color.RGBA{R: 176, G: 32, B: 32}
	}
	if bonusCount > 0 {
		stage := (bonusCount + 7) >> 3
		if stage > 4 {
			stage = 4
		}
		return stage, color.RGBA{R: 216, G: 188, B: 72}
	}
	if radSuitTics > 4*32 || radSuitTics&8 != 0 {
		return 1, color.RGBA{R: 48, G: 160, B: 48}
	}
	return 0, color.RGBA{}
}

func flashOverlayAlpha(stage int, clr color.RGBA) uint8 {
	if stage <= 0 {
		return 0
	}
	switch clr {
	case (color.RGBA{R: 176, G: 32, B: 32}):
		return uint8(min(160, 18+stage*18))
	case (color.RGBA{R: 216, G: 188, B: 72}):
		return uint8(min(96, 12+stage*18))
	case (color.RGBA{R: 48, G: 160, B: 48}):
		return 56
	default:
		return 0
	}
}

type PauseMode int

const (
	PauseModeMain PauseMode = iota
	PauseModeOptions
	PauseModeSound
	PauseModeEpisode
	PauseModeSkill
)

type PauseOverlayInputs struct {
	ViewW                  int
	ViewH                  int
	Visible                bool
	SourcePortMode         bool
	Mode                   PauseMode
	OptionsMenuNames       []string
	OptionsMenuText        []string
	SoundMenuSFXLabel      string
	SoundMenuMusicLabel    string
	EpisodeMenuNames       []string
	SkillMenuNames         []string
	MainMenuNames          []string
	MessagesPatch          string
	ScreenSizeDot          int
	ScreenSizeLabel        string
	HUDScaleDot            int
	HUDScaleCount          int
	HUDScaleLabel          string
	ShowPerf               bool
	OptionsSkullX          int
	MouseSensitivityDot    int
	MouseSensitivityCount  int
	MouseSensitivityLabel  string
	MouseSensitivityX      int
	MouseSensitivityValueX int
	SFXVolumeDot           int
	MusicVolumeDot         int
	HUDMessagesEnabled     bool
	SelectedOptions        int
	SelectedSound          int
	SelectedEpisode        int
	SelectedSkill          int
	SelectedMain           int
	SelectedSkullAlternate bool
	StatusMessage          string
}

func DrawPauseOverlay(screen *ebiten.Image, in PauseOverlayInputs, drawPatch PatchDrawer, drawText TextDrawer, textWidth TextWidthFunc) {
	if !in.Visible || drawPatch == nil {
		return
	}
	const optionsMenuX = 36
	scale := float64(in.ViewW) / 320.0
	scaleY := float64(in.ViewH) / 200.0
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	ox := (float64(in.ViewW) - 320.0*scale) * 0.5
	oy := (float64(in.ViewH) - 200.0*scale) * 0.5
	ebitenutil.DrawRect(screen, ox, oy, 320.0*scale, 200.0*scale, color.RGBA{R: 8, G: 8, B: 8, A: 128})
	patchAt := func(name string, x, y float64) {
		drawPatch(screen, name, ox+x*scale, oy+y*scale, scale, scale)
	}
	textAt := func(text string, x, y, textScale float64) {
		if drawText == nil {
			return
		}
		drawText(screen, text, ox+x*scale, oy+y*scale, scale*textScale, scale*textScale)
	}
	switch in.Mode {
	case PauseModeOptions:
		patchAt("M_OPTTTL", optionsMenuX, 15)
		if drawText != nil {
			backLabel := "BACK: ESC"
			backW := len(backLabel) * 7
			if textWidth != nil {
				backW = textWidth(backLabel)
			}
			backX := 320 - 8 - int(math.Ceil(float64(backW)*1.2))
			textAt(backLabel, float64(backX), 17, 1.2)
		}
		if drawText != nil {
			for i, label := range in.OptionsMenuText {
				if strings.TrimSpace(label) == "" {
					continue
				}
				textAt(label, float64(optionsMenuX), float64(39+i*16), 1.2)
			}
		}
		if drawText != nil {
			label := "OFF"
			if in.HUDMessagesEnabled {
				label = "ON"
			}
			textAt(label, float64(optionsMenuX+215), 39, 1.2)
		}
		if drawText != nil && strings.TrimSpace(in.ScreenSizeLabel) != "" {
			textAt(in.ScreenSizeLabel, float64(optionsMenuX+215), 55, 1.2)
		}
		if drawText != nil && strings.TrimSpace(in.HUDScaleLabel) != "" {
			textAt(in.HUDScaleLabel, float64(optionsMenuX+215), 71, 1.2)
		}
		if drawText != nil {
			label := "OFF"
			if in.ShowPerf {
				label = "ON"
			}
			textAt(label, float64(optionsMenuX+215), 87, 1.2)
		}
		label := in.MouseSensitivityLabel
		if strings.TrimSpace(label) == "" {
			label = fmt.Sprintf("%.2f", 0.0)
		}
		if drawText != nil {
			textAt(label, float64(optionsMenuX+215), 103, 1.2)
		}
		if drawText != nil {
			textAt(fmt.Sprintf("%d", in.SFXVolumeDot), float64(optionsMenuX+215), 119, 1.2)
			textAt(fmt.Sprintf("%d", in.MusicVolumeDot), float64(optionsMenuX+215), 135, 1.2)
		}
	case PauseModeSound:
		patchAt("M_SVOL", 60, 38)
		patchAt(in.SoundMenuSFXLabel, 80, 64)
		patchAt(in.SoundMenuMusicLabel, 80, 96)
		if drawText != nil {
			textAt(fmt.Sprintf("%d", in.SFXVolumeDot), 235, 66, 1.2)
			textAt(fmt.Sprintf("%d", in.MusicVolumeDot), 235, 98, 1.2)
		}
	case PauseModeEpisode:
		patchAt("M_NEWG", 96, 14)
		patchAt("M_EPISOD", 54, 38)
		for i, name := range in.EpisodeMenuNames {
			if strings.TrimSpace(name) == "" {
				continue
			}
			patchAt(name, 48, float64(63+i*16))
		}
	case PauseModeSkill:
		patchAt("M_NEWG", 96, 14)
		patchAt("M_SKILL", 54, 38)
		for i, name := range in.SkillMenuNames {
			patchAt(name, 48, float64(63+i*16))
		}
	default:
		patchAt("M_PAUSE", 126, 4)
		patchAt("M_DOOM", 94, 2)
		for i, name := range in.MainMenuNames {
			patchAt(name, 97, float64(64+i*16))
		}
	}
	skull := "M_SKULL1"
	if in.SelectedSkullAlternate {
		skull = "M_SKULL2"
	}
	switch in.Mode {
	case PauseModeOptions:
		skullX := in.OptionsSkullX
		if skullX <= 0 {
			skullX = optionsMenuX - 32
		}
		patchAt(skull, float64(skullX), float64(37+in.SelectedOptions*16))
	case PauseModeSound:
		skullY := 64
		if in.SelectedSound != 0 {
			skullY += 2 * 16
		}
		patchAt(skull, 48, float64(skullY))
	case PauseModeEpisode:
		patchAt(skull, 16, float64(63+in.SelectedEpisode*16))
	case PauseModeSkill:
		patchAt(skull, 16, float64(63+in.SelectedSkill*16))
	default:
		patchAt(skull, 65, float64(64+in.SelectedMain*16))
	}
	if drawText != nil && strings.TrimSpace(in.StatusMessage) != "" {
		statusX := int(ox + 160.0*scale - float64(len(in.StatusMessage))*3)
		statusY := int(oy + 182.0*scale)
		ebitenutil.DebugPrintAt(screen, in.StatusMessage, statusX, statusY)
	}
}

type PerfInputs struct {
	ViewW      int
	ViewH      int
	SourcePort bool
	HUDScale   float64
	FPSDisplay string
	TicDisplay string
	HostDisplay string
	BenchLine  string
}

func DrawPerfOverlay(screen *ebiten.Image, in PerfInputs, textWidth TextWidthFunc, drawText TextDrawer) {
	if textWidth == nil || drawText == nil {
		return
	}
	sx, sy, _, _ := Transform(in.ViewW, in.ViewH, in.SourcePort, in.HUDScale)
	if in.SourcePort {
		sx = in.HUDScale
		sy = in.HUDScale
		if sx <= 0 {
			sx = 1
		}
		if sy <= 0 {
			sy = 1
		}
	}
	w := textWidth(in.FPSDisplay)
	if w2 := textWidth(in.TicDisplay); w2 > w {
		w = w2
	}
	if strings.TrimSpace(in.HostDisplay) != "" {
		if w2 := textWidth(in.HostDisplay); w2 > w {
			w = w2
		}
	}
	if strings.TrimSpace(in.BenchLine) != "" {
		if w2 := textWidth(in.BenchLine); w2 > w {
			w = w2
		}
	}
	maxX := float64(in.ViewW)
	x := int(maxX - float64(w)*sx - 10*sx)
	if x < 4 {
		x = 4
	}
	drawText(screen, in.FPSDisplay, float64(x), 10*sy, sx, sy)
	drawText(screen, in.TicDisplay, float64(x), 20*sy, sx, sy)
	if strings.TrimSpace(in.HostDisplay) != "" {
		drawText(screen, in.HostDisplay, float64(x), 30*sy, sx, sy)
	}
	if strings.TrimSpace(in.BenchLine) != "" {
		y := 30 * sy
		if strings.TrimSpace(in.HostDisplay) != "" {
			y = 40 * sy
		}
		drawText(screen, in.BenchLine, float64(x), y, sx, sy)
	}
}

func drawPauseThermo(screen *ebiten.Image, x, y, width, dot int, sx, sy float64, drawPatch PatchDrawer) {
	if width < 1 {
		width = 1
	}
	if dot < 0 {
		dot = 0
	}
	if dot > width-1 {
		dot = width - 1
	}
	if !drawPatch(screen, "M_THERML", float64(x)*sx, float64(y)*sy, sx, sy) {
		return
	}
	for i := 0; i < width; i++ {
		drawPatch(screen, "M_THERMM", float64(x+8+i*8)*sx, float64(y)*sy, sx, sy)
	}
	drawPatch(screen, "M_THERMR", float64(x+8+width*8)*sx, float64(y)*sy, sx, sy)
	drawPatch(screen, "M_THERMO", float64(x+8+dot*8)*sx, float64(y)*sy, sx, sy)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
