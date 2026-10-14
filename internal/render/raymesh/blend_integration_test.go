//go:build raylib && cgo && integration && !js

package raymesh

import (
	"os"
	"runtime"
	"testing"

	"gddoom/internal/render/levelmesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestRaylibTextureCrossfadeStaysResidentAndPreservesMasks(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual crossfade shader checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(128, 128, "Resident texture crossfade regression")
	defer rl.CloseWindow()
	r, err := NewRendererWithTextureOptions(TextureOptions{Scale: 2, Filter: Nearest})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.SetLightingMode(FullbrightLighting)
	from, to := [3]byte{64, 128, 192}, [3]byte{224, 48, 96}
	fixedFrom, fixedTo := [3]byte{170, 170, 170}, [3]byte{40, 40, 40}
	tex := levelmesh.Texture{Width: 4, Height: 1, BlendAlpha: 1}
	for i := range 4 {
		a, b := byte(255), byte(255)
		if i >= 2 {
			a = 0
		}
		if i == 1 || i == 3 {
			b = 0
		}
		tex.RGBA = append(tex.RGBA, from[0], from[1], from[2], a)
		tex.BlendRGBA = append(tex.BlendRGBA, to[0], to[1], to[2], b)
		tex.FixedRGBA = append(tex.FixedRGBA, fixedFrom[0], fixedFrom[1], fixedFrom[2], a)
		tex.BlendFixedRGBA = append(tex.BlendFixedRGBA, fixedTo[0], fixedTo[1], fixedTo[2], b)
	}
	v := [4]levelmesh.Vertex{{X: 64, Y: -64, Z: -64, U: 0, V: 1}, {X: 64, Y: -64, Z: 64, U: 0}, {X: 64, Y: 64, Z: 64, U: 4}, {X: 64, Y: 64, Z: -64, U: 4, V: 1}}
	tris := []levelmesh.Triangle{{Masked: true, Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Masked: true, Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
	lookup := func(levelmesh.Triangle) levelmesh.Texture { return tex }
	r.Sync(tris, lookup, func(int) float64 { return 1 }, levelmesh.Textured)
	if len(r.meshes) != 1 || len(r.textures) != 2 || len(r.fixedTextures) != 2 {
		t.Fatal("crossfade failed to retain both normal and fixed frames")
	}
	ids := map[BatchKey]uint32{}
	for key, value := range r.textures {
		ids[key] = value.ID
	}
	// Every weight must reuse the same immutable mesh and texture uploads.
	for alpha := 1; alpha <= 255; alpha++ {
		tex.BlendAlpha = uint8(alpha)
		r.Sync(tris, lookup, func(int) float64 { return 1 }, levelmesh.Textured)
		if r.Stats().MeshUploads != 0 || r.Stats().BufferUpdates != 0 || len(r.meshes) != 1 {
			t.Fatal("blend weight rebuilt or rewrote resident mesh buffers")
		}
	}
	for _, filter := range []TextureFilter{Nearest, Trilinear, Anisotropic, Nearest} {
		if err := r.SetTextureFilter(filter); err != nil {
			t.Fatal(err)
		}
		for _, fixed := range []bool{false, true} {
			r.SetFixedColormap(fixed)
			for _, gamma := range []bool{false, true} {
				var table [256]uint8
				for i := range table {
					table[i] = byte(i)
					if gamma {
						table[i] = byte(255 - i)
					}
				}
				r.SetGammaTable(table)
				for _, alpha := range []uint8{1, 37, 128, 254, 255} {
					tex.BlendAlpha = alpha
					r.Sync(tris, lookup, func(int) float64 { return 1 }, levelmesh.Textured)
					rl.BeginDrawing()
					rl.ClearBackground(rl.Blue)
					r.Draw(levelmesh.Camera{}, 128, 128, levelmesh.Textured)
					img, err := CaptureWindow()
					if err != nil {
						t.Fatal(err)
					}
					colors := rl.LoadImageColors(img)
					a, b := from, to
					if fixed {
						a, b = fixedFrom, fixedTo
					}
					want := rl.Color{A: 255}
					ch := []*byte{&want.R, &want.G, &want.B}
					for i := range 3 {
						value := byte((uint32(a[i])*(255-uint32(alpha)) + uint32(b[i])*uint32(alpha) + 127) / 255)
						if gamma && !fixed {
							value = table[value]
						}
						*ch[i] = value
					}
					for _, x := range []int{48, 80, 112} {
						if got := colors[64*128+x]; got != want {
							t.Fatalf("filter=%s fixed=%v gamma=%v alpha=%d x=%d got=%v want=%v", filter, fixed, gamma, alpha, x, got, want)
						}
					}
					if colors[64*128+16] != rl.Blue {
						t.Fatal("union of cutouts occluded a hole shared by both frames")
					}
					rl.UnloadImageColors(colors)
					rl.UnloadImage(img)
					rl.EndDrawing()
					if r.Stats().MeshUploads != 0 || r.Stats().BufferUpdates != 0 {
						t.Fatal("draw changed resident geometry")
					}
				}
			}
		}
	}
	for key, id := range ids {
		if r.textures[key].ID != id {
			t.Fatal("filter/blink replaced resident animation frames")
		}
	}
}
