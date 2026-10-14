package doomruntime

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func TestShortSubsectorBoundsClipsBeforeOverlapRejection(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "front side", true: "back side"}[reverse], func(t *testing.T) {
			m := &mapdata.Map{
				Vertexes:   []mapdata.Vertex{{X: 0, Y: 64}, {X: 64, Y: 64}, {X: 64, Y: 0}, {X: 0, Y: 0}},
				Linedefs:   []mapdata.Linedef{{V1: 0, V2: 1}, {V1: 2, V2: 3}},
				Segs:       []mapdata.Seg{{StartVertex: 0, EndVertex: 1, Linedef: 0}, {StartVertex: 2, EndVertex: 3, Linedef: 1}},
				SubSectors: []mapdata.SubSector{{SegCount: 2}},
				Sectors:    []mapdata.Sector{{}},
			}
			if reverse {
				m.Linedefs[0].V1, m.Linedefs[0].V2 = 1, 0
				m.Segs[0].Direction = 1
			}
			// The BSP restricts X, while the two walls bound Y. Before wall
			// clipping, this cell has less than 1% sector-box overlap.
			g := &game{
				m:                m,
				subSectorPoly:    [][]worldPt{{{x: 0, y: -10000}, {x: 64, y: -10000}, {x: 64, y: 10000}, {x: 0, y: 10000}}},
				subSectorPolySrc: []uint8{subPolySrcNodes},
				subSectorSec:     []int{0},
				sectorBBox:       []worldBBox{{minX: 0, minY: 0, maxX: 64, maxY: 64}},
				subSectorBBox:    []worldBBox{{minX: 0, minY: 0, maxX: 64, maxY: 64}},
			}
			g.constrainAmbiguousNodePolysToSectorBounds()
			if len(g.subSectorPoly[0]) != 4 || g.subSectorPolySrc[0] != subPolySrcNodes {
				t.Fatalf("valid short leaf was discarded: polygon=%v source=%d", g.subSectorPoly[0], g.subSectorPolySrc[0])
			}
			if area := math.Abs(polygonArea2(g.subSectorPoly[0])) / 2; area != 4096 {
				t.Fatalf("bounded area=%g want 4096", area)
			}
			if b := worldPolyBBox(g.subSectorPoly[0]); b != g.sectorBBox[0] {
				t.Fatalf("bounded polygon box=%+v want %+v", b, g.sectorBBox[0])
			}
		})
	}
}

func TestShortSubsectorBoundsUsesOriginalWallDespiteRoundedSEG(t *testing.T) {
	m := &mapdata.Map{
		Vertexes:   []mapdata.Vertex{{X: 0, Y: 0}, {X: 64, Y: 32}, {X: 17, Y: 9}},
		Linedefs:   []mapdata.Linedef{{V1: 0, V2: 1}},
		Segs:       []mapdata.Seg{{StartVertex: 2, EndVertex: 1, Linedef: 0}},
		SubSectors: []mapdata.SubSector{{SegCount: 1}},
	}
	g := &game{m: m}
	poly := []worldPt{{x: 0, y: 0}, {x: 64, y: 0}, {x: 64, y: 64}, {x: 0, y: 64}}
	got := g.clipSubSectorPolyBySegBounds(0, poly)
	if area := math.Abs(polygonArea2(got)) / 2; area != 1024 {
		t.Fatalf("area=%g want 1024 (the original wall is y=x/2)", area)
	}
	for _, p := range got {
		if p.y > p.x/2+1e-6 {
			t.Fatalf("polygon leaks beyond original wall: %+v", p)
		}
	}
	if !pointInWorldPoly(worldPt{x: 32, y: 15}, got) {
		t.Fatal("bounded cell lost an interior point")
	}
	if poly[2].y != 64 {
		t.Fatal("clipping mutated the input polygon")
	}
}

func TestShortSubsectorBoundsRejectsExteriorCell(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Vertexes:   []mapdata.Vertex{{X: 0, Y: 0}, {X: 64, Y: 0}},
		Linedefs:   []mapdata.Linedef{{V1: 0, V2: 1}},
		Segs:       []mapdata.Seg{{Linedef: 0}},
		SubSectors: []mapdata.SubSector{{SegCount: 1}},
	}}
	poly := []worldPt{{x: 0, y: 10}, {x: 64, y: 10}, {x: 64, y: 64}, {x: 0, y: 64}}
	if got := g.clipSubSectorPolyByLinedefBounds(0, poly); len(got) != 0 {
		t.Fatalf("exterior BSP cell survived: %v", got)
	}
}
