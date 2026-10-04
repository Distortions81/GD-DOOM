//go:build raylib && cgo && integration && !js

package raymesh

import (
	"os"
	"runtime"
	"slices"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestRaylibReducedDetailResolvesMSAAAndRetainsSharpOverlays(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(128, 96, "Reduced detail")
	if !rl.IsWindowReady() {
		t.Fatal("window unavailable")
	}
	defer rl.CloseWindow()
	d := &Detail{}
	defer d.Close()
	c := &CRT{}
	defer c.Close()
	pixels := func() []rl.Color {
		t.Helper()
		img, err := CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		out := slices.Clone(colors)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		return out
	}
	for _, size := range [][2]int{{128, 96}, {32, 24}, {32, 24}, {64, 48}, {32, 24}} {
		w, h := size[0], size[1]
		for _, crt := range []bool{false, true} {
			rl.BeginDrawing()
			rl.ClearBackground(rl.Purple)
			// Single-pixel stripes exercise nearest sampling, orientation and
			// MSAA resolving without depending on a world texture filter.
			for y := range h {
				for x := range w {
					color := rl.NewColor(uint8(x*7), uint8(y*9), uint8((x+y)*3), 255)
					rl.DrawRectangle(int32(x), int32(y), 1, 1, color)
				}
			}
			// Cutout sprite edges and translucent map markers must be copied
			// once, without blending against the original window a second time.
			rl.DrawRectangle(0, 0, int32(w/2), int32(h/2), rl.NewColor(180, 90, 210, 100))
			if crt {
				if err := c.ApplyRegion(123, w, h); err != nil {
					t.Fatal(err)
				}
			}
			before := pixels()
			oldID := d.scene.Texture.ID
			oldW, oldH := d.scene.Texture.Width, d.scene.Texture.Height
			if err := d.Present(w, h, 128, 96); err != nil {
				t.Fatal(err)
			}
			if w == 128 && d.scene.ID != 0 {
				t.Fatal("full resolution allocated detail cache")
			}
			if oldID != 0 && oldW == int32(w) && oldH == int32(h) && d.scene.Texture.ID != oldID {
				t.Fatal("same-size frame replaced resident detail texture")
			}
			rl.DrawRectangle(123, 91, 3, 3, rl.Magenta)
			after := pixels()
			rl.EndDrawing()
			for y := range 96 {
				for x := range 128 {
					want := before[(y*h/96)*128+x*w/128]
					if x >= 123 && x < 126 && y >= 91 && y < 94 {
						want = rl.Magenta
					}
					if after[y*128+x] != want {
						t.Fatalf("%dx%d CRT=%t pixel %d,%d=%v want %v", w, h, crt, x, y, after[y*128+x], want)
					}
				}
			}
		}
	}
	d.Close()
	if d.scene.ID != 0 {
		t.Fatal("close retained detail scene")
	}
}
