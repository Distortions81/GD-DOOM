package mapview

import (
	"fmt"
	"strings"

	"gddoom/internal/render/mapview/linepolicy"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type RenderState struct {
	DrawFloorTextures2D  bool
	DrawGrid             bool
	IsSourcePort         bool
	DrawThings           bool
	ShowLegend           bool
	ModeLabel            string
	MapName              string
	SkillLevel           int
	Zoom                 float64
	LinePolicyState      linepolicy.State
	ShowGrid             bool
	MarksCount           int
	LineColorMode        string
	Health               int
	Armor                int
	Bullets              int
	Shells               int
	Rockets              int
	Cells                int
	KeySummary           string
	WeaponName           string
	CheatLevel           int
	Invulnerable         bool
	MapFloorWorldState   string
	ThingRenderModeLabel string
	HUDMessage           string
	ShowHUDMessage       bool
	IsDead               bool
	Paused               bool
	ShowPerf             bool
}

// Backend is the narrow bridge that lets TAB map-mode presentation live
// outside the monolithic automap package while state extraction is still
// incremental.
type Backend interface {
	MapViewPrepareRenderState()
	MapViewDrawFloorTextures2D(screen *ebiten.Image)
	MapViewDrawGrid(screen *ebiten.Image)
	MapViewDrawLines(screen *ebiten.Image)
	MapViewDrawUseSpecialLines(screen *ebiten.Image)
	MapViewDrawUseTargetHighlight(screen *ebiten.Image)
	MapViewDrawThings(screen *ebiten.Image)
	MapViewDrawMarks(screen *ebiten.Image)
	MapViewDrawPlayer(screen *ebiten.Image)
	MapViewDrawPeerPlayers(screen *ebiten.Image)
	MapViewDrawThingLegend(screen *ebiten.Image)
	MapViewDrawHUDMessage(screen *ebiten.Image, msg string)
	MapViewDrawDeathOverlay(screen *ebiten.Image)
	MapViewDrawFlashOverlay(screen *ebiten.Image)
	MapViewDrawPauseOverlay(screen *ebiten.Image)
	MapViewDrawPerfOverlay(screen *ebiten.Image)
}

func Draw(screen *ebiten.Image, state RenderState, b Backend) {
	if screen == nil || b == nil {
		return
	}
	b.MapViewPrepareRenderState()
	if state.DrawFloorTextures2D {
		b.MapViewDrawFloorTextures2D(screen)
	}
	if state.DrawGrid {
		b.MapViewDrawGrid(screen)
	}
	b.MapViewDrawLines(screen)
	if state.IsSourcePort {
		b.MapViewDrawUseSpecialLines(screen)
		b.MapViewDrawUseTargetHighlight(screen)
	}
	if state.DrawThings {
		b.MapViewDrawThings(screen)
	}
	b.MapViewDrawMarks(screen)
	b.MapViewDrawPlayer(screen)
	b.MapViewDrawPeerPlayers(screen)
	drawModeOverlay(screen, state, b)
	drawCommonOverlays(screen, state, b)
}

func drawModeOverlay(screen *ebiten.Image, state RenderState, b Backend) {
	if !state.IsSourcePort {
		return
	}
	revealText := "normal"
	if state.LinePolicyState.Reveal == linepolicy.RevealAllMap {
		revealText = "allmap"
	}
	overlay := fmt.Sprintf("map=%s mode=%s skill=%d zoom=%.2f reveal=%s iddt=%d grid=%t marks=%d colors=%s",
		state.MapName,
		state.ModeLabel,
		state.SkillLevel,
		state.Zoom,
		revealText,
		state.LinePolicyState.IDDT,
		state.ShowGrid,
		state.MarksCount,
		state.LineColorMode,
	)
	ebitenutil.DebugPrintAt(screen, overlay, 12, 12)
	stats := fmt.Sprintf("hp=%d ar=%d am=%d sh=%d ro=%d ce=%d keys=%s wp=%s",
		state.Health,
		state.Armor,
		state.Bullets,
		state.Shells,
		state.Rockets,
		state.Cells,
		state.KeySummary,
		state.WeaponName,
	)
	ebitenutil.DebugPrintAt(screen, stats, 12, 28)
	cheat := fmt.Sprintf("cheat=%d invuln=%t", state.CheatLevel, state.Invulnerable)
	ebitenutil.DebugPrintAt(screen, cheat, 12, 60)
	floor2D := fmt.Sprintf("floor2d=textured %s", state.MapFloorWorldState)
	ebitenutil.DebugPrintAt(screen, floor2D, 12, 76)
	thingRender := fmt.Sprintf("things=%s", strings.ToLower(state.ThingRenderModeLabel))
	ebitenutil.DebugPrintAt(screen, thingRender, 12, 92)
	if state.ShowLegend {
		b.MapViewDrawThingLegend(screen)
	}
}

func drawCommonOverlays(screen *ebiten.Image, state RenderState, b Backend) {
	if state.ShowHUDMessage {
		b.MapViewDrawHUDMessage(screen, state.HUDMessage)
	}
	if state.IsDead {
		b.MapViewDrawDeathOverlay(screen)
	}
	b.MapViewDrawFlashOverlay(screen)
	if state.Paused {
		b.MapViewDrawPauseOverlay(screen)
	}
	if state.ShowPerf {
		b.MapViewDrawPerfOverlay(screen)
	}
}
