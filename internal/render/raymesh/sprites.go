package raymesh

import (
	"math"

	"gddoom/internal/render/levelmesh"
)

// SpriteTriangles emits camera-facing cards with Doom patch offsets. Texture
// holes are alpha-tested and use the same depth buffer as the level geometry.
func SpriteTriangles(dst []levelmesh.Triangle, sprites []levelmesh.Sprite, yaw float64) []levelmesh.Triangle {
	dst = dst[:0]
	rx, ry := math.Sin(yaw), -math.Cos(yaw)
	for i, s := range sprites {
		w, h := float64(s.Texture.Width), float64(s.Texture.Height)
		if w <= 0 || h <= 0 || len(s.Texture.RGBA) != s.Texture.Width*s.Texture.Height*4 {
			continue
		}
		sy := s.ScaleY
		if sy <= 0 {
			sy = 1
		}
		left, right := -s.OffsetX, w-s.OffsetX
		top, bottom := s.Z+s.OffsetY*sy, s.Z+(s.OffsetY-h)*sy
		u0, u1 := 0.0, w
		if s.Flip {
			u0, u1 = u1, u0
		}
		v := [4]levelmesh.Vertex{
			{X: s.X + rx*left, Y: s.Y + ry*left, Z: bottom, U: u0, V: h},
			{X: s.X + rx*right, Y: s.Y + ry*right, Z: bottom, U: u1, V: h},
			{X: s.X + rx*right, Y: s.Y + ry*right, Z: top, U: u1, V: 0},
			{X: s.X + rx*left, Y: s.Y + ry*left, Z: top, U: u0, V: 0},
		}
		kind := levelmesh.Billboard
		if s.Fullbright {
			kind = levelmesh.EmissiveBillboard
		}
		if s.Shadow {
			kind = levelmesh.ShadowBillboard
		}
		for _, indices := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
			dst = append(dst, levelmesh.Triangle{Sector: i, Sidedef: -1, Kind: kind, Masked: true, Vertices: [3]levelmesh.Vertex{v[indices[0]], v[indices[1]], v[indices[2]]}})
		}
	}
	return dst
}
