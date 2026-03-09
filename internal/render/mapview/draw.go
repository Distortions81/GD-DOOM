package mapview

import "github.com/hajimehoshi/ebiten/v2"

// Renderer is the narrow bridge that lets TAB map-mode presentation live
// outside the monolithic automap package while the underlying runtime is
// extracted incrementally.
type Renderer interface {
	MapViewPrepareRenderState()
	MapViewShouldDrawFloorTextures2D() bool
	MapViewDrawFloorTextures2D(screen *ebiten.Image)
	MapViewShouldDrawGrid() bool
	MapViewDrawGrid(screen *ebiten.Image)
	MapViewDrawLines(screen *ebiten.Image)
	MapViewIsSourcePort() bool
	MapViewDrawUseSpecialLines(screen *ebiten.Image)
	MapViewDrawUseTargetHighlight(screen *ebiten.Image)
	MapViewShouldDrawThings() bool
	MapViewDrawThings(screen *ebiten.Image)
	MapViewDrawMarks(screen *ebiten.Image)
	MapViewDrawPlayer(screen *ebiten.Image)
	MapViewDrawPeerPlayers(screen *ebiten.Image)
	MapViewDrawModeOverlay(screen *ebiten.Image)
	MapViewDrawCommonOverlays(screen *ebiten.Image)
}

func Draw(screen *ebiten.Image, r Renderer) {
	if screen == nil || r == nil {
		return
	}
	r.MapViewPrepareRenderState()
	if r.MapViewShouldDrawFloorTextures2D() {
		r.MapViewDrawFloorTextures2D(screen)
	}
	if r.MapViewShouldDrawGrid() {
		r.MapViewDrawGrid(screen)
	}
	r.MapViewDrawLines(screen)
	if r.MapViewIsSourcePort() {
		r.MapViewDrawUseSpecialLines(screen)
		r.MapViewDrawUseTargetHighlight(screen)
	}
	if r.MapViewShouldDrawThings() {
		r.MapViewDrawThings(screen)
	}
	r.MapViewDrawMarks(screen)
	r.MapViewDrawPlayer(screen)
	r.MapViewDrawPeerPlayers(screen)
	r.MapViewDrawModeOverlay(screen)
	r.MapViewDrawCommonOverlays(screen)
}
