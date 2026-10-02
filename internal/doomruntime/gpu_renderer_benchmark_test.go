//go:build integration

package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/mapdata"
	"github.com/hajimehoshi/ebiten/v2"
)

// ReadPixels waits for the submitted work so these timings include shader
// execution, rather than just command recording. Readbacks are benchmark-only.
func BenchmarkGPUIndexedDraw(b *testing.B) {
	driver := &gpuComparisonDriver{}
	driver.run = func() {
		palette := make([]byte, 256*4)
		for i := 0; i < 256; i++ {
			palette[i*4], palette[i*4+1], palette[i*4+2], palette[i*4+3] = byte(i), byte(255-i), byte(i*53), 255
		}
		initWallShadePackedLUT(palette)
		doomColormapEnabled = false
		for _, size := range [][2]int{{320, 200}, {1920, 1080}} {
			g := &game{opts: Options{GPURenderer: true, SourcePortMode: true, DoomPaletteRGBA: palette}, viewW: size[0], viewH: size[1], m: &mapdata.Map{Name: "E1M1"}}
			g.beginGPUFrame()
			r := g.gpuFrame
			if r == nil {
				b.Fatal("GPU renderer failed to initialize")
			}
			tex, _ := r.texture(planeTestIndexedTexture(), nil, 64, 64)
			r.metadata.WritePixels(r.metadataPixels)
			pixels := make([]byte, size[0]*size[1]*4)
			for mode, name := range []string{"planes", "walls", "sprites"} {
				r.baseCommands.reset()
				if mode == 0 {
					for y := 0; y < size[1]; y++ {
						r.rect(&r.baseCommands, 0, y, size[0]-1, y, tex, tex, mode, 192, 0, 0, 0, fracUnit/3, fracUnit/7)
					}
				} else if mode == 1 {
					for x := 0; x < size[0]; x++ {
						r.rect(&r.baseCommands, x, 0, x, size[1]-1, tex, tex, mode, 192, 0, float32(x%64), 0, 0, fracUnit/3)
					}
				} else {
					r.rect(&r.baseCommands, 0, 0, size[0]-1, size[1]-1, tex, tex, mode, 192, 0, 0, 0, 64/float32(size[0]), 64/float32(size[1]))
				}
				// Compile and warm the backend before timing.
				r.drawCommands(r.frame, &r.baseCommands, ebiten.BlendSourceOver, nil, 0)
				r.frame.ReadPixels(pixels)
				b.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], name), func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						r.drawCommands(r.frame, &r.baseCommands, ebiten.BlendSourceOver, nil, 0)
						r.frame.ReadPixels(pixels)
					}
				})
			}
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		b.Fatal(err)
	}
}
