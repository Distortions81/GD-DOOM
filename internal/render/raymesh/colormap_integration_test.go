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

func TestRaylibFixedColormapRetainsGeometryAndCutouts(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for framebuffer checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(128, 128, "Fixed colormap regression")
	defer rl.CloseWindow()
	r, err := NewRendererWithTextureOptions(TextureOptions{Scale: 2, Filter: Nearest})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.SetLightingMode(DoomLighting); err != nil {
		t.Fatal(err)
	}
	tex := levelmesh.Texture{Width: 2, Height: 1, RGBA: []byte{255, 30, 70, 255, 255, 30, 70, 0}}
	variant := []byte{220, 220, 220, 255, 220, 220, 220, 0}
	v := [4]levelmesh.Vertex{{X: 64, Y: -64, Z: -64, U: 0, V: 1}, {X: 64, Y: -64, Z: 64, U: 0}, {X: 64, Y: 64, Z: 64, U: 2}, {X: 64, Y: 64, Z: -64, U: 2, V: 1}}
	tris := []levelmesh.Triangle{{Masked: true, Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Masked: true, Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	draw := func(fixed bool) []rl.Color {
		tex.FixedRGBA = nil
		if fixed {
			tex.FixedRGBA = variant
		}
		r.Sync(tris, func(levelmesh.Triangle) levelmesh.Texture { return tex }, func(int) float64 { return .1 }, levelmesh.Textured)
		r.SetFixedColormap(fixed)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Blue)
		r.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
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
	base := draw(false)
	inverse := draw(true)
	if s := r.Stats(); s.MeshUploads != 0 || s.BufferUpdates != 0 {
		t.Fatalf("fixed palette rebuilt geometry: %+v", s)
	}
	gray, blue := 0, 0
	for i, c := range inverse {
		if c == (rl.Color{R: 220, G: 220, B: 220, A: 255}) {
			gray++
			if base[i].R >= 220 {
				t.Fatal("fixed palette did not override dark Doom shading")
			}
		}
		if c == rl.Blue {
			blue++
		}
	}
	if gray < 3000 || blue < 3000 {
		t.Fatalf("fixed texture or alpha holes missing: gray=%d blue=%d", gray, blue)
	}
	if len(r.fixedTextures) != 1 || len(r.meshes) != 1 {
		t.Fatal("fixed variant created extra geometry batches")
	}
	key := fixedTextureKey{Batch: r.active[0].Key, Pixels: &variant[0]}
	textureID := r.fixedTextures[key].ID
	for _, filter := range []TextureFilter{Trilinear, Anisotropic, Nearest} {
		if err := r.SetTextureFilter(filter); err != nil {
			t.Fatal(err)
		}
		draw(false)
		draw(true)
		if r.fixedTextures[key].ID != textureID || r.Stats().MeshUploads != 0 || r.Stats().BufferUpdates != 0 {
			t.Fatal("blink/filter changes replaced cached geometry or textures")
		}
	}
	if restored := draw(false); !slices.Equal(restored, base) {
		t.Fatal("blink-off failed to restore original pixels")
	}
	// A new immutable colormap image must update the selected texture while
	// retaining the same meshes; returning to the old image must reuse its ID.
	originalVariant := variant
	variant = []byte{130, 130, 130, 255, 130, 130, 130, 0}
	changed := draw(true)
	if changed[64*128+32] == inverse[64*128+32] && changed[64*128+96] == inverse[64*128+96] {
		t.Fatal("new fixed-colormap metadata was ignored by a cached batch")
	}
	if r.Stats().MeshUploads != 0 || r.Stats().BufferUpdates != 0 {
		t.Fatal("new colormap rebuilt geometry")
	}
	variant = originalVariant
	if restored := draw(true); !slices.Equal(restored, inverse) || r.fixedTextures[key].ID != textureID {
		t.Fatal("returning to the cached fixed colormap changed its pixels or texture")
	}
}
