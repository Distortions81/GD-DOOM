package menufont

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"gddoom/internal/media"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func originalPatches(t *testing.T) map[string]media.WallTexture {
	t.Helper()
	file, err := wad.Open(filepath.Join("..", "..", "..", "DOOM1.WAD"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := doomtex.LoadFromWAD(file)
	if err != nil {
		t.Fatal(err)
	}
	patches := map[string]media.WallTexture{}
	for _, name := range []string{"M_NGAME", "M_NEWG"} {
		rgba, w, h, x, y, err := set.BuildPatchRGBA(name, 0)
		if err != nil {
			t.Fatal(err)
		}
		patches[name] = media.WallTexture{RGBA: rgba, Width: w, Height: h, OffsetX: x, OffsetY: y}
	}
	return patches
}

func TestOriginalMenuGlyphsAndRecoloredWAD(t *testing.T) {
	for _, recolor := range []bool{false, true} {
		patches := originalPatches(t)
		if recolor {
			for _, p := range patches {
				for i := 0; i < len(p.RGBA); i += 4 {
					p.RGBA[i+1], p.RGBA[i] = p.RGBA[i], 0
				}
			}
		}
		font := New(patches)
		for _, test := range []struct {
			text, patch string
			x           int
		}{{"N", "M_NGAME", 0}, {"M", "M_NEWG", 90}} {
			got, ok := font.Compose(test.text)
			if !ok {
				t.Fatal("missing glyph")
			}
			if got.Width != 16 || got.Height != 15 {
				t.Fatalf("glyph dimensions: %dx%d", got.Width, got.Height)
			}
			p := patches[test.patch]
			for y := 0; y < 15; y++ {
				want := p.RGBA[(y*p.Width+test.x)*4 : (y*p.Width+test.x+16)*4]
				if !bytes.Equal(got.RGBA[y*16*4:(y+1)*16*4], want) {
					t.Fatalf("recolor=%v glyph %s row %d differs from original menu", recolor, test.text, y)
				}
			}
		}
		// W includes the brightest atlas color; it must not clamp to N/M's shade.
		got, _ := font.Compose("W")
		i := (11*got.Width + 3) * 4
		channel := 0
		if recolor {
			channel = 1
		}
		if got.RGBA[i+channel] != 255 {
			t.Fatalf("brightest glyph shade=%v", got.RGBA[i:i+4])
		}
	}
}

func TestMenuFontCaseCoverageAndBaseline(t *testing.T) {
	font := New(nil)
	for _, ch := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!?-:.,'/()[]" {
		if _, ok := font.glyphs[ch]; !ok {
			t.Fatalf("missing required menu glyph %q", ch)
		}
	}
	mixed, ok := font.Compose("Multiplayer")
	if !ok || mixed.Width != 136 || mixed.Height != 15 {
		t.Fatalf("menu label: %dx%d ok=%v", mixed.Width, mixed.Height, ok)
	}
	upper, _ := font.Compose("MULTIPLAYER")
	if upper.Width == mixed.Width || bytes.Equal(upper.RGBA, mixed.RGBA) {
		t.Fatal("capital and small-cap forms were collapsed")
	}
	lower, _ := font.Compose("a")
	if lower.Height != 15 || !bytes.Equal(lower.RGBA[:lower.Width*3*4], make([]byte, lower.Width*3*4)) {
		t.Fatal("small caps lost their three-pixel baseline offset")
	}
	accented, _ := font.Compose("É")
	if accented.Height != 19 || accented.OffsetY != 4 {
		t.Fatalf("accent bounds/offset: %+v", accented)
	}
	multiline, _ := font.Compose("N\nN")
	if multiline.Width != 16 || multiline.Height != 32 {
		t.Fatalf("line spacing: %dx%d", multiline.Width, multiline.Height)
	}
	for _, text := range []string{"Multiplayer", "É", "N\nN"} {
		w, h := font.Measure(text)
		p, _ := font.Compose(text)
		if w != p.Width || h != p.Height {
			t.Fatalf("measure/draw disagree for %q", text)
		}
	}
}

func TestMenuFontFallbackBoundsAndOwnership(t *testing.T) {
	font := New(nil)
	missing, _ := font.Compose("🚀")
	question, _ := font.Compose("?")
	if !bytes.Equal(missing.RGBA, question.RGBA) {
		t.Fatal("unknown glyph must remain visible as ?")
	}
	for _, text := range []string{"", "   ", strings.Repeat("W", 1000), strings.Repeat("N\n", 1000)} {
		if p, ok := font.Compose(text); ok || len(p.RGBA) != 0 {
			t.Fatalf("accepted empty/oversized label len=%d", len(text))
		}
		if w, h := font.Measure(text); w != 0 || h != 0 {
			t.Fatal("invalid label measured nonzero")
		}
	}
	p, _ := font.Compose("Multiplayer")
	pristine := bytes.Clone(p.RGBA)
	clear(p.RGBA)
	again, _ := font.Compose("Multiplayer")
	if !bytes.Equal(again.RGBA, pristine) {
		t.Fatal("composed patches alias font pixels")
	}
	wide, ok := font.Compose(strings.Repeat("W", 30))
	if !ok || wide.Width <= 256 || len(wide.OpaqueRuns) != 0 || len(wide.OpaqueRowRuns) != 0 || len(wide.OpaqueMask) != wide.Width*wide.Height {
		t.Fatal("wide text must not publish truncated eight-bit opaque runs")
	}
}

func TestMenuFontDoesNotMixConflictingPatchPalettes(t *testing.T) {
	patches := originalPatches(t)
	// Only the main menu is green; the NEW GAME heading remains red.
	p := patches["M_NGAME"]
	for i := 0; i < len(p.RGBA); i += 4 {
		p.RGBA[i+1], p.RGBA[i] = p.RGBA[i], 0
	}
	got, _ := New(patches).Compose("N")
	for i := 0; i < len(got.RGBA); i += 4 {
		if got.RGBA[i] != 0 {
			t.Fatal("heading red contaminated the main-menu palette")
		}
	}
}
