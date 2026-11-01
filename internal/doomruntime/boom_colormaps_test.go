package doomruntime

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"path/filepath"
	"strings"
	"testing"
)

func boomColormapGame(t *testing.T, sourcePort bool) *game {
	t.Helper()
	w, err := wad.Open(filepath.Join("..", "..", "testdata", "boom", "water-actors.wad"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(w, "MAP02")
	if err != nil {
		t.Fatal(err)
	}
	palette, base := make([]byte, 1024), make([]byte, 34*256)
	for i := 0; i < 256; i++ {
		palette[i*4], palette[i*4+1], palette[i*4+2], palette[i*4+3] = byte(i), byte(i), byte(i), 255
	}
	for i := range base {
		base[i] = byte(i)
	}
	g := newGame(m, Options{Headless: true, SourcePortMode: sourcePort, DoomPaletteRGBA: palette, DoomColorMap: base, DoomColorMapRows: 34, BoomColorMaps: doomtex.LoadBoomColormaps(w), SpritePatchBank: map[string]WallTexture{"PISGA0": {Width: 1, Height: 1, RGBA: []byte{8, 8, 8, 255}, Indexed: []byte{8}}}})
	g.setGammaLevel(0)
	return g
}
func TestBoomColormapZoneBoundariesAndRestore(t *testing.T) {
	for _, sourcePort := range []bool{false, true} {
		g := boomColormapGame(t, sourcePort)
		for _, tc := range []struct {
			eye  float64
			name string
		}{{-1.0 / 65536, "BWLOW"}, {0, "BWMID"}, {128, "BWMID"}, {128 + 1.0/65536, "BWHIGH"}, {41, "BWMID"}} {
			g.activateBoomColormap(128, 0, tc.eye)
			if g.boomActiveColormap != tc.name || !doomColormapEnabled || g.renderColorMap()[0] != g.opts.BoomColorMaps[tc.name][0] {
				t.Fatalf("eye=%v active=%q", tc.eye, g.boomActiveColormap)
			}
			want := g.opts.DoomPaletteRGBA[int(g.renderColorMap()[0])*4]
			if channelR(doomColormapRGBA[0]) != want {
				t.Fatal("CPU colormap bank did not switch")
			}
		}
		generation := doomColormapGeneration
		g.activateBoomColormap(-128, 0, -16)
		if g.boomActiveColormap != "" || doomColormapEnabled == sourcePort || g.renderColorMap()[0] != 0 || doomColormapGeneration == generation {
			t.Fatal("leaving height sector did not restore base table/mode")
		}
		side := g.renderSidedef(len(g.m.Sidedefs) - 1)
		if side.Bottom != "-" || side.Mid != "-" || side.Top != "-" {
			t.Fatal("colormap names interpreted as wall textures")
		}
	}
}
func TestBoomColormapWeaponCacheIdentity(t *testing.T) {
	g := boomColormapGame(t, false)
	g.activateBoomColormap(128, 0, -16)
	low := g.weaponCompositeToken("PISGA0", "")
	g.opts.DisableDoomLighting = true
	if token := g.weaponCompositeToken("PISGA0", ""); !strings.HasPrefix(token, "L0:BWLOW@") {
		t.Fatalf("fullbright did not remap through custom row zero: %q", token)
	}
	g.opts.DisableDoomLighting = false
	g.activateBoomColormap(128, 0, 41)
	middle := g.weaponCompositeToken("PISGA0", "")
	if low == middle {
		t.Fatal("weapon token reused across different colormaps")
	}
	_, a, ok := g.resolveSpritePatchTexture(middle)
	if !ok {
		t.Fatal("missing lit weapon patch")
	}
	g.activateBoomColormap(128, 0, -16)
	_, b, ok := g.resolveSpritePatchTexture(low)
	if !ok {
		t.Fatal("missing underwater weapon patch")
	}
	if a.RGBA[0] == b.RGBA[0] {
		t.Fatal("weapon retained previous water tint")
	}
}
