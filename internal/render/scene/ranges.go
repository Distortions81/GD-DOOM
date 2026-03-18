package scene

type ScreenSpan struct {
	L int
	R int
}

func SpanFullyCovered(spans []ScreenSpan, l, r int) bool {
	if l > r {
		return true
	}
	cur := l
	for _, s := range spans {
		if s.R < cur {
			continue
		}
		if s.L > cur {
			return false
		}
		if s.R+1 > cur {
			cur = s.R + 1
		}
		if cur > r {
			return true
		}
	}
	return false
}

func AddSpan(spans []ScreenSpan, l, r int) []ScreenSpan {
	if l > r {
		return spans
	}
	ns := ScreenSpan{L: l, R: r}
	out := make([]ScreenSpan, 0, len(spans)+1)
	inserted := false
	for _, s := range spans {
		if s.R+1 < ns.L {
			out = append(out, s)
			continue
		}
		if ns.R+1 < s.L {
			if !inserted {
				out = append(out, ns)
				inserted = true
			}
			out = append(out, s)
			continue
		}
		if s.L < ns.L {
			ns.L = s.L
		}
		if s.R > ns.R {
			ns.R = s.R
		}
	}
	if !inserted {
		out = append(out, ns)
	}
	return out
}

func AddSpanInPlace(spans []ScreenSpan, l, r int) []ScreenSpan {
	if l > r {
		return spans
	}
	n := len(spans)
	if n == 0 {
		return append(spans, ScreenSpan{L: l, R: r})
	}
	i := 0
	for i < n && spans[i].R+1 < l {
		i++
	}
	if i == n {
		return append(spans, ScreenSpan{L: l, R: r})
	}
	if r+1 < spans[i].L {
		spans = append(spans, ScreenSpan{})
		copy(spans[i+1:], spans[i:n])
		spans[i] = ScreenSpan{L: l, R: r}
		return spans
	}
	if spans[i].L < l {
		l = spans[i].L
	}
	if spans[i].R > r {
		r = spans[i].R
	}
	j := i + 1
	for j < n && spans[j].L-1 <= r {
		if spans[j].R > r {
			r = spans[j].R
		}
		j++
	}
	spans[i] = ScreenSpan{L: l, R: r}
	if j > i+1 {
		copy(spans[i+1:], spans[j:n])
		spans = spans[:n-(j-(i+1))]
	}
	return spans
}

func ClipRangeAgainstSpans(l, r int, covered []ScreenSpan, out []ScreenSpan) []ScreenSpan {
	out = out[:0]
	if r < l {
		return out
	}
	if len(covered) == 0 {
		return append(out, ScreenSpan{L: l, R: r})
	}
	cur := l
	for _, s := range covered {
		if s.R < cur {
			continue
		}
		if s.L > r {
			break
		}
		if s.L > cur {
			right := min(r, s.L-1)
			if right >= cur {
				out = append(out, ScreenSpan{L: cur, R: right})
			}
		}
		if s.R+1 > cur {
			cur = s.R + 1
		}
		if cur > r {
			break
		}
	}
	if cur <= r {
		out = append(out, ScreenSpan{L: cur, R: r})
	}
	return out
}
