package doomruntime

import (
	"testing"

	"gddoom/internal/platformcfg"
)

func TestDefaultCLIWindowSize(t *testing.T) {
	w, h := DefaultCLIWindowSize()
	if w != doomLogicalW*defaultCLIWindowScale || h != doomLogicalH*defaultCLIWindowScale {
		t.Fatalf("DefaultCLIWindowSize()=%dx%d want %dx%d", w, h, doomLogicalW*defaultCLIWindowScale, doomLogicalH*defaultCLIWindowScale)
	}
}

func TestNormalizeRunDimensionsSourcePortDefaults(t *testing.T) {
	opts := Options{SourcePortMode: true}
	got, ww, wh := normalizeRunDimensions(opts)
	if got.Width != sourcePortDefaultWindowW || got.Height != sourcePortDefaultWindowH {
		t.Fatalf("sourceport normalized render=%dx%d want %dx%d", got.Width, got.Height, sourcePortDefaultWindowW, sourcePortDefaultWindowH)
	}
	if ww != sourcePortDefaultWindowW || wh != sourcePortDefaultWindowH {
		t.Fatalf("sourceport window=%dx%d want %dx%d", ww, wh, sourcePortDefaultWindowW, sourcePortDefaultWindowH)
	}
}

func TestNormalizeRunDimensionsFaithfulFitsToDisplayAspect(t *testing.T) {
	opts := Options{SourcePortMode: false, Width: 1000, Height: 700}
	got, ww, wh := normalizeRunDimensions(opts)
	if got.Width != doomLogicalW || got.Height != doomLogicalH {
		t.Fatalf("faithful normalized render=%dx%d want %dx%d", got.Width, got.Height, doomLogicalW, doomLogicalH)
	}
	if ww != 933 || wh != 700 {
		t.Fatalf("faithful window=%dx%d want 933x700", ww, wh)
	}
}

func TestNormalizeRunDimensionsFaithfulNoAspectCorrection(t *testing.T) {
	opts := Options{
		SourcePortMode:          false,
		DisableAspectCorrection: true,
		Width:                   1000,
		Height:                  700,
	}
	got, ww, wh := normalizeRunDimensions(opts)
	if got.Width != doomLogicalW || got.Height != doomLogicalH {
		t.Fatalf("faithful normalized render=%dx%d want %dx%d", got.Width, got.Height, doomLogicalW, doomLogicalH)
	}
	if ww != 1000 || wh != 625 {
		t.Fatalf("faithful window=%dx%d want 1000x625", ww, wh)
	}
}

func TestEnsurePositiveRenderSize(t *testing.T) {
	opts := Options{SourcePortMode: false}
	ensurePositiveRenderSize(&opts)
	if opts.Width != doomLogicalW || opts.Height != doomLogicalH {
		t.Fatalf("faithful render defaults=%dx%d want %dx%d", opts.Width, opts.Height, doomLogicalW, doomLogicalH)
	}
	opts = Options{SourcePortMode: true}
	ensurePositiveRenderSize(&opts)
	if opts.Width != sourcePortDefaultWindowW || opts.Height != sourcePortDefaultWindowH {
		t.Fatalf("sourceport render defaults=%dx%d want %dx%d", opts.Width, opts.Height, sourcePortDefaultWindowW, sourcePortDefaultWindowH)
	}
}

func TestClampSourcePortLayoutSizeForWASM(t *testing.T) {
	w, h := clampSourcePortLayoutSizeForPlatform(2560, 1440, true)
	if w != 1280 || h != 720 {
		t.Fatalf("layout=%dx%d want 1280x720", w, h)
	}
}

func TestClampSourcePortLayoutSizeForNativeLeavesSizeUnchanged(t *testing.T) {
	w, h := clampSourcePortLayoutSizeForPlatform(2560, 1440, false)
	if w != 2560 || h != 1440 {
		t.Fatalf("layout=%dx%d want 2560x1440", w, h)
	}
}

func TestSourcePortLayoutWASMClampDoesNotClampRenderView(t *testing.T) {
	prev := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(prev)

	g := &game{
		opts:       Options{SourcePortMode: true},
		viewW:      1,
		viewH:      1,
		skyOutputW: 1,
		skyOutputH: 1,
	}
	sg := &sessionGame{
		opts: Options{SourcePortMode: true},
		g:    g,
		rt:   g,
	}

	layoutW, layoutH := sg.Layout(2560, 1440)
	if layoutW != 1280 || layoutH != 720 {
		t.Fatalf("layout=%dx%d want 1280x720", layoutW, layoutH)
	}
	if sg.g.viewW != 2560 || sg.g.viewH != 1440 {
		t.Fatalf("render view=%dx%d want 2560x1440", sg.g.viewW, sg.g.viewH)
	}
	if sg.g.skyOutputW != 1280 || sg.g.skyOutputH != 720 {
		t.Fatalf("sky output=%dx%d want 1280x720", sg.g.skyOutputW, sg.g.skyOutputH)
	}
}
