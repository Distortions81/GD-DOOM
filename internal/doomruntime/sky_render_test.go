package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/scene"
)

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
	hiCol = append([]int(nil), hiCol...)
	hiRow = append([]int(nil), hiRow...)

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
		hiX := scene.ProjectedSampleIndex(x, loW, outW)
		if got, want := hiCol[hiX], loCol[x]; got != want {
			t.Fatalf("column lookup mismatch at x=%d: hi=%d low=%d", x, got, want)
		}
	}
	for y := 0; y < loH; y++ {
		hiY := scene.ProjectedSampleIndex(y, loH, outH)
		if got, want := hiRow[hiY], loRow[y]; got != want {
			t.Fatalf("row lookup mismatch at y=%d: hi=%d low=%d", y, got, want)
		}
	}
}

func TestSetSkyOutputSize_InvalidatesSkyProjectionStateOnResize(t *testing.T) {
	colCache := make([]int, 3, 8)
	rowCache := make([]int, 3, 9)
	g := &game{
		opts: Options{
			SourcePortMode: true,
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
		skyColUCache:        colCache,
		skyRowVCache:        rowCache,
	}

	g.setSkyOutputSize(1280, 800)

	if g.skyOutputW != 1280 || g.skyOutputH != 800 {
		t.Fatalf("sky output size=%dx%d want 1280x800", g.skyOutputW, g.skyOutputH)
	}
	if g.skyLayerTexKey != "SKY1" || g.skyLayerTexW != 256 || g.skyLayerTexH != 128 {
		t.Fatalf("sky texture state changed: key=%q size=%dx%d", g.skyLayerTexKey, g.skyLayerTexW, g.skyLayerTexH)
	}
	if g.skyLayerProjDrawW != 0 || g.skyLayerProjDrawH != 0 || g.skyLayerProjSampleW != 0 || g.skyLayerProjSampleH != 0 {
		t.Fatalf("sky projection state not reset: draw=%dx%d sample=%dx%d", g.skyLayerProjDrawW, g.skyLayerProjDrawH, g.skyLayerProjSampleW, g.skyLayerProjSampleH)
	}
	if g.skyColViewW != 0 || g.skyRowViewH != 0 {
		t.Fatalf("sky cache metadata not reset: colViewW=%d rowViewH=%d", g.skyColViewW, g.skyRowViewH)
	}
	if cap(g.skyColUCache) != cap(colCache) || cap(g.skyRowVCache) != cap(rowCache) {
		t.Fatal("sky lookup caches should retain backing capacity on resize")
	}
}

func TestPrimarySkyTextureKey(t *testing.T) {
	tests := []struct {
		name   mapdata.MapName
		want   string
		wantOK bool
	}{
		{name: "E1M1", want: "SKY1", wantOK: true},
		{name: "e2m8", want: "SKY2", wantOK: true},
		{name: "E4M1", want: "SKY4", wantOK: true},
		{name: "MAP01", want: "SKY1", wantOK: true},
		{name: "map15", want: "SKY2", wantOK: true},
		{name: "MAP21", want: "SKY3", wantOK: true},
		{name: "MAP00", wantOK: false},
		{name: "TITLE", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := primarySkyTextureKey(tt.name)
		if got != tt.want || ok != tt.wantOK {
			t.Fatalf("primarySkyTextureKey(%q)=(%q,%v) want (%q,%v)", tt.name, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestSkyTextureEntryForMap_PrefersPrimaryThenFallback(t *testing.T) {
	bank := map[string]WallTexture{
		"SKY1": {RGBA: make([]byte, 4), Width: 1, Height: 1},
		"SKY2": {RGBA: make([]byte, 4), Width: 1, Height: 1},
	}
	if got, _, ok := skyTextureEntryForMap("E2M1", bank); !ok || got != "SKY2" {
		t.Fatalf("skyTextureEntryForMap(E2M1)=(%q,%v) want (SKY2,true)", got, ok)
	}
	delete(bank, "SKY2")
	if got, _, ok := skyTextureEntryForMap("E2M1", bank); !ok || got != "SKY1" {
		t.Fatalf("skyTextureEntryForMap(E2M1 fallback)=(%q,%v) want (SKY1,true)", got, ok)
	}
}
