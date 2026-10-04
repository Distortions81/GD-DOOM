package levelmesh

import (
	"fmt"
	"math"
	"slices"
)

// PlaneEdge is an original directed sector side. Its endpoints need not join
// another side by vertex ID, and internal lines can occur in both directions.
type PlaneEdge struct{ A, B Point2 }

// TriangulateEdges fills finite intervals between directed wall crossings with
// nonzero winding, like the textured automap. It does not reconstruct rings or
// extend an unmatched side to infinity. Horizontal slabs split at endpoints
// and edge intersections so each interval is a trapezoid with linear sides.
// This also handles overlapping regions, T junctions, and opposite internal
// sides without closing gaps or altering the original map.
func TriangulateEdges(input []PlaneEdge) ([]PlaneTriangle, error) {
	type edge struct {
		a, b  Point2 // Ordered by Y so reverse sides interpolate identically.
		delta int
	}
	var edges []edge
	var ys []float64
	for _, e := range input {
		for _, p := range []Point2{e.A, e.B} {
			if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
				return nil, fmt.Errorf("nonfinite sector side")
			}
			ys = append(ys, p.Y)
		}
		if e.A.Y == e.B.Y {
			continue
		}
		a, b, delta := e.A, e.B, 1
		if a.Y > b.Y {
			a, b, delta = b, a, -1
		}
		edges = append(edges, edge{a, b, delta})
	}
	xAt := func(e edge, y float64) float64 {
		if y == e.a.Y {
			return e.a.X
		}
		if y == e.b.Y {
			return e.b.X
		}
		return e.a.X + (e.b.X-e.a.X)*((y-e.a.Y)/(e.b.Y-e.a.Y))
	}
	// Crossings must keep their order throughout a slab. Split where two
	// nonhorizontal sides cross, including when directed regions overlap.
	for i, a := range edges {
		for _, b := range edges[i+1:] {
			lo, hi := math.Max(a.a.Y, b.a.Y), math.Min(a.b.Y, b.b.Y)
			if lo >= hi || math.Max(a.a.X, a.b.X) < math.Min(b.a.X, b.b.X) || math.Max(b.a.X, b.b.X) < math.Min(a.a.X, a.b.X) {
				continue
			}
			dlo, dhi := xAt(a, lo)-xAt(b, lo), xAt(a, hi)-xAt(b, hi)
			if (dlo < 0 && dhi > 0) || (dlo > 0 && dhi < 0) {
				ys = append(ys, lo+(hi-lo)*(dlo/(dlo-dhi)))
			}
		}
	}
	slices.Sort(ys)
	ys = slices.Compact(ys)
	type crossing struct {
		e edge
		x float64
	}
	var crossings []crossing
	var out []PlaneTriangle
	for i := 0; i+1 < len(ys); i++ {
		bottom, top := ys[i], ys[i+1]
		mid := bottom + (top-bottom)/2
		crossings = crossings[:0]
		for _, e := range edges {
			if e.a.Y < mid && e.b.Y > mid {
				crossings = append(crossings, crossing{e, xAt(e, mid)})
			}
		}
		slices.SortFunc(crossings, func(a, b crossing) int {
			if a.x < b.x {
				return -1
			}
			if a.x > b.x {
				return 1
			}
			return 0
		})
		winding := 0
		for j := 0; j+1 < len(crossings); j++ {
			winding += crossings[j].e.delta
			if winding == 0 {
				continue
			}
			left, right := crossings[j].e, crossings[j+1].e
			v := [4]Point2{{xAt(left, bottom), bottom}, {xAt(right, bottom), bottom}, {xAt(right, top), top}, {xAt(left, top), top}}
			if v[0].X > v[1].X+1e-8 || v[3].X > v[2].X+1e-8 {
				return nil, fmt.Errorf("sector sides change order inside slab at Y=%g", mid)
			}
			for _, t := range []PlaneTriangle{{v[0], v[1], v[2]}, {v[0], v[2], v[3]}} {
				area := (t[1].X-t[0].X)*(t[2].Y-t[0].Y) - (t[1].Y-t[0].Y)*(t[2].X-t[0].X)
				if area > 1e-8 {
					out = append(out, t)
				}
			}
		}
	}
	return out, nil
}
