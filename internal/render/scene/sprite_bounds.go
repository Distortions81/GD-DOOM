package scene

import "math"

func FloorSpriteTop(dstH, yb float64) float64 {
	return yb - dstH
}

func ClampedSpriteBounds(dstX, dstY, dstW, dstH float64, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 || dstW <= 0 || dstH <= 0 {
		return 0, -1, 0, -1, false
	}
	x0 := int(math.Floor(dstX))
	y0 := int(math.Floor(dstY))
	x1 := int(math.Ceil(dstX+dstW)) - 1
	y1 := int(math.Ceil(dstY+dstH)) - 1
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 >= viewW {
		x1 = viewW - 1
	}
	if y1 >= viewH {
		y1 = viewH - 1
	}
	if y0 < clipTop {
		y0 = clipTop
	}
	if y1 > clipBottom {
		y1 = clipBottom
	}
	if x0 > x1 || y0 > y1 {
		return x0, x1, y0, y1, false
	}
	return x0, x1, y0, y1, true
}

func SpritePatchBounds(sx, yb, worldH float64, texW, texH, offsetX int, clipTop, clipBottom, viewW, viewH int, floorAnchor bool) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 || texW <= 0 || texH <= 0 {
		return 0, -1, 0, -1, false
	}
	scale := worldH / float64(texH)
	if scale <= 0 {
		return 0, -1, 0, -1, false
	}
	dstW := float64(texW) * scale
	dstH := float64(texH) * scale
	dstX := sx - float64(offsetX)*scale
	dstY := yb - float64(0)
	if floorAnchor {
		dstY = FloorSpriteTop(dstH, yb)
	}
	return ClampedSpriteBounds(dstX, dstY, dstW, dstH, clipTop, clipBottom, viewW, viewH)
}

func ProjectileFallbackBounds(sx, yb, worldH float64, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 {
		return 0, -1, 0, -1, false
	}
	rad := worldH * 0.5
	if rad <= 0 {
		return 0, -1, 0, -1, false
	}
	cy := yb - rad
	dstX := sx - rad
	dstY := cy - rad
	dstW := rad * 2
	dstH := rad * 2
	return ClampedSpriteBounds(dstX, dstY, dstW, dstH, clipTop, clipBottom, viewW, viewH)
}

func SpritePatchBoundsFromScale(sx, sy, scale float64, texW, texH, offsetX, offsetY int, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 || texW <= 0 || texH <= 0 || scale <= 0 {
		return 0, -1, 0, -1, false
	}
	dstW := float64(texW) * scale
	dstH := float64(texH) * scale
	dstX := sx - float64(offsetX)*scale
	dstY := sy - float64(offsetY)*scale
	return ClampedSpriteBounds(dstX, dstY, dstW, dstH, clipTop, clipBottom, viewW, viewH)
}

func CircleScreenBounds(sx, sy, r float64, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 || r <= 0 {
		return 0, -1, 0, -1, false
	}
	dstX := sx - r
	dstY := sy - r
	dstW := r * 2
	dstH := r * 2
	return ClampedSpriteBounds(dstX, dstY, dstW, dstH, clipTop, clipBottom, viewW, viewH)
}

func OpaqueRectScreenBounds(minX, minY, maxX, maxY int, dstX, dstY, scale float64, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	return OpaqueRectScreenBoundsXY(minX, minY, maxX, maxY, dstX, dstY, scale, scale, clipTop, clipBottom, viewW, viewH)
}

func OpaqueRectScreenBoundsXY(minX, minY, maxX, maxY int, dstX, dstY, scaleX, scaleY float64, clipTop, clipBottom, viewW, viewH int) (int, int, int, int, bool) {
	if scaleX <= 0 || scaleY <= 0 || viewW <= 0 || viewH <= 0 {
		return 0, -1, 0, -1, false
	}
	x0 := int(math.Floor(dstX + float64(minX)*scaleX))
	y0 := int(math.Floor(dstY + float64(minY)*scaleY))
	x1 := int(math.Ceil(dstX+float64(maxX+1)*scaleX)) - 1
	y1 := int(math.Ceil(dstY+float64(maxY+1)*scaleY)) - 1
	if x1 < 0 || y1 < 0 || x0 >= viewW || y0 >= viewH {
		return 0, -1, 0, -1, false
	}
	return ClampedSpriteBounds(float64(x0), float64(y0), float64(x1-x0+1), float64(y1-y0+1), clipTop, clipBottom, viewW, viewH)
}
