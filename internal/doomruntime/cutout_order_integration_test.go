//go:build integration

package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/mapdata"
)

func gpuCompareFuzzDrawOrder(t *testing.T) {
	palette := make([]byte, 256*4)
	colormap := make([]byte, 32*256)
	for index := 0; index < 256; index++ {
		palette[index*4], palette[index*4+1], palette[index*4+2], palette[index*4+3] = byte(index), byte(index), byte(index), 255
		for row := 0; row < 32; row++ {
			colormap[row*256+index] = byte(max(index-row*3, 0))
		}
	}
	for _, sourcePort := range []bool{false, true} {
		for _, size := range [][2]int{{320, 200}, {640, 400}} {
			w, h := size[0], size[1]
			g := newGame(&mapdata.Map{Name: "E1M1"}, Options{Width: w, Height: h, SourcePortMode: sourcePort, GPURenderer: true, DisableBillboardClipping: true, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: 32})
			g.viewW, g.viewH = w, h
			g.ensureWallLayer()
			g.ensure3DFrameBuffers()
			g.beginGPUFrame()
			r := g.gpuFrame
			background := make([]byte, w*h*4)
			for i := 0; i < w*h; i++ {
				putGPUPackedPixel(background, i*4, wallShadePackedLUT[256][240])
			}
			makeSprite := func(index byte, hole bool) *WallTexture {
				tex := &WallTexture{Width: 8, Height: 12, Indexed: make([]byte, 8*12), OpaqueMask: make([]byte, 8*12)}
				for i := range tex.Indexed {
					tex.Indexed[i], tex.OpaqueMask[i] = index, 1
					if hole && i%8 >= 3 && i%8 <= 4 {
						tex.OpaqueMask[i] = 0
					}
				}
				return tex
			}
			item := func(tex *WallTexture, distance float64, shadow bool) cutoutItem {
				return cutoutItem{kind: billboardQueueMonsters, boundsOK: true, tex: tex, shadow: shadow, dist: distance, depthQ: encodeDepthQ(distance), shadeMul: 256, scale: float64(w) / 320 * 5, scaleY: float64(h) / 200 * 5, dstX: float64(w) / 320 * 40, dstY: float64(h) / 200 * 30, x0: w * 40 / 320, x1: w*80/320 - 1, y0: h * 30 / 200, y1: h*90/200 - 1}
			}
			for _, hole := range []bool{false, true} {
				front := item(makeSprite(200, hole), 32, false)
				fuzz := item(makeSprite(0, false), 64, true)
				far := item(makeSprite(120, false), 96, false)
				rearFuzz := item(makeSprite(0, false), 128, true)
				for _, items := range [][]cutoutItem{{front, fuzz, far}, {fuzz, front, far}, {front, fuzz, far, rearFuzz}, {fuzz, rearFuzz}} {
					if len(items) == 3 && items[0].shadow {
						items[0].dist, items[1].dist = 16, 32
					}
					g.billboardQueueScratch = append(g.billboardQueueScratch[:0], items...)
					g.sortCutoutItemsFrontToBack()
					copy(g.wallPix, background)
					g.clearCutoutCoverage()
					g.spectreFuzzPos = 16
					g.gpuFrame = nil
					g.drawSceneCutouts(160, 160)
					g.beginGPUFrame()
					g.spectreFuzzPos = 16
					g.drawSceneCutouts(160, 160)
					r.frame.WritePixels(background)
					r.metadata.WritePixels(r.metadataPixels)
					r.drawSceneCutoutPasses()
					actual := make([]byte, len(background))
					r.frame.ReadPixels(actual)
					if diff := gpuPixelDifferences(g.wallPix, actual, 0); diff != 0 {
						t.Fatalf("fuzz order modern=%t %dx%d holes=%t layers=%d: %d CPU/GPU pixels differ", sourcePort, w, h, hole, len(items), diff)
					}
					if !g.billboardQueueScratch[0].shadow {
						x, y := w*45/320, h*35/200
						i := (y*w + x) * 4
						want := wallShadePackedLUT[256][200]
						if actual[i] != byte(want>>pixelRShift) || actual[i+1] != byte(want>>pixelGShift) || actual[i+2] != byte(want>>pixelBShift) {
							t.Fatal("spectre behind an opaque enemy blurred that enemy")
						}
					}
					gpuCapture(t, fmt.Sprintf("fuzz-order-%t-%dx%d-%t-%d", sourcePort, w, h, hole, len(items)), g.wallPix, actual, w, h)
				}
			}
		}
	}
}
