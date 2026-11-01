package doomruntime

import (
	"gddoom/internal/render/scene"
	"slices"
)

type boomCutoutDepth struct {
	index int
	depth float64
}

func (g *game) hasTranslucentCutout() bool {
	for _, it := range g.billboardQueueScratch {
		if it.kind == billboardQueueMaskedMids && it.idx >= 0 && it.idx < len(g.maskedMidSegsScratch) && len(g.maskedMidSegsScratch[it.idx].tranmap) == 65536 {
			return true
		}
	}
	return false
}

// A wall can cross an actor or another wall. Group adjacent columns with the
// same far-to-near order, retaining batching without relying on center depth.
func (g *game) drawBoomTranslucentCutouts(focal, focalV float64) {
	g.cutoutPainterOrder = true
	if g.gpuFrame != nil {
		g.gpuFrame.orderedCutouts = true
	}
	refs := g.boomCutoutDepthScratch[:0]
	previous := g.boomCutoutPreviousScratch[:0]
	current := g.boomCutoutCurrentScratch[:0]
	start := 0
	flush := func(end int) {
		for _, index := range previous {
			it := g.billboardQueueScratch[index]
			it.x0 = max(it.x0, start)
			it.x1 = min(it.x1, end)
			if it.x0 <= it.x1 {
				g.drawCutoutItem(it, focal, focalV)
			}
		}
	}
	for x := 0; x < g.viewW; x++ {
		refs = refs[:0]
		for i := len(g.billboardQueueScratch) - 1; i >= 0; i-- {
			it := g.billboardQueueScratch[i]
			if it.debugOverlay || x < it.x0 || x > it.x1 {
				continue
			}
			depth := it.dist
			if it.kind == billboardQueueMaskedMids {
				ms := g.maskedMidSegsScratch[it.idx]
				d, _, ok := scene.ProjectedWallSampleAtX(ms.Projection, x)
				if !ok {
					continue
				}
				depth, _ = ms.classic.sample(x, d, 0)
			}
			refs = append(refs, boomCutoutDepth{i, depth})
		}
		slices.SortStableFunc(refs, func(a, b boomCutoutDepth) int {
			if a.depth > b.depth {
				return -1
			}
			if a.depth < b.depth {
				return 1
			}
			return 0
		})
		current = current[:0]
		for _, ref := range refs {
			current = append(current, ref.index)
		}
		if x == 0 {
			previous = append(previous[:0], current...)
			continue
		}
		if !slices.Equal(previous, current) {
			flush(x - 1)
			start = x
			previous = append(previous[:0], current...)
		}
	}
	flush(g.viewW - 1)
	g.boomCutoutDepthScratch = refs
	g.boomCutoutPreviousScratch = previous
	g.boomCutoutCurrentScratch = current
	g.cutoutPainterOrder = false
	for _, it := range g.billboardQueueScratch {
		if it.debugOverlay {
			g.drawCutoutItem(it, focal, focalV)
		}
	}
}
