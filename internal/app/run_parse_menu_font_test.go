package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func TestMenuPatchBankGeneratesLargeMultiplayerLabel(t *testing.T) {
	wf, err := wad.Open(findLocalWADOrSkip(t, "DOOM1.WAD", "DOOMU.WAD", "DOOM2.WAD"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	bank := buildMenuPatchBank(set)
	patch, ok := bank["M_MULTI"]
	if !ok || patch.Height < 14 || patch.Height > 17 || patch.Width < 100 || patch.Width > 223 ||
		len(patch.RGBA) != patch.Width*patch.Height*4 || len(patch.OpaqueMask) != patch.Width*patch.Height || len(patch.OpaqueColumnTop) != patch.Width {
		t.Fatalf("generated label must match the large menu lettering and fit its row: %dx%d, present=%t", patch.Width, patch.Height, ok)
	}
	// Existing menu artwork remains exactly as supplied by the loaded WAD.
	want, _, _, _, _, err := set.BuildPatchRGBA("M_NGAME", 0)
	if err != nil || !bytes.Equal(bank["M_NGAME"].RGBA, want) {
		t.Fatal("generating Multiplayer changed original menu artwork")
	}
}

func TestMenuPatchBankHonorsAuthoredMultiplayerArtwork(t *testing.T) {
	basePath := findLocalWADOrSkip(t, "DOOM1.WAD", "DOOMU.WAD", "DOOM2.WAD")
	base, err := wad.Open(basePath)
	if err != nil {
		t.Fatal(err)
	}
	lump, ok := base.LumpByName("M_OPTION")
	if !ok {
		t.Fatal("fixture has no menu artwork")
	}
	authored, err := base.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "authored", false: "malformed-falls-back"}[valid], func(t *testing.T) {
			data := authored
			if !valid {
				data = []byte("invalid")
			}
			path := filepath.Join(t.TempDir(), "menu.wad")
			if err := os.WriteFile(path, buildAppTestWAD("PWAD", []appTestLump{{name: "M_MULTI", data: data}}), 0600); err != nil {
				t.Fatal(err)
			}
			wf, err := wad.OpenFiles(basePath, path)
			if err != nil {
				t.Fatal(err)
			}
			set, err := doomtex.LoadFromWAD(wf)
			if err != nil {
				t.Fatal(err)
			}
			patch, ok := buildMenuPatchBank(set)["M_MULTI"]
			if !ok {
				t.Fatal("Multiplayer artwork is missing")
			}
			if valid {
				pixels, w, h, x, y, err := set.BuildPatchRGBA("M_MULTI", 0)
				if err != nil || patch.Width != w || patch.Height != h || patch.OffsetX != x || patch.OffsetY != y || !bytes.Equal(patch.RGBA, pixels) {
					t.Fatal("authored Multiplayer artwork was replaced or altered")
				}
			} else if patch.Height < 14 || patch.Width < 100 {
				t.Fatal("malformed override suppressed the generated label")
			}
		})
	}
}
