package doomruntime

import (
	"testing"

	"gddoom/internal/gameplay"
	"gddoom/internal/platformcfg"
	"github.com/hajimehoshi/ebiten/v2"
)

type layoutCountRuntime struct {
	layoutCalls int
	viewW       int
	viewH       int
	skyOutputW  int
	skyOutputH  int
}

func (r *layoutCountRuntime) SampleInput()       {}
func (r *layoutCountRuntime) Update() error      { return nil }
func (r *layoutCountRuntime) Draw(*ebiten.Image) {}
func (r *layoutCountRuntime) Layout(w, h int) (int, int) {
	r.layoutCalls++
	r.viewW = w
	r.viewH = h
	return w, h
}
func (r *layoutCountRuntime) sessionSignals() gameplay.SessionSignals {
	return gameplay.SessionSignals{}
}
func (r *layoutCountRuntime) clearPendingSoundState() {}
func (r *layoutCountRuntime) clearSpritePatchCache()  {}
func (r *layoutCountRuntime) initSkyLayerShader()     {}
func (r *layoutCountRuntime) setSkyOutputSize(w, h int) {
	r.skyOutputW = w
	r.skyOutputH = h
}
func (r *layoutCountRuntime) sessionAcknowledgeSaveGame()       {}
func (r *layoutCountRuntime) sessionAcknowledgeLoadGame()       {}
func (r *layoutCountRuntime) sessionSetQuitPromptActive(bool)   {}
func (r *layoutCountRuntime) sessionSetFrontendActive(bool)     {}
func (r *layoutCountRuntime) sessionAcknowledgeNewGameRequest() {}
func (r *layoutCountRuntime) sessionAcknowledgeQuitPrompt()     {}
func (r *layoutCountRuntime) sessionAcknowledgeReadThis()       {}
func (r *layoutCountRuntime) sessionAcknowledgeLevelRestart()   {}
func (r *layoutCountRuntime) sessionAcknowledgeMusicPlayer()    {}
func (r *layoutCountRuntime) sessionAcknowledgeFrontendMenu()   {}
func (r *layoutCountRuntime) sessionAcknowledgeSoundMenu()      {}
func (r *layoutCountRuntime) sessionToggleHUDMessages() bool    { return false }
func (r *layoutCountRuntime) sessionTogglePerfOverlay() bool    { return false }
func (r *layoutCountRuntime) sessionCycleDetail() int           { return 0 }
func (r *layoutCountRuntime) sessionMouseLookSpeed() float64    { return 0 }
func (r *layoutCountRuntime) sessionSetMouseLookSpeed(float64)  {}
func (r *layoutCountRuntime) sessionMusicVolume() float64       { return 0 }
func (r *layoutCountRuntime) sessionSetMusicVolume(float64)     {}
func (r *layoutCountRuntime) sessionSFXVolume() float64         { return 0 }
func (r *layoutCountRuntime) sessionSetSFXVolume(float64)       {}
func (r *layoutCountRuntime) sessionPublishRuntimeSettings()    {}
func (r *layoutCountRuntime) sessionDrawHUTextAt(*ebiten.Image, string, float64, float64, float64, float64) {
}
func (r *layoutCountRuntime) sessionPlaySoundEvent(soundEvent) {}
func (r *layoutCountRuntime) sessionTickSound()                {}

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

func TestClampSourcePortGameSizeForWASMLeavesSizeUnchanged(t *testing.T) {
	w, h := clampSourcePortGameSizeForPlatform(2560, 1440, true)
	if w != 2560 || h != 1440 {
		t.Fatalf("game=%dx%d want 2560x1440", w, h)
	}
}

func TestClampSourcePortGameSizeForNativeLeavesSizeUnchanged(t *testing.T) {
	w, h := clampSourcePortGameSizeForPlatform(2560, 1440, false)
	if w != 2560 || h != 1440 {
		t.Fatalf("game=%dx%d want 2560x1440", w, h)
	}
}

func TestSourcePortLayoutWASMDoesNotClampLogicalSizeOrRenderView(t *testing.T) {
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
	if layoutW != 2560 || layoutH != 1440 {
		t.Fatalf("layout=%dx%d want 2560x1440", layoutW, layoutH)
	}
	if sg.g.viewW != 2560 || sg.g.viewH != 1440 {
		t.Fatalf("render view=%dx%d want 2560x1440", sg.g.viewW, sg.g.viewH)
	}
	if sg.g.skyOutputW != 2560 || sg.g.skyOutputH != 1440 {
		t.Fatalf("sky output=%dx%d want 2560x1440", sg.g.skyOutputW, sg.g.skyOutputH)
	}
}

func TestSourcePortLayoutWASMOversizeDoesNotRepeatedlyInvokeRuntimeLayout(t *testing.T) {
	prev := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(prev)

	rt := &layoutCountRuntime{
		viewW:      2560,
		viewH:      1440,
		skyOutputW: 2560,
		skyOutputH: 1440,
	}
	g := &game{
		opts:       Options{SourcePortMode: true},
		viewW:      2560,
		viewH:      1440,
		skyOutputW: 2560,
		skyOutputH: 1440,
	}
	sg := &sessionGame{
		opts: Options{SourcePortMode: true},
		g:    g,
		rt:   rt,
	}

	layoutW, layoutH := sg.Layout(2560, 1440)
	if layoutW != 2560 || layoutH != 1440 {
		t.Fatalf("layout=%dx%d want 2560x1440", layoutW, layoutH)
	}
	if rt.layoutCalls != 0 {
		t.Fatalf("runtime Layout() calls=%d want 0", rt.layoutCalls)
	}
}

func TestCanDrawSourcePortDirect(t *testing.T) {
	dst := ebiten.NewImage(640, 400)
	g := &game{viewW: 640, viewH: 400}
	sg := &sessionGame{
		opts: Options{SourcePortMode: true},
		g:    g,
	}

	if !sg.canDrawSourcePortDirect(dst, g) {
		t.Fatal("canDrawSourcePortDirect()=false want true")
	}
}

func TestCanDrawSourcePortDirectRejectsMismatchedLayoutSize(t *testing.T) {
	dst := ebiten.NewImage(1280, 720)
	g := &game{viewW: 640, viewH: 400}
	sg := &sessionGame{
		opts: Options{SourcePortMode: true},
		g:    g,
	}

	if sg.canDrawSourcePortDirect(dst, g) {
		t.Fatal("canDrawSourcePortDirect()=true want false")
	}
}

func TestCanDrawSourcePortDirectRejectsNonSourcePortMode(t *testing.T) {
	dst := ebiten.NewImage(640, 400)
	g := &game{viewW: 640, viewH: 400}
	sg := &sessionGame{
		opts: Options{},
		g:    g,
	}

	if sg.canDrawSourcePortDirect(dst, g) {
		t.Fatal("canDrawSourcePortDirect()=true want false")
	}
}
