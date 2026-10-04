package levelmesh

import (
	"math"
	"testing"
)

func directedRingEdges(rings ...[]Point2) []PlaneEdge {
	var edges []PlaneEdge
	for _, ring := range rings {
		for i, a := range ring {
			edges = append(edges, PlaneEdge{a, ring[(i+1)%len(ring)]})
		}
	}
	return edges
}

func planeTriangleHits(tris []PlaneTriangle, p Point2) int {
	hits := 0
	for _, tri := range tris {
		inside := true
		for i, a := range tri {
			b := tri[(i+1)%3]
			if (b.X-a.X)*(p.Y-a.Y)-(b.Y-a.Y)*(p.X-a.X) < -1e-9 {
				inside = false
				break
			}
		}
		if inside {
			hits++
		}
	}
	return hits
}

func TestTriangulateEdgesJunctionHolesAndInternalSides(t *testing.T) {
	edges := directedRingEdges(
		[]Point2{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
		[]Point2{{2, 2}, {2, 6}, {6, 6}, {6, 2}},
		[]Point2{{3, 3}, {5, 3}, {5, 5}, {3, 5}},
		[]Point2{{20, 0}, {24, 0}, {24, 4}, {20, 4}},
	)
	// Preserve the overhanging side instead of snapping or closing it.
	edges[0].B.X = 11
	edges = append(edges, PlaneEdge{Point2{1, 1}, Point2{9, 9}}, PlaneEdge{Point2{9, 9}, Point2{1, 1}})
	checkDirectedPlanes(t, edges, 104)
}

func TestTriangulateEdgesOverlappingAndCrossingRegions(t *testing.T) {
	t.Run("nonzero winding", func(t *testing.T) {
		checkDirectedPlanes(t, directedRingEdges(
			[]Point2{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
			[]Point2{{5, 0}, {15, 0}, {15, 10}, {5, 10}},
		), 150)
	})
	t.Run("sloped intersections", func(t *testing.T) {
		// The diamonds' sides cross halfway through an endpoint slab.
		checkDirectedPlanes(t, directedRingEdges(
			[]Point2{{0, 5}, {5, 0}, {10, 5}, {5, 10}},
			[]Point2{{4, 5}, {9, 0}, {14, 5}, {9, 10}},
		), 82)
	})
}

func checkDirectedPlanes(t *testing.T, edges []PlaneEdge, wantArea float64) {
	t.Helper()
	tris, err := TriangulateEdges(edges)
	if err != nil {
		t.Fatal(err)
	}
	area := 0.0
	for _, tri := range tris {
		a, b, c := tri[0], tri[1], tri[2]
		cross := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
		if cross <= 0 {
			t.Fatal("nonpositive plane triangle")
		}
		area += cross / 2
	}
	if math.Abs(area-wantArea) > 1e-8 {
		t.Fatalf("area=%g want %g", area, wantArea)
	}
	// Independent point winding queries verify coverage and absence of
	// overlaps, including the interior of overlapping directed regions.
	for y := -0.823; y < 12; y += 0.31 {
		for x := -0.731; x < 26; x += 0.37 {
			winding := 0
			for _, e := range edges {
				cross := (e.B.X-e.A.X)*(y-e.A.Y) - (e.B.Y-e.A.Y)*(x-e.A.X)
				if e.A.Y <= y && e.B.Y > y && cross > 0 {
					winding++
				}
				if e.A.Y > y && e.B.Y <= y && cross < 0 {
					winding--
				}
			}
			hits := planeTriangleHits(tris, Point2{x, y})
			if (hits > 0) != (winding != 0) || hits > 1 {
				t.Fatalf("(%g,%g): triangle hits=%d winding=%d", x, y, hits, winding)
			}
		}
	}
}

func TestTriangulateEdgesEmptyAndUnboundedSides(t *testing.T) {
	for _, edges := range [][]PlaneEdge{
		nil,
		{{Point2{0, 0}, Point2{0, 10}}},
		{{Point2{0, 0}, Point2{10, 10}}, {Point2{10, 10}, Point2{0, 0}}},
	} {
		tris, err := TriangulateEdges(edges)
		if err != nil || len(tris) != 0 {
			t.Fatalf("isolated or cancelling sides produced triangles: %v, %v", tris, err)
		}
	}
}

func TestTriangulateEdgesRejectsNonfinite(t *testing.T) {
	for _, x := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := TriangulateEdges([]PlaneEdge{{Point2{0, 0}, Point2{x, 1}}}); err == nil {
			t.Fatal("nonfinite side accepted")
		}
	}
}

func TestTriangulateEdgesFiniteOpenIntervals(t *testing.T) {
	// Open rows match the automap's finite-interval policy: an unmatched
	// winding may fill between walls, but never beyond the final crossing.
	edges := []PlaneEdge{
		{Point2{0, 0}, Point2{0, 10}},
		{Point2{10, 0}, Point2{10, 10}},
		{Point2{20, 0}, Point2{20, 10}},
	}
	tris, err := TriangulateEdges(edges)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []float64{-1.7, 5.3, 15.1, 21.9} {
		want := x > 0 && x < 20
		if hits := planeTriangleHits(tris, Point2{x, 4.7}); (hits > 0) != want || hits > 1 {
			t.Fatalf("open row X=%g: hits=%d want %t", x, hits, want)
		}
	}
}
