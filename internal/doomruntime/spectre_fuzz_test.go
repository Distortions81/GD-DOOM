package doomruntime

import (
	"slices"
	"testing"

	"gddoom/internal/mapdata"
)

func TestSpectreFuzzSpansKeepLogicalPostsAcrossResolutions(t *testing.T) {
	mask := []byte{1, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1}
	tex := &WallTexture{Width: 2, Height: 6, Indexed: make([]byte, 12), OpaqueMask: mask}
	var expected []spectreFuzzSpan
	for _, scale := range []int{1, 2, 6} {
		g := &game{viewW: 320 * scale, viewH: 200 * scale, spectreFuzzPos: 48}
		it := cutoutItem{tex: tex, scale: float64(scale), dstX: float64(10 * scale), dstY: float64(20 * scale), x0: 10 * scale, x1: 12*scale - 1, y0: 20 * scale, y1: 26*scale - 1}
		var actual []spectreFuzzSpan
		g.walkSpectreFuzzSpans(it, func(s spectreFuzzSpan) {
			s.x0 /= scale
			s.x1 = (s.x1+1)/scale - 1
			s.y0 /= scale
			s.y1 = (s.y1+1)/scale - 1
			actual = append(actual, s)
		})
		if scale == 1 {
			expected = []spectreFuzzSpan{
				{10, 10, 20, 23, 10, 20, 48},
				{10, 10, 25, 25, 10, 25, 2},
				{11, 11, 20, 20, 11, 20, 3},
				{11, 11, 22, 25, 11, 22, 4},
			}
		}
		if !slices.Equal(actual, expected) {
			t.Fatalf("scale=%d spans=%v want %v", scale, actual, expected)
		}
		if g.spectreFuzzPos != 8 {
			t.Fatalf("scale=%d next phase=%d want 8", scale, g.spectreFuzzPos)
		}
	}
}

func TestSpectreFuzzMatchesClassicColumnLoop(t *testing.T) {
	palette := make([]byte, 256*4)
	colormap := make([]byte, 32*256)
	for index := 0; index < 256; index++ {
		palette[index*4], palette[index*4+1], palette[index*4+2], palette[index*4+3] = byte(index), byte(index), byte(index), 255
		for row := 0; row < 32; row++ {
			colormap[row*256+index] = byte(max(index-row*3, 0))
		}
	}
	g := newGame(&mapdata.Map{Name: "E1M1"}, Options{Width: 320, Height: 200, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: 32, DisableBillboardClipping: true})
	g.ensureWallLayer()
	want := make([]byte, 320*200)
	for i := range want {
		want[i] = byte(100 + i/320%100)
		g.wallPix32[i] = wallShadePackedLUT[256][want[i]]
	}
	phase := 16
	// Reference Doom's downward, in-place column loop directly, including
	// phase wrap and row-six remapping of already remapped pixels.
	for x := 10; x <= 11; x++ {
		for y := 1; y <= 198; y++ {
			source := (y+doomFuzzOffsets[phase])*320 + x
			want[y*320+x] = colormap[6*256+int(want[source])]
			phase = (phase + 1) % len(doomFuzzOffsets)
		}
	}
	tex := &WallTexture{Width: 1, Height: 1, Indexed: []byte{0}, OpaqueMask: []byte{1}}
	g.spectreFuzzPos = 16
	g.drawShadowSpriteCutout(cutoutItem{tex: tex, scale: 1, x0: 10, x1: 11, y0: 0, y1: 199})
	for i, index := range want {
		if g.wallPix32[i] != wallShadePackedLUT[256][index] {
			t.Fatalf("classic fuzz differs at (%d,%d)", i%320, i/320)
		}
	}
	if g.spectreFuzzPos != phase {
		t.Fatalf("phase=%d want %d", g.spectreFuzzPos, phase)
	}
}

func TestFuzzCellBoundsPartitionNonIntegerScale(t *testing.T) {
	for _, size := range []int{200, 240, 1080, 2160} {
		last := -1
		for cell := 0; cell < 200; cell++ {
			lo, hi := fuzzCellBounds(cell, size, 200)
			if lo != last+1 || hi < lo {
				t.Fatalf("size=%d cell=%d bounds=%d..%d after %d", size, cell, lo, hi, last)
			}
			for pixel := lo; pixel <= hi; pixel++ {
				if pixel*200/size != cell {
					t.Fatalf("size=%d pixel=%d maps to wrong cell", size, pixel)
				}
			}
			last = hi
		}
		if last != size-1 {
			t.Fatalf("size=%d last pixel=%d", size, last)
		}
	}
}

func TestGPUFuzzPaletteRecoversExactColorsWithoutChangingFallback(t *testing.T) {
	pixels := make([]byte, 256*256*4)
	palette := make([]uint32, 256)
	for index := range palette {
		palette[index] = packRGBA(byte(index), byte(index*53), byte(255-index))
	}
	for bucket := 0; bucket < gpuFuzzPaletteBuckets; bucket++ {
		pixels[(gpuPaletteLookupOffset+bucket)*4+2] = byte(bucket)
	}
	probes := putGPUFuzzPalette(pixels, palette)
	for index, color := range palette {
		bucket, found := fuzzPaletteHash(color), false
		for probe := 0; probe < probes; probe++ {
			i := (gpuPaletteLookupOffset + bucket) * 4
			if pixels[i+1] == 0 {
				break
			}
			if palette[int(pixels[i])] == color {
				if int(pixels[i]) != index {
					t.Fatalf("palette index=%d got %d", index, pixels[i])
				}
				found = true
				break
			}
			bucket = (bucket + 1) % gpuFuzzPaletteBuckets
		}
		if !found {
			t.Fatalf("palette index=%d not found in %d probes", index, probes)
		}
	}
	for bucket := 0; bucket < gpuFuzzPaletteBuckets; bucket++ {
		if pixels[(gpuPaletteLookupOffset+bucket)*4+2] != byte(bucket) {
			t.Fatal("exact lookup overwrote quantized fallback")
		}
	}
}
