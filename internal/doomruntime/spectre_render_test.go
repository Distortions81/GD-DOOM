package doomruntime

import (
	"testing"

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

func TestShadePackedSpectreFuzz_SourcePortUsesRowSixFallback(t *testing.T) {
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
	want := shadePackedRGBA(src, 128)
	if got != want {
		t.Fatalf("spectre fuzz=%08x want=%08x", got, want)
	}
}

func TestBeginSourcePortSpectreFuzzFrame_AdvancesAtDoomRate(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}}
	g.beginSourcePortSpectreFuzzFrame(0)
	if got := g.spectreFuzzPos; got != 0 {
		t.Fatalf("initial fuzz pos=%d want 0", got)
	}
	g.worldTic = 1
	g.beginSourcePortSpectreFuzzFrame(0.3)
	if got := g.spectreFuzzPos; got != 1 {
		t.Fatalf("fuzz pos after 1 tic=%d want 1", got)
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

func TestDrawSpriteCutoutItem_ShadowIgnoresExistingCoverageWithoutMarkingAfterWrite(t *testing.T) {
	g := &game{
		viewW: 4,
		viewH: 4,
		wallPix32: []uint32{
			packRGBA(10, 20, 30), packRGBA(11, 21, 31), packRGBA(12, 22, 32), packRGBA(13, 23, 33),
			packRGBA(40, 50, 60), packRGBA(41, 51, 61), packRGBA(42, 52, 62), packRGBA(43, 53, 63),
			packRGBA(70, 80, 90), packRGBA(71, 81, 91), packRGBA(72, 82, 92), packRGBA(73, 83, 93),
			packRGBA(100, 110, 120), packRGBA(101, 111, 121), packRGBA(102, 112, 122), packRGBA(103, 113, 123),
		},
		cutoutCoverageBits: make([]uint64, 1),
	}
	g.markCutoutCoveredAtIndex(5)

	tex := &WallTexture{
		Width:  1,
		Height: 1,
		RGBA:   []byte{255, 255, 255, 255},
	}

	before := g.wallPix32[5]
	g.drawSpriteCutoutItem(cutoutItem{
		kind:       billboardQueueMonsters,
		shadow:     true,
		tex:        tex,
		boundsOK:   true,
		x0:         1,
		x1:         1,
		y0:         1,
		y1:         1,
		clipTop:    1,
		clipBottom: 1,
		dstX:       1,
		dstY:       1,
		scale:      1,
	})

	if got := g.wallPix32[5]; got == before {
		t.Fatalf("shadow pixel=%08x want changed despite existing coverage", got)
	}
	if got := g.cutoutCoveredAtIndex(5); !got {
		t.Fatal("existing coverage bit should remain set")
	}
}

func TestDrawSpriteCutoutItem_ShadowDoesNotCreateNewCoverage(t *testing.T) {
	g := &game{
		viewW:              4,
		viewH:              4,
		wallPix32:          make([]uint32, 16),
		cutoutCoverageBits: make([]uint64, 1),
	}

	tex := &WallTexture{
		Width:  1,
		Height: 1,
		RGBA:   []byte{255, 255, 255, 255},
	}

	g.drawSpriteCutoutItem(cutoutItem{
		kind:       billboardQueueMonsters,
		shadow:     true,
		tex:        tex,
		boundsOK:   true,
		x0:         1,
		x1:         1,
		y0:         1,
		y1:         1,
		clipTop:    1,
		clipBottom: 1,
		dstX:       1,
		dstY:       1,
		scale:      1,
	})

	if g.cutoutCoveredAtIndex(5) {
		t.Fatal("shadow draw should not create cutout coverage")
	}
}

func TestDrawSpriteCutoutItem_SourcePortShadowDoesNotCreateCoverage(t *testing.T) {
	g := &game{
		opts:               Options{SourcePortMode: true},
		viewW:              1,
		viewH:              1,
		wallPix32:          []uint32{packRGBA(10, 20, 30)},
		cutoutCoverageBits: make([]uint64, 1),
	}

	tex := &WallTexture{
		Width:  1,
		Height: 1,
		RGBA:   []byte{255, 255, 255, 255},
	}

	g.drawSpriteCutoutItem(cutoutItem{
		kind:       billboardQueueMonsters,
		shadow:     true,
		tex:        tex,
		boundsOK:   true,
		x0:         0,
		x1:         0,
		y0:         0,
		y1:         0,
		clipTop:    0,
		clipBottom: 0,
		dstX:       0,
		dstY:       0,
		scale:      1,
	})

	if g.cutoutCoveredAtIndex(0) {
		t.Fatal("source-port shadow draw should not create cutout coverage")
	}
}
