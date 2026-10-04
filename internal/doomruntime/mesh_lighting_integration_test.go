//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"math"
	"os"
	"runtime"
	"testing"

	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeMeshLightingMatchesDoom(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual lighting framebuffer checks")
	}
	resetLightingMathState(t)
	oldRows, oldRamp := doomColormapRows, doomRowShadeMulLUT
	t.Cleanup(func() { doomColormapRows, doomRowShadeMulLUT = oldRows, oldRamp })
	doomColormapRows, doomRowShadeMulLUT = 32, nil
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(128, 128, "Doom mesh lighting regression")
	defer rl.CloseWindow()
	r, err := raymesh.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	white := levelmesh.Texture{Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}}
	texture := func(levelmesh.Triangle) levelmesh.Texture { return white }
	lightValue := 160
	light := func(int) float64 { return float64(lightValue) / 256 }
	readPixel := func(camera levelmesh.Camera, x, y int, mode levelmesh.Mode) rl.Color {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Red)
		r.Draw(camera, 128, 128, mode)
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		colors := rl.LoadImageColors(img)
		c := colors[y*128+x]
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		return c
	}
	checkShade := func(t *testing.T, c rl.Color, want int) {
		t.Helper()
		if math.Abs(float64(c.R)-float64(want)) > 2 || c.R != c.G || c.R != c.B || c.A != 255 {
			t.Fatalf("GPU shade %v differs from independent Doom reference %d", c, want)
		}
	}
	card := func(depth, fx, fy float64) []levelmesh.Triangle {
		// A large vertical card facing the camera with constant view depth.
		px, py := -fy*4096, fx*4096
		x, y := fx*depth, fy*depth
		v := [4]levelmesh.Vertex{{X: x - px, Y: y - py, Z: -4096}, {X: x - px, Y: y - py, Z: 4096}, {X: x + px, Y: y + py, Z: 4096}, {X: x + px, Y: y + py, Z: -4096}}
		return []levelmesh.Triangle{{Kind: levelmesh.Middle, Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Kind: levelmesh.Middle, Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	}
	if err := r.SetLightingMode(raymesh.DoomLighting); err != nil {
		t.Fatal(err)
	}
	for _, axis := range []struct {
		name        string
		fx, fy, yaw float64
		bias        int
	}{
		{"vertical", 1, 0, 0, 1}, {"horizontal", 0, 1, math.Pi / 2, -1}, {"diagonal", math.Sqrt(0.5), math.Sqrt(0.5), math.Pi / 4, 0},
	} {
		for _, depth := range []float64{64, 256, 1024} {
			// Keep all GL calls on this locked OS thread; t.Run would execute
			// its body in a separate goroutine without the window context.
			t.Logf("%s wall at %.0f map units", axis.name, depth)
			tris := card(depth, axis.fx, axis.fy)
			r.Sync(tris, texture, light, levelmesh.Textured)
			c := readPixel(levelmesh.Camera{Yaw: axis.yaw}, 64, 64, levelmesh.Textured)
			shade := doomShadeMulFromRowF(doomWallLightRowF(int16(lightValue), axis.bias, depth, 160))
			checkShade(t, c, int(math.Round(float64(shade)*255/256)))
		}
	}
	// A real floor: compute its depth from a pixel-center camera ray, then
	// compare the fragment with the original plane lighting calculation.
	v := [4]levelmesh.Vertex{{X: 2, Y: -4096}, {X: 4096, Y: -4096}, {X: 4096, Y: 4096}, {X: 2, Y: 4096}}
	floor := []levelmesh.Triangle{{Kind: levelmesh.Floor, Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Kind: levelmesh.Floor, Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	camera := levelmesh.Camera{Z: 41}
	r.Sync(floor, texture, light, levelmesh.Textured)
	fov := raymesh.Camera(camera, 128, 128).Fovy
	focalY := 64 / math.Tan(float64(fov)*math.Pi/360)
	for _, y := range []int{70, 80, 110} {
		depth := camera.Z * focalY / (float64(y) + 0.5 - 64)
		shade := doomShadeMulFromRowF(doomPlaneLightRowF(int16(lightValue), depth))
		checkShade(t, readPixel(camera, 64, y, levelmesh.Textured), int(math.Round(float64(shade)*255/256)))
	}
	// Render-only comparisons cannot change resident geometry. Sector-only
	// matches the old renderer; fullbright and a light-amp override stay white.
	tris := card(256, 1, 0)
	r.Sync(tris, texture, light, levelmesh.Textured)
	r.Sync(tris, texture, light, levelmesh.Textured)
	for _, mode := range []raymesh.LightingMode{raymesh.SectorLighting, raymesh.FullbrightLighting, raymesh.DoomLighting} {
		if err := r.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		c := readPixel(levelmesh.Camera{}, 64, 64, levelmesh.Textured)
		if mode == raymesh.SectorLighting {
			checkShade(t, c, 159)
		}
		if mode == raymesh.FullbrightLighting {
			checkShade(t, c, 255)
		}
		if s := r.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 0 {
			t.Fatalf("lighting toggle uploaded geometry: %+v", s)
		}
	}
	r.SetFullbright(true)
	checkShade(t, readPixel(levelmesh.Camera{}, 64, 64, levelmesh.Textured), 255)
	r.SetFullbright(false)
	// Supply a non-linear WAD-like row ramp: the shader must use the upload
	// rather than its fallback brightness equation.
	var ramp [32]float32
	for i := range ramp {
		ramp[i] = 0.25
	}
	r.SetLightRamp(ramp)
	checkShade(t, readPixel(levelmesh.Camera{}, 64, 64, levelmesh.Textured), 64)
}
