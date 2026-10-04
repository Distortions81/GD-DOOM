//go:build raylib && cgo && integration && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
	"image/color"
	"os"
	"runtime"
	"testing"
)

func TestNativeAutomapGPUFloorUpdatesAndResize(t *testing.T) {
	if os.Getenv("GD_RAYLIB_MAP_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_MAP_INTEGRATION=1 for framebuffer checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(1280, 800, "Native automap regression")
	defer rl.CloseWindow()
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M3")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	opts.NoMonsters = true
	game := doomruntime.NewNativeMeshGame(m, opts)
	game.SetPose(-600, -1600, 89, 0)
	game.SetMapActive(true)
	game.MapViewport(1280, 640)
	game.TypeCheats([]rune("iddt"))
	game.Tick(doomruntime.NativeMeshInput{Map: &doomruntime.NativeMapInput{ToggleRotate: true, ToggleLegend: true}})
	for range 70 {
		game.Tick(doomruntime.NativeMeshInput{Map: &doomruntime.NativeMapInput{InputState: mapview.InputState{ZoomInHeld: true}}})
	}
	game.Frame(1)
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	a := &nativeAutomap{}
	defer a.Close()
	draw := func(f doomruntime.NativeMapFrame) []color.RGBA {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		a.draw(f, p)
		img, err := raymesh.CaptureWindow()
		rl.EndDrawing()
		if err != nil {
			t.Fatal(err)
		}
		defer rl.UnloadImage(img)
		colors := rl.LoadImageColors(img)
		defer rl.UnloadImageColors(colors)
		return append([]color.RGBA(nil), colors...)
	}
	f := game.MapFrame(1280, 640)
	pixels := draw(f)
	sampled, matched := 0, 0
	for y := 20; y < 620; y += 8 {
		for x := 20; x < 1260; x += 8 {
			i := (y*1280 + x) * 4
			if f.Pixels[i+3] != 255 {
				continue
			}
			sampled++
			c := pixels[y*1280+x]
			if c.R == f.Pixels[i] && c.G == f.Pixels[i+1] && c.B == f.Pixels[i+2] {
				matched++
			}
		}
	}
	if sampled < 500 || matched*100 < sampled*90 {
		t.Fatalf("floor texture did not reach MSAA framebuffer: matched=%d sampled=%d", matched, sampled)
	}
	// Change one unobstructed pixel to prove updates reach the existing GPU
	// texture, then resize to prove its old storage is replaced.
	id := a.texture.ID
	f.Pixels[0], f.Pixels[1], f.Pixels[2], f.Pixels[3] = 23, 211, 77, 255
	pixels = draw(f)
	if a.texture.ID != id || pixels[0] != (color.RGBA{23, 211, 77, 255}) {
		t.Fatalf("dynamic floor upload stale or recreated: pixel=%v", pixels[0])
	}
	f = game.MapFrame(640, 320)
	draw(f)
	if a.texture.Width != 640 || a.texture.Height != 320 {
		t.Fatal("resize retained obsolete texture dimensions")
	}
	// Return to the larger viewport and save the actual exit-room result.
	f = game.MapFrame(1280, 640)
	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	a.draw(f, p)
	if err := captureScreen("../../build/raylib-captures/native-automap-exit-verified.png"); err != nil {
		t.Fatal(err)
	}
	rl.EndDrawing()
}
