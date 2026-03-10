package mapview

import (
	"gddoom/internal/render/mapview/linepolicy"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type RenderState struct {
	DrawFloorTextures2D bool
	DrawGrid            bool
	IsSourcePort        bool
	DrawThings          bool
	ShowLegend          bool
	ModeLabel           string
	MapName             string
	SkillLevel          int
	Zoom                float64
	LinePolicyState     linepolicy.State
	ShowGrid            bool
	MarksCount          int
	LineColorMode       string
	ModeOverlayText     string
	StatsOverlayText    string
	CheatOverlayText    string
	FloorOverlayText    string
	ThingOverlayText    string
	HUDMessage          string
	ShowHUDMessage      bool
	IsDead              bool
	Paused              bool
	ShowPerf            bool
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
	ebitenutil.DebugPrintAt(screen, state.ModeOverlayText, 12, 12)
	ebitenutil.DebugPrintAt(screen, state.StatsOverlayText, 12, 28)
	ebitenutil.DebugPrintAt(screen, state.CheatOverlayText, 12, 60)
	ebitenutil.DebugPrintAt(screen, state.FloorOverlayText, 12, 76)
	ebitenutil.DebugPrintAt(screen, state.ThingOverlayText, 12, 92)
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
