package automap

type plane3DKey struct {
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
