package mapview

type LineResolver func(index int) (CachedLine, bool)

func BuildLineCache(dst []CachedLine, indices []int, resolve LineResolver) []CachedLine {
	for _, index := range indices {
		draw, ok := resolve(index)
		if !ok {
			continue
		}
		dst = append(dst, draw)
	}
	return dst
}
