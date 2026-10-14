//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type nativeHUDOverlaySample struct {
	width, height                 int
	dead                          bool
	damage, bonus, strength, suit int
}

func nativeHUDOverlaySamples() []nativeHUDOverlaySample {
	var samples []nativeHUDOverlaySample
	for _, size := range [][2]int{{320, 200}, {640, 400}, {960, 540}} {
		for _, dead := range []bool{false, true} {
			for _, flash := range [][4]int{{}, {72, 40, 0, 200}, {1, 40, 0, 200}, {0, 40, 0, 200}, {0, 0, 1, 200}, {0, 0, 0, 200}, {0, 0, 0, 128}, {0, 0, 0, 8}} {
				samples = append(samples, nativeHUDOverlaySample{width: size[0], height: size[1], dead: dead, damage: flash[0], bonus: flash[1], strength: flash[2], suit: flash[3]})
			}
		}
	}
	return samples
}

func applyNativeHUDOverlaySample(g *game, s nativeHUDOverlaySample) {
	g.viewW, g.viewH, g.isDead = s.width, s.height, s.dead
	g.statusDamageCount, g.statusBonusCount = s.damage, s.bonus
	g.inventory.StrengthCount, g.inventory.RadSuitTics = s.strength, s.suit
	g.setHUDMessage("PICKED UP A SHOTGUN!\n\nGAME SAVED", 70)
}

// Use a separate process for the reference: both backends own a GLFW lifecycle
// and must not initialize/terminate each other's windows in one process.
func TestNativeHUDOverlayEbitenReference(t *testing.T) {
	dir := os.Getenv("GD_RAYLIB_OVERLAY_REFERENCE_DIR")
	if dir == "" {
		t.Skip("reference child used by the native HUD comparison")
	}
	c := nativeOverlayFixture(t)
	g := c.Game.g
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		for i, s := range nativeHUDOverlaySamples() {
			applyNativeHUDOverlaySample(g, s)
			img := ebiten.NewImage(s.width, s.height)
			img.Fill(color.RGBA{40, 80, 120, 255})
			ebitenutil.DrawRect(img, 0, float64(s.height-32), float64(s.width), 32, color.RGBA{90, 60, 30, 255})
			if s.dead {
				g.drawDeathOverlay(img)
			}
			g.drawFlashOverlay(img)
			g.drawHUDMessage(img, g.useText, 0, 0)
			pixels := make([]byte, s.width*s.height*4)
			img.ReadPixels(pixels)
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.rgba", i)), pixels, 0600); err != nil {
				t.Fatal(err)
			}
			img.Deallocate()
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

// Compare actual Ebiten and Raylib framebuffers, including tint priority,
// the bottom HUD area, WAD font placement and the death/flash/message order.
func TestNativeHUDOverlayFramebuffersMatchMain(t *testing.T) {
	if os.Getenv("GD_RAYLIB_OVERLAY_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_OVERLAY_INTEGRATION=1 for both host framebuffer comparisons")
	}
	dir := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestNativeHUDOverlayEbitenReference$", "-test.v")
	child.Env = append(os.Environ(), "GD_RAYLIB_OVERLAY_REFERENCE_DIR="+dir)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("Ebiten reference: %v\n%s", err, output)
	}
	c := nativeOverlayFixture(t)
	g := c.Game.g
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(320, 200, "Shared HUD overlay comparison")
	defer rl.CloseWindow()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	samples := nativeHUDOverlaySamples()
	for sampleIndex, s := range samples {
		applyNativeHUDOverlaySample(g, s)
		pixels, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d.rgba", sampleIndex)))
		if err != nil || len(pixels) != s.width*s.height*4 {
			t.Fatalf("invalid reference framebuffer: %v", err)
		}
		if int(rl.GetRenderWidth()) != s.width || int(rl.GetRenderHeight()) != s.height {
			rl.SetWindowSize(s.width, s.height)
			for range 2 {
				rl.BeginDrawing()
				rl.ClearBackground(rl.Black)
				rl.EndDrawing()
			}
		}
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(40, 80, 120, 255))
		rl.DrawRectangle(0, int32(s.height-32), int32(s.width), 32, rl.NewColor(90, 60, 30, 255))
		death := c.DeathOverlay(s.width, s.height)
		raymesh.DrawScreenTint(death.Tint, s.width, s.height)
		p.DrawScreenPatches(death.Patches)
		raymesh.DrawScreenTint(c.Game.Frame(1).FlashOverlay, s.width, s.height)
		p.DrawScreenPatches(c.MessagePatches(s.width, s.height))
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		for i, got := range colors {
			want := pixels[i*4 : i*4+4]
			for channel, value := range []byte{got.R, got.G, got.B, got.A} {
				if delta := int(value) - int(want[channel]); delta < -1 || delta > 1 {
					t.Fatalf("%dx%d dead=%v flash=%d/%d/%d/%d pixel=(%d,%d) got=%v main=%v", s.width, s.height, s.dead, s.damage, s.bonus, s.strength, s.suit, i%s.width, i/s.width, got, want)
				}
			}
		}
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		rl.EndDrawing()
	}
	t.Log(fmt.Sprintf("%d actual framebuffer comparisons passed", len(samples)))
}
