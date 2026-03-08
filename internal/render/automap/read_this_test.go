package automap

import "testing"

func TestReadThisPageNamesPreferDoomOrder(t *testing.T) {
	sg := &sessionGame{
		g: &game{
			opts: Options{
				IntermissionPatchBank: map[string]WallTexture{
					"HELP1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
					"HELP2": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				},
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
		g: &game{
			opts: Options{
				IntermissionPatchBank: map[string]WallTexture{
					"CREDIT": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				},
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
			active:           true,
			mode:             frontendModeReadThis,
			readThisFromGame: true,
		},
	}

	sg.closeReadThis()

	if sg.frontend.active {
		t.Fatal("expected read-this close from game to deactivate frontend")
	}
	if sg.frontend.mode != frontendModeNone {
		t.Fatalf("expected mode to reset to none, got %d", sg.frontend.mode)
	}
}
