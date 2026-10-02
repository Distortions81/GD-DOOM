package doomruntime

// The queue is sorted front to back. With fuzz present, paint it in reverse so
// each spectre samples only surfaces behind it and nearer sprites cover it.
// Frames without fuzz retain the existing front-to-back coverage optimization.
func (g *game) drawSceneCutouts(focal, focalV float64) {
	hasFuzz := false
	for _, it := range g.billboardQueueScratch {
		if it.shadow && !it.debugOverlay {
			hasFuzz = true
			break
		}
	}
	if g.gpuFrame != nil {
		g.gpuFrame.orderedCutouts = hasFuzz
	}
	g.cutoutPainterOrder = hasFuzz
	if hasFuzz {
		for i := len(g.billboardQueueScratch) - 1; i >= 0; i-- {
			it := g.billboardQueueScratch[i]
			if !it.debugOverlay {
				g.drawCutoutItem(it, focal, focalV)
			}
		}
	} else {
		for _, it := range g.billboardQueueScratch {
			if !it.debugOverlay {
				g.drawCutoutItem(it, focal, focalV)
			}
		}
	}
	g.cutoutPainterOrder = false
	for _, it := range g.billboardQueueScratch {
		if it.debugOverlay {
			g.drawCutoutItem(it, focal, focalV)
		}
	}
}
