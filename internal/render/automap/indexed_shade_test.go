package automap

import "testing"

func TestShadePaletteIndexPacked_UsesPaletteEntryAndShade(t *testing.T) {
	pal := make([]byte, 256*4)
	pal[7*4+0] = 200
	pal[7*4+1] = 100
	pal[7*4+2] = 50
	pal[7*4+3] = 255

	initWallShadePackedLUT(pal)
	got := shadePaletteIndexPacked(7, 128)

	want := packRGBA(100, 50, 25)
	if got != want {
		t.Fatalf("shadePaletteIndexPacked=%#08x want %#08x", got, want)
	}
}

func TestShadePackedRGBA_FallsBackToPackedPaletteLUT(t *testing.T) {
	pal := make([]byte, 256*4)
	pal[9*4+0] = 160
	pal[9*4+1] = 80
	pal[9*4+2] = 40
	pal[9*4+3] = 255

	initWallShadePackedLUT(pal)
	doomColormapEnabled = false

	src := packRGBA(160, 80, 40)
	got := shadePackedRGBA(src, 128)
	want := shadePaletteIndexPacked(9, 128)
	if got != want {
		t.Fatalf("shadePackedRGBA=%#08x want %#08x", got, want)
	}
}
