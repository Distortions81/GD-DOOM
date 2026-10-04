//go:build raylib && cgo && integration && !js

package raymesh

import (
	"context"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const mapFontSample = "THING LEGEND\nplayer starts\nmonsters\nitems/pickups\nkeys / misc\nrender: sprites\nLINE COLORS\nuse switch/button\n0123456789\nÀÁÄÅÈÉÖÜÿ\nUnicode skipped: 世界"

type mapFontReference struct {
	done bool
	run  func()
}

func (d *mapFontReference) Update() error {
	if d.done {
		return ebiten.Termination
	}
	return nil
}
func (d *mapFontReference) Draw(*ebiten.Image) {
	if !d.done {
		d.run()
		d.done = true
	}
}
func (d *mapFontReference) Layout(int, int) (int, int) { return 320, 256 }

func TestRaylibDebugTextMatchesMainBitmapFont(t *testing.T) {
	if path := os.Getenv("GD_MAP_FONT_REFERENCE_PATH"); path != "" {
		d := &mapFontReference{run: func() {
			img := ebiten.NewImage(320, 256)
			img.Fill(color.RGBA{38, 62, 93, 255})
			ebitenutil.DebugPrintAt(img, mapFontSample, 7, 9)
			pixels := make([]byte, 320*256*4)
			img.ReadPixels(pixels)
			if err := os.WriteFile(path, pixels, 0600); err != nil {
				t.Fatal(err)
			}
		}}
		ebiten.SetVsyncEnabled(false)
		if err := ebiten.RunGame(d); err != nil {
			t.Fatal(err)
		}
		return
	}
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1")
	}
	path := filepath.Join(t.TempDir(), "font.rgba")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestRaylibDebugTextMatchesMainBitmapFont$", "-test.count=1")
	cmd.Env = append(os.Environ(), "GD_MAP_FONT_REFERENCE_PATH="+path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("main font reference: %v\n%s", err, out)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(320, 256, "Shared automap font")
	if !rl.IsWindowReady() {
		t.Fatal("window unavailable")
	}
	defer rl.CloseWindow()
	f := &DebugText{}
	defer f.Close()
	var id uint32
	for range 2 {
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(38, 62, 93, 255))
		f.Draw(mapFontSample, 7, 9)
		img, err := CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		for i, c := range colors {
			for channel, v := range []uint8{c.R, c.G, c.B, c.A} {
				delta := int(v) - int(want[i*4+channel])
				if delta < -1 || delta > 1 {
					t.Fatalf("font pixel %d,%d channel %d=%d main=%d", i%320, i/320, channel, v, want[i*4+channel])
				}
			}
		}
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		rl.EndDrawing()
		if id != 0 && id != f.texture.ID {
			t.Fatal("font replaced its cached atlas")
		}
		id = f.texture.ID
	}
	f.Close()
	if f.texture.ID != 0 {
		t.Fatal("font cache not released")
	}
}
