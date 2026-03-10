package linecache

type Resolver func(index int) (Draw, bool)

func Build(dst []Draw, indices []int, resolve Resolver) []Draw {
	for _, index := range indices {
		draw, ok := resolve(index)
		if !ok {
			continue
		}
		dst = append(dst, draw)
	}
	return dst
}
