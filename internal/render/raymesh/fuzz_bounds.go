package raymesh

import (
	"gddoom/internal/render/levelmesh"
	"math"
)

// The shared fuzz walker clips logical posts to floor/ceil pixel bounds. Snap
// only shadow-card edges to those same bounds; ordinary sprites keep their
// original subpixel geometry and filtering.
func snapFuzzBounds(tri *levelmesh.Triangle, s levelmesh.Sprite, c levelmesh.Camera, width, height int) {
	x, y, sx, sy, depth, visible := levelmesh.ProjectSprite(s, c, width, height)
	if !visible {
		return
	}
	left, right := math.Floor(x), math.Ceil(x+float64(s.Texture.Width)*sx)
	top, bottom := math.Floor(y), math.Ceil(y+float64(s.Texture.Height)*sy)
	ca, sa := math.Cos(c.Yaw), math.Sin(c.Yaw)
	focal := float64(width) * .5
	for i, v := range tri.Vertices {
		px := float64(width)*.5 + ((v.X-c.X)*sa-(v.Y-c.Y)*ca)*focal/depth
		py := float64(height)*.5 - (v.Z-c.Z)*focal*1.2/depth
		if px < x+float64(s.Texture.Width)*sx*.5 {
			px = left
		} else {
			px = right
		}
		if py < y+float64(s.Texture.Height)*sy*.5 {
			py = top
		} else {
			py = bottom
		}
		side := (px - float64(width)*.5) * depth / focal
		v.X, v.Y = c.X+ca*depth+sa*side, c.Y+sa*depth-ca*side
		v.Z = c.Z + (float64(height)*.5-py)*depth/(focal*1.2)
		tri.Vertices[i] = v
	}
}
