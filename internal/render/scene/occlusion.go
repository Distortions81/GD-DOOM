package scene

import "math"

// Returns true if any corner or center sample remains visible.
func QuadTriMaybeVisible(x0, x1, y0, y1 int, pointOccluded func(x, y int) bool) bool {
	if !pointOccluded(x0, y0) {
		return true
	}
	if !pointOccluded(x1, y0) {
		return true
	}
	if !pointOccluded(x1, y1) {
		return true
	}
	if !pointOccluded(x0, y1) {
		return true
	}
	cx := (x0 + x1) >> 1
	cy := (y0 + y1) >> 1
	return !pointOccluded(cx, cy)
}

// Returns:
// 0 = visible
// 1 = maybe occluded
// 2 = fully occluded
func TriangleOcclusionState(ax, ay, bx, by, cx, cy, viewW, viewH int, pointOccluded func(x, y int) bool, bboxFullyOccluded func(x0, x1, y0, y1 int) bool) int {
	if viewW <= 0 || viewH <= 0 {
		return 2
	}
	edgeMaybeVisible := func(x0, y0, x1, y1 int) bool {
		dx := x1 - x0
		if dx < 0 {
			dx = -dx
		}
		dy := y1 - y0
		if dy < 0 {
			dy = -dy
		}
		steps := dx
		if dy > steps {
			steps = dy
		}
		if steps < 1 {
			steps = 1
		}
		if steps > 32 {
			steps = 32
		}
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps)
			x := int(math.Floor(float64(x0) + float64(x1-x0)*t))
			y := int(math.Floor(float64(y0) + float64(y1-y0)*t))
			if !pointOccluded(x, y) {
				return true
			}
		}
		return false
	}
	if !pointOccluded(ax, ay) {
		return 0
	}
	if !pointOccluded(bx, by) {
		return 0
	}
	if !pointOccluded(cx, cy) {
		return 0
	}
	mx := (ax + bx + cx) / 3
	my := (ay + by + cy) / 3
	if !pointOccluded(mx, my) {
		return 0
	}
	if edgeMaybeVisible(ax, ay, bx, by) || edgeMaybeVisible(bx, by, cx, cy) || edgeMaybeVisible(cx, cy, ax, ay) {
		return 0
	}
	x0 := ax
	if bx < x0 {
		x0 = bx
	}
	if cx < x0 {
		x0 = cx
	}
	x1 := ax
	if bx > x1 {
		x1 = bx
	}
	if cx > x1 {
		x1 = cx
	}
	y0 := ay
	if by < y0 {
		y0 = by
	}
	if cy < y0 {
		y0 = cy
	}
	y1 := ay
	if by > y1 {
		y1 = by
	}
	if cy > y1 {
		y1 = cy
	}
	if bboxFullyOccluded(x0, x1, y0, y1) {
		return 2
	}
	return 1
}

// Returns:
// 0 = visible
// 1 = maybe occluded
// 2 = fully occluded
func TriangleOcclusionStateInView(ax, ay, bx, by, cx, cy, viewW, viewH int, pointOccluded func(x, y int) bool, bboxFullyOccluded func(x0, x1, y0, y1 int) bool) int {
	if viewW <= 0 || viewH <= 0 {
		return 2
	}
	inView := func(x, y int) bool {
		return x >= 0 && x < viewW && y >= 0 && y < viewH
	}
	edgeMaybeVisible := func(x0, y0, x1, y1 int) bool {
		dx := x1 - x0
		if dx < 0 {
			dx = -dx
		}
		dy := y1 - y0
		if dy < 0 {
			dy = -dy
		}
		steps := dx
		if dy > steps {
			steps = dy
		}
		if steps < 1 {
			steps = 1
		}
		if steps > 32 {
			steps = 32
		}
		tested := false
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps)
			x := int(math.Floor(float64(x0) + float64(x1-x0)*t))
			y := int(math.Floor(float64(y0) + float64(y1-y0)*t))
			if !inView(x, y) {
				continue
			}
			tested = true
			if !pointOccluded(x, y) {
				return true
			}
		}
		if !tested {
			return true
		}
		return false
	}
	tested := false
	testPointOccluded := func(x, y int) bool {
		if !inView(x, y) {
			return true
		}
		tested = true
		return pointOccluded(x, y)
	}
	if !testPointOccluded(ax, ay) || !testPointOccluded(bx, by) || !testPointOccluded(cx, cy) {
		return 0
	}
	mx := (ax + bx + cx) / 3
	my := (ay + by + cy) / 3
	if !testPointOccluded(mx, my) {
		return 0
	}
	if !tested {
		return 0
	}
	if edgeMaybeVisible(ax, ay, bx, by) || edgeMaybeVisible(bx, by, cx, cy) || edgeMaybeVisible(cx, cy, ax, ay) {
		return 0
	}
	x0 := ax
	if bx < x0 {
		x0 = bx
	}
	if cx < x0 {
		x0 = cx
	}
	x1 := ax
	if bx > x1 {
		x1 = bx
	}
	if cx > x1 {
		x1 = cx
	}
	y0 := ay
	if by < y0 {
		y0 = by
	}
	if cy < y0 {
		y0 = cy
	}
	y1 := ay
	if by > y1 {
		y1 = by
	}
	if cy > y1 {
		y1 = cy
	}
	x0, x1, y0, y1, ok := ClampBBoxToView(x0, x1, y0, y1, viewW, viewH)
	if !ok {
		return 0
	}
	if bboxFullyOccluded(x0, x1, y0, y1) {
		return 2
	}
	return 1
}
