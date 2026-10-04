package levelmesh

import "math"

// ProjectSprite follows the resident camera's 90-degree FOV and pixel aspect.
// The returned scale includes patch offsets and any aspect-exempt sprite scale.
func ProjectSprite(s Sprite, c Camera, width, height int) (x, y, sx, sy, depth float64, visible bool) {
	if width <= 0 || height <= 0 || s.Texture.Width <= 0 || s.Texture.Height <= 0 {
		return
	}
	dx, dy := s.X-c.X, s.Y-c.Y
	ca, sa := math.Cos(c.Yaw), math.Sin(c.Yaw)
	depth = dx*ca + dy*sa
	if depth < 2 {
		return
	}
	scaleY := s.ScaleY
	if scaleY <= 0 {
		scaleY = 1
	}
	sx = float64(width) * .5 / depth
	sy = sx * 1.2 * scaleY
	x = float64(width)*.5 + (dx*sa-dy*ca)*sx - s.OffsetX*sx
	y = float64(height)*.5 - (s.Z-c.Z)*sx*1.2 - s.OffsetY*sy
	visible = x < float64(width) && y < float64(height) && x+float64(s.Texture.Width)*sx > 0 && y+float64(s.Texture.Height)*sy > 0
	return
}
