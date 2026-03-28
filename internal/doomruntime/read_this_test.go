package doomruntime

import "testing"

func TestReadThisPageNamesPreferDoomOrder(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			IntermissionPatchBank: map[string]WallTexture{
				"HELP1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"HELP2": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	pages := sg.readThisPageNames()
	if len(pages) != 2 {
		t.Fatalf("expected 2 help pages, got %d", len(pages))
	}
	if pages[0] != "HELP2" || pages[1] != "HELP1" {
		t.Fatalf("expected Doom help order HELP2, HELP1; got %q", pages)
	}
}

func TestReadThisPageNamesFallbackToCreditWhenHelpMissing(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			IntermissionPatchBank: map[string]WallTexture{
				"CREDIT": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	pages := sg.readThisPageNames()
	if len(pages) != 1 || pages[0] != "CREDIT" {
		t.Fatalf("expected CREDIT fallback, got %q", pages)
	}
}

func TestCloseReadThisFromGameReturnsToGameplay(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active:           true,
			Mode:             frontendModeReadThis,
			ReadThisFromGame: true,
		},
	}

	sg.closeReadThis()

	if sg.frontend.Active {
		t.Fatal("expected read-this close from game to deactivate frontend")
	}
	if sg.frontend.Mode != frontendModeNone {
		t.Fatalf("expected mode to reset to none, got %d", sg.frontend.Mode)
	}
}

func TestIntermissionPatchCachesImage(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			IntermissionPatchBank: map[string]WallTexture{
				"HELP1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	img1, _, ok := sg.intermissionPatch("HELP1")
	if !ok || img1 == nil {
		t.Fatal("expected HELP1 patch")
	}
	img2, _, ok := sg.intermissionPatch("HELP1")
	if !ok || img2 == nil {
		t.Fatal("expected cached HELP1 patch")
	}
	if img1 != img2 {
		t.Fatal("expected intermission patch image to be cached")
	}
}

func TestMenuPatchCachesImage(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			MenuPatchBank: map[string]WallTexture{
				"M_DOOM": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	img1, _, ok := sg.menuPatch("M_DOOM")
	if !ok || img1 == nil {
		t.Fatal("expected M_DOOM patch")
	}
	img2, _, ok := sg.menuPatch("M_DOOM")
	if !ok || img2 == nil {
		t.Fatal("expected cached M_DOOM patch")
	}
	if img1 != img2 {
		t.Fatal("expected menu patch image to be cached")
	}
}
