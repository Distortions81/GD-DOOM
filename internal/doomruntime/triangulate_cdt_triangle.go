//go:build cgo

package doomruntime

import (
	"math"
	"os"
	"sort"

	"github.com/pradeep-pyro/triangle"
)

type triKey struct {
	a int
	b int
	c int
}

type cdtSegKey struct {
	a int
	b int
}

func canonicalCDTSegKey(a, b int) cdtSegKey {
	if a < b {
		return cdtSegKey{a: a, b: b}
	}
	return cdtSegKey{a: b, b: a}
}

func triangulateWorldPolygonCDT(verts []worldPt) ([][3]int, bool) {
	if !cdtTriangulationAvailable() {
		return nil, false
	}
	n := len(verts)
	if n < 3 {
		return nil, false
	}
	pts := make([][2]float64, n)
	segs := make([][2]int32, n)
	for i, v := range verts {
		pts[i] = [2]float64{v.x, v.y}
		segs[i] = [2]int32{int32(i), int32((i + 1) % n)}
	}
	in := triangle.NewTriangulateIO()
	defer triangle.FreeTriangulateIO(in)
	in.SetPoints(pts)
	in.SetPointMarkers(make([]int32, len(pts)))
	in.SetSegments(segs)
	in.SetSegmentMarkers(make([]int32, len(segs)))

	opts := triangle.NewOptions()
	opts.ConformingDelaunay = false
	opts.SegmentSplitting = triangle.NoSplitting
	opts.Area = 1e12
	opts.Angle = 0
	opts.MaxSteinerPoints = 0

	outIO := triangle.Triangulate(in, opts, false)
	defer triangle.FreeTriangulateIO(outIO)

	outPts := outIO.Points()
	outTris := outIO.Triangles()
	if len(outPts) == 0 || len(outTris) == 0 {
		return nil, false
	}
	remap := make([]int, len(outPts))
	const eps = 1e-7
	for i, p := range outPts {
		match := -1
		best := math.MaxFloat64
		for j, v := range verts {
			dx := p[0] - v.x
			dy := p[1] - v.y
			d2 := dx*dx + dy*dy
			if d2 < best {
				best = d2
				match = j
			}
		}
		if match < 0 || best > eps*eps {
			return nil, false
		}
		remap[i] = match
	}

	out := make([][3]int, 0, len(outTris))
	seen := make(map[triKey]struct{}, len(outTris))
	triArea := 0.0
	for _, t := range outTris {
		if t[0] < 0 || t[1] < 0 || t[2] < 0 {
			continue
		}
		if int(t[0]) >= len(remap) || int(t[1]) >= len(remap) || int(t[2]) >= len(remap) {
			continue
		}
		a := remap[t[0]]
		b := remap[t[1]]
		c := remap[t[2]]
		if a == b || b == c || c == a {
			continue
		}
		wa := verts[a]
		wb := verts[b]
		wc := verts[c]
		if math.Abs(orient2D(wa, wb, wc)) <= 1e-9 {
			continue
		}
		cent := worldPt{x: (wa.x + wb.x + wc.x) / 3, y: (wa.y + wb.y + wc.y) / 3}
		if !pointInWorldPoly(cent, verts) {
			continue
		}
		idx := []int{a, b, c}
		sort.Ints(idx)
		k := triKey{a: idx[0], b: idx[1], c: idx[2]}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, [3]int{a, b, c})
		triArea += math.Abs(orient2D(wa, wb, wc)) * 0.5
	}
	if len(out) == 0 {
		return nil, false
	}
	polyArea := math.Abs(polygonArea2(verts)) * 0.5
	if polyArea <= 1e-9 {
		return nil, false
	}
	// Reject incomplete fills and trust fallback triangulation in those cases.
	if triArea < polyArea*0.98 || triArea > polyArea*1.02 {
		return nil, false
	}
	return out, true
}

func cdtTriangulationAvailable() bool {
	return os.Getenv("GD_DISABLE_CDT_TRIANGULATION") == ""
}

func triangulateSectorLoopsCDT(set sectorLoopSet) ([]worldTri, bool) {
	if !cdtTriangulationAvailable() {
		return nil, false
	}
	if len(set.rings) == 0 {
		return nil, false
	}
	type qpt struct {
		x int64
		y int64
	}
	const q = 1e6
	pts := make([][2]float64, 0, 256)
	segs := make([][2]int32, 0, 512)
	holes := make([][2]float64, 0, 8)
	byQ := make(map[qpt]int, 256)
	idxFor := func(p worldPt) int {
		k := qpt{
			x: int64(math.Round(p.x * q)),
			y: int64(math.Round(p.y * q)),
		}
		if i, ok := byQ[k]; ok {
			return i
		}
		i := len(pts)
		pts = append(pts, [2]float64{p.x, p.y})
		byQ[k] = i
		return i
	}
	segSeen := make(map[cdtSegKey]struct{}, 512)
	for _, ring := range set.rings {
		if len(ring) < 3 {
			continue
		}
		rr := ring
		if nearlyEqualWorldPt(rr[0], rr[len(rr)-1], 1e-9) {
			rr = rr[:len(rr)-1]
		}
		if len(rr) < 3 {
			continue
		}
		ids := make([]int, len(rr))
		for i, p := range rr {
			ids[i] = idxFor(p)
		}
		unique := make([]int, 0, len(ids))
		for _, id := range ids {
			if len(unique) > 0 && unique[len(unique)-1] == id {
				continue
			}
			unique = append(unique, id)
		}
		if len(unique) >= 2 && unique[0] == unique[len(unique)-1] {
			unique = unique[:len(unique)-1]
		}
		if len(unique) < 3 {
			return nil, false
		}
		for i := 0; i < len(ids); i++ {
			a := ids[i]
			b := ids[(i+1)%len(ids)]
			if a == b {
				continue
			}
			key := canonicalCDTSegKey(a, b)
			if _, ok := segSeen[key]; ok {
				continue
			}
			segSeen[key] = struct{}{}
			segs = append(segs, [2]int32{int32(a), int32(b)})
		}
		c, ok := worldPolygonCentroid(rr)
		if ok && !pointInRingsEvenOdd(c.x, c.y, set.rings) && !pointOnAnyRingEdge(c, set.rings, 1e-6) {
			holes = append(holes, [2]float64{c.x, c.y})
		}
	}
	if len(pts) < 3 || len(segs) < 3 {
		return nil, false
	}
	if !cdtSegmentsValid(pts, segs) {
		return nil, false
	}
	in := triangle.NewTriangulateIO()
	defer triangle.FreeTriangulateIO(in)
	in.SetPoints(pts)
	in.SetPointMarkers(make([]int32, len(pts)))
	in.SetSegments(segs)
	in.SetSegmentMarkers(make([]int32, len(segs)))
	if len(holes) > 0 {
		in.SetHoles(holes)
	}
	opts := triangle.NewOptions()
	opts.ConformingDelaunay = false
	opts.SegmentSplitting = triangle.NoSplittingInBoundary
	opts.Area = 1e12
	opts.Angle = 0
	opts.MaxSteinerPoints = 0

	outIO := triangle.Triangulate(in, opts, false)
	defer triangle.FreeTriangulateIO(outIO)
	outPts := outIO.Points()
	outTris := outIO.Triangles()
	if len(outPts) == 0 || len(outTris) == 0 {
		return nil, false
	}
	out := make([]worldTri, 0, len(outTris))
	for _, t := range outTris {
		if t[0] < 0 || t[1] < 0 || t[2] < 0 {
			continue
		}
		if int(t[0]) >= len(outPts) || int(t[1]) >= len(outPts) || int(t[2]) >= len(outPts) {
			continue
		}
		a := worldPt{x: outPts[t[0]][0], y: outPts[t[0]][1]}
		b := worldPt{x: outPts[t[1]][0], y: outPts[t[1]][1]}
		c := worldPt{x: outPts[t[2]][0], y: outPts[t[2]][1]}
		if math.Abs(orient2D(a, b, c)) <= 1e-9 {
			continue
		}
		if !pointInRingsOrOnEdge(a, set.rings, 1e-6) ||
			!pointInRingsOrOnEdge(b, set.rings, 1e-6) ||
			!pointInRingsOrOnEdge(c, set.rings, 1e-6) {
			continue
		}
		cent := worldPt{x: (a.x + b.x + c.x) / 3, y: (a.y + b.y + c.y) / 3}
		if !pointInRingsEvenOdd(cent.x, cent.y, set.rings) {
			continue
		}
		out = append(out, worldTri{a: a, b: b, c: c})
	}
	return out, len(out) > 0
}

func cdtSegmentsValid(pts [][2]float64, segs [][2]int32) bool {
	if len(pts) < 3 || len(segs) < 3 {
		return false
	}
	worldPts := make([]worldPt, len(pts))
	for i, p := range pts {
		worldPts[i] = worldPt{x: p[0], y: p[1]}
	}
	for i, seg := range segs {
		a := int(seg[0])
		b := int(seg[1])
		if a < 0 || b < 0 || a >= len(worldPts) || b >= len(worldPts) || a == b {
			return false
		}
		a1 := worldPts[a]
		a2 := worldPts[b]
		if nearlyEqualWorldPt(a1, a2, 1e-9) {
			return false
		}
		for j := i + 1; j < len(segs); j++ {
			other := segs[j]
			c := int(other[0])
			d := int(other[1])
			if c < 0 || d < 0 || c >= len(worldPts) || d >= len(worldPts) || c == d {
				return false
			}
			if a == c || a == d || b == c || b == d {
				continue
			}
			if segmentsIntersectStrict(a1, a2, worldPts[c], worldPts[d]) {
				return false
			}
		}
	}
	return true
}
