package scene

func ClampBBoxToView(x0, x1, y0, y1, viewW, viewH int) (int, int, int, int, bool) {
	if viewW <= 0 || viewH <= 0 || x0 > x1 || y0 > y1 {
		return 0, -1, 0, -1, false
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= viewW {
		x1 = viewW - 1
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= viewH {
		y1 = viewH - 1
	}
	if x0 > x1 || y0 > y1 {
		return 0, -1, 0, -1, false
	}
	return x0, x1, y0, y1, true
}

func BBoxFullyOccluded(x0, x1, y0, y1, viewW, viewH int, columnOccluded func(x, y0, y1 int) bool) bool {
	x0, x1, y0, y1, ok := ClampBBoxToView(x0, x1, y0, y1, viewW, viewH)
	if !ok {
		return true
	}
	for x := x0; x <= x1; x++ {
		if !columnOccluded(x, y0, y1) {
			return false
		}
	}
	return true
}

func BBoxHasAnyOccluder(x0, x1, y0, y1, viewW, viewH int, columnHasAnyOccluder func(x, y0, y1 int) bool) bool {
	x0, x1, y0, y1, ok := ClampBBoxToView(x0, x1, y0, y1, viewW, viewH)
	if !ok {
		return false
	}
	for x := x0; x <= x1; x++ {
		if columnHasAnyOccluder(x, y0, y1) {
			return true
		}
	}
	return false
}
