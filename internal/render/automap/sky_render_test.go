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
