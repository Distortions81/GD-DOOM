//go:build raylib && cgo && integration && !js

package main

import (
	"bytes"
	"gddoom/internal/doomrand"
	"gddoom/internal/doomruntime"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/sessiontransition"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"runtime"
	"testing"
)

func TestNativeWipeGPUEqualsMainMeltAndResizes(t *testing.T) {
	if os.Getenv("GD_RAYLIB_WIPE_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_WIPE_INTEGRATION=1 for framebuffer melt checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	const width, height = 641, 403
	rl.InitWindow(width, height, "Native melt regression")
	defer rl.CloseWindow()
	w := &nativeWipe{}
	defer w.Close()
	from, to := make([]byte, width*height*4), make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			from[i], from[i+1], from[i+2], from[i+3] = byte(x%191+32), byte(y%163+20), byte((x+y)%127), 255
			to[i], to[i+1], to[i+2], to[i+3] = byte(y%191+32), byte(x%163+20), byte((x*3+y)%127), 255
		}
	}
	texture := func(pixels []byte) rl.Texture2D {
		return rl.LoadTextureFromImage(rl.NewImage(pixels, width, height, 1, rl.UncompressedR8g8b8a8))
	}
	fromTexture, toTexture := texture(from), texture(to)
	defer rl.UnloadTexture(fromTexture)
	defer rl.UnloadTexture(toTexture)
	drawFixture := func(texture rl.Texture2D, w, h int) {
		rl.ClearBackground(rl.Black)
		rl.DrawTexturePro(texture, rl.NewRectangle(0, 0, width, height), rl.NewRectangle(0, 0, float32(w), float32(h)), rl.Vector2{}, 0, rl.White)
	}
	read := func() []byte {
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		defer rl.UnloadImage(img)
		colors := rl.LoadImageColors(img)
		defer rl.UnloadImageColors(colors)
		pixels := make([]byte, len(colors)*4)
		for i, c := range colors {
			pixels[4*i], pixels[4*i+1], pixels[4*i+2], pixels[4*i+3] = c.R, c.G, c.B, c.A
		}
		return pixels
	}
	rl.BeginDrawing()
	drawFixture(fromTexture, width, height)
	if err := w.CaptureLastFrame(); err != nil {
		t.Fatal(err)
	}
	rl.EndDrawing()
	fromID := w.from.Texture.ID
	w.Queue()
	rnd, prnd := doomrand.State()
	rl.BeginDrawing()
	drawFixture(toTexture, width, height)
	if err := w.Prepare(); err != nil {
		t.Fatal(err)
	}
	w.Draw(width, height)
	if pixels := read(); !bytes.Equal(pixels, from) {
		t.Fatal("first wipe frame was not the previous image (orientation or MSAA resolve)")
	}
	rl.EndDrawing()
	toID := w.to.Texture.ID
	nativeRnd, nativePrnd := doomrand.State()
	doomrand.SetState(rnd, prnd)
	y := sessiontransition.InitMeltColumnsScaled(sessiontransition.SourcePortMeltColumns, sessiontransition.SourcePortMeltRNGScale(height))
	refRnd, refPrnd := doomrand.State()
	if nativeRnd != refRnd || nativePrnd != refPrnd {
		t.Fatal("melt initialization changed Doom RNG consumption")
	}
	expected := append([]byte(nil), from...)
	done := false
	tics := 0
	for w.Active() && tics < 100 {
		w.Tick()
		done = sessiontransition.StepMeltSlicesVirtual(y, sessiontransition.MeltVirtualH, width, height, from, to, expected, 1, sessiontransition.SourcePortMeltColumns)
		if w.Active() == done {
			t.Fatal("native transition completion differs from main")
		}
		rl.BeginDrawing()
		if w.Active() {
			w.Draw(width, height)
		} else {
			drawFixture(toTexture, width, height)
		}
		pixels := read()
		if !bytes.Equal(pixels, expected) {
			for i := range pixels {
				if pixels[i] != expected[i] {
					t.Fatalf("melt differs from main at tic %d pixel %d,%d channel %d: %d vs %d", tics, (i/4)%width, (i/4)/width, i%4, pixels[i], expected[i])
				}
			}
		}
		if tics == 15 {
			if err := captureScreen("../../build/raylib-captures/native-melt.png"); err != nil {
				t.Fatal(err)
			}
		}
		rl.EndDrawing()
		tics++
		if w.from.Texture.ID != fromID || w.to.Texture.ID != toID {
			t.Fatal("wipe snapshots reallocated while moving")
		}
	}
	if w.Active() || !done {
		t.Fatal("melt did not finish")
	}
	// Resize mid-wipe: reinitialize the incoming snapshot/timeline while keeping
	// the old frame, and scale that frame rather than displaying black gaps.
	w.Queue()
	rl.BeginDrawing()
	drawFixture(toTexture, width, height)
	if err := w.Prepare(); err != nil {
		t.Fatal(err)
	}
	rl.EndDrawing()
	w.Tick()
	rl.SetWindowSize(333, 277)
	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	rl.EndDrawing()
	if !w.NeedsResize() {
		t.Fatal("active wipe did not notice resized framebuffer")
	}
	rl.BeginDrawing()
	drawFixture(toTexture, 333, 277)
	if err := w.Prepare(); err != nil {
		t.Fatal(err)
	}
	if w.from.Texture.ID != fromID || w.to.Texture.Width != 333 || w.to.Texture.Height != 277 {
		t.Fatal("resize did not retain old frame and replace target")
	}
	w.Draw(333, 277)
	resized := read()
	for row := 0; row < 277; row++ {
		for col := 0; col < 333; col++ {
			sx, sy := int((float64(col)+.5)*width/333), int((float64(row)+.5)*height/277)
			for channel := 0; channel < 4; channel++ {
				if resized[(row*333+col)*4+channel] != from[(sy*width+sx)*4+channel] {
					t.Fatalf("resized old snapshot changed at %d,%d", col, row)
				}
			}
		}
	}
	rl.EndDrawing()
	w.Clear()
	rl.BeginDrawing()
	drawFixture(toTexture, 333, 277)
	if err := w.CaptureLastFrame(); err != nil {
		t.Fatal(err)
	}
	rl.EndDrawing()
	if w.from.Texture.Width != 333 || w.from.Texture.Height != 277 {
		t.Fatal("completed frame cache did not resize")
	}
}

func TestNativeCampaignWipeFreezesNewLevelGPU(t *testing.T) {
	if os.Getenv("GD_RAYLIB_WIPE_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_WIPE_INTEGRATION=1 for campaign wipe checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(1280, 800, "Native campaign melt regression")
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
	w := &nativeWipe{}
	defer w.Close()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	renderer, err := raymesh.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	if err := advanceNativeCampaign(c, w, doomruntime.NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if w.Active() || c.Phase() != doomruntime.NativeCampaignIntermission {
		t.Fatal("exit added a wipe where the main engine enters statistics directly")
	}
	c.TakeMusicRequest()
	for i := 0; i < 1500 && c.Phase() != doomruntime.NativeCampaignPlaying; i++ {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		p.DrawUI(c.Patches(), 1280, 800)
		if err := w.CaptureLastFrame(); err != nil {
			t.Fatal(err)
		}
		rl.EndDrawing()
		if err := advanceNativeCampaign(c, w, doomruntime.NativeMeshInput{}, i%15 == 0); err != nil {
			t.Fatal(err)
		}
	}
	if c.Map().Name != "E1M2" || !w.Active() || w.Ready() {
		t.Fatal("real level transition did not queue a pending melt")
	}
	if c.TakeMusicRequest() != "D_E1M2" {
		t.Fatal("next-map music request missing")
	}
	// The host defers this request until Active() clears, preserving intermission
	// music through the transition. Prepare the incoming map's actual GPU image.
	f := c.Game.Frame(1)
	if f.WorldTic != 0 {
		t.Fatal("new map advanced before destination capture")
	}
	renderer.SetLightRamp(c.Game.LightRamp())
	renderer.Sync(f.Triangles, c.Game.Texture, c.Game.Light, levelmesh.Textured)
	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	viewH := 800 - raymesh.HUDHeight(1280, 800)
	p.DrawSky(f.Sky, f.Camera, 1280, viewH)
	rl.DrawRenderBatchActive()
	rl.Viewport(0, int32(800-viewH), 1280, int32(viewH))
	renderer.Draw(f.Camera, 1280, viewH, levelmesh.Textured)
	rl.Viewport(0, 0, 1280, 800)
	p.DrawHUD(f.HUD, 1280, 800)
	if err := w.Prepare(); err != nil {
		t.Fatal(err)
	}
	w.Draw(1280, 800)
	rl.EndDrawing()
	tics := 0
	for w.Active() && tics < 100 {
		if err := advanceNativeCampaign(c, w, doomruntime.NativeMeshInput{Forward: 1, Fire: true}, true); err != nil {
			t.Fatal(err)
		}
		if c.Game.Frame(1).WorldTic != 0 {
			t.Fatal("new level simulated during melt")
		}
		if w.Active() {
			rl.BeginDrawing()
			w.Draw(1280, 800)
			if tics == 18 {
				if err := captureScreen("../../build/raylib-captures/native-campaign-melt.png"); err != nil {
					t.Fatal(err)
				}
			}
			rl.EndDrawing()
		}
		tics++
	}
	if w.Active() {
		t.Fatal("campaign melt never completed")
	}
	if err := advanceNativeCampaign(c, w, doomruntime.NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if c.Game.Frame(1).WorldTic != 1 {
		t.Fatal("new level did not resume after melt")
	}
}
