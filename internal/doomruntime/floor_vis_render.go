package doomruntime

import (
	"math"
	"sort"
)

func floorPolygonXRange(poly []screenPt, viewW int) (int, int, bool) {
	if len(poly) < 3 || viewW <= 0 {
		return 0, 0, false
	}
	minX := viewW - 1
	maxX := 0
	for _, p := range poly {
		ix := int(math.Floor(p.x))
		ax := int(math.Ceil(p.x)) - 1
		if ix < minX {
			minX = ix
		}
		if ix > maxX {
			maxX = ix
		}
		if ax > maxX {
			maxX = ax
		}
	}
	if minX < 0 {
		minX = 0
	}
	if maxX >= viewW {
		maxX = viewW - 1
	}
	if minX > maxX {
		return 0, 0, false
	}
	return minX, maxX, true
}

func (g *game) markScreenPolygonColumns(pl *floorVisplane, poly []screenPt) {
	if pl == nil || len(poly) < 3 {
		return
	}
	minX := g.viewW - 1
	maxX := 0
	for _, p := range poly {
		ix := int(math.Floor(p.x))
		ax := int(math.Ceil(p.x)) - 1
		if ix < minX {
			minX = ix
		}
		if ix > maxX {
			maxX = ix
		}
		if ax > maxX {
			maxX = ax
		}
	}
	if minX < 0 {
		minX = 0
	}
	if maxX >= g.viewW {
		maxX = g.viewW - 1
	}
	if minX > maxX {
		return
	}

	ys := make([]float64, 0, len(poly))
	for x := minX; x <= maxX; x++ {
		ys = ys[:0]
		sx := float64(x) + 0.5
		for i := 0; i < len(poly); i++ {
			a := poly[i]
			b := poly[(i+1)%len(poly)]
			if (a.x <= sx && b.x > sx) || (b.x <= sx && a.x > sx) {
				t := (sx - a.x) / (b.x - a.x)
				y := a.y + (b.y-a.y)*t
				ys = append(ys, y)
			}
		}
		if len(ys) < 2 {
			continue
		}
		sort.Float64s(ys)
		for i := 0; i+1 < len(ys); i += 2 {
			// Slight epsilon expansion avoids 1px cracks between adjacent polygons.
			const eps = 1e-4
			top := int(math.Ceil(ys[i] - eps))
			bottom := int(math.Floor(ys[i+1] + eps))
			if markFloorColumnRange(pl, x, top, bottom, g.floorClip, g.ceilingClip) {
				g.floorFrame.markedCols++
			} else {
				g.floorFrame.rejectedSpan++
			}
		}
	}
}

func (g *game) buildFloorVisplaneMarks() {
	for ss := range g.m.SubSectors {
		worldVerts, cx, cy, ok := g.subSectorConvexVertices(ss)
		if !ok {
			worldVerts, cx, cy, ok = g.subSectorWorldVertices(ss)
		}
		if !ok {
			worldVerts, cx, cy, ok = g.subSectorVerticesFromSegList(ss)
		}
		if !ok {
			continue
		}
		screenPoly := make([]screenPt, 0, len(worldVerts))
		for _, v := range worldVerts {
			sx, sy := g.worldToScreen(v.x, v.y)
			screenPoly = append(screenPoly, screenPt{x: sx, y: sy})
		}
		secIdx, ok := g.subSectorSectorIndex(ss)
		if !ok || secIdx < 0 || secIdx >= len(g.m.Sectors) {
			secIdx = g.sectorAt(int64(cx*fracUnit), int64(cy*fracUnit))
			if secIdx < 0 || secIdx >= len(g.m.Sectors) {
				continue
			}
		}
		sec := g.m.Sectors[secIdx]
		key := floorPlaneKey{
			flat:   g.resolveAnimatedFlatName(sec.FloorPic),
			floorH: sec.FloorHeight,
			light:  g.sectorLightForRender(secIdx, &sec),
		}
		minX, maxX, ok := floorPolygonXRange(screenPoly, g.viewW)
		if !ok {
			continue
		}
		pl, _ := g.ensureFloorVisplaneForRange(key, minX, maxX)
		if pl == nil {
			continue
		}
		// Triangulated marking is more robust than direct odd-even polygon fill
		// when subsector vertex ordering is imperfect.
		tris, triOK := triangulateWorldPolygon(worldVerts)
		if !triOK || len(tris) == 0 {
			tris, triOK = triangulateByAngleFan(worldVerts)
		}
		if triOK && len(tris) > 0 {
			for _, tri := range tris {
				i0, i1, i2 := tri[0], tri[1], tri[2]
				if i0 < 0 || i1 < 0 || i2 < 0 || i0 >= len(screenPoly) || i1 >= len(screenPoly) || i2 >= len(screenPoly) {
					continue
				}
				g.markScreenPolygonColumns(pl, []screenPt{screenPoly[i0], screenPoly[i1], screenPoly[i2]})
			}
			continue
		}
		g.markScreenPolygonColumns(pl, screenPoly)
	}
}
