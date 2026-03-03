package automap

const plane3DUnset int16 = -1

type plane3DVisplane struct {
	key    plane3DKey
	minX   int
	maxX   int
	top    []int16
	bottom []int16
}

func newPlane3DVisplane(key plane3DKey, start, stop, viewW int) *plane3DVisplane {
	pl := &plane3DVisplane{
		key:    key,
		minX:   start,
		maxX:   stop,
		top:    make([]int16, viewW+2),
		bottom: make([]int16, viewW+2),
	}
	for i := range pl.top {
		pl.top[i] = plane3DUnset
		pl.bottom[i] = plane3DUnset
	}
	return pl
}

// ensurePlane3DForRange emulates Doom's R_FindPlane + R_CheckPlane behavior.
// If a same-key visplane has no set columns in the overlap range, we reuse it;
// otherwise we allocate a new same-key visplane.
func ensurePlane3DForRange(planes map[plane3DKey][]*plane3DVisplane, key plane3DKey, start, stop, viewW int) (*plane3DVisplane, bool) {
	if start > stop {
		start, stop = stop, start
	}
	if start < 0 {
		start = 0
	}
	if stop >= viewW {
		stop = viewW - 1
	}
	if start > stop {
		return nil, false
	}
	list := planes[key]
	for _, pl := range list {
		intrl := start
		if pl.minX > intrl {
			intrl = pl.minX
		}
		intrh := stop
		if pl.maxX < intrh {
			intrh = pl.maxX
		}
		conflict := false
		if intrl <= intrh {
			for x := intrl; x <= intrh; x++ {
				ix := x + 1
				if ix >= 0 && ix < len(pl.top) && pl.top[ix] != plane3DUnset {
					conflict = true
					break
				}
			}
		}
		if conflict {
			continue
		}
		if start < pl.minX {
			pl.minX = start
		}
		if stop > pl.maxX {
			pl.maxX = stop
		}
		return pl, false
	}
	pl := newPlane3DVisplane(key, start, stop, viewW)
	planes[key] = append(list, pl)
	return pl, true
}

func markPlane3DColumnRange(pl *plane3DVisplane, x, top, bottom int, ceilingclip, floorclip []int) bool {
	if pl == nil || x < 0 || x >= len(ceilingclip) || x >= len(floorclip) {
		return false
	}
	ix := x + 1
	if ix < 0 || ix >= len(pl.top) || ix >= len(pl.bottom) {
		return false
	}
	t := top
	b := bottom
	clipTop := ceilingclip[x] + 1
	clipBottom := floorclip[x] - 1
	if t < clipTop {
		t = clipTop
	}
	if b > clipBottom {
		b = clipBottom
	}
	if t > b {
		return false
	}
	if pl.top[ix] != plane3DUnset {
		return false
	}
	pl.top[ix] = int16(t)
	pl.bottom[ix] = int16(b)
	if x < pl.minX {
		pl.minX = x
	}
	if x > pl.maxX {
		pl.maxX = x
	}
	return true
}

func makePlane3DSpans(pl *plane3DVisplane, viewH int, out []plane3DSpan) []plane3DSpan {
	if pl == nil || viewH <= 0 || pl.minX > pl.maxX {
		return out
	}
	spanstart := make([]int, viewH)
	colRange := func(screenX int) (int, int) {
		ix := screenX + 1
		if ix < 0 || ix >= len(pl.top) || ix >= len(pl.bottom) {
			return 1, 0
		}
		t := int(pl.top[ix])
		b := int(pl.bottom[ix])
		if t == int(plane3DUnset) || b == int(plane3DUnset) || t > b {
			return 1, 0
		}
		if t < 0 {
			t = 0
		}
		if b >= viewH {
			b = viewH - 1
		}
		if t > b {
			return 1, 0
		}
		return t, b
	}

	t1, b1 := colRange(pl.minX - 1)
	for x := pl.minX; x <= pl.maxX+1; x++ {
		t2, b2 := colRange(x)
		out = makePlane3DSpansTransition(out, pl.key, x, t1, b1, t2, b2, spanstart)
		t1, b1 = t2, b2
	}
	return out
}

func makePlane3DSpansTransition(out []plane3DSpan, key plane3DKey, x, t1, b1, t2, b2 int, spanstart []int) []plane3DSpan {
	for t1 < t2 && t1 <= b1 {
		out = appendPlane3DSpan(out, t1, spanstart[t1], x-1, key)
		t1++
	}
	for b1 > b2 && b1 >= t1 {
		out = appendPlane3DSpan(out, b1, spanstart[b1], x-1, key)
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
