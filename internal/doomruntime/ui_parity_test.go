package doomruntime

import (
	"strings"
	"testing"
	"time"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview"
	"gddoom/internal/render/mapview/presenter"
)

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
		State: mapview.ViewState{
			CamX:       100,
			CamY:       200,
			Zoom:       3,
			FollowMode: true,
			FitZoom:    0.75,
		},
		bounds: bounds{
			minX: -1000, maxX: 1000,
			minY: -500, maxY: 500,
		},
	}
	g.toggleBigMap()
	if !g.bigMap.Enabled() {
		t.Fatalf("bigMap should be enabled after first toggle")
	}
	if g.State.Snapshot().FollowEnabled() {
		t.Fatalf("follow mode should be disabled in big-map")
	}
	g.toggleBigMap()
	if g.bigMap.Enabled() {
		t.Fatalf("bigMap should be disabled after second toggle")
	}
	view := g.State.Snapshot()
	camX, camY := view.Camera()
	if camX != 100 || camY != 200 || view.ZoomLevel() != 3 || !view.FollowEnabled() {
		t.Fatalf("restored view mismatch: cam=(%v,%v) zoom=%v follow=%t", camX, camY, view.ZoomLevel(), view.FollowEnabled())
	}
}

func TestSourcePortDefaultsEnableLegend(t *testing.T) {
	g := newGame(&mapdata.Map{}, Options{SourcePortMode: true})
	if !g.showLegend {
		t.Fatal("sourceport default should enable legend")
	}
	if !presenter.ShouldDrawThings(2) {
		t.Fatal("presenter iddt policy should still draw things at level 2")
	}
	if g.opts.SourcePortThingRenderMode != "items" {
		t.Fatalf("sourceport default thing render mode=%q want items", g.opts.SourcePortThingRenderMode)
	}
}

func TestSourcePortThingRenderModeCycle(t *testing.T) {
	if got := cycleSourcePortThingRenderMode("glyphs"); got != "items" {
		t.Fatalf("cycle glyphs=%q want items", got)
	}
	if got := cycleSourcePortThingRenderMode("items"); got != "sprites" {
		t.Fatalf("cycle items=%q want sprites", got)
	}
	if got := cycleSourcePortThingRenderMode("sprites"); got != "glyphs" {
		t.Fatalf("cycle sprites=%q want glyphs", got)
	}
}

func TestShouldDrawMapThingSpriteHonorsMode(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true, SourcePortThingRenderMode: "glyphs"}}
	if g.shouldDrawMapThingSprite(mapdata.Thing{Type: 2011}) {
		t.Fatal("glyph mode should not draw item sprites")
	}
	g.opts.SourcePortThingRenderMode = "items"
	if !g.shouldDrawMapThingSprite(mapdata.Thing{Type: 2011}) {
		t.Fatal("items mode should draw pickup sprites")
	}
	if g.shouldDrawMapThingSprite(mapdata.Thing{Type: 3004}) {
		t.Fatal("items mode should not draw monster sprites")
	}
	g.opts.SourcePortThingRenderMode = "sprites"
	if !g.shouldDrawMapThingSprite(mapdata.Thing{Type: 3004}) {
		t.Fatal("sprites mode should draw monster sprites")
	}
}

func TestMapThingSpriteName_UsesMonsterSpritePath(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"TROOA1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}
	g.p.x = -100 * fracUnit
	g.p.y = 0
	if got := g.mapThingSpriteName(0, mapdata.Thing{Type: 3001, X: 0, Y: 0, Angle: 0}); got != "TROOA1" {
		t.Fatalf("monster map sprite=%q want TROOA1", got)
	}
}

func TestMapThingSpriteName_PlayerStartUsesPlayerSprite(t *testing.T) {
	g := &game{}
	if got := g.mapThingSpriteName(0, mapdata.Thing{Type: 1}); got != "PLAYN0" {
		t.Fatalf("player start map sprite=%q want PLAYN0", got)
	}
}

func TestMapThingSpriteName_WorldThingBlendFramesCanBeDisabled(t *testing.T) {
	g := &game{
		worldTic:                   2,
		textureAnimCrossfadeFrames: 2,
		opts: Options{
			SourcePortMode:             true,
			SourcePortThingBlendFrames: false,
			SpritePatchBank: map[string]WallTexture{
				"SMGTA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"SMGTB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"SMGTC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"SMGTD0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}
	if got := g.mapThingSpriteName(0, mapdata.Thing{Type: 56}); got != "SMGTA0" {
		t.Fatalf("blend disabled map sprite=%q want SMGTA0", got)
	}
	g.opts.SourcePortThingBlendFrames = true
	g.simTickScale = 1.0
	g.lastUpdate = time.Now().Add(-time.Second / (2 * doomTicsPerSecond))
	if got := g.mapThingSpriteName(0, mapdata.Thing{Type: 56}); !strings.Contains(got, ">") {
		t.Fatalf("blend enabled map sprite=%q want blend token", got)
	}
}

func TestMapRotationActive_DisabledWhenFollowIsOff(t *testing.T) {
	g := &game{
		State:      mapview.ViewState{FollowMode: true},
		mode:       viewMap,
		rotateView: true,
	}
	if !g.mapRotationActive() {
		t.Fatal("follow-on map rotation should be active")
	}
	g.State.SetFollowMode(false)
	if g.mapRotationActive() {
		t.Fatal("follow-off map rotation should be disabled")
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
