//go:build raylib && cgo && integration && !js

package main

import (
	"math"
	"os"
	"runtime"
	"testing"

	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeSpectreFuzzWithDoomAssets(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for real-level fuzz checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(640, 400, "Doom spectre regression")
	defer rl.CloseWindow()
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M5")
	if err != nil {
		t.Fatal(err)
	}
	g := doomruntime.NewNativeMeshGame(m, opts)
	for range 35 {
		g.Tick(doomruntime.NativeMeshInput{})
	}
	r, err := raymesh.NewRendererWithTextureOptions(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	r.SetLightRamp(g.LightRamp())
	p.Sprites.SetLightRamp(g.LightRamp())
	if err := r.SetLightingMode(raymesh.DoomLighting); err != nil {
		t.Fatal(err)
	}
	if err := p.Sprites.SetLightingMode(raymesh.DoomLighting); err != nil {
		t.Fatal(err)
	}
	const w, h = 640, 400
	viewH := h - raymesh.HUDHeight(w, h)
	detail := &raymesh.Detail{}
	defer detail.Close()
	draw := func(withSpectres bool, divisor int) []rl.Color {
		sceneW, sceneH := w/divisor, h/divisor
		sceneViewH := sceneH - raymesh.HUDHeight(sceneW, sceneH)
		p.SetSceneSize(sceneW, sceneH)
		f := g.Frame(1)
		sprites := f.Sprites
		if !withSpectres {
			sprites = make([]levelmesh.Sprite, 0, len(f.Sprites))
			for _, s := range f.Sprites {
				if !s.Shadow {
					sprites = append(sprites, s)
				}
			}
		}
		r.Sync(f.Triangles, g.Texture, g.Light, levelmesh.Textured)
		r.SetGammaTable(g.GammaTable())
		p.Sprites.SetGammaTable(g.GammaTable())
		p.SyncSpritesViewport(sprites, f.Camera, sceneW, sceneViewH)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Blue)
		p.DrawSky(f.Sky, f.Camera, sceneW, sceneViewH)
		rl.DrawRenderBatchActive()
		rl.Viewport(0, int32(h-sceneViewH), int32(sceneW), int32(sceneViewH))
		r.Draw(f.Camera, sceneW, sceneViewH, levelmesh.Textured)
		if err := p.DrawSprites(f.Camera, sceneW, sceneViewH, func(index, width, height int) ([]levelmesh.FuzzSpan, levelmesh.FuzzColors) {
			return g.SpectreFuzz(sprites[index], f.Camera, width, height), g.SpectreFuzzColors()
		}); err != nil {
			t.Fatal(err)
		}
		rl.Viewport(0, 0, w, h)
		if err := detail.Present(sceneW, sceneH, w, h); err != nil {
			t.Fatal(err)
		}
		p.DrawWeapon(f.WeaponPatches, w, viewH)
		p.DrawHUD(f.HUD, w, h)
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		out := append([]rl.Color(nil), colors...)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		if withSpectres {
			if err := captureScreen("../../build/raylib-captures/native-spectre-fuzz.png"); err != nil {
				t.Fatal(err)
			}
		}
		rl.EndDrawing()
		return out
	}
	// Find an unobstructed inspection pose near a real spectre. Walls still
	// clip the card, and the filtered WAD scenery supplies the sampled colors.
	for _, s := range g.Frame(1).Sprites {
		if !s.Shadow {
			continue
		}
		for direction := range 8 {
			yaw := float64(direction) * math.Pi / 4
			if err := g.SetPose(s.X-96*math.Cos(yaw), s.Y-96*math.Sin(yaw), s.Z+41, yaw); err != nil {
				t.Fatal(err)
			}
			base, fuzz := draw(false, 1), draw(true, 1)
			diff := 0
			for i := range base[:w*viewH] {
				if base[i] != fuzz[i] {
					diff++
				}
			}
			for i := w * viewH; i < len(base); i++ {
				if base[i] != fuzz[i] {
					t.Fatal("spectre changed real status bar")
				}
			}
			if diff > 128 {
				t.Logf("real E1M5 fuzz changes %d scene pixels at pose (%g,%g)", diff, s.X-96*math.Cos(yaw), s.Y-96*math.Sin(yaw))
				for _, divisor := range []int{2, 3, 4} {
					base, fuzz := draw(false, divisor), draw(true, divisor)
					changed := 0
					for i := range base {
						if base[i] != fuzz[i] {
							if i >= w*viewH {
								t.Fatalf("detail divisor %d fuzz changed HUD", divisor)
							}
							changed++
						}
					}
					if changed < 128 {
						t.Fatalf("detail divisor %d lost spectre: %d pixels", divisor, changed)
					}
				}
				return
			}
		}
	}
	t.Fatal("no visible real spectre changed the world framebuffer")
}
