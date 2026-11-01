//go:build integration

package doomruntime

import (
	"gddoom/internal/mapdata"
	"testing"
)

// Exercise every destination/source pair and a second layer in real shader
// draws. Unique palette colors make table addressing independent of RGB ties.
func gpuCompareBoomTranslucency(t *testing.T) {
	p, cm := make([]byte, 1024), make([]byte, 34*256)
	indices := make([]byte, 256)
	for i := 0; i < 256; i++ {
		p[i*4], p[i*4+1], p[i*4+2], p[i*4+3] = byte(i), byte(i*53), byte(255-i), 255
		indices[i] = byte(i)
	}
	for i := range cm {
		cm[i] = byte(i)
	}
	a, b := make([]byte, 65536), make([]byte, 65536)
	for dst := 0; dst < 256; dst++ {
		for src := 0; src < 256; src++ {
			a[dst*256+src] = byte((dst*3 + src*5) / 8)
			b[dst*256+src] = byte((dst*5 + src*3) / 8)
		}
	}
	g := newGame(&mapdata.Map{Name: "MAP01"}, Options{Width: 256, Height: 256, GPURenderer: true, DoomPaletteRGBA: p, DoomColorMap: cm, DoomColorMapRows: 34})
	g.viewW, g.viewH = 256, 256
	g.setGammaLevel(0)
	g.beginGPUFrame()
	r := g.gpuFrame
	if r == nil {
		t.Fatal("GPU fallback")
	}
	tex, ok := r.texture(indices, nil, 256, 1)
	if !ok {
		t.Fatal("missing test atlas")
	}
	for y := 0; y < 256; y++ {
		r.rect(&r.baseCommands, 0, y, 255, y, tex, tex, 0, 256, 0, float32(y*fracUnit), 0, 0, 0)
	}
	r.orderedCutouts = true
	for layer, table := range [][]byte{a, b} {
		commands := r.startTranslucentCutout(table, 0, 255)
		for x := 0; x < 256; x++ {
			source := x
			if layer == 1 {
				source = 255 - x
			}
			r.rect(commands, x, 0, x, 255, tex, tex, 13, -258, 0, float32(source), 0, 0, 0)
		}
	}
	g.finishGPUFrame(r.cutouts, 0, 128)
	pixels := make([]byte, 256*256*4)
	r.cutouts.ReadPixels(pixels)
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			index := int(b[int(a[y*256+x])*256+255-x])
			i := (y*256 + x) * 4
			if pixels[i] != p[index*4] || pixels[i+1] != p[index*4+1] || pixels[i+2] != p[index*4+2] {
				t.Fatalf("two-layer TRANMAP (%d,%d)=%v want=%v", x, y, pixels[i:i+3], p[index*4:index*4+3])
			}
		}
	}
}
