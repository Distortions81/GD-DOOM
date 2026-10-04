package raymesh

import (
	"testing"

	"gddoom/internal/render/levelmesh"
)

func TestBatchesPreserveWindingUVAndResolvedMaterials(t *testing.T) {
	tex := levelmesh.Texture{RGBA: make([]byte, 64*128*4), Width: 64, Height: 128}
	t1 := levelmesh.Triangle{Sector: 7, Kind: levelmesh.Floor, Vertices: [3]levelmesh.Vertex{{X: 0, Y: 0, Z: 48, U: 0, V: 0}, {X: 64, Y: 0, Z: 48, U: 64, V: 0}, {X: 0, Y: 64, Z: 48, U: 0, V: 128}}}
	t2 := t1
	t2.Sidedef = 10
	sky := t1
	sky.Sky = true
	masked := t1
	masked.Masked = true
	var builder Builder
	b := builder.Build([]levelmesh.Triangle{t1, t2, sky, masked}, func(levelmesh.Triangle) levelmesh.Texture { return tex }, func(int) float64 { return 1 }, levelmesh.Textured)
	if len(b) != 2 || len(b[0].Positions) != 18 || len(b[1].Positions) != 9 {
		t.Fatalf("incorrect batching: %+v", b)
	}
	p, uv := b[0].Positions, b[0].UVs
	if p[0] != 0 || p[1] != 0.75 || p[2] != 0 || p[3] != 1 || p[4] != 0.75 || p[5] != 0 || p[6] != 0 || p[7] != 0.75 || p[8] != -1 || uv[2] != 1 || uv[5] != 1 {
		t.Fatalf("coordinate or UV conversion: %v %v", p, uv)
	}
	// A floor's upward normal must remain upward after conversion.
	if normalY := (p[5]-p[2])*(p[6]-p[0]) - (p[3]-p[0])*(p[8]-p[2]); normalY <= 0 {
		t.Fatal("GPU coordinate conversion reversed plane winding")
	}
	old := b[0]
	b = builder.Build([]levelmesh.Triangle{t1}, func(levelmesh.Triangle) levelmesh.Texture { return tex }, func(int) float64 { return 1 }, levelmesh.Textured)
	if len(b) != 1 || b[0] != old || len(b[0].Positions) != 9 {
		t.Fatal("static batch wasn't reused or retained stale triangles")
	}
	if len(builder.Build(nil, func(levelmesh.Triangle) levelmesh.Texture { return tex }, func(int) float64 { return 1 }, levelmesh.Textured)) != 0 {
		t.Fatal("closed geometry still active")
	}
}

func TestTextureBatchPreservesDifferentSectorLighting(t *testing.T) {
	tex := levelmesh.Texture{RGBA: []byte{255, 255, 255, 255}, Width: 1, Height: 1}
	var builder Builder
	tris := []levelmesh.Triangle{{Sector: 7}, {Sector: 8}}
	texture := func(levelmesh.Triangle) levelmesh.Texture { return tex }
	light := func(sector int) float64 {
		if sector == 7 {
			return 0.5
		}
		return 1
	}
	b := builder.Build(tris, texture, light, levelmesh.Textured)
	if len(b) != 1 {
		t.Fatal("different sector lighting prevented shared texture batching")
	}
	for vertex := range 6 {
		want := byte(127)
		if vertex >= 3 {
			want = 255
		}
		for component := range 3 {
			if b[0].Colors[vertex*4+component] != want {
				t.Fatal("batch merged away individual sector lighting")
			}
		}
	}
}

func TestCrossfadeBatchesKeepIndependentSwitchWeightsAndResidentIdentity(t *testing.T) {
	tex := levelmesh.Texture{RGBA: []byte{255, 0, 0, 255}, Width: 1, Height: 1, BlendRGBA: []byte{0, 255, 0, 255}, BlendAlpha: 64}
	var builder Builder
	tris := []levelmesh.Triangle{{Sidedef: 1}, {Sidedef: 2}}
	lookup := func(t levelmesh.Triangle) levelmesh.Texture {
		out := tex
		out.BlendInstance, out.BlendAlpha = t.Sidedef, tex.BlendAlpha+uint8(t.Sidedef)
		return out
	}
	b := builder.Build(tris, lookup, func(int) float64 { return 1 }, levelmesh.Textured)
	if len(b) != 2 || b[0].Texture.BlendAlpha == b[1].Texture.BlendAlpha || b[0].Key.textureBatch() != b[1].Key.textureBatch() {
		t.Fatal("switch phases were merged or duplicate source uploads were requested")
	}
	a, other := b[0], b[1]
	for alpha := 1; alpha < 250; alpha++ {
		tex.BlendAlpha = uint8(alpha)
		b = builder.Build(tris, lookup, func(int) float64 { return 1 }, levelmesh.Textured)
		if len(builder.batches) != 2 || b[0] != a || b[1] != other || b[0].Texture.BlendAlpha != uint8(alpha+1) {
			t.Fatal("blend weight changed resident batch identity or retained a stale weight")
		}
	}
}
