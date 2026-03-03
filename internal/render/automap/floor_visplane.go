package automap

const floorUnset int16 = -1

type floorPlaneKey struct {
	flat   string
	floorH int16
	light  int16
}

type floorVisplane struct {
	key    floorPlaneKey
	minX   int
	maxX   int
	top    []int16
	bottom []int16
}

type floorSpan struct {
	y   int
	x1  int
	x2  int
	key floorPlaneKey
}

func (g *game) resetFloorVisplaneFrame() {
	w := max(g.viewW, 1)
	h := max(g.viewH, 1)
	if len(g.floorClip) != w {
		g.floorClip = make([]int16, w)
	}
	if len(g.ceilingClip) != w {
		g.ceilingClip = make([]int16, w)
	}
	for i := 0; i < w; i++ {
		g.floorClip[i] = int16(h)
		g.ceilingClip[i] = floorUnset
	}
	g.floorSpans = g.floorSpans[:0]
	if g.floorPlanes == nil {
		g.floorPlanes = make(map[floorPlaneKey]*floorVisplane, 32)
	}
	for _, pl := range g.floorPlanes {
		pl.minX = w
		pl.maxX = -1
		for i := range pl.top {
			pl.top[i] = floorUnset
			pl.bottom[i] = floorUnset
		}
	}
}

func (g *game) floorVisplaneForKey(key floorPlaneKey) *floorVisplane {
	w := max(g.viewW, 1)
	if g.floorPlanes == nil {
		g.floorPlanes = make(map[floorPlaneKey]*floorVisplane, 32)
	}
	if pl, ok := g.floorPlanes[key]; ok {
		return pl
	}
	pl := &floorVisplane{
		key:    key,
		minX:   w,
		maxX:   -1,
		top:    make([]int16, w+2),
		bottom: make([]int16, w+2),
	}
	for i := range pl.top {
		pl.top[i] = floorUnset
		pl.bottom[i] = floorUnset
	}
	g.floorPlanes[key] = pl
	return pl
}
