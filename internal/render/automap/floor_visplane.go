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
	g.floorPlaneOrd = g.floorPlaneOrd[:0]
	if g.floorPlanes == nil {
		g.floorPlanes = make(map[floorPlaneKey][]*floorVisplane, 32)
	}
	for _, list := range g.floorPlanes {
		for _, pl := range list {
			pl.minX = w
			pl.maxX = -1
			for i := range pl.top {
				pl.top[i] = floorUnset
				pl.bottom[i] = floorUnset
			}
		}
	}
}

func newFloorVisplane(key floorPlaneKey, start, stop, viewW int) *floorVisplane {
	pl := &floorVisplane{
		key:    key,
		minX:   start,
		maxX:   stop,
		top:    make([]int16, viewW+2),
		bottom: make([]int16, viewW+2),
	}
	for i := range pl.top {
		pl.top[i] = floorUnset
		pl.bottom[i] = floorUnset
	}
	return pl
}

// ensureFloorVisplaneForRange emulates Doom's R_FindPlane/R_CheckPlane behavior.
// If the same-key plane has already used a column in the overlap range, split.
func (g *game) ensureFloorVisplaneForRange(key floorPlaneKey, start, stop int) (*floorVisplane, bool) {
	w := max(g.viewW, 1)
	if start > stop {
		start, stop = stop, start
	}
	if start < 0 {
		start = 0
	}
	if stop >= w {
		stop = w - 1
	}
	if start > stop {
		return nil, false
	}
	if g.floorPlanes == nil {
		g.floorPlanes = make(map[floorPlaneKey][]*floorVisplane, 32)
	}
	list := g.floorPlanes[key]
	for _, pl := range list {
		intrl := start
		if pl.minX > intrl {
			intrl = pl.minX
		}
		intrh := stop
		if pl.maxX < intrh {
			intrh = pl.maxX
		}
		conflict := false
		if intrl <= intrh {
			for x := intrl; x <= intrh; x++ {
				ix := x + 1
				if ix >= 0 && ix < len(pl.top) && pl.top[ix] != floorUnset {
					conflict = true
					break
				}
			}
		}
		if conflict {
			continue
		}
		if start < pl.minX {
			pl.minX = start
		}
		if stop > pl.maxX {
			pl.maxX = stop
		}
		return pl, false
	}
	pl := newFloorVisplane(key, start, stop, w)
	g.floorPlanes[key] = append(list, pl)
	g.floorPlaneOrd = append(g.floorPlaneOrd, pl)
	return pl, true
}

// floorVisplaneForKey is kept for tests and helper callers that need a single
// mutable plane scratch bucket without split logic.
func (g *game) floorVisplaneForKey(key floorPlaneKey) *floorVisplane {
	w := max(g.viewW, 1)
	if g.floorPlanes == nil {
		g.floorPlanes = make(map[floorPlaneKey][]*floorVisplane, 32)
	}
	if list := g.floorPlanes[key]; len(list) > 0 {
		return list[0]
	}
	pl := newFloorVisplane(key, w, -1, w)
	g.floorPlanes[key] = []*floorVisplane{pl}
	g.floorPlaneOrd = append(g.floorPlaneOrd, pl)
	return pl
}
