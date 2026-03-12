package scene

const PlaneUnset int16 = -1

type PlaneKey struct {
	Height   int16
	Light    int16
	Flat     string
	Fallback bool
	Sky      bool
	Floor    bool
}

type PlaneSpan struct {
	Y   int
	X1  int
	X2  int
	Key PlaneKey
}

type PlaneVisplane struct {
	Key    PlaneKey
	MinX   int
	MaxX   int
	Top    []int16
	Bottom []int16
}

type PlaneAllocator func(key PlaneKey, start, stop, viewW int) *PlaneVisplane

type spanRange struct {
	l int
	r int
}

func NewPlaneVisplane(key PlaneKey, start, stop, viewW int) *PlaneVisplane {
	pl := &PlaneVisplane{
		Key:    key,
		MinX:   start,
		MaxX:   stop,
		Top:    make([]int16, viewW+2),
		Bottom: make([]int16, viewW+2),
	}
	for i := range pl.Top {
		pl.Top[i] = PlaneUnset
		pl.Bottom[i] = PlaneUnset
	}
	return pl
}

func EnsurePlaneForRange(planes map[PlaneKey][]*PlaneVisplane, key PlaneKey, start, stop, viewW int) (*PlaneVisplane, bool) {
	return EnsurePlaneForRangeAlloc(planes, key, start, stop, viewW, NewPlaneVisplane)
}

func EnsurePlaneForRangeAlloc(planes map[PlaneKey][]*PlaneVisplane, key PlaneKey, start, stop, viewW int, alloc PlaneAllocator) (*PlaneVisplane, bool) {
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
	if alloc == nil {
		alloc = NewPlaneVisplane
	}
	list := planes[key]
	for _, pl := range list {
		intrl := start
		if pl.MinX > intrl {
			intrl = pl.MinX
		}
		intrh := stop
		if pl.MaxX < intrh {
			intrh = pl.MaxX
		}
		conflict := false
		if intrl <= intrh {
			for x := intrl; x <= intrh; x++ {
				ix := x + 1
				if ix >= 0 && ix < len(pl.Top) && pl.Top[ix] != PlaneUnset {
					conflict = true
					break
				}
			}
		}
		if conflict {
			continue
		}
		if start < pl.MinX {
			pl.MinX = start
		}
		if stop > pl.MaxX {
			pl.MaxX = stop
		}
		return pl, false
	}
	pl := alloc(key, start, stop, viewW)
	planes[key] = append(list, pl)
	return pl, true
}

func MarkPlaneColumnRange(pl *PlaneVisplane, x, top, bottom int, ceilingclip, floorclip []int) bool {
	if pl == nil || x < 0 || x >= len(ceilingclip) || x >= len(floorclip) {
		return false
	}
	ix := x + 1
	if ix < 0 || ix >= len(pl.Top) || ix >= len(pl.Bottom) {
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
	if pl.Top[ix] == PlaneUnset || t < int(pl.Top[ix]) {
		pl.Top[ix] = int16(t)
	}
	if pl.Bottom[ix] == PlaneUnset || b > int(pl.Bottom[ix]) {
		pl.Bottom[ix] = int16(b)
	}
	if x < pl.MinX {
		pl.MinX = x
	}
	if x > pl.MaxX {
		pl.MaxX = x
	}
	return true
}

func MakePlaneSpans(pl *PlaneVisplane, viewH int, out []PlaneSpan) []PlaneSpan {
	if pl == nil || viewH <= 0 || pl.MinX > pl.MaxX {
		return out
	}
	spanstart := make([]int, viewH)
	colRange := func(screenX int) (int, int) {
		ix := screenX + 1
		if ix < 0 || ix >= len(pl.Top) || ix >= len(pl.Bottom) {
			return 1, 0
		}
		t := int(pl.Top[ix])
		b := int(pl.Bottom[ix])
		if t == int(PlaneUnset) || b == int(PlaneUnset) || t > b {
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

	t1, b1 := colRange(pl.MinX - 1)
	for x := pl.MinX; x <= pl.MaxX+1; x++ {
		t2, b2 := colRange(x)
		out = makePlaneSpansTransition(out, pl.Key, x, t1, b1, t2, b2, spanstart)
		t1, b1 = t2, b2
	}
	return out
}

func AppendPlaneSpan(out []PlaneSpan, y, x1, x2 int, key PlaneKey) []PlaneSpan {
	if x2 < x1 || y < 0 {
		return out
	}
	return append(out, PlaneSpan{Y: y, X1: x1, X2: x2, Key: key})
}

func BucketSpanByKey(buckets map[PlaneKey][]PlaneSpan, order []PlaneKey, y, x1, x2 int, key PlaneKey) ([]PlaneKey, map[PlaneKey][]PlaneSpan) {
	if x2 < x1 || y < 0 {
		return order, buckets
	}
	if _, ok := buckets[key]; !ok {
		order = append(order, key)
		buckets[key] = make([]PlaneSpan, 0, 64)
	}
	buckets[key] = append(buckets[key], PlaneSpan{Y: y, X1: x1, X2: x2, Key: key})
	return order, buckets
}

func makePlaneSpansTransition(out []PlaneSpan, key PlaneKey, x, t1, b1, t2, b2 int, spanstart []int) []PlaneSpan {
	for t1 < t2 && t1 <= b1 {
		out = AppendPlaneSpan(out, t1, spanstart[t1], x-1, key)
		t1++
	}
	for b1 > b2 && b1 >= t1 {
		out = AppendPlaneSpan(out, b1, spanstart[b1], x-1, key)
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

func ClipRangeAgainstCovered(x1, x2 int, covered [][2]int) [][2]int {
	if x2 < x1 {
		return nil
	}
	out := []spanRange{{l: x1, r: x2}}
	for _, c := range covered {
		next := make([]spanRange, 0, len(out))
		for _, r := range out {
			if c[1] < r.l || c[0] > r.r {
				next = append(next, r)
				continue
			}
			if c[0] > r.l {
				next = append(next, spanRange{l: r.l, r: c[0] - 1})
			}
			if c[1] < r.r {
				next = append(next, spanRange{l: c[1] + 1, r: r.r})
			}
		}
		out = next
		if len(out) == 0 {
			break
		}
	}
	converted := make([][2]int, 0, len(out))
	for _, r := range out {
		converted = append(converted, [2]int{r.l, r.r})
	}
	return converted
}

func AddCoveredRange(covered [][2]int, x1, x2 int) [][2]int {
	if x2 < x1 {
		return covered
	}
	ns := spanRange{l: x1, r: x2}
	internal := make([]spanRange, 0, len(covered))
	for _, c := range covered {
		internal = append(internal, spanRange{l: c[0], r: c[1]})
	}
	out := make([]spanRange, 0, len(internal)+1)
	inserted := false
	for _, c := range internal {
		if c.r+1 < ns.l {
			out = append(out, c)
			continue
		}
		if ns.r+1 < c.l {
			if !inserted {
				out = append(out, ns)
				inserted = true
			}
			out = append(out, c)
			continue
		}
		if c.l < ns.l {
			ns.l = c.l
		}
		if c.r > ns.r {
			ns.r = c.r
		}
	}
	if !inserted {
		out = append(out, ns)
	}
	converted := make([][2]int, 0, len(out))
	for _, r := range out {
		converted = append(converted, [2]int{r.l, r.r})
	}
	return converted
}
