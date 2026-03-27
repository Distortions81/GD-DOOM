package doomruntime

import (
	"runtime"
	"testing"

	"gddoom/internal/platformcfg"
)

func TestRendererWorkerCountUsesSingleThreadPathOnWASM(t *testing.T) {
	prev := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(prev)

	g := &game{opts: Options{RendererWorkers: 8}}
	if got := g.rendererWorkerCount(); got != 1 {
		t.Fatalf("rendererWorkerCount()=%d want 1", got)
	}
}

func TestRendererWorkerCountRespectsGOMAXPROCS(t *testing.T) {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	g := &game{opts: Options{RendererWorkers: 8}}
	if got := g.rendererWorkerCount(); got != 1 {
		t.Fatalf("rendererWorkerCount()=%d want 1", got)
	}
}

func TestSourcePortAudioEnabledDisablesSourcePortAudioOnWASM(t *testing.T) {
	prev := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(prev)

	if sourcePortAudioEnabled(Options{SourcePortMode: true}) {
		t.Fatal("sourcePortAudioEnabled(...)=true want false on WASM")
	}
	if sourcePortAudioEnabled(Options{SourcePortMode: false}) {
		t.Fatal("sourcePortAudioEnabled(...)=true want false")
	}
}
