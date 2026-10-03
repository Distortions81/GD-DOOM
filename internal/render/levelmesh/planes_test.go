package levelmesh

import (
	"math"
	"testing"
)

func TestTriangulateRingsConcavityHolesIslands(t *testing.T) {
	// A concave outline, a hole inside it, an island inside the hole, and
	// a disconnected component. Ring winding deliberately varies.
	rings := [][]Point2{
		{{0, 0}, {12, 0}, {12, 12}, {8, 12}, {8, 8}, {0, 8}},
		{{2, 2}, {2, 6}, {6, 6}, {6, 2}},
		{{3, 3}, {5, 3}, {5, 5}, {3, 5}},
		{{20, 0}, {20, 4}, {24, 4}, {24, 0}},
	}
	tris, err := TriangulateRings(rings)
	if err != nil {
		t.Fatal(err)
	}
	area := 0.0
	for _, tri := range tris {
		a, b, c := tri[0], tri[1], tri[2]
		cross := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
		if cross <= 0 {
			t.Fatal("nonpositive triangle")
		}
		area += cross / 2
	}
	if math.Abs(area-116) > 1e-8 {
		t.Fatalf("filled area=%g want 116", area)
	}
	// Compare independently evaluated point membership against the triangle
	// union. Count overlaps as well: nested holes cannot become hidden fills.
	for y := 0.173; y < 13; y += 0.37 {
		for x := 0.219; x < 25; x += 0.41 {
			p := Point2{x, y}
			want := false
			for _, ring := range rings {
				for i, a := range ring {
					b := ring[(i+1)%len(ring)]
					if (a.Y > y) != (b.Y > y) && x < (b.X-a.X)*(y-a.Y)/(b.Y-a.Y)+a.X {
						want = !want
					}
				}
			}
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
			if (hits > 0) != want || hits > 1 {
				t.Fatalf("point %+v: hits=%d inside rings=%t", p, hits, want)
			}
		}
	}
}

func TestTriangulateRingsRejectsNonfinite(t *testing.T) {
	_, err := TriangulateRings([][]Point2{{{0, 0}, {1, 0}, {math.NaN(), 1}}})
	if err == nil {
		t.Fatal("nonfinite vertex accepted")
	}
}
