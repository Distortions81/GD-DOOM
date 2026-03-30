package doomruntime

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func findLocalWAD(t *testing.T, candidates ...string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		for _, name := range candidates {
			cand := filepath.Join(dir, name)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand
			}
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Skipf("WAD not found from %s (tried %v)", wd, candidates)
	return ""
}

func buildWallTextureBankForTest(t *testing.T, ts *doomtex.Set) map[string]WallTexture {
	t.Helper()
	names := ts.TextureNames()
	bank := make(map[string]media.WallTexture, len(names))
	for _, name := range names {
		indexed, iw, ih, ierr := ts.BuildTextureIndexed(name)
		rgba, w, h, berr := ts.BuildTextureRGBA(name, 0)
		if berr != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		colMajor := []uint32(nil)
		if len(rgba32) == w*h {
			colMajor = make([]uint32, len(rgba32))
			for tx := 0; tx < w; tx++ {
				colBase := tx * h
				for ty := 0; ty < h; ty++ {
					colMajor[colBase+ty] = rgba32[ty*w+tx]
				}
			}
		}
		indexedColMajor := []byte(nil)
		if ierr == nil && iw == w && ih == h && len(indexed) == w*h {
			indexedColMajor = make([]byte, len(indexed))
			for tx := 0; tx < w; tx++ {
				colBase := tx * h
				for ty := 0; ty < h; ty++ {
					indexedColMajor[colBase+ty] = indexed[ty*w+tx]
				}
			}
		}
		bank[name] = media.WallTexture{
			RGBA:            rgba,
			RGBA32:          rgba32,
			ColMajor:        colMajor,
			Indexed:         indexed,
			IndexedColMajor: indexedColMajor,
			Width:           w,
			Height:          h,
		}
	}
	return bank
}

func TestDOOMUWallTextureBankContainsFIREWALL(t *testing.T) {
	wadPath := findLocalWAD(t, "DOOMU.WAD", "doomu.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}
	rgba, w, h, err := ts.BuildTextureRGBA("FIREWALL", 0)
	if err != nil {
		t.Fatalf("BuildTextureRGBA(FIREWALL): %v", err)
	}
	if w <= 0 || h <= 0 || len(rgba) != w*h*4 {
		t.Fatalf("BuildTextureRGBA(FIREWALL) returned invalid texture: %dx%d rgba=%d", w, h, len(rgba))
	}
	bank := buildWallTextureBankForTest(t, ts)
	if tex, ok := bank["FIREWALL"]; !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		t.Fatalf("wall texture bank missing FIREWALL or texture invalid: ok=%t tex=%+v", ok, tex)
	}
	seqs := doomtex.LoadWallTextureAnimSequences(ts, doomtex.DoomWallAnimDefs)
	frames := seqs["FIREWALL"]
	if got, want := len(frames), 3; got != want {
		t.Fatalf("FIREWALL anim len=%d want=%d frames=%v", got, want, frames)
	}
	if frames[0] != "FIREWALA" || frames[1] != "FIREWALB" || frames[2] != "FIREWALL" {
		t.Fatalf("FIREWALL anim frames=%v want [FIREWALA FIREWALB FIREWALL]", frames)
	}
}

func TestWallTextureBlendResolvesFIREWALL(t *testing.T) {
	wadPath := findLocalWAD(t, "DOOMU.WAD", "doomu.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatalf("load E1M1: %v", err)
	}
	g := newGame(m, Options{
		Width:          320,
		Height:         200,
		SourcePortMode: true,
		WallTexBank:    buildWallTextureBankForTest(t, ts),
	})
	g.syncRenderState()
	g.prepareRenderState()
	if _, ok := g.wallTextureBlend("FIREWALL", -1, switchTextureSlotMid); !ok {
		t.Fatal("wallTextureBlend(FIREWALL) returned !ok")
	}
}

func TestWallTextureBlendResolvesFIREWALLAnimSequence(t *testing.T) {
	wadPath := findLocalWAD(t, "DOOMU.WAD", "doomu.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatalf("load textures from %s: %v", wadPath, err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatalf("load E1M1: %v", err)
	}
	g := newGame(m, Options{
		Width:          320,
		Height:         200,
		SourcePortMode: true,
		WallTexBank:    buildWallTextureBankForTest(t, ts),
	})
	g.syncRenderState()
	g.prepareRenderState()
	for tic := 0; tic < textureAnimTics*12; tic++ {
		g.worldTic = tic
		if _, ok := g.wallTextureBlend("FIREWALL", -1, switchTextureSlotMid); !ok {
			t.Fatalf("wallTextureBlend(FIREWALL) returned !ok at worldTic=%d resolved=%q", tic, g.resolveAnimatedWallName("FIREWALL"))
		}
	}
}
