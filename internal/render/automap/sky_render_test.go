package automap

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func TestSkyTextureCandidates(t *testing.T) {
	tests := []struct {
		name    mapdata.MapName
		wantTop string
	}{
		{name: "E1M1", wantTop: "SKY1"},
		{name: "E2M4", wantTop: "SKY2"},
		{name: "E4M1", wantTop: "SKY4"},
		{name: "MAP07", wantTop: "SKY1"},
		{name: "MAP15", wantTop: "SKY2"},
		{name: "MAP24", wantTop: "SKY3"},
		{name: "CUSTOM", wantTop: "SKY1"},
	}
	for _, tc := range tests {
		got := skyTextureCandidates(tc.name)
		if len(got) == 0 {
			t.Fatalf("%s produced no candidates", tc.name)
		}
		if got[0] != tc.wantTop {
			t.Fatalf("%s top candidate=%s want %s", tc.name, got[0], tc.wantTop)
		}
	}
}

func TestSkyTextureForMap_UsesFallbackWhenPreferredMissing(t *testing.T) {
	mk := func(w, h int) WallTexture {
		return WallTexture{
			RGBA:   make([]byte, w*h*4),
			Width:  w,
			Height: h,
		}
	}
	bank := map[string]WallTexture{
		"SKY1": mk(256, 128),
	}
	tex, ok := skyTextureForMap("E2M1", bank)
	if !ok {
		t.Fatal("expected fallback sky texture")
	}
	if tex.Width != 256 || tex.Height != 128 {
		t.Fatalf("unexpected fallback sky dimensions: %dx%d", tex.Width, tex.Height)
	}
}

func TestSkySampleUV_WrapAndClamp(t *testing.T) {
	const (
		viewW = 640
		viewH = 400
		focal = 480
		texW  = 256
		texH  = 128
	)
	u0, v0 := skySampleUV(319, 0, viewW, viewH, focal, 0, texW, texH)
	u1, v1 := skySampleUV(319, 0, viewW, viewH, focal, 2*math.Pi, texW, texH)
	if u0 != u1 {
		t.Fatalf("expected 2pi wrap equality, got u0=%d u1=%d", u0, u1)
	}
	if v0 != v1 {
		t.Fatalf("expected v unchanged on angle wrap, got v0=%d v1=%d", v0, v1)
	}
	_, vTop := skySampleUV(0, 0, viewW, viewH, focal, 0, texW, texH)
	_, vBottom := skySampleUV(0, viewH, viewW, viewH, focal, 0, texW, texH)
	if vTop < 0 || vTop >= texH {
		t.Fatalf("top v out of range: %d", vTop)
	}
	if vBottom != texH-1 {
		t.Fatalf("bottom v=%d want %d", vBottom, texH-1)
	}
}

func TestSkySampleUV_HorizontalDirectionMatchesProjectionSign(t *testing.T) {
	const (
		viewW = 640
		focal = 480
	)
	aLeft := skySampleAngle(0, viewW, focal, 0)
	aRight := skySampleAngle(viewW-1, viewW, focal, 0)
	if aLeft <= aRight {
		t.Fatalf("expected left sample angle > right sample angle; left=%f right=%f", aLeft, aRight)
	}
}

func TestEffectiveSkyTexHeight_ClipsTransparentBottomRows(t *testing.T) {
	w, h := 8, 8
	rgba := make([]byte, w*h*4)
	for y := 0; y < h-2; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			rgba[i+3] = 255
		}
	}
	got := effectiveSkyTexHeight(WallTexture{RGBA: rgba, Width: w, Height: h})
	if got != h-2 {
		t.Fatalf("effective sky height=%d want %d", got, h-2)
	}
}

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
