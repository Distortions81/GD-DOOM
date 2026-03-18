package scene

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func normalizeName(v string) string { return v }

func TestTextureCandidates(t *testing.T) {
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
		got := TextureCandidates(tc.name, normalizeName)
		if len(got) == 0 {
			t.Fatalf("%s produced no candidates", tc.name)
		}
		if got[0] != tc.wantTop {
			t.Fatalf("%s top candidate=%s want %s", tc.name, got[0], tc.wantTop)
		}
	}
}

func TestTextureForMap_UsesFallbackWhenPreferredMissing(t *testing.T) {
	mk := func(w, h int) Texture {
		return Texture{RGBA: make([]byte, w*h*4), Width: w, Height: h}
	}
	bank := map[string]Texture{"SKY1": mk(256, 128)}
	tex, ok := TextureForMap("E2M1", bank, normalizeName)
	if !ok {
		t.Fatal("expected fallback sky texture")
	}
	if tex.Width != 256 || tex.Height != 128 {
		t.Fatalf("unexpected fallback sky dimensions: %dx%d", tex.Width, tex.Height)
	}
}

func TestSampleUV_WrapAndClamp(t *testing.T) {
	const (
		viewW = 640
		viewH = 400
		focal = 480
		texW  = 256
		texH  = 128
	)
	u0, v0 := SampleUV(319, 0, viewW, viewH, focal, 0, texW, texH)
	u1, v1 := SampleUV(319, 0, viewW, viewH, focal, 2*math.Pi, texW, texH)
	if u0 != u1 {
		t.Fatalf("expected 2pi wrap equality, got u0=%d u1=%d", u0, u1)
	}
	if v0 != v1 {
		t.Fatalf("expected v unchanged on angle wrap, got v0=%d v1=%d", v0, v1)
	}
	_, vTop := SampleUV(0, 0, viewW, viewH, focal, 0, texW, texH)
	_, vBottom := SampleUV(0, viewH, viewW, viewH, focal, 0, texW, texH)
	if vTop < 0 || vTop >= texH {
		t.Fatalf("top v out of range: %d", vTop)
	}
	if vBottom != texH-1 {
		t.Fatalf("bottom v=%d want %d", vBottom, texH-1)
	}
}

func TestSampleUV_HorizontalDirectionMatchesProjectionSign(t *testing.T) {
	const (
		viewW = 640
		focal = 480
	)
	aLeft := SampleAngle(0, viewW, focal, 0)
	aRight := SampleAngle(viewW-1, viewW, focal, 0)
	if aLeft <= aRight {
		t.Fatalf("expected left sample angle > right sample angle; left=%f right=%f", aLeft, aRight)
	}
}

func TestEffectiveTextureHeight_ClipsTransparentBottomRows(t *testing.T) {
	w, h := 8, 8
	rgba := make([]byte, w*h*4)
	for y := 0; y < h-2; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			rgba[i+3] = 255
		}
	}
	got := EffectiveTextureHeight(Texture{RGBA: rgba, Width: w, Height: h})
	if got != h-2 {
		t.Fatalf("effective sky height=%d want %d", got, h-2)
	}
}

func TestProjectedSampleUV_MatchesLookup(t *testing.T) {
	const (
		drawW = 640
		drawH = 400
		outW  = 1280
		outH  = 800
		texW  = 256
		texH  = 128
	)
	camAng := 0.37
	col, row := BuildLookup(drawW, drawH, outW, outH, 320, camAng, texW, texH)
	if len(col) != drawW || len(row) != drawH {
		t.Fatalf("lookup size mismatch: col=%d row=%d", len(col), len(row))
	}
	for _, pt := range [][2]int{
		{0, 0},
		{1, 1},
		{drawW / 2, drawH / 2},
		{drawW - 2, drawH - 2},
		{137, 91},
	} {
		u, v := ProjectedSampleUV(pt[0], pt[1], drawW, drawH, outW, outH, 320, camAng, texW, texH)
		if u != col[pt[0]] {
			t.Fatalf("u mismatch at %v: projected=%d lookup=%d", pt, u, col[pt[0]])
		}
		if v != row[pt[1]] {
			t.Fatalf("v mismatch at %v: projected=%d lookup=%d", pt, v, row[pt[1]])
		}
	}
}
