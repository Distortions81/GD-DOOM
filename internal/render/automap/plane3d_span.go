package automap

type plane3DKey struct {
	height   int16
	light    int16
	flat     string
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
	if x2 < x1 {
		return nil
	}
	out := []spanRange{{l: x1, r: x2}}
	for _, c := range covered {
		next := make([]spanRange, 0, len(out))
		for _, r := range out {
			if c.r < r.l || c.l > r.r {
				next = append(next, r)
				continue
			}
			if c.l > r.l {
				next = append(next, spanRange{l: r.l, r: c.l - 1})
			}
			if c.r < r.r {
				next = append(next, spanRange{l: c.r + 1, r: r.r})
			}
		}
		out = next
		if len(out) == 0 {
			break
		}
	}
	return out
}

func addCoveredRange(covered []spanRange, x1, x2 int) []spanRange {
	if x2 < x1 {
		return covered
	}
	ns := spanRange{l: x1, r: x2}
	out := make([]spanRange, 0, len(covered)+1)
	inserted := false
	for _, c := range covered {
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
	return out
}
