package doomruntime

import "testing"

func TestDefaultScreenBlocks(t *testing.T) {
	if got := defaultScreenBlocks(Options{}); got != doomScreenBlocksFull {
		t.Fatalf("defaultScreenBlocks without status bar = %d, want %d", got, doomScreenBlocksFull)
	}
	opts := Options{
		StatusPatchBank: map[string]WallTexture{
			"STBAR": {},
		},
	}
	if got := defaultScreenBlocks(opts); got != doomScreenBlocksDefault {
		t.Fatalf("defaultScreenBlocks with status bar = %d, want %d", got, doomScreenBlocksDefault)
	}
	opts.SourcePortMode = true
	if got := defaultScreenBlocks(opts); got != doomScreenBlocksOverlay {
		t.Fatalf("defaultScreenBlocks sourceport with status bar = %d, want %d", got, doomScreenBlocksOverlay)
	}
}

func TestClampScreenBlocks_DisallowsOverlayInFaithful(t *testing.T) {
	opts := Options{
		StatusPatchBank: map[string]WallTexture{
			"STBAR": {},
		},
	}
	if got := clampScreenBlocks(opts, doomScreenBlocksOverlay); got != doomScreenBlocksDefault {
		t.Fatalf("clampScreenBlocks faithful overlay = %d, want %d", got, doomScreenBlocksDefault)
	}
	opts.SourcePortMode = true
	if got := clampScreenBlocks(opts, doomScreenBlocksOverlay); got != doomScreenBlocksOverlay {
		t.Fatalf("clampScreenBlocks sourceport overlay = %d, want %d", got, doomScreenBlocksOverlay)
	}
	if got := clampScreenBlocks(opts, doomScreenBlocksDefault); got != doomScreenBlocksOverlay {
		t.Fatalf("clampScreenBlocks sourceport bottom = %d, want %d", got, doomScreenBlocksOverlay)
	}
}

func TestDefaultHUDScaleStep(t *testing.T) {
	if got := defaultHUDScaleStep(Options{}); got != 1 {
		t.Fatalf("defaultHUDScaleStep without sourceport = %d, want 1", got)
	}
	if got := defaultHUDScaleStep(Options{SourcePortMode: true}); got != 1 {
		t.Fatalf("defaultHUDScaleStep with sourceport = %d, want 1", got)
	}
	if got := defaultHUDScaleStep(Options{StatusPatchBank: map[string]WallTexture{"STBAR": {}}, SourcePortMode: false}); got != 1 {
		t.Fatalf("defaultHUDScaleStep faithful bottom bar = %d, want 1", got)
	}
}

func TestWalkRenderViewportRectClipsAboveStatusBar(t *testing.T) {
	g := &game{
		opts: Options{
			Width:           1280,
			Height:          720,
			SourcePortMode:  true,
			StatusPatchBank: map[string]WallTexture{"STBAR": {}},
		},
		viewW:        1280,
		viewH:        720,
		screenBlocks: doomScreenBlocksDefault,
	}
	rect := g.walkRenderViewportRect()
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		t.Fatalf("walkRenderViewportRect = %v, want positive size", rect)
	}
	if rect.Min.X != (g.viewW-rect.Dx())/2 {
		t.Fatalf("walkRenderViewportRect minX = %d, want centered", rect.Min.X)
	}
	if rect.Max.Y > g.walkStatusBarTop() {
		t.Fatalf("walkRenderViewportRect maxY = %d, want <= %d", rect.Max.Y, g.walkStatusBarTop())
	}
	if rect.Min.X != 0 || rect.Min.Y != 0 {
		t.Fatalf("walkRenderViewportRect origin = %v, want top-left", rect.Min)
	}
	if rect.Dx() != g.viewW || rect.Dy() != g.walkStatusBarTop() {
		t.Fatalf("walkRenderViewportRect = %v, want full draw area %dx%d", rect, g.viewW, g.walkStatusBarTop())
	}
	g.screenBlocks = doomScreenBlocksOverlay
	if got := g.walkRenderViewportRect(); got.Dx() != g.viewW || got.Dy() != g.viewH {
		t.Fatalf("walkRenderViewportRect overlay = %v, want full %dx%d", got, g.viewW, g.viewH)
	}
	g.screenBlocks = doomScreenBlocksFull
	if got := g.walkRenderViewportRect(); got.Dx() != g.viewW || got.Dy() != g.viewH {
		t.Fatalf("walkRenderViewportRect full = %v, want full %dx%d", got, g.viewW, g.viewH)
	}
}

func TestWalkWeaponViewportRectIgnoresHUDScale(t *testing.T) {
	g := &game{
		opts: Options{
			Width:           1280,
			Height:          720,
			SourcePortMode:  true,
			StatusPatchBank: map[string]WallTexture{"STBAR": {}},
		},
		viewW:           1280,
		viewH:           720,
		screenBlocks:    doomScreenBlocksDefault,
		hudLogicalLayout: true,
		hudScaleStep:    0,
	}
	base := g.walkWeaponViewportRect()
	g.hudScaleStep = len(sourcePortHUDScaleSteps) - 1
	got := g.walkWeaponViewportRect()
	if got != base {
		t.Fatalf("walkWeaponViewportRect changed with hud scale: base=%v got=%v", base, got)
	}
}
