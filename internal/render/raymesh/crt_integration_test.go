//go:build raylib && cgo && integration && !js

package raymesh

import (
	"os"
	"runtime"
	"slices"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestRaylibCRTRetainsSnapshotResizesAndLeavesOverlaysSharp(t *testing.T) {
	if os.Getenv("GD_RAYLIB_CRT_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_CRT_INTEGRATION=1")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(128, 128, "CRT retention")
	defer rl.CloseWindow()
	c := &CRT{}
	defer c.Close()
	if c.shader.ID != 0 || c.scene.ID != 0 {
		t.Fatal("disabled CRT allocated GPU resources")
	}
	draw := func(tic int, enabled bool) []rl.Color {
		rl.BeginDrawing()
		rl.ClearBackground(rl.White)
		if enabled {
			if err := c.Apply(tic); err != nil {
				t.Fatal(err)
			}
		}
		// A weapon/HUD-style opaque patch must bypass the effect and its warp.
		rl.DrawRectangle(0, 0, 16, 16, rl.Magenta)
		img, err := CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		out := slices.Clone(colors)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		rl.EndDrawing()
		for y := range 16 {
			for x := range 16 {
				if out[y*int(rl.GetRenderWidth())+x] != rl.Magenta {
					t.Fatal("CRT filtered a later overlay")
				}
			}
		}
		return out
	}
	baseline := draw(0, false)
	if c.shader.ID != 0 || c.scene.ID != 0 {
		t.Fatal("disabled path allocated a snapshot")
	}
	first := draw(123, true)
	shader, texture := c.shader.ID, c.scene.Texture.ID
	if slices.Equal(first, baseline) {
		t.Fatal("CRT did not process the scene")
	}
	for range 8 {
		if !slices.Equal(draw(123, true), first) {
			t.Fatal("paused CRT animation did not freeze")
		}
		if c.shader.ID != shader || c.scene.Texture.ID != texture {
			t.Fatal("world time replaced GPU resources")
		}
	}
	if slices.Equal(draw(124, true), first) {
		t.Fatal("CRT scanlines did not advance with world time")
	}
	if !slices.Equal(draw(124, false), baseline) || c.scene.Texture.ID != texture {
		t.Fatal("disabling CRT changed the unprocessed scene or discarded its cache")
	}
	rl.SetWindowSize(256, 192)
	for range 2 {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		rl.EndDrawing()
	}
	draw(124, true)
	if c.scene.Texture.Width != 256 || c.scene.Texture.Height != 192 || c.shader.ID != shader {
		t.Fatal("CRT resize did not retain the shader and update scene dimensions")
	}
	texture = c.scene.Texture.ID
	draw(124, true)
	if c.scene.Texture.ID != texture {
		t.Fatal("resized snapshot was not retained")
	}
	c.Close()
	if c.scene.ID != 0 || c.shader.ID != 0 {
		t.Fatal("close retained GPU resources")
	}
}
