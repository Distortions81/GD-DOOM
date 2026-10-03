package levelmesh

import (
	"bytes"
	"math"
	"testing"
)

// A triangle at constant X, facing the camera at the origin (yaw zero).
func facingTriangle(x float64, name string) Triangle {
	return Triangle{Texture: name, Vertices: [3]Vertex{{X: x, Y: 20, Z: -20}, {X: x, Y: -20, Z: -20}, {X: x, Y: 0, Z: 20}}}
}

func solidTexture(red, green, blue, alpha byte) Texture {
	return Texture{RGBA: []byte{red, green, blue, alpha}, Width: 1, Height: 1}
}

func TestDepthOrderAndAlphaHoles(t *testing.T) {
	near, far := facingTriangle(10, "near"), facingTriangle(20, "far")
	lookup := func(tri Triangle) Texture {
		if tri.Texture == "near" {
			return solidTexture(255, 0, 0, 255)
		}
		return solidTexture(0, 255, 0, 255)
	}
	var a, b Rasterizer
	a.Render([]Triangle{near, far}, 64, 64, Camera{}, Textured, lookup, nil)
	b.Render([]Triangle{far, near}, 64, 64, Camera{}, Textured, lookup, nil)
	if !bytes.Equal(a.Pixels, b.Pixels) {
		t.Fatal("frame depends on triangle submission order")
	}
	center := (32*64 + 32) * 4
	if a.Pixels[center] != 255 || a.Depth[32*64+32] != 0.1 {
		t.Fatal("near face did not win depth test")
	}
	near.Masked = true
	lookup = func(tri Triangle) Texture {
		if tri.Texture == "near" {
			return solidTexture(255, 0, 0, 0)
		}
		return solidTexture(0, 255, 0, 255)
	}
	a.Render([]Triangle{near, far}, 64, 64, Camera{}, Textured, lookup, nil)
	if a.Pixels[center+1] != 255 || a.Depth[32*64+32] != 0.05 {
		t.Fatal("alpha hole occludes rear face")
	}
}

func TestMaskedMidRepeatsWithOffsetsAndAlphaHoles(t *testing.T) {
	// A 3x2 texture exercises non-power-of-two U wrapping, V wrapping above
	// and below the image, and transparent texels in the repeated rows.
	tex := Texture{Width: 3, Height: 2, RGBA: []byte{
		20, 0, 0, 255, 40, 0, 0, 255, 60, 0, 0, 255,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}}
	for _, tc := range []struct {
		u, v       float64
		red, green byte
	}{{-0.25, 2.25, 60, 0}, {7.25, -1.75, 40, 0}, {7.25, 4.25, 40, 0}, {7.25, 3.25, 0, 255}} {
		near, far := facingTriangle(10, "mid"), facingTriangle(20, "rear")
		near.Masked = true
		for i := range near.Vertices {
			near.Vertices[i].U, near.Vertices[i].V = tc.u, tc.v
		}
		var r Rasterizer
		r.Render([]Triangle{near, far}, 64, 64, Camera{}, Textured, func(tri Triangle) Texture {
			if tri.Texture == "mid" {
				return tex
			}
			return solidTexture(0, 255, 0, 255)
		}, nil)
		p := (32*64 + 32) * 4
		if r.Pixels[p] != tc.red || r.Pixels[p+1] != tc.green {
			t.Fatalf("UV=(%g,%g): got (%d,%d) want (%d,%d)", tc.u, tc.v, r.Pixels[p], r.Pixels[p+1], tc.red, tc.green)
		}
	}
}

func TestNearClipAndBackface(t *testing.T) {
	tri := facingTriangle(10, "wall")
	tri.Vertices[0].X = 0.5
	var r Rasterizer
	r.Render([]Triangle{tri}, 64, 64, Camera{}, Textured, func(Triangle) Texture { return solidTexture(255, 255, 255, 255) }, nil)
	covered := 0
	for _, depth := range r.Depth {
		if math.IsNaN(depth) || math.IsInf(depth, 0) || depth > 0.5+1e-8 {
			t.Fatalf("invalid clipped depth %f", depth)
		}
		if depth > 0 {
			covered++
		}
	}
	if covered < 100 {
		t.Fatal("near-plane crossing triangle disappeared")
	}
	tri.Vertices[1], tri.Vertices[2] = tri.Vertices[2], tri.Vertices[1]
	r.Render([]Triangle{tri}, 64, 64, Camera{}, Textured, nil, nil)
	if r.Drawn != 0 {
		t.Fatal("back face rendered")
	}
}

func TestPerspectiveCorrectUV(t *testing.T) {
	// These project to (8,8), (56,8), (32,56). At (32.5,32.5),
	// barycentrics are 0.234375, 0.2552083, 0.5104167. The farther
	// vertex must contribute less U than affine interpolation would.
	v := [3]cameraVertex{{x: -7.5, y: -7.5, z: 10, u: 0}, {x: 30, y: -30, z: 40, u: 64}, {x: 0, y: 7.5, z: 10, u: 0}}
	var r Rasterizer
	r.Render(nil, 64, 64, Camera{}, Textured, nil, nil)
	tex := Texture{Width: 64, Height: 1, RGBA: make([]byte, 64*4)}
	for i := 0; i < 64; i++ {
		tex.RGBA[i*4] = byte(i)
		tex.RGBA[i*4+3] = 255
	}
	r.triangle(v, Camera{Focal: 32, FocalY: 32}, Triangle{}, tex, 1, Textured)
	w0, w1, w2 := 0.234375, 0.2552083333333333, 0.5104166666666666
	want := byte(math.Floor((w1 * 64 / 40) / (w0/10 + w1/40 + w2/10)))
	if got := r.Pixels[(32*64+32)*4]; got != want {
		t.Fatalf("sample U=%d want %d (affine would be 16)", got, want)
	}
}
