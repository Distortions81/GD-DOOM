//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	"testing"
)

func TestNativeAutomapInputAndDrawingUseSameHUDViewport(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	if opts.SourcePortThingRenderMode != "sprites" {
		t.Fatal("native map default differs from main sprites")
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	g := doomruntime.NewNativeMeshGame(m, opts)
	c := doomruntime.NewNativeCampaign(g, opts, nil)
	g.SetMapActive(true)
	for _, blocks := range []int{1, 2} {
		g.SetMenuHUDSettings(true, blocks, 4)
		if blocks == 2 {
			c.CycleDetail()
		} // AUTO after the first sweep returns to 1x.
		for level := range 4 {
			for _, size := range [][2]int{{640, 400}, {1280, 720}, {641, 401}, {1, 1}} {
				w, h := c.SceneSize(size[0], size[1])
				if w != max(1, size[0]/(level+1)) || h != max(1, size[1]/(level+1)) {
					t.Fatal("detail sweep selected the wrong divisor")
				}
				inputHeight := nativeViewHeight(w, h, g.HUDMode())
				snapshot := g.Frame(1)
				drawHeight := nativeViewHeight(w, h, snapshot.HUDMode)
				// Both native source-port modes use a full scene: the default
				// bar overlays it, and the other mode hides the bar entirely.
				want := h
				if inputHeight != want || drawHeight != want {
					t.Fatalf("blocks=%d size=%v detail=%d input=%d draw=%d want=%d", blocks, size, level, inputHeight, drawHeight, want)
				}
				g.MapViewport(w, inputHeight)
				frame := g.MapFrame(w, drawHeight)
				if frame.Width != w || frame.Height != want {
					t.Fatal("map frame changed the input viewport")
				}
			}
			c.CycleDetail()
		}
	}
	if got := nativeViewHeight(640, 400, 0); got != 400-raymesh.HUDHeight(640, 400) {
		t.Fatalf("bottom HUD viewport=%d", got)
	}
}
