//go:build raylib && cgo && integration && !js

package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeSavePreviewGPUAndRestoredFrame(t *testing.T) {
	if os.Getenv("GD_RAYLIB_SAVE_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_SAVE_INTEGRATION=1 for save/framebuffer checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(1280, 800, "Native save regression")
	defer rl.CloseWindow()
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	opts.NoMonsters, opts.SkillLevel = true, 3
	opts.NewGameLoader = func(name string) (*mapdata.Map, error) { return mapdata.LoadMap(wf, mapdata.MapName(name)) }
	m, err := mapdata.LoadMap(wf, "E1M3")
	if err != nil {
		t.Fatal(err)
	}
	game := doomruntime.NewNativeMeshGame(m, opts)
	if err := game.SetPose(-600, -1600, 89, 0); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		game.Tick(doomruntime.NativeMeshInput{})
	}
	campaign := doomruntime.NewNativeCampaign(game, opts, nil)
	renderer, err := raymesh.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	preview := &nativeSavePreview{}
	defer preview.Close()
	capturePath, err := filepath.Abs("../../build/raylib-captures/native-save-menu.png")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	drawWorld := func() {
		f := campaign.Game.Frame(1)
		renderer.Sync(f.Triangles, campaign.Game.Texture, campaign.Game.Light, levelmesh.Textured)
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(42, 49, 65, 255))
		viewH := 800 - raymesh.HUDHeight(1280, 800)
		p.DrawSky(f.Sky, f.Camera, 1280, viewH)
		rl.DrawRenderBatchActive()
		rl.Viewport(0, int32(800-viewH), 1280, int32(viewH))
		renderer.Draw(f.Camera, 1280, viewH, levelmesh.Textured)
		rl.Viewport(0, 0, 1280, 800)
		p.DrawWeapon(f.WeaponPatches, 1280, viewH)
		p.DrawHUD(f.HUD, 1280, 800)
	}
	if err := campaign.SaveSlot(2); err != nil {
		t.Fatal(err)
	}
	drawWorld()
	if err := saveNativeThumbnail(2); err != nil {
		t.Fatal(err)
	}
	data, _, err := doomruntime.NativeSaveThumbnail(2)
	if err != nil {
		t.Fatal(err)
	}
	thumbnail, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if thumbnail.Bounds().Dx() != 320 || thumbnail.Bounds().Dy() != 200 {
		t.Fatal("thumbnail dimensions differ from main storage")
	}
	menu := newNativeMenu(opts, []mapdata.MapName{"E1M1", "E1M3"}, "E1M3")
	menu.open(menuLoad)
	menu.saveSlots = campaign.SaveSlots(false)
	menu.row = 1
	rl.DrawRectangle(0, 0, 1280, 800, rl.NewColor(0, 0, 0, 180))
	p.DrawUI(menu.draw(nativeSettings{}, 0), 1280, 800)
	preview.Draw(2, 1280, 800)
	id := preview.texture.ID
	if id == 0 {
		t.Fatal("saved preview was not uploaded")
	}
	preview.Draw(2, 1280, 800)
	if preview.texture.ID != id {
		t.Fatal("unchanged preview reuploaded")
	}
	if err := captureScreen(capturePath); err != nil {
		t.Fatal(err)
	}
	rl.EndDrawing()
	// Mutate and restore the real player pose; the rebuilt geometry must match.
	before := campaign.Game.Frame(1)
	savedCamera, savedTic := before.Camera, before.WorldTic
	for range 20 {
		campaign.Game.Tick(doomruntime.NativeMeshInput{Forward: 1})
	}
	if err := campaign.LoadSlot(2); err != nil {
		t.Fatal(err)
	}
	restored := campaign.Game.Frame(1)
	if restored.Camera != savedCamera || restored.WorldTic != savedTic || len(restored.Triangles) == 0 || len(restored.HUD) == 0 {
		t.Fatal("loaded save did not restore renderable gameplay")
	}
	drawWorld()
	// Save the loaded frame and require identical gameplay pixels to the old save.
	if err := saveNativeThumbnail(3); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := doomruntime.NativeSaveThumbnail(3)
	if err != nil {
		t.Fatal(err)
	}
	after, err := png.Decode(bytes.NewReader(loaded))
	if err != nil {
		t.Fatal(err)
	}
	// Both hosts reset the transient face animation on load; it is absent
	// from their shared snapshot. Every other gameplay pixel must match.
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			if x >= 150 && x <= 175 && y >= 177 {
				continue
			}
			if thumbnail.At(x, y) != after.At(x, y) {
				t.Fatalf("restored gameplay pixel differs at %d,%d", x, y)
			}
		}
	}

	rl.EndDrawing()
	// Overwriting releases the stale preview and reloads one texture.
	preview.Close()
	if err := campaign.SaveSlot(2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := doomruntime.NativeSaveThumbnail(2); !os.IsNotExist(err) {
		t.Fatal("overwritten save retained old screenshot")
	}
}
