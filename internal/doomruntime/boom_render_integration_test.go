//go:build integration

package doomruntime

import (
	"gddoom/internal/mapdata"
	"github.com/hajimehoshi/ebiten/v2"
	"testing"
)

func TestBoomConveyorPlaneRendering(t *testing.T) {
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		palette := make([]byte, 256*4)
		for i := 0; i < 256; i++ {
			palette[i*4] = byte(i)
			palette[i*4+1] = byte(i * 7)
			palette[i*4+2] = byte(255 - i)
			palette[i*4+3] = 255
		}
		initWallShadePackedLUT(palette)
		doomColormapEnabled = false
		for _, modern := range []bool{false, true} {
			g := &game{opts: Options{GPURenderer: true, SourcePortMode: modern, DoomPaletteRGBA: palette, DisableDoomLighting: true}, viewW: 320, viewH: 200, m: &mapdata.Map{Sectors: []mapdata.Sector{{Light: 192}}}}
			g.boomFloorScroll = []boomScrollVector{{-2 * fracUnit, 3 * fracUnit}}
			g.boomCeilingScroll = make([]boomScrollVector, 1)
			g.worldTic = 35
			key := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], true)
			g.ensureWallLayer()
			g.ensure3DFrameBuffers()
			g.beginGPUFrame()
			if g.gpuFrame == nil {
				t.Fatal("GPU did not initialize")
			}
			g.gpuFrame.solid(&g.gpuFrame.baseCommands, 0, 0, 319, 199, pixelOpaqueA)
			for i := range g.wallPix32 {
				g.wallPix32[i] = pixelOpaqueA
			}
			sample := flatTextureBlendSample{fromIndexed: planeTestIndexedTexture()}
			for y := 120; y < 195; y++ {
				state, ok := g.planeRowRenderState(y, key, 41, 13, -27, 1, 0, 160, 160, 160, 100)
				if !ok {
					t.Fatal("plane projection failed")
				}
				baseKey := key
				baseKey.offsetX = 0
				baseKey.offsetY = 0
				base, _ := g.planeRowRenderState(y, baseKey, 41, 13, -27, 1, 0, 160, 160, 160, 100)
				for _, x := range []int{0, 81, 319} {
					u, v := state.textureStart(x)
					bu, bv := base.textureStart(x)
					if u != bu+key.offsetX || v != bv+key.offsetY {
						t.Fatal("scroll did not translate original plane coordinates")
					}
				}
				g.gpuPlaneSpan(y*320, 0, 319, sample, state)
				active := g.gpuFrame
				g.gpuFrame = nil
				g.drawPlaneTexturedSpanAtDepth(g.wallPix32, y*320, 0, 319, key, sample, state)
				g.gpuFrame = active
			}
			g.finishGPUFrame(ebiten.NewImage(320, 200), 0, 160)
			actual := make([]byte, 320*200*4)
			g.gpu.frame.ReadPixels(actual)
			if n := gpuPixelDifferences(g.wallPix, actual, 0); n != 0 {
				t.Fatalf("modern=%v conveyor plane CPU/GPU differences=%d", modern, n)
			}
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
