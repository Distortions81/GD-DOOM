package levelmesh

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func portalMap() *mapdata.Map {
	return &mapdata.Map{
		Vertexes: []mapdata.Vertex{{X: 0, Y: 0}, {X: 64, Y: 0}},
		Linedefs: []mapdata.Linedef{{V2: 1, Flags: 4, SideNum: [2]int16{0, 1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0, TextureOffset: 7, RowOffset: 3, Top: "TOP", Bottom: "BOT", Mid: "FENCE"}, {Sector: 1, Top: "-", Bottom: "-", Mid: "-"}},
		Sectors:  []mapdata.Sector{{FloorPic: "FLOOR", CeilingPic: "CEIL"}, {FloorPic: "FLOOR", CeilingPic: "CEIL"}},
	}
}

func TestPlanesFaceInteriorAndFollowHeight(t *testing.T) {
	m := portalMap()
	m.Linedefs = nil
	planes := [][]PlaneTriangle{{{{X: 0, Y: 0}, {X: 0, Y: 64}, {X: 64, Y: 0}}}}
	for _, h := range []Heights{{0, 128}, {24.5, 80.25}} {
		tris := Build(nil, m, planes, []Heights{h, h}, nil, nil)
		if len(tris) != 2 {
			t.Fatalf("got %d triangles", len(tris))
		}
		for _, tri := range tris {
			a, b, c := tri.Vertices[0], tri.Vertices[1], tri.Vertices[2]
			n := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
			z := h.Floor
			if tri.Kind == Ceiling {
				z = h.Ceiling
				if n >= 0 {
					t.Fatal("ceiling faces out")
				}
			} else if n <= 0 {
				t.Fatal("floor faces out")
			}
			for _, v := range tri.Vertices {
				if v.Z != z || v.U != v.X || v.V != v.Y {
					t.Fatalf("bad plane vertex %+v", v)
				}
			}
		}
	}
}

func TestPortalWallsAndRepeatingMaskedStrip(t *testing.T) {
	m := portalMap()
	h := []Heights{{0, 128}, {32, 96}}
	tris := Build(nil, m, nil, h, func(string, int, Kind) float64 { return 64 }, nil)
	counts := map[Kind]int{}
	for _, tri := range tris {
		if tri.Sector != 0 {
			continue
		}
		counts[tri.Kind]++
		for _, v := range tri.Vertices {
			switch tri.Kind {
			case Upper:
				if v.Z < 96 || v.Z > 128 || v.V != 163-v.Z {
					t.Fatalf("bad upper %+v", v)
				}
			case Lower:
				if v.Z < 0 || v.Z > 32 || v.V != 35-v.Z {
					t.Fatalf("bad lower %+v", v)
				}
			case Middle:
				if !tri.Masked || v.Z < 32 || v.Z > 96 || v.V != 99-v.Z {
					t.Fatalf("bad masked strip %+v", v)
				}
			}
			if v.U != 7 && v.U != 71 {
				t.Fatalf("bad horizontal offset %+v", v)
			}
		}
	}
	for _, kind := range []Kind{Upper, Lower, Middle} {
		if counts[kind] != 2 {
			t.Fatalf("kind %d: %d triangles", kind, counts[kind])
		}
	}
	// A closed door must seal the opening, then expose it after rising.
	h[1].Ceiling = 32
	closed := Build(nil, m, nil, h, nil, nil)
	for _, tri := range closed {
		if tri.Masked {
			t.Fatal("closed portal has fence geometry")
		}
	}
	h[1].Ceiling = 128
	opened := Build(nil, m, nil, h, nil, nil)
	for _, tri := range opened {
		if tri.Sector == 0 && tri.Kind == Upper {
			t.Fatal("open door still has upper wall")
		}
	}
}

func TestPeggingAndSkyPortal(t *testing.T) {
	m := portalMap()
	m.Linedefs[0].Flags |= 8 | 16
	tris := Build(nil, m, nil, []Heights{{0, 128}, {32, 96}}, func(string, int, Kind) float64 { return 64 }, nil)
	for _, tri := range tris {
		if tri.Kind == Upper || tri.Kind == Lower {
			for _, v := range tri.Vertices {
				if math.Abs(v.V-(131-v.Z)) > 1e-8 {
					t.Fatalf("pegged %+v", v)
				}
			}
		}
	}
	m.Sectors[0].CeilingPic = "F_SKY1"
	m.Sectors[1].CeilingPic = "F_SKY1"
	tris = Build(nil, m, nil, []Heights{{0, 128}, {32, 96}}, nil, nil)
	for _, tri := range tris {
		if tri.Kind == Upper {
			t.Fatal("sky-to-sky boundary has upper wall")
		}
	}
}
