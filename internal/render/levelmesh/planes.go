package levelmesh

import (
	"fmt"
	"math"
	"slices"
)

// TriangulateRings fills the even-odd interior of closed sector rings. It
// decomposes the polygon at every vertex X into non-overlapping trapezoids,
// then splits those into triangles. Concavities, islands and holes need no
// bridge edges. Adjacent slabs evaluate the same boundary edges at the same X,
// avoiding the rounded BSP SEG endpoints that leave cracks in subsector meshes.
// Rings must be simple and must not cross each other; the runtime's sector ring
// extractor already validates individual rings. Odd intersections are rejected.
func TriangulateRings(rings [][]Point2) ([]PlaneTriangle, error) {
	type edge struct{ a, b Point2 }
	var edges []edge
	var xs []float64
	for _, ring := range rings {
		if len(ring) < 3 {
			return nil, fmt.Errorf("sector ring has fewer than three vertices")
		}
		for i, a := range ring {
			if math.IsNaN(a.X) || math.IsNaN(a.Y) || math.IsInf(a.X, 0) || math.IsInf(a.Y, 0) {
				return nil, fmt.Errorf("nonfinite sector vertex")
			}
			b := ring[(i+1)%len(ring)]
			xs = append(xs, a.X)
			if a.X > b.X {
				a, b = b, a
			}
			if a.X != b.X {
				edges = append(edges, edge{a, b})
			}
		}
	}
	slices.Sort(xs)
	xs = slices.Compact(xs)
	yAt := func(e edge, x float64) float64 {
		// Preserve exact original endpoints, including where edges meet.
		if x == e.a.X {
			return e.a.Y
		}
		if x == e.b.X {
			return e.b.Y
		}
		return e.a.Y + (e.b.Y-e.a.Y)*((x-e.a.X)/(e.b.X-e.a.X))
	}
	type crossing struct {
		e edge
		y float64
	}
	var crossings []crossing
	var out []PlaneTriangle
	for i := 0; i+1 < len(xs); i++ {
		left, right := xs[i], xs[i+1]
		mid := left + (right-left)/2
		crossings = crossings[:0]
		for _, e := range edges {
			if e.a.X < mid && e.b.X > mid {
				crossings = append(crossings, crossing{e, yAt(e, mid)})
			}
		}
		slices.SortFunc(crossings, func(a, b crossing) int {
			if a.y < b.y {
				return -1
			}
			if a.y > b.y {
				return 1
			}
			return 0
		})
		if len(crossings)%2 != 0 {
			return nil, fmt.Errorf("odd boundary crossings at X=%g", mid)
		}
		for j := 0; j < len(crossings); j += 2 {
			bottom, top := crossings[j].e, crossings[j+1].e
			v := [4]Point2{{left, yAt(bottom, left)}, {right, yAt(bottom, right)}, {right, yAt(top, right)}, {left, yAt(top, left)}}
			if v[0].Y > v[3].Y+1e-8 || v[1].Y > v[2].Y+1e-8 {
				return nil, fmt.Errorf("crossing sector boundaries")
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
