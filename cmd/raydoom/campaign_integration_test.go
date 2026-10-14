//go:build raylib && cgo && integration && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"os"
	"runtime"
	"testing"
)

// Exercise an actual use-triggered exit rather than setting a test-only flag.
func nativeCampaignAtExit(t *testing.T, wf *wad.File, opts doomruntime.Options) *doomruntime.NativeCampaign {
	t.Helper()
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	game := doomruntime.NewNativeMeshGame(m, opts)
	for _, line := range m.Linedefs {
		if line.Special != 11 || line.SideNum[0] < 0 {
			continue
		}
		a, b := m.Vertexes[line.V1], m.Vertexes[line.V2]
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		length := math.Hypot(dx, dy)
		if length == 0 {
			continue
		}
		front := m.Sectors[m.Sidedefs[line.SideNum[0]].Sector]
		for _, distance := range []float64{24, 40} {
			x, y := (float64(a.X)+float64(b.X))/2+dy/length*distance, (float64(a.Y)+float64(b.Y))/2-dx/length*distance
			if err := game.SetPose(x, y, float64(front.FloorHeight)+41, math.Atan2(dx, -dy)); err != nil {
				t.Fatal(err)
			}
			game.Tick(doomruntime.NativeMeshInput{})
			game.Tick(doomruntime.NativeMeshInput{Use: true})
			if game.Frame(1).Exited {
				return doomruntime.NewNativeCampaign(game, opts, func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
					name, err := mapdata.NextMapName(wf, current, secret)
					if err != nil {
						return nil, "", err
					}
					m, err := mapdata.LoadMap(wf, name)
					return m, name, err
				})
			}
		}
	}
	t.Fatal("could not activate E1M1's exit switch")
	return nil
}

func TestNativeCampaignIntermissionGPUAndNextLevel(t *testing.T) {
	if os.Getenv("GD_RAYLIB_CAMPAIGN_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_CAMPAIGN_INTEGRATION=1 for framebuffer checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(1280, 800, "Native campaign regression")
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
	c := nativeCampaignAtExit(t, wf, opts)
	if err := c.Tick(doomruntime.NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if c.Phase() != doomruntime.NativeCampaignIntermission {
		t.Fatal("real exit did not enter intermission")
	}
	for i := 0; i < 35; i++ {
		if err := c.Tick(doomruntime.NativeMeshInput{}, i == 20); err != nil {
			t.Fatal(err)
		}
	}
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	draw := func(name string) {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		p.DrawUI(c.Patches(), 1280, 800)
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		pixels := rl.LoadImageColors(img)
		nonblack := 0
		for _, c := range pixels {
			if c.R != 0 || c.G != 0 || c.B != 0 {
				nonblack++
			}
		}
		rl.UnloadImageColors(pixels)
		rl.UnloadImage(img)
		if err := captureScreen("../../build/raylib-captures/" + name + ".png"); err != nil {
			t.Fatal(err)
		}
		rl.EndDrawing()
		if nonblack < 200000 {
			t.Fatalf("intermission art absent from GPU framebuffer: nonblack=%d", nonblack)
		}
	}
	draw("native-intermission-stats")
	for i := 0; i < 1000 && c.Phase() != doomruntime.NativeCampaignPlaying; i++ {
		if err := c.Tick(doomruntime.NativeMeshInput{}, i == 0); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			draw("native-intermission-route")
		}
	}
	if c.Map().Name != "E1M2" || c.Phase() != doomruntime.NativeCampaignPlaying {
		t.Fatal("intermission did not load next level")
	}
	// The shared carryover path clears psprites; gameplay raises the weapon
	// on the next tics, exactly as the main host does.
	for range 20 {
		if err := c.Tick(doomruntime.NativeMeshInput{}, false); err != nil {
			t.Fatal(err)
		}
	}
	frame := c.Game.Frame(1)
	if len(frame.Triangles) == 0 || len(frame.HUD) == 0 || len(frame.WeaponPatches) == 0 {
		t.Fatal("next level did not rebuild geometry, HUD and weapon")
	}
}
