package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/platformcfg"
)

func TestMouseLookSensitivitySurvivesPresentationChanges(t *testing.T) {
	previousWASM := platformcfg.ForcedWASMMode()
	defer platformcfg.SetForcedWASMMode(previousWASM)
	for _, wasm := range []bool{false, true} {
		t.Run(fmt.Sprintf("wasm=%t", wasm), func(t *testing.T) {
			platformcfg.SetForcedWASMMode(wasm)
			for _, invert := range []bool{false, true} {
				t.Run(fmt.Sprintf("invert=%t", invert), func(t *testing.T) {
					g := &game{opts: Options{MouseLook: true, MouseLookSpeed: 0.75, MouseInvert: invert}}
					sg := &sessionGame{g: g, rt: &layoutCountRuntime{}}
					// Exercise resizing, mode switching, aspect correction and render
					// detail on the same input sampler. Layout returns window-sized
					// coordinates even when the renderer uses a smaller buffer.
					for _, step := range []struct {
						modern, noAspect      bool
						width, height, detail int
					}{
						{true, false, 1920, 1080, 0},
						{false, false, 1920, 1080, 0},
						{false, true, 1920, 1080, 1},
						{false, false, 1170, 2532, 2},
						{false, false, 320, 200, 0},
						{true, false, 640, 480, 0},
					} {
						sg.opts.SourcePortMode = step.modern
						sg.opts.DisableAspectCorrection = step.noAspect
						g.opts.SourcePortMode = step.modern
						g.detailLevel = step.detail
						w, h := sg.Layout(step.width, step.height)
						if w != step.width || h != step.height {
							t.Fatalf("input layout=%dx%d want %dx%d", w, h, step.width, step.height)
						}
						g.sampleMouseLookPosition(100)
						g.clearSampledInput()
						g.sampleMouseLookPosition(110)
						want := mouseLookTurnRawWithWidth(10, 0.75, step.width, invert)
						if got := g.input.mouseTurnRawAccum; got != want {
							t.Fatalf("modern=%t window=%dx%d detail=%d noAspect=%t turn=%d want=%d", step.modern, w, h, step.detail, step.noAspect, got, want)
						}
					}
				})
			}
		})
	}
}

func TestMouseLookTurnRawWithWidthIgnoresResolution(t *testing.T) {
	base := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW, false)
	if base >= 0 {
		t.Fatalf("base turn=%d want negative for +dx", base)
	}
	doubleW := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW*2, false)
	if doubleW >= 0 {
		t.Fatalf("double-width turn=%d want negative for +dx", doubleW)
	}
	halfW := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW/2, false)
	if halfW >= 0 {
		t.Fatalf("half-width turn=%d want negative for +dx", halfW)
	}
	if doubleW != base {
		t.Fatalf("double-width turn=%d want=%d", doubleW, base)
	}
	if halfW != base {
		t.Fatalf("half-width turn=%d want=%d", halfW, base)
	}
}

func TestMouseLookTurnRawWithWidthPreservesDirectionAndMinimumStep(t *testing.T) {
	if got := mouseLookTurnRawWithWidth(0, 1.0, doomLogicalW, false); got != 0 {
		t.Fatalf("dx=0 got=%d want=0", got)
	}
	if got := mouseLookTurnRawWithWidth(1, 0.0000001, doomLogicalW, false); got != -1 {
		t.Fatalf("+tiny dx got=%d want=-1", got)
	}
	if got := mouseLookTurnRawWithWidth(-1, 0.0000001, doomLogicalW, false); got != 1 {
		t.Fatalf("-tiny dx got=%d want=+1", got)
	}
}

func TestMouseLookTurnRawWithWidthSupportsHorizontalInvert(t *testing.T) {
	if got := mouseLookTurnRawWithWidth(4, 1.0, doomLogicalW, true); got <= 0 {
		t.Fatalf("inverted +dx got=%d want positive", got)
	}
	if got := mouseLookTurnRawWithWidth(-4, 1.0, doomLogicalW, true); got >= 0 {
		t.Fatalf("inverted -dx got=%d want negative", got)
	}
}

func TestMouseLookTurnRawScaledTracksPresentationScale(t *testing.T) {
	base := mouseLookTurnRawScaled(10, 1.0, 1.0, false)
	scaled := mouseLookTurnRawScaled(10, 1.0, 6.0, false)
	if scaled != base*6 {
		t.Fatalf("scaled turn=%d want=%d", scaled, base*6)
	}
}
