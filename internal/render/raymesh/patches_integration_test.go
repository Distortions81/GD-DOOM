//go:build raylib && cgo && integration && !js

package raymesh

import (
	"os"
	"runtime"
	"slices"
	"testing"

	"gddoom/internal/render/levelmesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestRaylibPatchEdgesDoNotWrapWithMSAA(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual GPU checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(128, 128, "Weapon patch edge regression")
	defer rl.CloseWindow()
	p, err := NewPresentation(TextureOptions{Scale: 2, Filter: Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	// A different color on each edge makes wrapping to the opposite edge
	// observable. Fractional weapon bob positions put pixel centers outside
	// the card even when some of their MSAA samples are covered.
	tex := levelmesh.Texture{Width: 4, Height: 4, RGBA: make([]byte, 4*4*4)}
	for y := range 4 {
		for x := range 4 {
			i := (y*4 + x) * 4
			tex.RGBA[i+2], tex.RGBA[i+3] = 255, 255
			if y == 3 {
				tex.RGBA[i], tex.RGBA[i+1], tex.RGBA[i+2] = 255, 255, 0
			}
			if x == 0 {
				tex.RGBA[i], tex.RGBA[i+1], tex.RGBA[i+2] = 255, 0, 0
			} else if x == 3 {
				tex.RGBA[i], tex.RGBA[i+1], tex.RGBA[i+2] = 0, 255, 0
			}
		}
	}
	original := slices.Clone(tex.RGBA)
	patches := []levelmesh.Patch{{Texture: tex, X: 32.75, Y: 32.75, W: 31.5, H: 31.5}}
	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	p.drawPatches(patches, 0, 0, 1, 1)
	img, err := CaptureWindow()
	if err != nil {
		t.Fatal(err)
	}
	rl.EndDrawing()
	defer rl.UnloadImage(img)
	colors := rl.LoadImageColors(img)
	defer rl.UnloadImageColors(colors)
	for y := 40; y < 58; y++ {
		for x := 32; x < 37; x++ {
			if c := colors[y*128+x]; c.G != 0 {
				t.Fatalf("right-edge green bled onto left at %d,%d: %v", x, y, c)
			}
		}
		for x := 61; x <= 64; x++ {
			if c := colors[y*128+x]; c.R != 0 {
				t.Fatalf("left-edge red bled onto right at %d,%d: %v", x, y, c)
			}
		}
	}
	for x := 42; x < 56; x++ {
		for y := 32; y < 37; y++ {
			if c := colors[y*128+x]; c.R != 0 || c.G != 0 {
				t.Fatalf("bottom-edge yellow bled onto top at %d,%d: %v", x, y, c)
			}
		}
		for y := 61; y <= 64; y++ {
			if c := colors[y*128+x]; c.B != 0 {
				t.Fatalf("top-edge blue bled onto bottom at %d,%d: %v", x, y, c)
			}
		}
	}
	if c := colors[48*128+35]; c != (rl.Color{R: 255, A: 255}) {
		t.Fatalf("left interior lost its original pixel: %v", c)
	}
	if c := colors[48*128+61]; c != (rl.Color{G: 255, A: 255}) {
		t.Fatalf("right interior lost its original pixel: %v", c)
	}
	if !slices.Equal(tex.RGBA, original) {
		t.Fatal("patch upload modified original WAD pixels")
	}
	// A repeating sky must never reuse the padded/clamped patch upload, even
	// if both callers happen to share exactly the same CPU image.
	patch, _ := p.texture(tex, true)
	sky, _ := p.texture(tex, false)
	if patch.ID == sky.ID || patch.Width != 6 || patch.Height != 6 || sky.Width != 4 || sky.Height != 4 {
		t.Fatal("repeating sky and isolated patch shared texture sampling state")
	}
	if again, _ := p.texture(tex, true); again.ID != patch.ID {
		t.Fatal("patch gutter upload was not cached")
	}
}
