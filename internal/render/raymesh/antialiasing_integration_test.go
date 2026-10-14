//go:build raylib && cgo && integration && !js

package raymesh

import (
	"os"
	"runtime"
	"slices"
	"testing"

	"gddoom/internal/render/levelmesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestRaylibMultisampledSeams(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual GPU checks")
	}
	// Window and GL calls must stay on the same OS thread, including reads.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(128, 128, "Raylib seam antialiasing regression")
	if !rl.IsWindowReady() {
		t.Fatal("Raylib window could not initialize")
	}
	defer rl.CloseWindow()
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.SetLightingMode(FullbrightLighting); err != nil {
		t.Fatal(err)
	}
	red := levelmesh.Texture{RGBA: []byte{255, 0, 0, 255}, Width: 1, Height: 1}
	green := levelmesh.Texture{RGBA: []byte{0, 255, 0, 255}, Width: 1, Height: 1}
	texture := func(tri levelmesh.Triangle) levelmesh.Texture {
		if tri.Sector == 1 {
			return red
		}
		return green
	}
	// Two materials meet at a diagonal, coplanar triangle edge. Its endpoints
	// and the card silhouette project between pixel centers. Blue background
	// makes even a partial uncovered sample at the shared edge detectable.
	v := [4]levelmesh.Vertex{
		{X: 64, Y: -43.3, Z: -39.1}, {X: 64, Y: -43.3, Z: 44.7},
		{X: 64, Y: 48.7, Z: 44.7}, {X: 64, Y: 48.7, Z: -39.1},
	}
	tris := []levelmesh.Triangle{
		{Sector: 1, Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}},
		{Sector: 2, Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}},
	}
	r.Sync(tris, texture, func(int) float64 { return 1 }, levelmesh.Textured)
	draw := func() []rl.Color {
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(0, 0, 255, 255))
		r.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
		img, err := CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		rl.EndDrawing()
		colors := rl.LoadImageColors(img)
		pixels := slices.Clone(colors)
		// The cloned pixels outlive Raylib's image and temporary color storage.
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		return pixels
	}
	first := draw()
	seam, pureRed, pureGreen, silhouette := 0, 0, 0, 0
	for y := 24; y < 100; y++ {
		for x := 24; x < 100; x++ {
			c := first[y*128+x]
			if c.B != 0 || int(c.R)+int(c.G) < 254 {
				t.Fatalf("uncovered sample at shared seam %d,%d: %v", x, y, c)
			}
			switch {
			case c.R == 255 && c.G == 0:
				pureRed++
			case c.R == 0 && c.G == 255:
				pureGreen++
			default:
				seam++
			}
		}
	}
	for _, c := range first {
		if c.B > 0 && c.B < 255 && (c.R > 0 || c.G > 0) {
			silhouette++
		}
	}
	if seam < 40 || seam > 200 || silhouette < 100 {
		t.Fatalf("MSAA coverage missing or overly broad: seam=%d silhouette=%d", seam, silhouette)
	}
	if pureRed < 1000 || pureGreen < 1000 {
		t.Fatalf("material interiors lost original colors: red=%d green=%d", pureRed, pureGreen)
	}
	slices.Reverse(tris)
	r.Sync(tris, texture, func(int) float64 { return 1 }, levelmesh.Textured)
	if s := r.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 0 {
		t.Fatalf("MSAA changed resident geometry: %+v", s)
	}
	if !slices.Equal(first, draw()) {
		t.Fatal("shared seam depends on material submission order")
	}
	t.Logf("MSAA: %d shared-edge pixels, %d silhouette pixels; interiors retain exact texture colors", seam, silhouette)
}
