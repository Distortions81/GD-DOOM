//go:build raylib && cgo && integration && !js

package main

import (
	"context"
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

func TestNativeCRTLauncherPreservesHUDAndSourcePortAspect(t *testing.T) {
	if os.Getenv("GD_RAYLIB_CRT_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_CRT_INTEGRATION=1 after building build/raydoom")
	}
	launch := func(name string, args ...string) image.Image {
		path := filepath.Join(t.TempDir(), name+".png")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		base := []string{"-config=", "-wad=../../DOOM1.WAD", "-map=E1M1", "-nomonsters", "-nofps", "-sound=false", "-music=false", "-width=640", "-height=400", "-frames=1", "-capture=" + path}
		out, err := exec.CommandContext(ctx, "../../build/raydoom", append(base, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("launch %s: %v\n%s", name, err, out)
		}
		if !strings.Contains(string(out), "crt="+map[bool]string{false: "false", true: "true"}[name == "crt"]) {
			t.Fatalf("launcher did not apply CRT flag: %s", out)
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
	base := launch("base", "-crt-effect=false")
	crt := launch("crt", "-crt-effect=true")
	noAspect := launch("no-aspect", "-crt-effect=false", "-no-aspect-correction=true")
	viewH := 400 - raymesh.HUDHeight(640, 400)
	changed := 0
	for y := range 400 {
		for x := range 640 {
			r, g, b, a := base.At(x, y).RGBA()
			rc, gc, bc, ac := crt.At(x, y).RGBA()
			rn, gn, bn, an := noAspect.At(x, y).RGBA()
			if r != rn || g != gn || b != bn || a != an {
				t.Fatal("faithful presentation flag changed source-port projection")
			}
			if r != rc || g != gc || b != bc || a != ac {
				if y >= viewH {
					t.Fatalf("CRT changed HUD at %d,%d", x, y)
				}
				changed++
			}
		}
	}
	if changed < 100000 {
		t.Fatalf("CRT did not affect most scene pixels: %d", changed)
	}
}
