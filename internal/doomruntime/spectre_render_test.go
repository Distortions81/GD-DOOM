package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/mapdata"
)

func TestMonsterSpritePrefix_SpectreUsesDemonSprite(t *testing.T) {
	got, ok := monsterSpritePrefix(58)
	if !ok {
		t.Fatal("spectre should have a monster sprite prefix")
	}
	if got != "SARG" {
		t.Fatalf("prefix=%q want SARG", got)
	}
}

func TestWriteFuzzPixel_UsesDoomScaleNeighborSample(t *testing.T) {
	g := &game{
		viewW: 4,
		viewH: 4,
		wallPix32: []uint32{
			packRGBA(10, 20, 30), packRGBA(11, 21, 31), packRGBA(12, 22, 32), packRGBA(13, 23, 33),
			packRGBA(40, 50, 60), packRGBA(41, 51, 61), packRGBA(42, 52, 62), packRGBA(43, 53, 63),
			packRGBA(70, 80, 90), packRGBA(71, 81, 91), packRGBA(72, 82, 92), packRGBA(73, 83, 93),
			packRGBA(100, 110, 120), packRGBA(101, 111, 121), packRGBA(102, 112, 122), packRGBA(103, 113, 123),
		},
	}
	g.writeFuzzPixel(1, 1, 5)
	if g.wallPix32[5] == 0 || g.wallPix32[5] == packRGBA(41, 51, 61) {
		t.Fatalf("fuzz pixel=%08x want transformed vertically sampled neighbor", g.wallPix32[5])
	}
}

func TestWriteFuzzPixel_SourcePortUsesShadeLUTFallback(t *testing.T) {
	prevLighting := doomLightingEnabled
	prevColormap := doomColormapEnabled
	prevRows := doomColormapRows
	prevLUT := doomRowShadeMulLUT
	defer func() {
		doomLightingEnabled = prevLighting
		doomColormapEnabled = prevColormap
		doomColormapRows = prevRows
		doomRowShadeMulLUT = prevLUT
	}()

	doomLightingEnabled = true
	doomColormapEnabled = false
	doomColormapRows = 32
	doomRowShadeMulLUT = make([]uint16, doomColormapRows)
	doomRowShadeMulLUT[4] = 192
	doomRowShadeMulLUT[5] = 160
	doomRowShadeMulLUT[6] = 128

	g := &game{
		opts:  Options{SourcePortMode: true},
		viewW: 4,
		viewH: 4,
		wallPix32: []uint32{
			packRGBA(10, 20, 30), packRGBA(11, 21, 31), packRGBA(12, 22, 32), packRGBA(13, 23, 33),
			packRGBA(40, 50, 60), packRGBA(41, 51, 61), packRGBA(42, 52, 62), packRGBA(43, 53, 63),
			packRGBA(70, 80, 90), packRGBA(71, 81, 91), packRGBA(72, 82, 92), packRGBA(73, 83, 93),
			packRGBA(100, 110, 120), packRGBA(101, 111, 121), packRGBA(102, 112, 122), packRGBA(103, 113, 123),
		},
	}
	g.writeFuzzPixel(1, 1, 5)
	if got := g.wallPix32[5]; got == 0 || got == packRGBA(41, 51, 61) {
		t.Fatalf("fuzz pixel=%08x want darkened transformed sample", got)
	}
}

func TestShadePackedSpectreFuzz_SourcePortBiasesBrighterThanRowSix(t *testing.T) {
	prevLighting := doomLightingEnabled
	prevColormap := doomColormapEnabled
	prevRows := doomColormapRows
	prevLUT := doomRowShadeMulLUT
	defer func() {
		doomLightingEnabled = prevLighting
		doomColormapEnabled = prevColormap
		doomColormapRows = prevRows
		doomRowShadeMulLUT = prevLUT
	}()

	doomLightingEnabled = true
	doomColormapEnabled = false
	doomColormapRows = 32
	doomRowShadeMulLUT = make([]uint16, doomColormapRows)
	doomRowShadeMulLUT[4] = 192
	doomRowShadeMulLUT[5] = 160
	doomRowShadeMulLUT[6] = 128

	g := &game{opts: Options{SourcePortMode: true}}
	src := packRGBA(160, 80, 40)
	got := g.shadePackedSpectreFuzz(src)
	want := shadePackedRGBA(src, 192)
	if got != want {
		t.Fatalf("spectre fuzz=%08x want=%08x", got, want)
	}
}

func TestBeginSourcePortSpectreFuzzFrame_AdvancesAtDoomRate(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}}
	start := time.Unix(100, 0)
	g.beginSourcePortSpectreFuzzFrame(start)
	if got := g.spectreFuzzPos; got != 0 {
		t.Fatalf("initial fuzz pos=%d want 0", got)
	}
	g.beginSourcePortSpectreFuzzFrame(start.Add(10 * time.Millisecond))
	if got := g.spectreFuzzPos; got != 0 {
		t.Fatalf("fuzz pos after short frame=%d want 0", got)
	}
	g.beginSourcePortSpectreFuzzFrame(start.Add(30 * time.Millisecond))
	if got := g.spectreFuzzPos; got != 1 {
		t.Fatalf("fuzz pos after ~1 tic=%d want 1", got)
	}
}

func TestMonsterSpriteNameForView_SpectreResolvesSprite(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"SARGA1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}
	name, _ := g.monsterSpriteNameForView(0, mapdata.Thing{Type: 58}, 0, 64, 0)
	if name == "" {
		t.Fatal("spectre should resolve a sprite name")
	}
}
