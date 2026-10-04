//go:build raylib && cgo && integration && !js

package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gddoom/internal/render/raymesh"
)

func TestNativeDetailLauncherKeepsHUDSharpAcrossResolutions(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 after building build/raydoom")
	}
	launch := func(level int, crt, automap bool) image.Image {
		t.Helper()
		path := filepath.Join(t.TempDir(), "capture.png")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		args := []string{"-config=", "-wad=../../DOOM1.WAD", "-map=E1M1", "-nomonsters", "-nofps", "-sound=false", "-music=false", "-width=640", "-height=400", "-frames=1", "-capture=" + path, fmt.Sprintf("-detail-level=%d", level), fmt.Sprintf("-crt-effect=%t", crt), fmt.Sprintf("-automap=%t", automap)}
		out, err := exec.CommandContext(ctx, "../../build/raydoom", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("launcher: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), fmt.Sprintf("scene=%dx%d detail=%d auto-detail=false", 640/(level+1), 400/(level+1), level)) {
			t.Fatalf("wrong resolution: %s", out)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	base := launch(0, false, false)
	viewH := 400 - raymesh.HUDHeight(640, 400)
	for level := range 4 {
		for _, crt := range []bool{false, true} {
			img := launch(level, crt, false)
			changed := 0
			for y := range 400 {
				for x := range 640 {
					if img.At(x, y) != base.At(x, y) {
						if y >= viewH {
							t.Fatalf("detail=%d CRT=%t changed HUD at %d,%d", level, crt, x, y)
						}
						changed++
					}
				}
			}
			if (level > 0 || crt) && changed < 10000 {
				t.Fatalf("detail=%d CRT=%t scene unchanged: %d", level, crt, changed)
			}
		}
	}
	// Automap's HUD belongs to the reduced map surface. Check the complete
	// image is nearest scaled, including its CRT output, with integer divisors.
	for _, crt := range []bool{false, true} {
		for _, level := range []int{1, 3} {
			img := launch(level, crt, true)
			div := level + 1
			for y := 0; y < 400; y += div {
				for x := 0; x < 640; x += div {
					for dy := range div {
						for dx := range div {
							if img.At(x+dx, y+dy) != img.At(x, y) {
								t.Fatalf("automap detail=%d CRT=%t is not nearest scaled at %d,%d offset %d,%d: %v versus %v", level, crt, x, y, dx, dy, img.At(x+dx, y+dy), img.At(x, y))
							}
						}
					}
				}
			}
		}
	}
}
