package automap

import "testing"

func TestBuildSkyLookupParallel_SourcePortDetailDoesNotWarpSkyProjection(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}}
	camAng := 0.37
	const (
		w    = 640
		h    = 400
		texW = 256
		texH = 128
	)
	g.viewW, g.viewH = w, h
	g.detailLevel = 0
	baseCol, baseRow := g.buildSkyLookupParallel(w, h, doomFocalLength(w), camAng, texW, texH)
	g.detailLevel = 1
	detailCol, detailRow := g.buildSkyLookupParallel(w, h, doomFocalLength(w), camAng, texW, texH)
	if len(baseCol) != w || len(baseRow) != h || len(detailCol) != w || len(detailRow) != h {
		t.Fatalf("lookup size mismatch: base=%dx%d detail=%dx%d", len(baseCol), len(baseRow), len(detailCol), len(detailRow))
	}
	for x := 0; x < w; x++ {
		if baseCol[x] != detailCol[x] {
			t.Fatalf("column lookup changed with detail at x=%d: base=%d detail=%d", x, baseCol[x], detailCol[x])
		}
	}
	for y := 0; y < h; y++ {
		if baseRow[y] != detailRow[y] {
			t.Fatalf("row lookup changed with detail at y=%d: base=%d detail=%d", y, baseRow[y], detailRow[y])
		}
	}
}

func TestBuildSkyLookupParallel_SourcePortUsesOutputProjectionScale(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}}
	camAng := 0.37
	const (
		outW = 1280
		outH = 800
		texW = 256
		texH = 128
	)

	// Baseline: full-resolution render at output size.
	g.viewW, g.viewH = outW, outH
	g.setSkyOutputSize(outW, outH)
	hiCol, hiRow := g.buildSkyLookupParallel(outW, outH, doomFocalLength(outW), camAng, texW, texH)

	// Detail: render at half resolution but keep presentation size identical.
	loW := outW / 2
	loH := outH / 2
	g.viewW, g.viewH = loW, loH
	g.setSkyOutputSize(outW, outH)
	loCol, loRow := g.buildSkyLookupParallel(loW, loH, doomFocalLength(loW), camAng, texW, texH)

	if len(hiCol) != outW || len(hiRow) != outH {
		t.Fatalf("high-detail lookup has wrong size: col=%d row=%d", len(hiCol), len(hiRow))
	}
	if len(loCol) != loW || len(loRow) != loH {
		t.Fatalf("low-detail lookup has wrong size: col=%d row=%d", len(loCol), len(loRow))
	}
	// With 2x nearest upscaling, low-res samples should match odd output centers.
	for x := 0; x < loW; x++ {
		if got, want := hiCol[x*2+1], loCol[x]; got != want {
			t.Fatalf("column lookup mismatch at x=%d: hi=%d low=%d", x, got, want)
		}
	}
	for y := 0; y < loH; y++ {
		if got, want := hiRow[y*2+1], loRow[y]; got != want {
			t.Fatalf("row lookup mismatch at y=%d: hi=%d low=%d", y, got, want)
		}
	}
}

func TestSetSkyOutputSize_ResetsGPUSkyPipelineOnResize(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode: true,
			GPUSky:         true,
		},
		skyOutputW:          640,
		skyOutputH:          400,
		skyLayerTexKey:      "SKY1",
		skyLayerTexW:        256,
		skyLayerTexH:        128,
		skyLayerProjDrawW:   640,
		skyLayerProjDrawH:   400,
		skyLayerProjSampleW: 640,
		skyLayerProjSampleH: 400,
		skyColUCache:        []int{1, 2, 3},
		skyRowVCache:        []int{4, 5, 6},
	}

	g.setSkyOutputSize(1280, 800)

	if g.skyOutputW != 1280 || g.skyOutputH != 800 {
		t.Fatalf("sky output size=%dx%d want 1280x800", g.skyOutputW, g.skyOutputH)
	}
	if g.skyLayerTexKey != "" || g.skyLayerTexW != 0 || g.skyLayerTexH != 0 {
		t.Fatalf("sky texture state not reset: key=%q size=%dx%d", g.skyLayerTexKey, g.skyLayerTexW, g.skyLayerTexH)
	}
	if g.skyLayerProjDrawW != 0 || g.skyLayerProjDrawH != 0 || g.skyLayerProjSampleW != 0 || g.skyLayerProjSampleH != 0 {
		t.Fatalf("sky projection state not reset: draw=%dx%d sample=%dx%d", g.skyLayerProjDrawW, g.skyLayerProjDrawH, g.skyLayerProjSampleW, g.skyLayerProjSampleH)
	}
	if g.skyColUCache != nil || g.skyRowVCache != nil {
		t.Fatal("sky lookup caches should be cleared on resize")
	}
}

func TestNormalizeSkyUpscaleMode(t *testing.T) {
	if got := normalizeSkyUpscaleMode("", true); got != "sharp" {
		t.Fatalf("normalizeSkyUpscaleMode('', true)=%q want sharp", got)
	}
	if got := normalizeSkyUpscaleMode("sharp", true); got != "sharp" {
		t.Fatalf("normalizeSkyUpscaleMode('sharp', true)=%q want sharp", got)
	}
	if got := normalizeSkyUpscaleMode("bicubic", true); got != "sharp" {
		t.Fatalf("normalizeSkyUpscaleMode('bicubic', true)=%q want sharp", got)
	}
	if got := normalizeSkyUpscaleMode("bogus", true); got != "sharp" {
		t.Fatalf("normalizeSkyUpscaleMode('bogus', true)=%q want sharp", got)
	}
	if got := normalizeSkyUpscaleMode("sharp", false); got != "nearest" {
		t.Fatalf("normalizeSkyUpscaleMode('sharp', false)=%q want nearest", got)
	}
}
