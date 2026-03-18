package mapdata

import "testing"

func TestValidateHappyPath(t *testing.T) {
	m := &Map{
		Vertexes: []Vertex{{0, 0}, {16, 16}},
		Linedefs: []Linedef{{V1: 0, V2: 1, SideNum: [2]int16{0, -1}}},
		Sidedefs: []Sidedef{{Sector: 0}},
		Sectors:  []Sector{{}},
		Segs:     []Seg{{StartVertex: 0, EndVertex: 1, Linedef: 0}},
		SubSectors: []SubSector{{
			SegCount: 1,
			FirstSeg: 0,
		}},
		Nodes: []Node{{ChildID: [2]uint16{0x8000, 0x8000}}},
	}
	if err := Validate(m); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateDetectsOutOfRangeVertex(t *testing.T) {
	m := &Map{
		Vertexes: []Vertex{{0, 0}},
		Linedefs: []Linedef{{V1: 0, V2: 2, SideNum: [2]int16{-1, -1}}},
	}
	if err := Validate(m); err == nil {
		t.Fatal("Validate() expected error")
	}
}
