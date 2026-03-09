package automap

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func (g *game) MapViewPrepareRenderState() {
	g.prepareRenderState()
}

func (g *game) MapViewShouldDrawFloorTextures2D() bool {
	return g.opts.SourcePortMode && len(g.opts.FlatBank) > 0
}

func (g *game) MapViewDrawFloorTextures2D(screen *ebiten.Image) {
	g.drawMapFloorTextures2D(screen)
}

func (g *game) MapViewShouldDrawGrid() bool {
	return g.showGrid
}

func (g *game) MapViewDrawGrid(screen *ebiten.Image) {
	g.drawGrid(screen)
}

func (g *game) MapViewDrawLines(screen *ebiten.Image) {
	g.drawMapLines(screen)
}

func (g *game) MapViewIsSourcePort() bool {
	return g.opts.SourcePortMode
}

func (g *game) MapViewDrawUseSpecialLines(screen *ebiten.Image) {
	g.drawUseSpecialLines(screen)
}

func (g *game) MapViewDrawUseTargetHighlight(screen *ebiten.Image) {
	g.drawUseTargetHighlight(screen)
}

func (g *game) MapViewShouldDrawThings() bool {
	return shouldDrawThings(g.parity)
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

func (g *game) MapViewDrawModeOverlay(screen *ebiten.Image) {
	modeText := "MAP"
	if g.mode == viewWalk {
		modeText = "WALK"
	}
	revealText := "normal"
	if g.parity.reveal == revealAllMap {
		revealText = "allmap"
	}
	if g.opts.SourcePortMode {
		overlay := fmt.Sprintf("map=%s mode=%s skill=%d zoom=%.2f reveal=%s iddt=%d grid=%t marks=%d colors=%s",
			g.m.Name,
			modeText,
			g.opts.SkillLevel,
			g.zoom,
			revealText,
			g.parity.iddt,
			g.showGrid,
			len(g.marks),
			g.opts.LineColorMode,
		)
		ebitenutil.DebugPrintAt(screen, overlay, 12, 12)
		stats := fmt.Sprintf("hp=%d ar=%d am=%d sh=%d ro=%d ce=%d keys=%s wp=%s",
			g.stats.Health,
			g.stats.Armor,
			g.stats.Bullets,
			g.stats.Shells,
			g.stats.Rockets,
			g.stats.Cells,
			g.inventory.keySummary(),
			weaponName(g.inventory.ReadyWeapon),
		)
		ebitenutil.DebugPrintAt(screen, stats, 12, 28)
		cheat := fmt.Sprintf("cheat=%d invuln=%t", g.cheatLevel, g.invulnerable)
		ebitenutil.DebugPrintAt(screen, cheat, 12, 60)
		floor2D := fmt.Sprintf("floor2d=textured %s", g.mapFloorWorldState)
		ebitenutil.DebugPrintAt(screen, floor2D, 12, 76)
		thingRender := fmt.Sprintf("things=%s", strings.ToLower(sourcePortThingRenderModeLabel(g.opts.SourcePortThingRenderMode)))
		ebitenutil.DebugPrintAt(screen, thingRender, 12, 92)
		if g.showLegend {
			g.drawThingLegend(screen)
		}
	}
}

func (g *game) MapViewDrawCommonOverlays(screen *ebiten.Image) {
	if g.useFlash > 0 {
		g.drawHUDMessage(screen, g.useText, 0, 0)
	}
	if g.isDead {
		g.drawDeathOverlay(screen)
	}
	g.drawFlashOverlay(screen)
	if g.paused {
		g.drawPauseOverlay(screen)
	}
	if !g.opts.NoFPS {
		g.drawPerfOverlay(screen)
	}
}
