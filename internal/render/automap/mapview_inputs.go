package automap

import (
	"gddoom/internal/render/mapview/presenter"
)

func (g *game) mapViewPresenterInputs() presenter.Inputs {
	return presenter.Inputs{
		DrawFloorTextures2D: g.opts.SourcePortMode && len(g.opts.FlatBank) > 0,
		DrawGrid:            g.showGrid,
		IsSourcePort:        g.opts.SourcePortMode,
		DrawThings:          shouldDrawThings(g.parity),
		ShowLegend:          g.showLegend,
		HUDMessage:          g.useText,
		ShowHUDMessage:      g.useFlash > 0,
		IsDead:              g.isDead,
		Paused:              g.paused,
		ShowPerf:            !g.opts.NoFPS,
	}
}
