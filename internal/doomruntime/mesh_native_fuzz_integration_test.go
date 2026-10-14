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
