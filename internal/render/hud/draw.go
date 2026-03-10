package hud

import (
	"image/color"
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

func Transform(viewW, viewH int, sourcePort bool) (sx, sy, ox, oy float64) {
	sx = float64(max(viewW, 1)) / doomLogicalW
	sy = float64(max(viewH, 1)) / doomLogicalH
	if !sourcePort {
		return sx, sy, 0, 0
	}
	sx = sy
	ox = (float64(viewW) - doomLogicalW*sx) * 0.5
	oy = float64(viewH) - doomLogicalH*sy
	if ox < 0 {
		ox = 0
		sx = float64(max(viewW, 1)) / doomLogicalW
		sy = sx
		oy = (float64(viewH) - doomLogicalH*sy) * 0.5
		if oy < 0 {
			oy = 0
		}
	}
	return sx, sy, ox, oy
}

type StatusBarInputs struct {
	ViewW        int
	ViewH        int
	SourcePort   bool
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
	sx, sy, ox, oy := Transform(in.ViewW, in.ViewH, in.SourcePort)
	drawPatch(screen, "STBAR", ox, oy+statusBarY*sy, sx, sy)
	drawPatch(screen, "STARMS", ox+104*sx, oy+168*sy, sx, sy)

	if in.HasReadyAmmo {
		drawTallNum(screen, in.ReadyAmmo, 3, ox+44*sx, oy+171*sy, sx, sy)
	}
	drawPercent(screen, in.Health, ox+90*sx, oy+171*sy, sx, sy)
	drawPercent(screen, in.Armor, ox+221*sx, oy+171*sy, sx, sy)

	for i := 0; i < 6; i++ {
		slot := i + 2
		x := ox + float64(110+(i%3)*12)*sx
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
	Message    string
	X          float64
	Y          float64
}

func DrawHUDMessage(screen *ebiten.Image, in MessageInputs, drawText TextDrawer) {
	if drawText == nil || strings.TrimSpace(in.Message) == "" {
		return
	}
	sx, sy, _, _ := Transform(in.ViewW, in.ViewH, in.SourcePort)
	px := in.X * sx
	py := in.Y * sy
	if in.SourcePort {
		_, _, ox, _ := Transform(in.ViewW, in.ViewH, in.SourcePort)
		px += ox
	}
	drawText(screen, in.Message, px, py, sx, sy)
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

func DrawFlashOverlay(screen *ebiten.Image, viewW, viewH, damageFlashTic, bonusFlashTic int) {
	if damageFlashTic > 0 {
		a := uint8(40 + min(120, damageFlashTic*8))
		ebitenutil.DrawRect(screen, 0, 0, float64(viewW), float64(viewH), color.RGBA{R: 180, G: 20, B: 20, A: a})
	}
	if bonusFlashTic > 0 {
		a := uint8(20 + min(80, bonusFlashTic*6))
		ebitenutil.DrawRect(screen, 0, 0, float64(viewW), float64(viewH), color.RGBA{R: 210, G: 190, B: 80, A: a})
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
	SoundMenuSFXLabel      string
	SoundMenuMusicLabel    string
	EpisodeMenuNames       []string
	SkillMenuNames         []string
	MainMenuNames          []string
	MessagesPatch          string
	DetailPatch            string
	SourcePortDetailLabel  string
	MouseSensitivityDot    int
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

func DrawPauseOverlay(screen *ebiten.Image, in PauseOverlayInputs, drawPatch PatchDrawer, drawText TextDrawer) {
	if !in.Visible || drawPatch == nil {
		return
	}
	sx := float64(in.ViewW) / 320.0
	sy := float64(in.ViewH) / 200.0
	ebitenutil.DrawRect(screen, 0, 0, 320.0*sx, 200.0*sy, color.RGBA{R: 8, G: 8, B: 8, A: 128})
	switch in.Mode {
	case PauseModeOptions:
		drawPatch(screen, "M_OPTTTL", 108*sx, 15*sy, sx, sy)
		for i, name := range in.OptionsMenuNames {
			if strings.TrimSpace(name) == "" {
				continue
			}
			drawPatch(screen, name, 60*sx, float64(37+i*16)*sy, sx, sy)
		}
		drawPatch(screen, in.MessagesPatch, float64(180)*sx, float64(53)*sy, sx, sy)
		if in.SourcePortMode {
			if drawText != nil {
				drawText(screen, in.SourcePortDetailLabel, float64(235)*sx, float64(71)*sy, sx*1.6, sy*1.6)
			}
		} else {
			drawPatch(screen, in.DetailPatch, float64(235)*sx, float64(69)*sy, sx, sy)
		}
		drawPauseThermo(screen, 60, 133, 10, in.MouseSensitivityDot, sx, sy, drawPatch)
	case PauseModeSound:
		drawPatch(screen, "M_SVOL", 60*sx, 38*sy, sx, sy)
		drawPauseThermo(screen, 80, 80, 16, in.SFXVolumeDot, sx, sy, drawPatch)
		drawPauseThermo(screen, 80, 112, 16, in.MusicVolumeDot, sx, sy, drawPatch)
		drawPatch(screen, in.SoundMenuSFXLabel, 80*sx, 64*sy, sx, sy)
		drawPatch(screen, in.SoundMenuMusicLabel, 80*sx, 96*sy, sx, sy)
	case PauseModeEpisode:
		drawPatch(screen, "M_NEWG", 96*sx, 14*sy, sx, sy)
		drawPatch(screen, "M_EPISOD", 54*sx, 38*sy, sx, sy)
		for i, name := range in.EpisodeMenuNames {
			if strings.TrimSpace(name) == "" {
				continue
			}
			drawPatch(screen, name, 48*sx, float64(63+i*16)*sy, sx, sy)
		}
	case PauseModeSkill:
		drawPatch(screen, "M_NEWG", 96*sx, 14*sy, sx, sy)
		drawPatch(screen, "M_SKILL", 54*sx, 38*sy, sx, sy)
		for i, name := range in.SkillMenuNames {
			drawPatch(screen, name, 48*sx, float64(63+i*16)*sy, sx, sy)
		}
	default:
		drawPatch(screen, "M_PAUSE", 126*sx, 4*sy, sx, sy)
		drawPatch(screen, "M_DOOM", 94*sx, 2*sy, sx, sy)
		for i, name := range in.MainMenuNames {
			drawPatch(screen, name, 97*sx, float64(64+i*16)*sy, sx, sy)
		}
	}
	skull := "M_SKULL1"
	if in.SelectedSkullAlternate {
		skull = "M_SKULL2"
	}
	switch in.Mode {
	case PauseModeOptions:
		drawPatch(screen, skull, 28*sx, float64(37+in.SelectedOptions*16)*sy, sx, sy)
	case PauseModeSound:
		skullY := 64
		if in.SelectedSound != 0 {
			skullY += 2 * 16
		}
		drawPatch(screen, skull, 48*sx, float64(skullY)*sy, sx, sy)
	case PauseModeEpisode:
		drawPatch(screen, skull, 16*sx, float64(63+in.SelectedEpisode*16)*sy, sx, sy)
	case PauseModeSkill:
		drawPatch(screen, skull, 16*sx, float64(63+in.SelectedSkill*16)*sy, sx, sy)
	default:
		drawPatch(screen, skull, 65*sx, float64(64+in.SelectedMain*16)*sy, sx, sy)
	}
	if drawText != nil && strings.TrimSpace(in.StatusMessage) != "" {
		ebitenutil.DebugPrintAt(screen, in.StatusMessage, in.ViewW/2-len(in.StatusMessage)*3, int(182*sy))
	}
}

type PerfInputs struct {
	ViewW      int
	ViewH      int
	SourcePort bool
	FPSDisplay string
	TicDisplay string
}

func DrawPerfOverlay(screen *ebiten.Image, in PerfInputs, textWidth TextWidthFunc, drawText TextDrawer) {
	if textWidth == nil || drawText == nil {
		return
	}
	sx, sy, ox, _ := Transform(in.ViewW, in.ViewH, in.SourcePort)
	w := textWidth(in.FPSDisplay)
	if w2 := textWidth(in.TicDisplay); w2 > w {
		w = w2
	}
	maxX := float64(in.ViewW)
	if in.SourcePort {
		maxX = ox + doomLogicalW*sx
	}
	x := int(maxX - float64(w)*sx - 10*sx)
	if x < 4 {
		x = 4
	}
	drawText(screen, in.FPSDisplay, float64(x), 10*sy, sx, sy)
	drawText(screen, in.TicDisplay, float64(x), 20*sy, sx, sy)
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
