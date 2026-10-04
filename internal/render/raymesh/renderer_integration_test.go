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

func TestRaylibResidentMeshDepthAndMask(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual GPU checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(128, 128, "Raylib mesh regression")
	defer rl.CloseWindow()
	r, err := NewRendererWithTextureOptions(TextureOptions{Scale: 2, Filter: Nearest})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	red := levelmesh.Texture{RGBA: []byte{255, 0, 0, 255}, Width: 1, Height: 1}
	mask := levelmesh.Texture{RGBA: []byte{0, 0, 0, 0, 0, 255, 0, 255}, Width: 2, Height: 1}
	texture := func(tri levelmesh.Triangle) levelmesh.Texture {
		if tri.Masked {
			return mask
		}
		return red
	}
	quad := func(x float64, masked bool) []levelmesh.Triangle {
		v := [4]levelmesh.Vertex{{X: x, Y: -128, Z: -128, U: 0, V: 1}, {X: x, Y: -128, Z: 128, U: 0, V: 0}, {X: x, Y: 128, Z: 128, U: 8, V: 0}, {X: x, Y: 128, Z: -128, U: 8, V: 1}}
		// Four repeats of a two-texel mask. The visible center spans UVs
		// beyond 1.0; clamping would hide the red wall behind it entirely.
		return []levelmesh.Triangle{{Sector: 1, Masked: masked, Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Sector: 1, Masked: masked, Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	}
	tris := append(quad(64, true), quad(128, false)...)
	r.Sync(tris, texture, func(int) float64 { return 1 }, levelmesh.Textured)
	if r.Stats().MeshUploads != 2 {
		t.Fatalf("initial upload %+v", r.Stats())
	}
	r.Sync(tris, texture, func(int) float64 { return 1 }, levelmesh.Textured)
	if s := r.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 0 || s.ResidentMeshes != 2 {
		t.Fatalf("static geometry reuploaded %+v", s)
	}
	draw := func() []byte {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		r.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		defer rl.UnloadImage(img)
		colors := rl.LoadImageColors(img)
		defer rl.UnloadImageColors(colors)
		pixels := make([]byte, len(colors)*4)
		for i, c := range colors {
			pixels[i*4], pixels[i*4+1], pixels[i*4+2], pixels[i*4+3] = c.R, c.G, c.B, c.A
		}
		return pixels
	}
	first := draw()
	green, redPixels := 0, 0
	for i := 0; i < len(first); i += 4 {
		if first[i] == 0 && first[i+1] == 255 {
			green++
		}
		if first[i] == 255 && first[i+1] == 0 {
			redPixels++
		}
	}
	if green < 1000 || redPixels < 1000 {
		t.Fatalf("mask/depth failed: green=%d red=%d", green, redPixels)
	}
	for _, filter := range []TextureFilter{Trilinear, Anisotropic, Nearest} {
		if err := r.SetTextureFilter(filter); err != nil {
			t.Fatal(err)
		}
		r.Sync(tris, texture, func(int) float64 { return 1 }, levelmesh.Textured)
		if s := r.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 0 {
			t.Fatalf("filter toggle changed geometry: %+v", s)
		}
		for key, tex := range r.textures {
			if tex.Width != int32(key.Texture.Width*2) || tex.Height != int32(key.Texture.Height*2) || tex.Mipmaps <= 1 {
				t.Fatalf("%s did not retain doubled textures with mipmaps: %+v", filter, tex)
			}
		}
		filtered := draw()
		foreground, background := 0, 0
		for i := 0; i < len(filtered); i += 4 {
			switch {
			case filtered[i] == 0 && filtered[i+1] == 255:
				foreground++
			case filtered[i] == 255 && filtered[i+1] == 0:
				background++
			default:
				t.Fatalf("%s introduced a dark mask fringe at pixel %d: %v", filter, i/4, filtered[i:i+4])
			}
		}
		if foreground < 1000 || background < 1000 {
			t.Fatalf("%s lost repeated opaque/cutout areas: %d/%d", filter, foreground, background)
		}
		if filter == Nearest && !slices.Equal(filtered, first) {
			t.Fatal("nearest comparison retained mipmapped or anisotropic sampling")
		}
	}
	slices.Reverse(tris)
	r.Sync(tris, texture, func(int) float64 { return 1 }, levelmesh.Textured)
	if second := draw(); !slices.Equal(first, second) {
		t.Fatal("GPU image depends on submission order")
	}
	for i := range tris {
		if tris[i].Masked {
			for j := range tris[i].Vertices {
				tris[i].Vertices[j].X = 192
			}
		}
	}
	r.Sync(tris, texture, func(int) float64 { return 1 }, levelmesh.Textured)
	if s := r.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 1 {
		t.Fatalf("moving wall did not update exactly its vertex buffer: %+v", s)
	}
	if second := draw(); slices.Equal(first, second) {
		t.Fatal("moving wall did not reach GPU framebuffer")
	}
	r.Sync(tris, texture, func(int) float64 { return 0.5 }, levelmesh.Textured)
	if s := r.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 2 {
		t.Fatalf("sector light should update only the two color buffers: %+v", s)
	}
	dim := draw()
	center := (64*128 + 64) * 4
	if dim[center] != 127 || dim[center+1] != 0 || dim[center+2] != 0 {
		t.Fatalf("light update did not reach framebuffer: %v", dim[center:center+4])
	}
	t.Logf("hardware depth and alpha discard: %d opaque foreground and %d background pixels", green, redPixels)
}

func TestRaylibFilteredTextureMinification(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual GPU checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(128, 128, "Raylib mipmap regression")
	defer rl.CloseWindow()
	r, err := NewRendererWithTextureOptions(TextureOptions{Scale: 2, Filter: Trilinear})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	checker := levelmesh.Texture{Width: 16, Height: 16, RGBA: make([]byte, 16*16*4)}
	for y := range 16 {
		for x := range 16 {
			i := (y*16 + x) * 4
			c := byte(((x + y) & 1) * 255)
			checker.RGBA[i], checker.RGBA[i+1], checker.RGBA[i+2], checker.RGBA[i+3] = c, c, c, 255
		}
	}
	v := [4]levelmesh.Vertex{{X: 64, Y: -128, Z: -128, U: 0, V: 1024}, {X: 64, Y: -128, Z: 128, U: 0, V: 0}, {X: 64, Y: 128, Z: 128, U: 1024, V: 0}, {X: 64, Y: 128, Z: -128, U: 1024, V: 1024}}
	tris := []levelmesh.Triangle{{Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	r.Sync(tris, func(levelmesh.Triangle) levelmesh.Texture { return checker }, func(int) float64 { return 1 }, levelmesh.Textured)
	draw := func() []rl.Color {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Red)
		r.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		colors := rl.LoadImageColors(img)
		pixels := slices.Clone(colors)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		return pixels
	}
	// The projected checker cells are smaller than a screen pixel. A real
	// mipmapped sample must average black/white rather than alias between them.
	for _, filter := range []TextureFilter{Trilinear, Anisotropic} {
		if err := r.SetTextureFilter(filter); err != nil {
			t.Fatal(err)
		}
		pixels := draw()
		for y := 32; y < 96; y++ {
			for x := 32; x < 96; x++ {
				c := pixels[y*128+x]
				if c.R < 120 || c.R > 136 || c.R != c.G || c.R != c.B {
					t.Fatalf("%s did not sample minified mip average at %d,%d: %v", filter, x, y, c)
				}
			}
		}
	}
	if err := r.SetTextureFilter(Nearest); err != nil {
		t.Fatal(err)
	}
	for _, c := range draw() {
		if c.R != c.G || c.R != c.B || (c.R != 0 && c.R != 255) {
			t.Fatal("nearest retained mip averaging")
		}
	}
}
