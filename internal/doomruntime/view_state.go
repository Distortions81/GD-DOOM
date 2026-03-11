package doomruntime

import "gddoom/internal/render/mapview/viewstate"

func boundsViewMetrics(b bounds) (centerX, centerY, worldW, worldH float64) {
	centerX = (b.minX + b.maxX) / 2
	centerY = (b.minY + b.maxY) / 2
	worldW = b.maxX - b.minX
	worldH = b.maxY - b.minY
	return centerX, centerY, worldW, worldH
}

func (g *game) viewport() viewstate.Viewport {
	return viewstate.Viewport{
		Width:       g.viewW,
		Height:      g.viewH,
		RenderAngle: g.renderAngle,
		Rotate:      g.mapRotationActive(),
	}
}
