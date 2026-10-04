//go:build raylib && cgo && integration && !js

package main

import (
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

func TestNativeInvulnerabilityGPU(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for framebuffer checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(640, 400, "Native invulnerability regression")
	defer rl.CloseWindow()
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	opts.NoMonsters = true
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	g := doomruntime.NewNativeMeshGame(m, opts)
	for range 35 {
		g.Tick(doomruntime.NativeMeshInput{})
	} // Finish raising the starting weapon.
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
	if err := r.SetLightingMode(raymesh.DoomLighting); err != nil {
		t.Fatal(err)
	}
	draw := func(name string) (gray, colored int) {
		f := g.Frame(1)
		r.Sync(f.Triangles, g.Texture, g.Light, levelmesh.Textured)
		r.SetFixedColormap(f.FixedColormap)
		r.SetGammaTable(g.GammaTable())
		p.SyncSprites(f.Sprites, f.Camera)
		p.Sprites.SetFixedColormap(f.FixedColormap)
		p.Sprites.SetGammaTable(g.GammaTable())
		const w, h = 640, 400
		viewH := h - raymesh.HUDHeight(w, h)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Blue)
		p.DrawSky(f.Sky, f.Camera, w, viewH)
		rl.DrawRenderBatchActive()
		rl.Viewport(0, int32(h-viewH), w, int32(viewH))
		r.Draw(f.Camera, w, viewH, levelmesh.Textured)
		p.Sprites.Draw(f.Camera, w, viewH, levelmesh.Textured)
		rl.Viewport(0, 0, w, h)
		p.DrawWeapon(f.WeaponPatches, w, viewH)
		p.DrawHUD(f.HUD, w, h)
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		// Inspect only the walls in the upper part of the enclosed starting
		// room, avoiding unchanged weapon/HUD artwork and edge coverage.
		for y := 40; y < 200; y++ {
			for x := 80; x < 560; x++ {
				c := colors[y*w+x]
				if c.R == c.G && c.G == c.B {
					gray++
				} else {
					colored++
				}
			}
		}
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		if err := captureScreen("../../build/raylib-captures/" + name + ".png"); err != nil {
			t.Fatal(err)
		}
		rl.EndDrawing()
		return
	}
	baseGray, baseColor := draw("native-invulnerability-off")
	g.TypeCheats([]rune("idbeholdv"))
	if !g.Frame(1).FixedColormap {
		t.Fatal("IDBEHOLDV did not enable invulnerability palette")
	}
	gray, colored := draw("native-invulnerability-on")
	if gray < 70000 || colored >= baseColor || gray <= baseGray {
		t.Fatalf("inverse world palette absent: normal=%d/%d inverse=%d/%d", baseGray, baseColor, gray, colored)
	}
	if r.Stats().MeshUploads != 0 || r.Stats().BufferUpdates != 0 {
		t.Fatal("invulnerability rebuilt real level geometry")
	}
	g.TypeCheats([]rune("idbeholdv"))
	if g.Frame(1).FixedColormap {
		t.Fatal("powerup toggle failed to restore normal palette")
	}
	gray, colored = draw("native-invulnerability-restored")
	if gray != baseGray || colored != baseColor {
		t.Fatal("normal world failed to restore after powerup")
	}
}
