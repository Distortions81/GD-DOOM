package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/render/scene"
)

func BenchmarkGPUSpriteCommands(b *testing.B) {
	for _, height := range []int{200, 1080} {
		for _, runs := range []bool{false, true} {
			name := "rows"
			if runs {
				name = "runs"
			}
			b.Run(fmt.Sprintf("%d/%s", height, name), func(b *testing.B) {
				g, tex := gpuSpriteTestGame(height, height)
				it := cutoutItem{boundsOK: true, tex: tex, scale: float64(height) / 64, x1: height - 1, y1: height - 1, shadeMul: 192}
				r, commands := g.gpuFrame, &g.gpuFrame.cutoutCommands
				atlas, _ := r.wallTexture(tex)
				record := func() {
					commands.reset()
					if runs {
						g.gpuSprite(it, commands, false)
						return
					}
					// The original row-by-row command generation, including visibility.
					for y := it.y0; y <= it.y1; y++ {
						spans := g.spriteRowVisibleSpansDepthQ(y, it.x0, it.x1, it.depthQ, nil, g.solidClipScratch[:0])
						g.solidClipScratch = spans
						for _, span := range spans {
							u := float32((float64(span.L) + 0.5 - it.dstX) / it.scale)
							v := float32((float64(y) + 0.5 - it.dstY) / it.scale)
							r.rect(commands, span.L, y, span.R, y, atlas, atlas, 2, 192, 0, u, v, float32(1/it.scale), float32(1/it.scale))
						}
					}
				}
				record()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					record()
				}
				vertices, indices := 0, 0
				for _, batch := range commands.batches[:commands.used] {
					vertices += len(batch.vertices)
					indices += len(batch.indices)
				}
				b.ReportMetric(float64(vertices*48+indices*2), "command-B/op")
				b.ReportMetric(float64(vertices/4), "quads/op")
			})
		}
	}
}

// Unlike the empty occlusion fixture, these cases use the per-column buffers
// populated by real gameplay. A nearby sprite covers almost the entire view.
func BenchmarkGPUCloseSprite(b *testing.B) {
	for _, size := range [][2]int{{1920, 1080}, {3840, 2160}} {
		b.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(b *testing.B) {
			g, tex := gpuSpriteTestGame(size[0], size[1])
			g.wallDepthQCol = make([]uint16, size[0])
			g.wallDepthTopCol = make([]int, size[0])
			g.wallDepthBottomCol = make([]int, size[0])
			g.wallDepthClosedCol = make([]bool, size[0])
			g.maskedClipCols = make([][]scene.MaskedClipSpan, size[0])
			g.maskedClipFirstDepthQ = make([]uint16, size[0])
			for x := range g.wallDepthQCol {
				g.wallDepthQCol[x] = 500
			}
			it := cutoutItem{boundsOK: true, tex: tex, scale: 64, x1: size[0] - 1, y1: size[1] - 1, depthQ: 10, shadeMul: 192}
			commands := &g.gpuFrame.cutoutCommands
			g.gpuSprite(it, commands, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				commands.reset()
				g.gpuSprite(it, commands, false)
			}
		})
	}
}

func BenchmarkGPUCloseSpritePlaneOccluders(b *testing.B) {
	for _, bands := range []bool{false, true} {
		name := "rows"
		if bands {
			name = "bands"
		}
		b.Run(name, func(b *testing.B) {
			const width, height = 3840, 2160
			g := &game{viewW: width, viewH: height, wallDepthQCol: make([]uint16, width), wallDepthTopCol: make([]int, width), wallDepthBottomCol: make([]int, width), wallDepthClosedCol: make([]bool, width), maskedClipCols: make([][]scene.MaskedClipSpan, width), maskedClipFirstDepthQ: make([]uint16, width)}
			for x := range g.wallDepthQCol {
				g.wallDepthQCol[x] = 500
			}
			if bands {
				g.gpuFrame = &gpuRenderer{}
			}
			rects := []projectedOpaqueRect{packProjectedOpaqueRect(0, width-1, 0, height-1)}
			record := func() {
				g.ensureBillboardPlaneOccluderRows()
				g.appendProjectedOpaqueRectPlaneOccluders(rects, 10, nil)
			}
			record()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				record()
			}
		})
	}
}

func BenchmarkGPUWallCommands(b *testing.B) {
	for _, repeat := range []int{1, 8, 64} {
		b.Run(fmt.Sprintf("repeat%d", repeat), func(b *testing.B) {
			r, commands := &gpuRenderer{}, &gpuCommands{}
			tex := gpuTexture{id: 1, width: 64, height: 64}
			record := func() {
				commands.reset()
				for x := 0; x < 1920; x++ {
					r.rect(commands, x, 0, x, 1079, tex, tex, 1, 192, 0, float32(x/repeat%64), 0, 0, fracUnit/3)
				}
			}
			record()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				record()
			}
			vertices, indices := 0, 0
			for _, batch := range commands.batches[:commands.used] {
				vertices += len(batch.vertices)
				indices += len(batch.indices)
			}
			b.ReportMetric(float64(vertices*48+indices*2), "command-B/op")
			b.ReportMetric(float64(vertices/4), "quads/op")
		})
	}
}
