//go:build integration

package doomruntime

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"math"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// Original secondary shader from commit 3399c93. Keep this independent of the
// optimized implementation so sampling, seams and arithmetic have a GPU oracle.
//
//go:embed shaders/testdata/sky_backdrop_reference.kage
var skyBackdropReference []byte

type skyShaderCase struct {
	width, height, sampleW, sampleH int
	angle                           float64
	sharp                           bool
}

func skyShaderTexture(w, h int) (*ebiten.Image, *ebiten.Image) {
	// A nonzero source origin and distinctive edge texels expose incorrect
	// neighbor reuse across horizontal/vertical wraps.
	parent := newUnmanagedImage(w+9, h+11)
	source := parent.SubImage(image.Rect(3, 5, 3+w, 5+h)).(*ebiten.Image)
	pixels := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			i := (y*w + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = byte(x*37+y*13), byte(x*7+y*89), byte(x*73+y*31), 255
		}
	}
	source.WritePixels(pixels)
	return parent, source
}

func drawSkyShader(dst, source *ebiten.Image, shader *ebiten.Shader, c skyShaderCase) {
	w, h := float32(c.width), float32(c.height)
	tw, th := float32(source.Bounds().Dx()), float32(source.Bounds().Dy())
	vertices := []ebiten.Vertex{
		{DstX: 0, DstY: 0, SrcX: 0, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: 0, Custom1: 0},
		{DstX: w, DstY: 0, SrcX: tw, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: w, Custom1: 0},
		{DstX: 0, DstY: h, SrcX: 0, SrcY: th, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: 0, Custom1: h},
		{DstX: w, DstY: h, SrcX: tw, SrcY: th, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1, Custom0: w, Custom1: h},
	}
	sharp := float64(0)
	if c.sharp {
		sharp = 1
	}
	op := ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{source}, Uniforms: map[string]any{
		"CamAngle": c.angle, "Focal": doomFocalLength(c.sampleW), "DrawW": float64(c.width), "DrawH": float64(c.height),
		"SampleW": float64(c.sampleW), "SampleH": float64(c.sampleH), "SkyTexW": float64(tw), "SkyTexH": float64(th), "SharpUpscale": sharp,
	}}
	dst.DrawTrianglesShader(vertices, []uint16{0, 1, 2, 1, 2, 3}, shader, &op)
}

func TestGPUSkyShaderParity(t *testing.T) {
	if os.Getenv("GD_GPU_SKY_INTEGRATION") == "" {
		t.Skip("set GD_GPU_SKY_INTEGRATION=1")
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		reference, err := ebiten.NewShader(skyBackdropReference)
		if err != nil {
			t.Fatal(err)
		}
		defer reference.Deallocate()
		type candidate struct {
			name                  string
			shader                *ebiten.Shader
			differences, maxDelta int
		}
		var candidates []*candidate
		for _, variant := range []struct {
			name   string
			source []byte
		}{{"cached56", skyBackdropShaderSrc}} {
			shader, err := ebiten.NewShader(variant.source)
			if err != nil {
				t.Fatal(err)
			}
			defer shader.Deallocate()
			candidates = append(candidates, &candidate{name: variant.name, shader: shader})
		}
		comparisons := 0
		for _, textureSize := range [][2]int{{256, 128}, {257, 129}, {7, 15}, {1, 1}} {
			parent, source := skyShaderTexture(textureSize[0], textureSize[1])
			for _, size := range [][4]int{{320, 200, 320, 200}, {511, 383, 255, 191}, {639, 359, 320, 180}} {
				wantImage, gotImage := newUnmanagedImage(size[0], size[1]), newUnmanagedImage(size[0], size[1])
				want, got := make([]byte, size[0]*size[1]*4), make([]byte, size[0]*size[1]*4)
				for _, angle := range []float64{0, -.000001, 2*math.Pi - .000001, math.Pi/2 + .000001} {
					for _, sharp := range []bool{false, true} {
						c := skyShaderCase{size[0], size[1], size[2], size[3], angle, sharp}
						drawSkyShader(wantImage, source, reference, c)
						wantImage.ReadPixels(want)
						for _, candidate := range candidates {
							drawSkyShader(gotImage, source, candidate.shader, c)
							gotImage.ReadPixels(got)
							if !bytes.Equal(want, got) {
								for i := range want {
									if want[i] != got[i] {
										candidate.differences++
										delta := int(want[i]) - int(got[i])
										if delta < 0 {
											delta = -delta
										}
										if delta > candidate.maxDelta {
											candidate.maxDelta = delta
										}
									}
								}
							}
						}
						comparisons++
					}
				}
				wantImage.Deallocate()
				gotImage.Deallocate()
			}
			parent.Deallocate()
		}
		for _, candidate := range candidates {
			t.Logf("%s: %d cases differences=%d maxDelta=%d", candidate.name, comparisons, candidate.differences, candidate.maxDelta)
			if candidate.differences != 0 {
				t.Errorf("%s changed the reference framebuffer", candidate.name)
			}
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

// ReadPixels synchronizes each full draw. Its transfer cost is shared by all
// variants; neither readback nor reference shaders are used during gameplay.
func BenchmarkGPUSkyDraw(b *testing.B) {
	driver := &gpuComparisonDriver{}
	driver.run = func() {
		variants := []struct {
			name   string
			source []byte
		}{{"original80", skyBackdropReference}, {"cached56", skyBackdropShaderSrc}}
		if os.Getenv("GD_GPU_SKY_REVERSE") != "" {
			variants[0], variants[1] = variants[1], variants[0]
		}
		parent, source := skyShaderTexture(256, 128)
		defer parent.Deallocate()
		for _, variant := range variants {
			shader, err := ebiten.NewShader(variant.source)
			if err != nil {
				b.Fatal(err)
			}
			for _, size := range [][2]int{{320, 200}, {1920, 1080}} {
				dst := newUnmanagedImage(size[0], size[1])
				pixels := make([]byte, size[0]*size[1]*4)
				for _, sharp := range []bool{false, true} {
					mode := "point"
					if sharp {
						mode = "sharp"
					}
					c := skyShaderCase{size[0], size[1], size[0], size[1], 1.13, sharp}
					drawSkyShader(dst, source, shader, c)
					dst.ReadPixels(pixels)
					b.Run(fmt.Sprintf("%s/%dx%d/%s", variant.name, size[0], size[1], mode), func(b *testing.B) {
						b.ReportAllocs()
						b.ResetTimer()
						for range b.N {
							drawSkyShader(dst, source, shader, c)
							dst.ReadPixels(pixels)
						}
					})
				}
				dst.Deallocate()
			}
			shader.Deallocate()
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		b.Fatal(err)
	}
}
