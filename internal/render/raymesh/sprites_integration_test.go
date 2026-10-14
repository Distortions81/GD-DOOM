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

func TestRaylibSpritesShareWorldDepth(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual GPU checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(128, 128, "Native combat depth regression")
	if !rl.IsWindowReady() {
		t.Fatal("Raylib window unavailable")
	}
	defer rl.CloseWindow()
	world, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer world.Close()
	p, err := NewPresentation(TextureOptions{Scale: 2, Filter: Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	_ = world.SetLightingMode(FullbrightLighting)
	_ = p.Sprites.SetLightingMode(FullbrightLighting)
	red := levelmesh.Texture{Width: 1, Height: 1, RGBA: []byte{255, 0, 0, 255}}
	v := [4]levelmesh.Vertex{{X: 128, Y: -128, Z: -128}, {X: 128, Y: -128, Z: 128}, {X: 128, Y: 128, Z: 128}, {X: 128, Y: 128, Z: -128}}
	tris := []levelmesh.Triangle{{Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	world.Sync(tris, func(levelmesh.Triangle) levelmesh.Texture { return red }, func(int) float64 { return 1 }, levelmesh.Textured)
	mask := levelmesh.Texture{Width: 64, Height: 64, RGBA: make([]byte, 64*64*4)}
	for y := range 64 {
		for x := range 32 {
			i := (y*64 + x) * 4
			mask.RGBA[i+1], mask.RGBA[i+3] = 255, 255
		}
	}
	sprites := []levelmesh.Sprite{{Texture: mask, X: 64, Z: -32, OffsetX: 32, OffsetY: 64, ScaleY: 1, Light: 1}}
	draw := func() []rl.Color {
		p.SyncSprites(sprites, levelmesh.Camera{})
		rl.BeginDrawing()
		rl.ClearBackground(rl.Blue)
		world.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
		p.Sprites.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
		img, err := CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		rl.EndDrawing()
		colors := rl.LoadImageColors(img)
		out := slices.Clone(colors)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		return out
	}
	first := draw()
	if c := first[64*128+40]; c.R != 0 || c.G != 255 || c.B != 0 {
		t.Fatalf("foreground sprite missing: %v", c)
	}
	if c := first[64*128+88]; c.R != 255 || c.G != 0 || c.B != 0 {
		t.Fatalf("sprite cutout hid background: %v", c)
	}
	_ = draw()
	if s := p.Sprites.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 0 {
		t.Fatal("static sprite reuploaded")
	}
	sprites[0].Flip = true
	flipped := draw()
	if c := flipped[64*128+88]; c.G != 255 || c.R != 0 {
		t.Fatalf("paired sprite rotation did not flip on GPU: %v", c)
	}
	if s := p.Sprites.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 1 {
		t.Fatalf("flip updated more than UV buffer: %+v", s)
	}
	sprites[0].X = 192
	behind := draw()
	for y := 40; y < 88; y++ {
		for x := 40; x < 88; x++ {
			if c := behind[y*128+x]; c.R != 255 || c.G != 0 {
				t.Fatalf("sprite behind wall leaked through at %d,%d: %v", x, y, c)
			}
		}
	}
	// A genuine self-lit effect must stay bright in the Doom lighting mode.
	sprites[0].X, sprites[0].Light, sprites[0].Fullbright = 64, 0, true
	_ = p.Sprites.SetLightingMode(DoomLighting)
	if c := draw()[64*128+88]; c.G != 255 {
		t.Fatalf("emissive sprite inherited dark sector shading: %v", c)
	}
	// The 64-unit card at depth 64 must occupy 64 horizontal screen pixels
	// in a 128-wide viewport, matching the main renderer's focal = width/2.
	sprites[0].Texture = levelmesh.Texture{Width: 64, Height: 64, RGBA: make([]byte, 64*64*4)}
	for i := range 64 * 64 {
		sprites[0].Texture.RGBA[i*4+1], sprites[0].Texture.RGBA[i*4+3] = 255, 255
	}
	projected := draw()
	span := 0
	for x := range 128 {
		if projected[64*128+x].G > 0 {
			span++
		}
	}
	if span != 64 {
		t.Fatalf("horizontal FOV differs from Doom: card width=%d want=64", span)
	}
}
