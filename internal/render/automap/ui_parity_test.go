package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestShouldDrawThings(t *testing.T) {
	if shouldDrawThings(automapParityState{iddt: 1}) {
		t.Fatalf("iddt1 should not draw things")
	}
	if !shouldDrawThings(automapParityState{iddt: 2}) {
		t.Fatalf("iddt2 should draw things")
	}
}

func TestToggledLineColorMode(t *testing.T) {
	if got := toggledLineColorMode("doom"); got != "parity" {
		t.Fatalf("toggle doom => %q, want parity", got)
	}
	if got := toggledLineColorMode("parity"); got != "doom" {
		t.Fatalf("toggle parity => %q, want doom", got)
	}
}

func TestToggleBigMapRoundTrip(t *testing.T) {
	g := &game{
		camX:       100,
		camY:       200,
		zoom:       3,
		followMode: true,
		bounds: bounds{
			minX: -1000, maxX: 1000,
			minY: -500, maxY: 500,
		},
		fitZoom: 0.75,
	}
	g.toggleBigMap()
	if !g.bigMap {
		t.Fatalf("bigMap should be enabled after first toggle")
	}
	if g.followMode {
		t.Fatalf("follow mode should be disabled in big-map")
	}
	g.toggleBigMap()
	if g.bigMap {
		t.Fatalf("bigMap should be disabled after second toggle")
	}
	if g.camX != 100 || g.camY != 200 || g.zoom != 3 || !g.followMode {
		t.Fatalf("restored view mismatch: cam=(%v,%v) zoom=%v follow=%t", g.camX, g.camY, g.zoom, g.followMode)
	}
}

func TestSourcePortDefaultsEnableLegend(t *testing.T) {
	g := newGame(&mapdata.Map{}, Options{SourcePortMode: true})
	if !g.showLegend {
		t.Fatal("sourceport default should enable legend")
	}
	if g.pseudo3D {
		t.Fatal("sourceport default should keep pseudo3d off")
	}
	if g.walkRender != walkRendererDoomBasic {
		t.Fatal("sourceport default should use doom-basic walk renderer")
	}
}

func TestButtonHighlightEligible(t *testing.T) {
	if buttonHighlightEligible(0) {
		t.Fatal("special 0 should not highlight")
	}
	if !buttonHighlightEligible(11) {
		t.Fatal("use-trigger exit should highlight")
	}
	if buttonHighlightEligible(1) {
		t.Fatal("manual door should not highlight")
	}
}
