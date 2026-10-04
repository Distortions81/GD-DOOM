package raymesh

import (
	"testing"

	"gddoom/internal/render/levelmesh"
)

func TestBatchesPreserveDoomSurfaceLightWithoutExtraDrawCalls(t *testing.T) {
	tex := levelmesh.Texture{Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}}
	tris := []levelmesh.Triangle{
		{Kind: levelmesh.Middle, Vertices: [3]levelmesh.Vertex{{X: 0}, {X: 64}, {X: 64, Z: 128}}},
		{Kind: levelmesh.Upper, Vertices: [3]levelmesh.Vertex{{Y: 0}, {Y: 64}, {Y: 64, Z: 128}}},
		{Kind: levelmesh.Lower, Vertices: [3]levelmesh.Vertex{{}, {X: 64, Y: 64}, {X: 64, Y: 64, Z: 128}}},
		{Kind: levelmesh.Floor, Vertices: [3]levelmesh.Vertex{{}, {X: 64}, {Y: 64}}},
		{Kind: levelmesh.Ceiling, Vertices: [3]levelmesh.Vertex{{Z: 128}, {Y: 64, Z: 128}, {X: 64, Z: 128}}},
	}
	var builder Builder
	b := builder.Build(tris, func(levelmesh.Triangle) levelmesh.Texture { return tex }, func(int) float64 { return 160.0 / 256 }, levelmesh.Textured)
	if len(b) != 1 {
		t.Fatal("surface light classification split the shared texture batch")
	}
	for tri, want := range []float32{-1, 1, 0, 4, 4} {
		for vertex := range 3 {
			i := tri*3 + vertex
			if b[0].LightingUVs[i*2] != want || b[0].Colors[i*4+3] != 160 {
				t.Fatal("batch lost original sector light or Doom wall direction")
			}
		}
	}
}
