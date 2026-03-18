package doomruntime

func makePlaneSpans(pl *floorVisplane, viewH int, out []floorSpan) []floorSpan {
	if pl == nil || viewH <= 0 || pl.minX > pl.maxX {
		return out
	}
	spanstart := make([]int, viewH)

	colRange := func(screenX int) (int, int) {
		ix := screenX + 1
		if ix < 0 || ix >= len(pl.top) || ix >= len(pl.bottom) {
			return 1, 0 // empty
		}
		t := int(pl.top[ix])
		b := int(pl.bottom[ix])
		if t == int(floorUnset) || b == int(floorUnset) || t > b {
			return 1, 0 // empty
		}
		if t < 0 {
			t = 0
		}
		if b >= viewH {
			b = viewH - 1
		}
		if t > b {
			return 1, 0 // empty
		}
		return t, b
	}

	t1, b1 := colRange(pl.minX - 1)
	for x := pl.minX; x <= pl.maxX+1; x++ {
		t2, b2 := colRange(x)
		out = makeSpansTransition(out, pl.key, x, t1, b1, t2, b2, spanstart)
		t1, b1 = t2, b2
	}
	return out
}

func makeSpansTransition(out []floorSpan, key floorPlaneKey, x, t1, b1, t2, b2 int, spanstart []int) []floorSpan {
	for t1 < t2 && t1 <= b1 {
		out = appendSpanIfValid(out, key, t1, spanstart[t1], x-1)
		t1++
	}
	for b1 > b2 && b1 >= t1 {
		out = appendSpanIfValid(out, key, b1, spanstart[b1], x-1)
		b1--
	}
	for t2 < t1 && t2 <= b2 {
		spanstart[t2] = x
		t2++
	}
	for b2 > b1 && b2 >= t2 {
		spanstart[b2] = x
		b2--
	}
	return out
}

func appendSpanIfValid(out []floorSpan, key floorPlaneKey, y, x1, x2 int) []floorSpan {
	if y < 0 || x2 < x1 {
		return out
	}
	return append(out, floorSpan{y: y, x1: x1, x2: x2, key: key})
}
