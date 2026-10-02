//go:build integration

package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/mapdata"
)

func gpuCompareSpectreFuzz(t *testing.T) {
	palette := make([]byte, 256*4)
	colormap := make([]byte, 32*256)
	for index := 0; index < 256; index++ {
		palette[index*4], palette[index*4+1], palette[index*4+2], palette[index*4+3] = byte(index), byte(index*53), byte(255-index), 255
		for row := 0; row < 32; row++ {
			colormap[row*256+index] = byte(index*73 + row*19)
		}
	}
	mask := make([]byte, 8*12)
	for y := 0; y < 12; y++ {
		for x := 0; x < 8; x++ {
			if x != 0 && (y != 5 || x%2 == 0) {
				mask[y*8+x] = 1
			}
		}
	}
	tex := &WallTexture{Width: 8, Height: 12, Indexed: make([]byte, len(mask)), OpaqueMask: mask}
	for _, sourcePort := range []bool{false, true} {
		for _, gamma := range []int{0, 2} {
			var logicalReference []byte
			for _, size := range [][2]int{{320, 200}, {640, 400}, {1920, 1080}} {
				w, h := size[0], size[1]
				g := newGame(&mapdata.Map{Name: "E1M1"}, Options{Width: w, Height: h, SourcePortMode: sourcePort, GPURenderer: true, DisableBillboardClipping: true, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: 32})
				// Faithful normally selects a detail preset; exercise the shader
				// at every requested framebuffer size explicitly.
				g.viewW, g.viewH = w, h
				g.setGammaLevel(gamma)
				g.ensureWallLayer()
				g.ensure3DFrameBuffers()
				g.beginGPUFrame()
				if g.gpuFrame == nil {
					t.Fatal("fuzz GPU renderer unavailable")
				}
				r := g.gpuFrame
				background := make([]byte, w*h*4)
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						p := wallShadePackedLUT[256][(x*320/w*17+y*200/h*31)%256]
						putGPUPackedPixel(background, (y*w+x)*4, p)
						g.wallPix32[y*w+x] = p
					}
				}
				r.frame.WritePixels(background)
				r.snapshot.WritePixels(background)
				it := cutoutItem{boundsOK: true, shadow: true, tex: tex, scale: float64(w) / 320 * 5, scaleY: float64(h) / 200 * 5, dstX: float64(w) / 320 * 40, dstY: float64(h) / 200 * 30, x0: w * 40 / 320, x1: w*80/320 - 1, y0: h * 30 / 200, y1: h*90/200 - 1}
				// Start immediately before a four-negative-offset feedback chain.
				for _, phase := range []int{0, 16, 48} {
					for _, flip := range []bool{false, true} {
						it.flip = flip
						copy(g.wallPix, background)
						g.spectreFuzzPos = phase
						g.drawShadowSpriteCutout(it)
						r.frame.WritePixels(background)
						r.fuzzCommands.reset()
						r.fuzzLogicalCommands.reset()
						g.spectreFuzzPos = phase
						g.gpuSprite(it, &r.fuzzCommands, false)
						r.metadata.WritePixels(r.metadataPixels)
						r.drawSpectreFuzz(r.frame)
						actual := make([]byte, len(background))
						r.frame.ReadPixels(actual)
						if diff := gpuPixelDifferences(g.wallPix, actual, 0); diff != 0 {
							gpuCapture(t, fmt.Sprintf("fuzz-%t-%d-%dx%d-%d-%t", sourcePort, gamma, w, h, phase, flip), g.wallPix, actual, w, h)
							for i := 0; i < len(actual); i += 4 {
								if string(actual[i:i+4]) != string(g.wallPix[i:i+4]) {
									t.Errorf("fuzz modern=%t gamma=%d %dx%d phase=%d flip=%t: %d pixels differ, first (%d,%d) GPU=%v CPU=%v", sourcePort, gamma, w, h, phase, flip, diff, i/4%w, i/4/w, actual[i:i+4], g.wallPix[i:i+4])
									break
								}
							}
						}
						if phase == 0 && !flip {
							if w == 320 {
								logicalReference = append([]byte(nil), actual...)
							} else {
								for y := 0; y < h; y++ {
									for x := 0; x < w; x++ {
										i, j := (y*w+x)*4, (y*200/h*320+x*320/w)*4
										if string(actual[i:i+4]) != string(logicalReference[j:j+4]) {
											t.Fatalf("fuzz grain changed at %dx%d (%d,%d)", w, h, x, y)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}
