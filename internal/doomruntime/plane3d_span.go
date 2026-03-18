package doomruntime

import (
	"strconv"

	"gddoom/internal/render/scene"
)

type plane3DKey struct {
	height   int16
	light    int16
	flatID   uint16
	sky      bool
	floor    bool
}

type plane3DSpan struct {
	y   int
	x1  int
	x2  int
	key plane3DKey
}

type spanRange struct {
	l int
	r int
}

func appendPlane3DSpan(out []plane3DSpan, y, x1, x2 int, key plane3DKey) []plane3DSpan {
	if x2 < x1 || y < 0 {
		return out
	}
	return append(out, plane3DSpan{y: y, x1: x1, x2: x2, key: key})
}

func appendMergedPlane3DSpan(out []plane3DSpan, y, x1, x2 int, key plane3DKey) []plane3DSpan {
	if x2 < x1 || y < 0 {
		return out
	}
	n := len(out)
	if n == 0 {
		return append(out, plane3DSpan{y: y, x1: x1, x2: x2, key: key})
	}
	last := &out[n-1]
	if last.y == y && last.key == key && x1 <= last.x2+1 {
		if x2 > last.x2 {
			last.x2 = x2
		}

		return out
	}
	return append(out, plane3DSpan{y: y, x1: x1, x2: x2, key: key})
}

func makePlane3DSpansWithScratch(pl *plane3DVisplane, viewH int, out []plane3DSpan, spanstart []int) []plane3DSpan {
	if pl == nil || viewH <= 0 || pl.minX > pl.maxX {
		return out
	}
	if cap(spanstart) < viewH {
		spanstart = make([]int, viewH)
	} else {
		spanstart = spanstart[:viewH]
		clear(spanstart)
	}
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
		out = appendMergedPlane3DSpan(out, t1, spanstart[t1], x-1, key)
		t1++
	}
	for b1 > b2 && b1 >= t1 {
		out = appendMergedPlane3DSpan(out, b1, spanstart[b1], x-1, key)
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

func bucketSpanByKey(buckets map[plane3DKey][]plane3DSpan, order []plane3DKey, y, x1, x2 int, key plane3DKey) ([]plane3DKey, map[plane3DKey][]plane3DSpan) {
	if x2 < x1 || y < 0 {
		return order, buckets
	}
	if _, ok := buckets[key]; !ok {
		order = append(order, key)
		buckets[key] = make([]plane3DSpan, 0, 64)
	}
	buckets[key] = append(buckets[key], plane3DSpan{y: y, x1: x1, x2: x2, key: key})
	return order, buckets
}

func clipRangeAgainstCovered(x1, x2 int, covered []spanRange) []spanRange {
	raw := make([][2]int, 0, len(covered))
	for _, c := range covered {
		raw = append(raw, [2]int{c.l, c.r})
	}
	clipped := scene.ClipRangeAgainstCovered(x1, x2, raw)
	out := make([]spanRange, 0, len(clipped))
	for _, c := range clipped {
		out = append(out, spanRange{l: c[0], r: c[1]})
	}
	return out
}

func addCoveredRange(covered []spanRange, x1, x2 int) []spanRange {
	raw := make([][2]int, 0, len(covered))
	for _, c := range covered {
		raw = append(raw, [2]int{c.l, c.r})
	}
	merged := scene.AddCoveredRange(raw, x1, x2)
	out := make([]spanRange, 0, len(merged))
	for _, c := range merged {
		out = append(out, spanRange{l: c[0], r: c[1]})
	}
	return out
}

func plane3DKeyToScene(key plane3DKey) scene.PlaneKey {
	return scene.PlaneKey{
		Height: key.height, Light: key.light, Flat: strconv.FormatUint(uint64(key.flatID), 10),
		Sky: key.sky, Floor: key.floor,
	}
}

func plane3DKeyFromScene(key scene.PlaneKey) plane3DKey {
	flatID := uint16(0)
	if key.Flat != "" {
		if v, err := strconv.ParseUint(key.Flat, 10, 16); err == nil {
			flatID = uint16(v)
		}
	}
	return plane3DKey{
		height: key.Height,
		light:  key.Light,
		flatID: flatID,
		sky:    key.Sky,
		floor:  key.Floor,
	}
}
