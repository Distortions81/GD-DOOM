package presenter

import (
	"gddoom/internal/render/mapview"
	"math"
)

// AppendThingGlyph exposes the same map symbols to alternate drawing backends.
func AppendThingGlyph(dst []mapview.Segment, style ThingStyle, sx, sy float64, angleDeg int16, size float64) []mapview.Segment {
	line := func(ax, ay, bx, by, width float64) {
		dst = append(dst, mapview.Segment{X1: sx + ax, Y1: sy + ay, X2: sx + bx, Y2: sy + by, Width: float32(width), Color: style.Color})
	}
	cross := func(r float64) { line(-r, 0, r, 0, 1.5); line(0, -r, 0, r, 1.5) }
	switch style.Glyph {
	case GlyphSquare:
		r := size * .9
		line(-r, -r, r, -r, 1.4)
		line(r, -r, r, r, 1.4)
		line(r, r, -r, r, 1.4)
		line(-r, r, -r, -r, 1.4)
	case GlyphDiamond:
		r := size
		line(0, -r, r, 0, 1.4)
		line(r, 0, 0, r, 1.4)
		line(0, r, -r, 0, 1.4)
		line(-r, 0, 0, -r, 1.4)
	case GlyphTriangle:
		r := size * 1.15
		a := float64(angleDeg) * math.Pi / 180
		ax, ay := rotatePoint(0, -r, a)
		bx, by := rotatePoint(r*.85, r*.8, a)
		cx, cy := rotatePoint(-r*.85, r*.8, a)
		line(ax, ay, bx, by, 1.4)
		line(bx, by, cx, cy, 1.4)
		line(cx, cy, ax, ay, 1.4)
	case GlyphStar:
		r := size * 1.1
		cross(r)
		line(-r*.7, -r*.7, r*.7, r*.7, 1.3)
		line(-r*.7, r*.7, r*.7, -r*.7, 1.3)
	default:
		cross(size * .8)
	}
	return dst
}

func rotatePoint(x, y, angleRad float64) (float64, float64) {
	c, s := math.Cos(angleRad), math.Sin(angleRad)
	return x*c - y*s, x*s + y*c
}
