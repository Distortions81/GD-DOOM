//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"os"
	"runtime"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeDoomLightingMatchesMainSourcePortBytes(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1")
	}
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	lump, _ := wf.LumpByName("PLAYPAL")
	pal, err := wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	lump, _ = wf.LumpByName("COLORMAP")
	colormap, err := wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	palette := make([]byte, 256*4)
	indices := map[[3]byte]byte{}
	for i := range 256 {
		copy(palette[i*4:i*4+3], pal[i*3:i*3+3])
		palette[i*4+3] = 255
		indices[[3]byte{pal[i*3], pal[i*3+1], pal[i*3+2]}] = byte(i)
	}
	n := NewNativeMeshGame(&mapdata.Map{Name: "E1M1"}, Options{DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: len(colormap) / 256, SourcePortSectorLighting: true})
	if doomColormapEnabled {
		t.Fatal("main source-port default unexpectedly uses palette remapping")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(256, 128, "Source-port lighting bytes")
	if !rl.IsWindowReady() {
		t.Fatal("GPU test could not open its display")
	}
	defer rl.CloseWindow()
	r, err := raymesh.NewRendererWithTextureOptions(raymesh.TextureOptions{Scale: 1, Filter: raymesh.Nearest})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.SetLightRamp(n.LightRamp())
	r.SetLightRows(n.LightRows())
	if err := r.SetLightingMode(raymesh.DoomLighting); err != nil {
		t.Fatal(err)
	}
	tex := levelmesh.Texture{Width: 256, Height: 1, RGBA: palette}
	draw := func(kind levelmesh.Kind, masked bool, depth float64, light int16, gamma int, fullbright bool) []rl.Color {
		v := [4]levelmesh.Vertex{{X: depth, Y: -depth, Z: -depth, U: 0, V: 1}, {X: depth, Y: -depth, Z: depth, U: 0}, {X: depth, Y: depth, Z: depth, U: 256}, {X: depth, Y: depth, Z: -depth, U: 256, V: 1}}
		tris := []levelmesh.Triangle{{Kind: kind, Vertices: [3]levelmesh.Vertex{v[0], v[1], v[2]}}, {Kind: kind, Vertices: [3]levelmesh.Vertex{v[0], v[2], v[3]}}}
		for i := range tris {
			tris[i].Masked = masked
		}
		r.Sync(tris, func(levelmesh.Triangle) levelmesh.Texture { return tex }, func(int) float64 { return float64(light) / 256 }, levelmesh.Textured)
		r.SetFullbright(fullbright)
		r.SetGammaTable(doomGammaTables[gamma])
		rl.BeginDrawing()
		rl.ClearBackground(rl.Blue)
		r.Draw(levelmesh.Camera{}, 256, 128, levelmesh.Textured)
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		out := append([]rl.Color(nil), colors[64*256:65*256]...)
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		rl.EndDrawing()
		return out
	}
	base := draw(levelmesh.Floor, false, 64, 255, 0, true)
	for _, c := range base {
		if _, ok := indices[[3]byte{c.R, c.G, c.B}]; !ok {
			t.Fatalf("unshaded source pixel is not a WAD palette color: %v", c)
		}
	}
	comparisons := 0
	// Real WADs include two special rows; PWADs can replace COLORMAP with
	// fewer rows. Both cases must use the shared row-count/ramp policy.
	for _, rows := range []int{len(colormap) / 256, 32, 4} {
		n = NewNativeMeshGame(&mapdata.Map{Name: "E1M1"}, Options{DoomPaletteRGBA: palette, DoomColorMap: colormap[:rows*256], DoomColorMapRows: rows, SourcePortSectorLighting: true})
		r.SetLightRamp(n.LightRamp())
		r.SetLightRows(n.LightRows())
		for _, surface := range []struct {
			kind   levelmesh.Kind
			masked bool
		}{{levelmesh.Floor, false}, {levelmesh.Ceiling, false}, {levelmesh.Middle, false}, {levelmesh.Middle, true}, {levelmesh.Billboard, true}, {levelmesh.EmissiveBillboard, true}} {
			kind := surface.kind
			for _, depth := range []float64{8, 24, 64, 128, 256, 512, 1024} {
				for _, light := range []int16{0, 64, 128, 160, 192, 255} {
					row := doomPlaneLightRowF(light, depth)
					if kind == levelmesh.Middle {
						row = doomWallLightRowF(light, 1, depth, 128)
					}
					if kind == levelmesh.Billboard {
						row = doomWallLightRowF(light, 0, depth, 128)
					}
					mul := doomShadeMulFromRowF(row)
					if surface.masked && kind == levelmesh.Middle {
						mul, _ = n.g.maskedMidShade(light)
					}
					if kind == levelmesh.Billboard {
						mul = int(computeCutoutShadeMul(false, uint32(sectorLightMul(light)), depth, 2))
					}
					if kind == levelmesh.EmissiveBillboard {
						mul = 256
					}
					for gamma := range doomGammaLevels {
						got := draw(kind, surface.masked, depth, light, gamma, false)
						for x, c := range got {
							index := indices[[3]byte{base[x].R, base[x].G, base[x].B}]
							p := wallShadePackedBanks[gamma][mul][index]
							want := rl.Color{R: byte(p >> pixelRShift), G: byte(p >> pixelGShift), B: byte(p >> pixelBShift), A: 255}
							if c != want {
								t.Fatalf("kind=%v masked=%t depth=%g light=%d gamma=%d mul=%d x=%d got=%v main=%v", kind, surface.masked, depth, light, gamma, mul, x, c, want)
							}
						}
						comparisons++
					}
				}
			}
		}
	}
	t.Logf("%d WAD lighting/gamma draws matched all 256 source palette samples", comparisons)
}
