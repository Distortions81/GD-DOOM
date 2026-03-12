package doomruntime

import (
	"strconv"

	"gddoom/internal/render/scene"
)

type plane3DKey struct {
	height   int16
	light    int16
	flatID   uint16
	fallback bool
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
		Fallback: key.fallback, Sky: key.sky, Floor: key.floor,
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
		height:   key.Height,
		light:    key.Light,
		flatID:   flatID,
		fallback: key.Fallback,
		sky:      key.Sky,
		floor:    key.Floor,
	}
}
