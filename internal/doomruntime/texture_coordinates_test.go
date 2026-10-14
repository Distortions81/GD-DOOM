package doomruntime

import (
	"fmt"
	"math"
	"testing"
)

func TestPlaneTextureCoordinatesUseDoomWorldAxes(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}, viewW: 320, viewH: 200}
	for _, angle := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2, 0.37} {
		for _, surface := range []struct {
			height int
			y      int
		}{{0, 150}, {128, 50}} {
			ca, sa := math.Cos(angle), math.Sin(angle)
			state, ok := g.planeRowRenderState(surface.y, plane3DKey{height: float64(surface.height)}, 41, 13, -27, ca, sa, 160, 160, 160, 100)
			if !ok {
				t.Fatal("missing plane")
			}
			for _, x := range []int{0, 160, 319} {
				// Intersect a camera ray with the horizontal plane, then apply the
				// original R_MapPlane convention: U=world X, V=-world Y.
				rayX, rayY := ca+sa*(float64(x)+.5-160)/160, sa-ca*(float64(x)+.5-160)/160
				rayZ := (100 - float64(surface.y) - .5) / 160
				distance := (float64(surface.height) - 41) / rayZ
				wantU, wantV := 13+distance*rayX, 27-distance*rayY
				gotU := float64(state.rowBaseWXFixed+int64(x)*state.stepWXFixed) / fracUnit
				gotV := float64(state.rowBaseWYFixed+int64(x)*state.stepWYFixed) / fracUnit
				if math.Abs(gotU-wantU) > .005 || math.Abs(gotV-wantV) > .005 {
					t.Fatalf("angle=%g plane=%d x=%d UV=(%g,%g) want (%g,%g)", angle, surface.height, x, gotU, gotV, wantU, wantV)
				}
			}
		}
	}
}

func TestMaskedColumnNeverRepeatsOutsideTexture(t *testing.T) {
	row := installTestWallShadeRow(t)
	for _, metadata := range []bool{false, true} {
		for _, mid := range []float64{-8, 2, 8, 16} {
			t.Run(fmt.Sprintf("metadata%t/mid%g", metadata, mid), func(t *testing.T) {
				tex := WallTexture{Width: 1, Height: 4, Indexed: []byte{10, 20, 30, 40}, IndexedColMajor: []byte{10, 20, 30, 40}, OpaqueMask: []byte{1, 1, 0, 1}}
				if metadata {
					tex.EnsureOpaqueColumnBounds()
				}
				g := &game{viewW: 1, viewH: 16, wallPix32: make([]uint32, 16)}
				g.drawBasicWallColumnTexturedMasked(0, 0, 15, 64, 0, mid, 64, wallTextureBlendSample{from: &tex}, 256, 0)
				for y, got := range g.wallPix32 {
					v := int((int64(mid*65536) + int64(y-8)*65535) >> 16)
					want := uint32(0)
					if v >= 0 && v < 4 && tex.OpaqueMask[v] != 0 {
						want = row[tex.Indexed[v]]
					}
					if got != want {
						t.Fatalf("y=%d v=%d pixel=%x want %x", y, v, got, want)
					}
				}
			})
		}
	}
}
