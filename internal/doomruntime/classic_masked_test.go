package doomruntime

import (
	"gddoom/internal/media"
	"testing"
)

func TestClassicMaskedPostProjectsBeforeReciprocalSampling(t *testing.T) {
	row := installTestWallShadeRow(t)
	old := doomColormapEnabled
	doomColormapEnabled = false
	t.Cleanup(func() { doomColormapEnabled = old })
	g := &game{viewW: 1, viewH: 200, wallPix32: make([]uint32, 200)}
	source := make([]byte, 128)
	copy(source, []byte{10, 20, 30, 40})
	source[127] = 77
	tex := WallTexture{Width: 1, Height: 4, ClassicMasked: [][]media.WallPost{{{Top: 0, Length: 4, Indexed: source}}}}
	g.drawBasicWallColumnTexturedMasked(0, 0, 199, 160, 0, -5, 160, wallTextureBlendSample{from: &tex}, 256, 0)
	for y := 0; y < 200; y++ {
		want := uint32(0)
		if y >= 105 && y <= 108 {
			want = row[[]byte{77, 10, 20, 30}[y-105]]
		}
		if g.wallPix32[y] != want {
			t.Fatalf("row %d=%x want %x", y, g.wallPix32[y], want)
		}
	}
}

func TestClassicMaskedSavedClipKeepsBoundaryPixel(t *testing.T) {
	row := installTestWallShadeRow(t)
	old := doomColormapEnabled
	doomColormapEnabled = false
	t.Cleanup(func() { doomColormapEnabled = old })
	g := &game{viewW: 1, viewH: 200, wallPix32: make([]uint32, 200), wallDepthQCol: []uint16{1}, wallDepthTopCol: []int{98}, wallDepthBottomCol: []int{98}}
	source := make([]byte, 128)
	source[16] = 1
	tex := WallTexture{Width: 1, Height: 128, ClassicMasked: [][]media.WallPost{{{Top: 111, Length: 17, Indexed: source}}}}
	g.drawBasicWallColumnTexturedMasked(0, 84, 98, 160*65536.0/56932, 0, 8519628.0/65536, 160, wallTextureBlendSample{from: &tex}, 256, 0)
	if g.wallPix32[98] != row[1] {
		t.Fatalf("saved clip boundary=%x want %x", g.wallPix32[98], row[1])
	}
}
