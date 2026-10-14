//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestModernNativePresentationCapture(t *testing.T) {
	g, cases, out := modernPresentationFixtures(t)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(640, 480, "Modern presentation audit")
	defer rl.CloseWindow()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 1, Filter: raymesh.Nearest})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	n := &NativeMeshGame{g: g}
	for _, c := range cases {
		applyModernPresentationCase(t, g, c)
		if rl.GetRenderWidth() != c.Width || rl.GetRenderHeight() != c.Height {
			rl.SetWindowSize(c.Width, c.Height)
			for range 2 {
				rl.BeginDrawing()
				rl.EndDrawing()
			}
		}
		n.buildPresentation()
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		p.DrawWeapon(n.weapon, c.Width, c.Height)
		capture, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(capture)
		img := image.NewRGBA(image.Rect(0, 0, c.Width, c.Height))
		for i, c := range colors {
			img.SetRGBA(i%img.Rect.Dx(), i/img.Rect.Dx(), color.RGBA{c.R, c.G, c.B, c.A})
		}
		rl.UnloadImageColors(colors)
		rl.UnloadImage(capture)
		rl.EndDrawing()
		file, err := os.Create(filepath.Join(out, c.Name+"-native.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, img)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Ebiten composites premultiplied RGBA. Raylib's normal blend expects straight
// RGBA; interpolated sprite edges must not have their opacity multiplied twice.
func TestNativeModernCrossfadePreservesBrightness(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(320, 200, "Crossfade opacity")
	defer rl.CloseWindow()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 1, Filter: raymesh.Nearest})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	rgba := make([]byte, 256*4)
	for a := 0; a < 256; a++ {
		rgba[a*4], rgba[a*4+1], rgba[a*4+2], rgba[a*4+3] = byte(a), byte(a/2), byte(a/4), byte(a)
	}
	rl.BeginDrawing()
	rl.ClearBackground(rl.NewColor(20, 40, 80, 255))
	p.DrawScreenPatches([]levelmesh.Patch{{Texture: levelmesh.Texture{RGBA: rgba, Width: 256, Height: 1}, W: 256, H: 16}})
	capture, err := raymesh.CaptureWindow()
	if err != nil {
		t.Fatal(err)
	}
	colors := rl.LoadImageColors(capture)
	for a := 0; a < 256; a++ {
		got := colors[8*320+a]
		for channel, value := range []byte{got.R, got.G, got.B} {
			bg := []int{20, 40, 80}[channel]
			want := int(rgba[a*4+channel]) + (bg*(255-a)+127)/255
			delta := int(value) - want
			if delta < -1 || delta > 1 {
				t.Fatalf("alpha %d channel %d: got %d want %d (double-applied opacity)", a, channel, value, want)
			}
		}
	}
	rl.UnloadImageColors(colors)
	rl.UnloadImage(capture)
	rl.EndDrawing()
}
