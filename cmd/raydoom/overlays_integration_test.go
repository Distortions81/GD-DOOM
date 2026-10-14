//go:build raylib && cgo && integration && !js

package main

import (
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestNativeRecordingMarkerLauncher(t *testing.T) {
	if os.Getenv("GD_RAYLIB_OVERLAY_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_OVERLAY_INTEGRATION=1 after building build/raydoom")
	}
	path := filepath.Join(t.TempDir(), "record.lmp")
	capture, err := filepath.Abs("../../build/raylib-captures/native-recording-hud.png")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "../../build/raydoom", "-config=", "-wad=../../DOOM1.WAD", "-map=E1M1", "-record-demo="+path, "-nomonsters", "-nofps", "-sound=false", "-music=false", "-width=640", "-height=400", "-frames=3", "-capture="+capture).CombinedOutput()
	if err != nil {
		t.Fatalf("recording launch: %v\n%s", err, output)
	}
	script, err := demo.Load(path)
	if err != nil || len(script.Tics) != 3 {
		t.Fatalf("recording did not persist its three command tics: %v", err)
	}
	file, err := os.Open(capture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	opts.RecordDemoPath = path
	c := doomruntime.NewNativeCampaign(doomruntime.NewNativeMeshGame(m, opts), opts, nil)
	marker := c.RecordingOverlay(640, 400)
	r, g, b, _ := img.At(int(marker.X), int(marker.Y)).RGBA()
	if byte(r>>8) != marker.Color.R || byte(g>>8) != marker.Color.G || byte(b>>8) != marker.Color.B {
		t.Fatal("native launcher omitted the red recording circle")
	}
	checked := 0
	for _, patch := range marker.Patches {
		for y := range patch.Texture.Height {
			for x := range patch.Texture.Width {
				i := (y*patch.Texture.Width + x) * 4
				want := patch.Texture.RGBA[i : i+4]
				if want[3] != 255 {
					continue
				}
				r, g, b, _ := img.At(int(patch.X)+x, int(patch.Y)+y).RGBA()
				if byte(r>>8) != want[0] || byte(g>>8) != want[1] || byte(b>>8) != want[2] {
					t.Fatal("native REC label differs from shared WAD glyph pixels")
				}
				checked++
			}
		}
	}
	if checked < 30 {
		t.Fatal("recording label had too few foreground pixels to validate")
	}
}
