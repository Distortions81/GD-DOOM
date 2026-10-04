//go:build raylib && cgo && integration && !js

package raymesh

import (
	"gddoom/internal/render/levelmesh"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"os"
	"runtime"
	"slices"
	"testing"
)

func TestRaylibSpectreFuzzSharesDepthAndOrderedBackground(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for GPU fuzz checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	// Reserve a status-bar region below the world viewport, as the host does.
	rl.InitWindow(128, 160, "Spectre depth regression")
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
	red := levelmesh.Texture{RGBA: []byte{255, 0, 0, 255}, Width: 1, Height: 1}
	wall := func(depth float64) []levelmesh.Triangle {
		v := [4]levelmesh.Vertex{{X: depth, Y: -128, Z: -128}, {X: depth, Y: -128, Z: 128}, {X: depth, Y: 128, Z: 128}, {X: depth, Y: 128, Z: -128}}
		return []levelmesh.Triangle{{Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	}
	texture := func(r, g, b byte) levelmesh.Texture {
		out := levelmesh.Texture{RGBA: make([]byte, 64*64*4), Width: 64, Height: 64}
		for i := 0; i < len(out.RGBA); i += 4 {
			out.RGBA[i], out.RGBA[i+1], out.RGBA[i+2], out.RGBA[i+3] = r, g, b, 255
		}
		out.FixedRGBA = slices.Clone(out.RGBA)
		return out
	}
	green, magenta := texture(0, 255, 0), texture(255, 0, 255)
	sprite := func(depth float64, tex levelmesh.Texture, shadow bool) levelmesh.Sprite {
		return levelmesh.Sprite{Texture: tex, X: depth, Z: -32, OffsetX: 32, OffsetY: 64, Light: 1, Shadow: shadow}
	}
	phase := 0
	draw := func(sprites []levelmesh.Sprite, wallDepth float64) []rl.Color {
		world.Sync(wall(wallDepth), func(levelmesh.Triangle) levelmesh.Texture { return red }, func(int) float64 { return 1 }, levelmesh.Textured)
		p.SyncSpritesViewport(sprites, levelmesh.Camera{}, 128, 128)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Blue)
		rl.DrawRenderBatchActive()
		rl.Viewport(0, 32, 128, 128)
		world.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
		if err := p.DrawSprites(levelmesh.Camera{}, 128, 128, func(index, w, h int) ([]levelmesh.FuzzSpan, levelmesh.FuzzColors) {
			x, y, sx, sy, _, _ := levelmesh.ProjectSprite(sprites[index], levelmesh.Camera{}, w, h)
			var posts []levelmesh.FuzzSpan
			for cx := int(math.Floor(x)); cx < int(math.Ceil(x+float64(sprites[index].Texture.Width)*sx)); cx++ {
				posts = append(posts, levelmesh.FuzzSpan{X: cx, Y0: max(1, int(math.Floor(y))), Y1: min(h-2, int(math.Ceil(y+float64(sprites[index].Texture.Height)*sy))-1), Phase: phase})
			}
			colors := levelmesh.FuzzColors{Shade: .5}
			for i := range colors.Offsets {
				colors.Offsets[i] = 1
			}
			return posts, colors
		}); err != nil {
			t.Fatal(err)
		}
		rl.Viewport(0, 0, 128, 160)
		img, err := CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		out := slices.Clone(colors)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		rl.EndDrawing()
		return out
	}
	center := 64*128 + 64
	farNormal := []levelmesh.Sprite{sprite(64, magenta, true), sprite(96, green, false)}
	if c := draw(farNormal, 128)[center]; c.R != 0 || c.G != 127 || c.B != 0 {
		t.Fatalf("spectre failed to sample farther sprite: %v", c)
	}
	phase = 16
	draw(farNormal, 128)
	if s := p.Sprites.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 0 {
		t.Fatalf("phase rebuilt geometry: %+v", s)
	}
	farNormal[1].X = 32
	if c := draw(farNormal, 128)[center]; c.R != 0 || c.G != 255 || c.B != 0 {
		t.Fatalf("nearer sprite did not cover the spectre: %v", c)
	}
	blocked := draw(farNormal, 16)
	for _, c := range blocked[:128*128] {
		if c.R != 255 || c.G != 0 || c.B != 0 {
			t.Fatalf("spectre leaked through foreground wall: %v", c)
		}
	}
	for _, c := range blocked[128*128:] {
		if c != rl.Blue {
			t.Fatalf("spectre modified reserved HUD region: %v", c)
		}
	}
	if c := draw([]levelmesh.Sprite{sprite(64, magenta, true), sprite(96, magenta, true)}, 128)[center]; c.R != 63 || c.G != 0 || c.B != 0 {
		t.Fatalf("near spectre omitted farther fuzz from its snapshot: %v", c)
	}
	// Missing/offscreen shadow masks must preserve the batched normal path.
	offscreen := sprite(-64, magenta, true)
	p.SyncSpritesViewport([]levelmesh.Sprite{sprite(96, green, false), offscreen}, levelmesh.Camera{}, 128, 128)
	if len(p.ordered) != 0 {
		t.Fatal("offscreen spectre triggered snapshot ordering")
	}
	if len(p.Sprites.textures) != 2 || len(p.Sprites.fixedTextures) != 2 {
		t.Fatalf("ordered instances duplicated shared texture uploads: normal=%d inverse=%d", len(p.Sprites.textures), len(p.Sprites.fixedTextures))
	}
}
