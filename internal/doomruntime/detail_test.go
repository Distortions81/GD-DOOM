package doomruntime

import (
	"testing"

	"gddoom/internal/render/mapview"
)

func TestDetailPresetIndex(t *testing.T) {
	if got := detailPresetIndex(320, 200); got != 0 {
		t.Fatalf("detailPresetIndex(320,200)=%d want=0", got)
	}
	if got := detailPresetIndex(640, 400); got != 2 {
		t.Fatalf("detailPresetIndex(640,400)=%d want=2", got)
	}
}

func TestDefaultDetailLevelForModeSourcePortHalfDetail(t *testing.T) {
	if got := defaultDetailLevelForMode(1280, 800, true); got != 1 {
		t.Fatalf("defaultDetailLevelForMode(sourceport)=%d want=1", got)
	}
}

func TestDefaultDetailLevelForModeFaithfulStartsHigh(t *testing.T) {
	if got := defaultDetailLevelForMode(640, 400, false); got != 0 {
		t.Fatalf("defaultDetailLevelForMode(faithful)=%d want=0", got)
	}
}

func TestClampSourcePortDetailLevelForWASMSkipsFullRes(t *testing.T) {
	if got := clampSourcePortDetailLevelForPlatform(0, true); got != 1 {
		t.Fatalf("wasm sourceport detail=%d want=1", got)
	}
	if got := clampSourcePortDetailLevelForPlatform(2, true); got != 2 {
		t.Fatalf("wasm sourceport detail=%d want=2", got)
	}
}

func TestClampSourcePortDetailLevelForNativePreservesFullRes(t *testing.T) {
	if got := clampSourcePortDetailLevelForPlatform(0, false); got != 0 {
		t.Fatalf("native sourceport detail=%d want=0", got)
	}
}

func TestCycleDetailLevelFaithfulTogglesHighLow(t *testing.T) {
	g := &game{
		State: mapview.ViewState{
			FitZoom: 1,
			Zoom:    1,
		},
		viewW:       320,
		viewH:       200,
		detailLevel: 0,
		bounds: bounds{
			minX: 0, minY: 0, maxX: 1024, maxY: 1024,
		},
	}
	g.cycleDetailLevel()
	if g.viewW != 320 || g.viewH != 200 || g.detailLevel != 1 {
		t.Fatalf("after 1 cycle got %dx%d level=%d", g.viewW, g.viewH, g.detailLevel)
	}
	g.cycleDetailLevel()
	if g.viewW != 320 || g.viewH != 200 || g.detailLevel != 0 {
		t.Fatalf("after 2 cycles got %dx%d level=%d", g.viewW, g.viewH, g.detailLevel)
	}
}
