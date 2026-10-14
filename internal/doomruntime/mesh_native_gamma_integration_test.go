//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"os"
	"runtime"
	"slices"
	"testing"

	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeGammaGPUUsesMainTablesAfterLighting(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for gamma shader checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(256, 128, "Gamma shader regression")
	defer rl.CloseWindow()
	r, err := raymesh.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	tex := levelmesh.Texture{Width: 256, Height: 1, RGBA: make([]byte, 256*4)}
	for i := range 256 {
		tex.RGBA[i*4], tex.RGBA[i*4+1], tex.RGBA[i*4+2], tex.RGBA[i*4+3] = byte(i), byte(255-i), byte(i*53), 255
	}
	v := [4]levelmesh.Vertex{{X: 128, Y: -128, Z: -128, U: 0, V: 1}, {X: 128, Y: -128, Z: 128, U: 0}, {X: 128, Y: 128, Z: 128, U: 256}, {X: 128, Y: 128, Z: -128, U: 256, V: 1}}
	tris := []levelmesh.Triangle{{Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	draw := func(level int, light float64, fixed bool) []rl.Color {
		r.Sync(tris, func(levelmesh.Triangle) levelmesh.Texture { return tex }, func(int) float64 { return light }, levelmesh.Textured)
		r.SetGammaTable(doomGammaTables[level])
		r.SetFixedColormap(fixed)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Blue)
		r.Draw(levelmesh.Camera{}, 256, 128, levelmesh.Textured)
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		out := slices.Clone(colors[64*256 : 65*256])
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		rl.EndDrawing()
		return out
	}
	for _, light := range []float64{1, .5, .25} {
		base := draw(0, light, false)
		for _, level := range []int{1, 2, 3, 4, 0} {
			got := draw(level, light, false)
			for i, c := range base {
				want := rl.Color{R: doomGammaTables[level][c.R], G: doomGammaTables[level][c.G], B: doomGammaTables[level][c.B], A: 255}
				if got[i] != want {
					t.Fatalf("gamma=%d light=%g x=%d got=%v want=%v", level, light, i, got[i], want)
				}
			}
			if st := r.Stats(); st.MeshUploads != 0 || st.BufferUpdates != 0 {
				t.Fatal("gamma rebuilt GPU geometry")
			}
		}
	}
	// The fixed palette is already gamma corrected; applying it again is wrong.
	tex.FixedRGBA = slices.Clone(tex.RGBA)
	for i := 0; i < len(tex.FixedRGBA); i += 4 {
		for channel := range 3 {
			tex.FixedRGBA[i+channel] = doomGammaTables[4][tex.FixedRGBA[i+channel]]
		}
	}
	fixed := draw(4, .25, true)
	tex.FixedRGBA = nil
	base := draw(0, 1, false)
	for i, c := range base {
		want := rl.Color{R: doomGammaTables[4][c.R], G: doomGammaTables[4][c.G], B: doomGammaTables[4][c.B], A: 255}
		if fixed[i] != want {
			t.Fatal("fixed colors were gamma corrected twice")
		}
	}
}
