//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"os"
	"runtime"
	"testing"

	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeAnimatedWADFlatMatchesMainBlendPixels(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for real WAD animation pixels")
	}
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	fixture := loadMeshExperimentMap(t, "E1M3")
	opts := fixture.opts
	opts.FlatBankIndexed, err = doomtex.LoadFlatsIndexed(wf)
	if err != nil {
		t.Fatal(err)
	}
	opts.FlatTextureAnimSequences = doomtex.LoadFlatAnimSequences(wf, doomtex.DoomFlatAnimDefs)
	opts.TextureAnimCrossfadeFrames = 7
	// Choose a changing WAD texel so this fails if the shader silently samples
	// only the first frame, rather than passing on a static green background.
	first, second := opts.FlatBank["NUKAGE1"], opts.FlatBank["NUKAGE2"]
	best, difference := 0, 0
	for i := 0; i < 64*64; i++ {
		d := 0
		for channel := range 3 {
			a := int(first[i*4+channel]) - int(second[i*4+channel])
			if a < 0 {
				a = -a
			}
			d += a
		}
		if d > difference {
			best, difference = i, d
		}
	}
	if difference < 64 {
		t.Fatal("fixture has no sufficiently changing animation texel")
	}
	u, v := best%64, best/64
	n := NewNativeMeshGame(fixture.m, opts)
	n.SetGammaLevel(0)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(64, 64, "Real WAD floor crossfade")
	defer rl.CloseWindow()
	r, err := raymesh.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.SetLightingMode(raymesh.FullbrightLighting)
	var corners = [4]levelmesh.Vertex{{X: 64, Y: -64, Z: -64}, {X: 64, Y: -64, Z: 64}, {X: 64, Y: 64, Z: 64}, {X: 64, Y: 64, Z: -64}}
	for i := range corners {
		corners[i].U, corners[i].V = float64(u)+.5, float64(v)+.5
	}
	quad := []levelmesh.Triangle{{Kind: levelmesh.Floor, Texture: "NUKAGE3", Vertices: [3]levelmesh.Vertex{corners[0], corners[1], corners[2]}}, {Kind: levelmesh.Floor, Texture: "NUKAGE3", Vertices: [3]levelmesh.Vertex{corners[0], corners[2], corners[3]}}}
	blended := 0
	for tic := 0; tic < 32; tic++ {
		n.g.worldTic = tic
		for _, alpha := range []float64{0, .25, .75, 1} {
			frame := n.Frame(alpha)
			found := false
			for _, tri := range frame.Triangles {
				if tri.Kind == levelmesh.Floor && tri.Texture == "NUKAGE3" {
					found = true
					break
				}
			}
			if !found {
				t.Fatal("real E1M3 mesh lost its animated NUKAGE3 floor")
			}
			mainSample, ok := n.g.flatTextureBlend("NUKAGE3")
			if !ok {
				t.Fatal("WAD animation sample unavailable")
			}
			r.Sync(quad, n.Texture, func(int) float64 { return 1 }, levelmesh.Textured)
			rl.BeginDrawing()
			rl.ClearBackground(rl.Blue)
			r.Draw(levelmesh.Camera{}, 64, 64, levelmesh.Textured)
			img, err := raymesh.CaptureWindow()
			if err != nil {
				t.Fatal(err)
			}
			colors := rl.LoadImageColors(img)
			got := colors[32*64+32]
			packed := sampleFlatBlendPacked(mainSample, u, v)
			if mainSample.alpha > 0 && len(mainSample.toRGBA) > 0 {
				i := best * 4
				from := packRGBA(mainSample.fromRGBA[i], mainSample.fromRGBA[i+1], mainSample.fromRGBA[i+2])
				to := packRGBA(mainSample.toRGBA[i], mainSample.toRGBA[i+1], mainSample.toRGBA[i+2])
				if packed != from && packed != to {
					blended++
				}
			}
			want := rl.Color{R: byte(packed >> pixelRShift), G: byte(packed >> pixelGShift), B: byte(packed >> pixelBShift), A: 255}
			if got != want {
				t.Fatalf("tic=%d alpha=%g native=%v main=%v", tic, alpha, got, want)
			}
			rl.UnloadImageColors(colors)
			rl.UnloadImage(img)
			rl.EndDrawing()
		}
	}
	if blended < 8 {
		t.Fatal("comparison did not exercise real intermediate WAD colors")
	}
	if r.Stats().ResidentMeshes > 6 {
		t.Fatal("animation weights accumulated resident geometry")
	}
}
