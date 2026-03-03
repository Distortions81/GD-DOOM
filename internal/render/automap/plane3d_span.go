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
