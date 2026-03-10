package automap

import (
	"gddoom/internal/render/mapview"
	"gddoom/internal/render/mapview/linepolicy"

	"github.com/hajimehoshi/ebiten/v2"
)

func (g *game) buildMapViewRenderState() mapview.RenderState {
	view := g.State.Snapshot()
	modeLabel := "MAP"
	if g.mode == viewWalk {
		modeLabel = "WALK"
	}
	linePolicyState := linepolicy.StateForAutomap(g.parity.reveal == revealAllMap, g.parity.iddt)
	return mapview.RenderState{
		DrawFloorTextures2D:  g.opts.SourcePortMode && len(g.opts.FlatBank) > 0,
		DrawGrid:             g.showGrid,
		IsSourcePort:         g.opts.SourcePortMode,
		DrawThings:           shouldDrawThings(g.parity),
		ShowLegend:           g.showLegend,
		ModeLabel:            modeLabel,
		MapName:              string(g.m.Name),
		SkillLevel:           g.opts.SkillLevel,
		Zoom:                 view.ZoomLevel(),
		LinePolicyState:      linePolicyState,
		ShowGrid:             g.showGrid,
		MarksCount:           g.marks.Count(),
		LineColorMode:        g.opts.LineColorMode,
		Health:               g.stats.Health,
		Armor:                g.stats.Armor,
		Bullets:              g.stats.Bullets,
		Shells:               g.stats.Shells,
		Rockets:              g.stats.Rockets,
		Cells:                g.stats.Cells,
		KeySummary:           g.inventory.keySummary(),
		WeaponName:           weaponName(g.inventory.ReadyWeapon),
		CheatLevel:           g.cheatLevel,
		Invulnerable:         g.invulnerable,
		MapFloorWorldState:   g.mapFloorWorldState,
		ThingRenderModeLabel: sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode),
		HUDMessage:           g.useText,
		ShowHUDMessage:       g.useFlash > 0,
		IsDead:               g.isDead,
		Paused:               g.paused,
		ShowPerf:             !g.opts.NoFPS,
	}
}

func (g *game) MapViewPrepareRenderState() {
	g.prepareRenderState()
}

func (g *game) MapViewDrawFloorTextures2D(screen *ebiten.Image) {
	g.drawMapFloorTextures2D(screen)
}

func (g *game) MapViewDrawGrid(screen *ebiten.Image) {
	g.drawGrid(screen)
}

func (g *game) MapViewDrawLines(screen *ebiten.Image) {
	g.drawMapLines(screen)
}

func (g *game) MapViewDrawUseSpecialLines(screen *ebiten.Image) {
	g.drawUseSpecialLines(screen)
}

func (g *game) MapViewDrawUseTargetHighlight(screen *ebiten.Image) {
	g.drawUseTargetHighlight(screen)
}

func (g *game) MapViewDrawThings(screen *ebiten.Image) {
	g.drawThings(screen)
}

func (g *game) MapViewDrawMarks(screen *ebiten.Image) {
	g.drawMarks(screen)
}

func (g *game) MapViewDrawPlayer(screen *ebiten.Image) {
	g.drawPlayer(screen)
}

func (g *game) MapViewDrawPeerPlayers(screen *ebiten.Image) {
	g.drawPeerPlayers(screen)
}

func (g *game) MapViewDrawThingLegend(screen *ebiten.Image) {
	g.drawThingLegend(screen)
}

func (g *game) MapViewDrawHUDMessage(screen *ebiten.Image, msg string) {
	g.drawHUDMessage(screen, msg, 0, 0)
}

func (g *game) MapViewDrawDeathOverlay(screen *ebiten.Image) {
	g.drawDeathOverlay(screen)
}

func (g *game) MapViewDrawFlashOverlay(screen *ebiten.Image) {
	g.drawFlashOverlay(screen)
}

func (g *game) MapViewDrawPauseOverlay(screen *ebiten.Image) {
	g.drawPauseOverlay(screen)
}

func (g *game) MapViewDrawPerfOverlay(screen *ebiten.Image) {
	g.drawPerfOverlay(screen)
}
