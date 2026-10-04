package raymesh

import (
	"math"
	"testing"

	"gddoom/internal/render/levelmesh"
)

func TestSpriteCardsOffsetsWindingAndFlip(t *testing.T) {
	s := levelmesh.Sprite{Texture: levelmesh.Texture{Width: 2, Height: 3, RGBA: make([]byte, 24)}, X: 100, Y: 200, Z: 10, OffsetX: 1, OffsetY: 3, ScaleY: 1.2}
	for _, yaw := range []float64{0, math.Pi / 2, math.Pi, math.Pi * 1.5} {
		tris := SpriteTriangles(nil, []levelmesh.Sprite{s}, yaw)
		if len(tris) != 2 {
			t.Fatal("sprite card missing")
		}
		a, b, c := tris[0].Vertices[0], tris[0].Vertices[1], tris[0].Vertices[2]
		if a.Z != 10 || math.Abs(c.Z-13.6) > 1e-8 {
			t.Fatal("floor anchor or aspect scale lost")
		}
		nx := (b.Y-a.Y)*(c.Z-a.Z) - (b.Z-a.Z)*(c.Y-a.Y)
		ny := (b.Z-a.Z)*(c.X-a.X) - (b.X-a.X)*(c.Z-a.Z)
		if nx*math.Cos(yaw)+ny*math.Sin(yaw) >= 0 {
			t.Fatal("card backface faces viewer")
		}
		s.Flip = true
		flipped := SpriteTriangles(nil, []levelmesh.Sprite{s}, yaw)
		for i, v := range flipped[0].Vertices {
			original := tris[0].Vertices[i]
			if v.X != original.X || v.Y != original.Y || v.Z != original.Z || v.U != 2-original.U {
				t.Fatal("sprite flip moved the card instead of mirroring its UVs")
			}
		}
		s.Flip = false
	}
}
