//go:build integration

package doomruntime

import (
	"bytes"
	"fmt"
	"gddoom/internal/media"
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func gpuCompareTextureCoordinates(t *testing.T) {
	palette := make([]byte, 256*4)
	colormap := make([]byte, 32*256)
	for i := range 256 {
		palette[4*i], palette[4*i+1], palette[4*i+2], palette[4*i+3] = byte(i), byte(i*37), byte(i*71), 255
	}
	for i := range colormap {
		colormap[i] = byte(i)
	}
	tex := WallTexture{Width: 64, Height: 64, Indexed: make([]byte, 64*64), RGBA: make([]byte, 64*64*4)}
	for i := range tex.Indexed {
		tex.Indexed[i] = byte(i%64*3 + i/64*7)
		copy(tex.RGBA[i*4:], palette[int(tex.Indexed[i])*4:int(tex.Indexed[i])*4+4])
	}
	tex.ClassicMasked = make([][]media.WallPost, tex.Width)
	for x := 0; x < tex.Width; x++ {
		p := make([]byte, 128)
		for y := 0; y < 64; y++ {
			p[y] = tex.Indexed[y*tex.Width+x]
		}
		p[127] = 77
		tex.ClassicMasked[x] = []media.WallPost{{Top: 0, Length: 64, Indexed: p}}
	}
	for _, side := range []int{0, 1} {
		for _, masked := range []bool{false, true} {
			tc := wallTextureParityInput{x1: 64, y1: 128, x2: 64, y2: -128, frontSide: side, textureOffset: -17, rowOffset: 13, hasMid: true, midHeight: 64, front: wallParitySector{floorHeight: 0, ceilHeight: 128, light: 192}}
			if masked || side == 1 {
				tc.back = &wallParitySector{floorHeight: 16, ceilHeight: 112, light: 192}
			}
			m := syntheticParityMap(tc)
			m.Linedefs[0].V2 = 1
			if side == 1 {
				m.Linedefs[0].V1, m.Linedefs[0].V2 = 1, 0
				m.Sidedefs[0], m.Sidedefs[1] = m.Sidedefs[1], m.Sidedefs[0]
			}
			if !masked && side == 1 {
				m.Sectors[0].FloorHeight = 128
			}
			m.Linedefs[0].Special = 48
			g := newGame(m, Options{Width: 320, Height: 200, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: 32, WallTexBank: map[string]WallTexture{"MIDTEX": tex}})
			g.viewW, g.viewH = 320, 200
			g.renderPX, g.renderPY, g.renderAngle = 0, 0, 0
			g.playerViewZ = 41 * fracUnit
			g.setGammaLevel(0)
			dst := ebiten.NewImage(320, 200)
			draw := func(gpu bool) []byte {
				g.opts.GPURenderer = gpu
				g.wallSegStaticCache = nil
				g.ensureWallLayer()
				clear(g.wallPix)
				dst.Fill(color.Black)
				g.drawDoomBasic3D(dst)
				if gpu && (g.gpu == nil || g.gpu.failed) {
					t.Fatal("shader fallback")
				}
				p := make([]byte, 320*200*4)
				dst.ReadPixels(p)
				return p
			}
			var baseline []byte
			for _, tic := range []int{0, 1, 35, 63, 64, 129} {
				g.worldTic = tic
				m.Linedefs[0].Special = 48
				cpu, gpu := draw(false), draw(true)
				if tic == 0 {
					baseline = cpu
				} else if side == 1 && !bytes.Equal(cpu, baseline) {
					t.Fatal("back sidedef moved with the scrolling front side")
				} else if side == 0 && tic == 1 && bytes.Equal(cpu, baseline) {
					t.Fatal("fixture did not render a moving front texture")
				}
				// Oracle: P_UpdateSpecials mutates only sides[sidenum[0]].textureoffset.
				m.Linedefs[0].Special = 0
				m.Sidedefs[0].TextureOffset += int16(tic)
				want := draw(false)
				m.Sidedefs[0].TextureOffset -= int16(tic)
				label := fmt.Sprintf("side%d-masked%t-tic%d", side, masked, tic)
				if !bytes.Equal(cpu, want) {
					t.Errorf("%s CPU scroll differs from classic sidedef mutation", label)
				}
				if n := gpuPixelDifferences(cpu, gpu, 0); n != 0 {
					t.Errorf("%s CPU/GPU differ at %d pixels", label, n)
				}
			}
			dst.Deallocate()
		}
	}
}
