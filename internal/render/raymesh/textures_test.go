package raymesh

import (
	"slices"
	"testing"

	"gddoom/internal/render/levelmesh"
)

func TestTextureUploadScalePreservesColorsAlphaAndSource(t *testing.T) {
	tex := levelmesh.Texture{Width: 2, Height: 2, RGBA: []byte{255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 128, 10, 20, 30, 0}}
	original := slices.Clone(tex.RGBA)
	scaled := prepareTexture(tex, 2, false)
	if scaled.Width != 4 || scaled.Height != 4 {
		t.Fatalf("upload did not double both dimensions: %+v", scaled)
	}
	for y := range 4 {
		for x := range 4 {
			got := scaled.RGBA[(y*4+x)*4 : (y*4+x)*4+4]
			i := ((y/2)*2 + x/2) * 4
			if !slices.Equal(got, original[i:i+4]) {
				t.Fatal("enlargement resampled colors or alpha")
			}
		}
	}
	if !slices.Equal(tex.RGBA, original) {
		t.Fatal("upload scaling modified shared WAD pixels")
	}
}

func TestFilteredMaskBleedsAcrossRepeatWithoutChangingAlpha(t *testing.T) {
	// The last transparent texel is adjacent to red through the wrap seam,
	// and to blue on its other side. A boundary clamp would choose blue.
	tex := levelmesh.Texture{Width: 4, Height: 1, RGBA: []byte{255, 0, 0, 255, 0, 0, 0, 0, 0, 0, 255, 255, 0, 0, 0, 0}}
	original := slices.Clone(tex.RGBA)
	prepared := prepareTexture(tex, 2, true)
	for x := range 8 {
		i := x * 4
		if prepared.RGBA[i+3] != original[(x/2)*4+3] {
			t.Fatal("mask edge preparation changed cutout coverage")
		}
		if prepared.RGBA[i] == 0 && prepared.RGBA[i+2] == 0 {
			t.Fatal("black remained under filtered transparent texel")
		}
	}
	if prepared.RGBA[7*4] != 255 {
		t.Fatal("mask edge colors did not wrap across the repeat seam")
	}
	if !slices.Equal(tex.RGBA, original) {
		t.Fatal("mask preparation modified shared WAD pixels")
	}
	empty := levelmesh.Texture{Width: 2, Height: 1, RGBA: make([]byte, 8)}
	if p := prepareTexture(empty, 2, true); !slices.Equal(p.RGBA, make([]byte, 32)) {
		t.Fatal("fully transparent mask acquired visible pixels")
	}
}
