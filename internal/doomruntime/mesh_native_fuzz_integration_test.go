//go:build raylib && cgo && integration && !js

package doomruntime

import (
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"os"
	"runtime"
	"testing"
)

func TestNativeSpectreFuzzMatchesMainFramebuffer(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1 for actual fuzz checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(320, 200, "Native fuzz comparison")
	defer rl.CloseWindow()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	palette := make([]byte, 256*4)
	colormap := make([]byte, 33*256)
	for i := 0; i < 256; i++ {
		palette[i*4], palette[i*4+1], palette[i*4+2], palette[i*4+3] = byte(i), byte(i*53), byte(255-i), 255
		for row := 0; row < 33; row++ {
			colormap[row*256+i] = byte(i*73 + row*19)
		}
	}
	rgba := make([]byte, 8*12*4)
	for y := 0; y < 12; y++ {
		for x := 0; x < 8; x++ {
			if x != 0 && (y != 5 || x%2 == 0) {
				rgba[(y*8+x)*4+3] = 255
			}
		}
	}
	for _, variant := range []struct {
		gamma    int
		filtered bool
	}{{0, false}, {2, false}, {0, true}, {2, true}} {
		for _, size := range [][2]int{{320, 200}, {640, 400}, {960, 540}} {
			w, h := size[0], size[1]
			rl.SetWindowSize(w, h)
			// Raylib applies the framebuffer-size callback when EndDrawing polls
			// events. Render after that callback, as the interactive host does.
			for range 2 {
				rl.BeginDrawing()
				rl.ClearBackground(rl.Black)
				rl.EndDrawing()
			}
			if rl.GetRenderWidth() != w || rl.GetRenderHeight() != h {
				t.Fatal("resize was not applied")
			}
			g := newGame(&mapdata.Map{Name: "E1M1"}, Options{Width: w, Height: h, SourcePortMode: true, DisableBillboardClipping: true, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: 33})
			g.setGammaLevel(variant.gamma)
			g.viewW, g.viewH = w, h
			g.ensureWallLayer()
			n := &NativeMeshGame{g: g}
			// F11 must refresh the active palette and return to a cached bank.
			originalColors := n.SpectreFuzzColors()
			n.SetGammaLevel((variant.gamma + 1) % doomGammaLevels)
			changedColors := n.SpectreFuzzColors()
			n.SetGammaLevel(variant.gamma)
			if &n.SpectreFuzzColors().Palette.RGBA[0] != &originalColors.Palette.RGBA[0] || &changedColors.Palette.RGBA[0] == &originalColors.Palette.RGBA[0] {
				t.Fatal("gamma did not select and reuse distinct fuzz palettes")
			}
			s := levelmesh.Sprite{Texture: levelmesh.Texture{RGBA: rgba, Width: 8, Height: 12}, X: 32, Y: 24, Z: float64(h) * .35 / (float64(w) * .6 / 32), Shadow: true}
			bg := make([]byte, w*h*4)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					putGPUPackedPixel(bg, (y*w+x)*4, wallShadePackedLUT[256][(x*320/w*17+y*200/h*31)%256])
					if variant.filtered {
						// Filtered native textures can contain colors outside PLAYPAL.
						bg[(y*w+x)*4] ^= 3
						bg[(y*w+x)*4+1] ^= 5
					}
				}
			}
			img := rl.NewImage(bg, int32(w), int32(h), 1, rl.UncompressedR8g8b8a8)
			texture := rl.LoadTextureFromImage(img)
			rl.SetTextureFilter(texture, rl.FilterPoint)
			for _, flip := range []bool{false, true} {
				for _, phase := range []int{0, 16, 48} {
					for _, inverse := range []bool{false, true} {
						s.Flip = flip
						g.inventory.InvulnTics = 0
						if inverse {
							g.inventory.InvulnTics = 160
						}
						copy(g.wallPix, bg)
						g.spectreFuzzPos = phase
						x, y, sx, sy, _, _ := levelmesh.ProjectSprite(s, levelmesh.Camera{}, w, h)
						tex := WallTexture{RGBA: rgba, Width: 8, Height: 12}
						it := cutoutItem{tex: &tex, scale: sx, scaleY: sy, dstX: x, dstY: y, flip: flip, x0: int(math.Floor(x)), x1: int(math.Ceil(x+8*sx)) - 1, y0: int(math.Floor(y)), y1: int(math.Ceil(y+12*sy)) - 1}
						g.drawShadowSpriteCutout(it)
						g.spectreFuzzPos = phase
						p.SyncSpritesViewport([]levelmesh.Sprite{s}, levelmesh.Camera{}, w, h)
						rl.BeginDrawing()
						rl.ClearBackground(rl.Black)
						rl.DrawTexture(texture, 0, 0, rl.White)
						if err := p.DrawSprites(levelmesh.Camera{}, w, h, func(index, width, height int) ([]levelmesh.FuzzSpan, levelmesh.FuzzColors) {
							return n.SpectreFuzz(s, levelmesh.Camera{}, width, height), n.SpectreFuzzColors()
						}); err != nil {
							t.Fatal(err)
						}
						capture, err := raymesh.CaptureWindow()
						if err != nil {
							t.Fatal(err)
						}
						actual := rl.LoadImageColors(capture)
						if len(actual) != w*h {
							t.Fatalf("capture size=%d want %d", len(actual), w*h)
						}
						diff := 0
						for i, c := range actual {
							j := i * 4
							if c.R != g.wallPix[j] || c.G != g.wallPix[j+1] || c.B != g.wallPix[j+2] || c.A != g.wallPix[j+3] {
								if diff < 4 {
									t.Logf("fuzz mismatch (%d,%d): GPU=%v CPU=%v projection=(%g,%g) scale=(%g,%g)", i%w, i/w, c, g.wallPix[j:j+4], x, y, sx, sy)
								}
								diff++
							}
						}
						rl.UnloadImageColors(actual)
						rl.UnloadImage(capture)
						rl.EndDrawing()
						if diff != 0 {
							t.Fatalf("%s: %d pixels differ from main fuzz", fmt.Sprintf("%v gamma=%d filtered=%t phase=%d flip=%t inverse=%t", size, variant.gamma, variant.filtered, phase, flip, inverse), diff)
						}
						if st := p.Sprites.Stats(); phase != 0 && !flip && (st.MeshUploads != 0 || st.BufferUpdates != 0) {
							t.Fatal("fuzz phase changed sprite geometry")
						}
					}
				}
			}
			rl.UnloadTexture(texture)
		}
	}
}

func TestNativeWeaponFuzzMatchesClassicColumnSampling(t *testing.T) {
	if os.Getenv("GD_RAYLIB_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_INTEGRATION=1")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(320, 200, "Weapon fuzz parity")
	defer rl.CloseWindow()
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 1, Filter: raymesh.Nearest})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	palette := make([]byte, 1024)
	cm := make([]byte, 33*256)
	for i := 0; i < 256; i++ {
		palette[i*4], palette[i*4+1], palette[i*4+2], palette[i*4+3] = byte(i), byte(i*53), byte(255-i), 255
		for row := 0; row < 33; row++ {
			cm[row*256+i] = byte(i*73 + row*19)
		}
	}
	mask := make([]byte, 24*180*4)
	for i := 0; i < 24*180; i++ {
		if i/24 != 48 {
			mask[i*4+3] = 255
		}
	}
	for _, size := range [][2]int{{320, 200}, {640, 400}, {960, 540}} {
		w, h := size[0], size[1]
		rl.SetWindowSize(w, h)
		for range 2 {
			rl.BeginDrawing()
			rl.EndDrawing()
		}
		g := newGame(&mapdata.Map{Name: "E1M1"}, Options{Width: w, Height: h, SourcePortMode: true, DoomPaletteRGBA: palette, DoomColorMap: cm, DoomColorMapRows: 33})
		g.setGammaLevel(0)
		n := &NativeMeshGame{g: g, weapon: []levelmesh.Patch{{Texture: levelmesh.Texture{RGBA: mask, Width: 24, Height: 180}, X: 120, Y: 20, W: 24, H: 180}}}
		g.inventory.InvisTics = 129
		bg := make([]byte, w*h*4)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := (x*320/w*17 + y*200/h*31) % 256
				copy(bg[(y*w+x)*4:], palette[i*4:i*4+4])
			}
		}
		texture := rl.LoadTextureFromImage(rl.NewImage(bg, int32(w), int32(h), 1, rl.UncompressedR8g8b8a8))
		for _, phase := range []int{0, 16, 48} {
			for _, inv := range []int{0, 129} {
				g.inventory.InvulnTics = inv
				g.spectreFuzzPos = phase
				spans := n.WeaponFuzz(w, h)
				logical := make([]byte, 320*200*4)
				for y := 0; y < 200; y++ {
					for x := 0; x < 320; x++ {
						i := (fuzzSamplePixel(y, h, 200)*w + fuzzSamplePixel(x, w, 320)) * 4
						copy(logical[(y*320+x)*4:], bg[i:i+4])
					}
				}
				expected := append([]byte(nil), bg...)
				for _, s := range spans {
					g.spectreFuzzPos = s.Phase
					for y := s.Y0; y <= s.Y1; y++ {
						g.fuzzWeaponPixel(logical, 320, s.X, y)
						x0, x1 := fuzzCellBounds(s.X, w, 320)
						y0, y1 := fuzzCellBounds(y, h, 200)
						for yy := y0; yy <= y1; yy++ {
							for xx := x0; xx <= x1; xx++ {
								copy(expected[(yy*w+xx)*4:], logical[(y*320+s.X)*4:(y*320+s.X)*4+4])
							}
						}
					}
				}
				rl.BeginDrawing()
				rl.ClearBackground(rl.Black)
				rl.DrawTexture(texture, 0, 0, rl.White)
				if err := p.DrawWeaponFuzz(spans, n.WeaponFuzzColors(), w, h); err != nil {
					t.Fatal(err)
				}
				img, err := raymesh.CaptureWindow()
				if err != nil {
					t.Fatal(err)
				}
				colors := rl.LoadImageColors(img)
				for i, c := range colors {
					v := expected[i*4:]
					if c.R != v[0] || c.G != v[1] || c.B != v[2] {
						t.Fatalf("%v phase=%d invuln=%d pixel %d,%d: %v want %v", size, phase, inv, i%w, i/w, c, v[:4])
					}
				}
				rl.UnloadImageColors(colors)
				rl.UnloadImage(img)
				rl.EndDrawing()
			}
		}
		rl.UnloadTexture(texture)
	}
}
