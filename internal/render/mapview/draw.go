package mapview

import (
	"github.com/hajimehoshi/ebiten/v2"
)

type RenderState struct {
	DrawFloorTextures2D bool
	DrawGrid            bool
	IsSourcePort        bool
	DrawThings          bool
	ShowLegend          bool
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
	MapViewDrawUseOverlays(screen *ebiten.Image)
	MapViewDrawThings(screen *ebiten.Image)
	MapViewDrawActorOverlays(screen *ebiten.Image)
	MapViewDrawOverlays(screen *ebiten.Image, state RenderState)
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
		b.MapViewDrawUseOverlays(screen)
	}
	if state.DrawThings {
		b.MapViewDrawThings(screen)
	}
	b.MapViewDrawActorOverlays(screen)
	drawCommonOverlays(screen, state, b)
}

func drawCommonOverlays(screen *ebiten.Image, state RenderState, b Backend) {
	b.MapViewDrawOverlays(screen, state)
}
