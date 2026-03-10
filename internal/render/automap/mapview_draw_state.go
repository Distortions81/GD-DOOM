package automap

import (
	"fmt"
	"strings"

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
	revealText := "normal"
	if linePolicyState.Reveal == linepolicy.RevealAllMap {
		revealText = "allmap"
	}
	return mapview.RenderState{
		DrawFloorTextures2D: g.opts.SourcePortMode && len(g.opts.FlatBank) > 0,
		DrawGrid:            g.showGrid,
		IsSourcePort:        g.opts.SourcePortMode,
		DrawThings:          shouldDrawThings(g.parity),
		ShowLegend:          g.showLegend,
		ModeLabel:           modeLabel,
		MapName:             string(g.m.Name),
		SkillLevel:          g.opts.SkillLevel,
		Zoom:                view.ZoomLevel(),
		LinePolicyState:     linePolicyState,
		ShowGrid:            g.showGrid,
		MarksCount:          g.marks.Count(),
		LineColorMode:       g.opts.LineColorMode,
		ModeOverlayText: fmt.Sprintf("map=%s mode=%s skill=%d zoom=%.2f reveal=%s iddt=%d grid=%t marks=%d colors=%s",
			g.m.Name,
			modeLabel,
			g.opts.SkillLevel,
			view.ZoomLevel(),
			revealText,
			linePolicyState.IDDT,
			g.showGrid,
			g.marks.Count(),
			g.opts.LineColorMode,
		),
		StatsOverlayText: fmt.Sprintf("hp=%d ar=%d am=%d sh=%d ro=%d ce=%d keys=%s wp=%s",
			g.stats.Health,
			g.stats.Armor,
			g.stats.Bullets,
			g.stats.Shells,
			g.stats.Rockets,
			g.stats.Cells,
			g.inventory.keySummary(),
			weaponName(g.inventory.ReadyWeapon),
		),
		CheatOverlayText: fmt.Sprintf("cheat=%d invuln=%t", g.cheatLevel, g.invulnerable),
		FloorOverlayText: fmt.Sprintf("floor2d=textured %s", g.mapFloorWorldState),
		ThingOverlayText: fmt.Sprintf("things=%s", strings.ToLower(sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode))),
		HUDMessage:       g.useText,
		ShowHUDMessage:   g.useFlash > 0,
		IsDead:           g.isDead,
		Paused:           g.paused,
		ShowPerf:         !g.opts.NoFPS,
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

func (g *game) MapViewDrawOverlays(screen *ebiten.Image, state mapview.RenderState) {
	if state.ShowLegend {
		g.drawThingLegend(screen)
	}
	if state.ShowHUDMessage {
		g.drawHUDMessage(screen, state.HUDMessage, 0, 0)
	}
	if state.IsDead {
		g.drawDeathOverlay(screen)
	}
	g.drawFlashOverlay(screen)
	if state.Paused {
		g.drawPauseOverlay(screen)
	}
	if state.ShowPerf {
		g.drawPerfOverlay(screen)
	}
}
