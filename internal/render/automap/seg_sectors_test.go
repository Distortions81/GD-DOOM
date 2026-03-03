package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestSegSectors_OneSidedReversedDirectionKeepsFront(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}},
			Linedefs: []mapdata.Linedef{{SideNum: [2]int16{0, -1}}},
			Segs:     []mapdata.Seg{{Linedef: 0, Direction: 1}},
		},
	}

	front, back := g.segSectors(0)
	if front == nil {
		t.Fatal("expected front sector for reversed one-sided seg")
	}
	if back != nil {
		t.Fatal("expected nil back sector for one-sided seg")
	}
}

func TestSubSectorSectorIndex_UsesSegFrontSide(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{}, {}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
			Linedefs: []mapdata.Linedef{{SideNum: [2]int16{0, 1}}},
			Segs: []mapdata.Seg{
				{Linedef: 0, Direction: 1},
			},
			SubSectors: []mapdata.SubSector{{FirstSeg: 0, SegCount: 1}},
		},
	}

	sec, ok := g.subSectorSectorIndex(0)
	if !ok {
		t.Fatal("expected subsector sector lookup to succeed")
	}
	if sec != 1 {
		t.Fatalf("sector index=%d want=1", sec)
	}
}

func TestSubSectorSectorIndex_OneSidedReverseFallsBack(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}},
			Linedefs: []mapdata.Linedef{{SideNum: [2]int16{0, -1}}},
			Segs: []mapdata.Seg{
				{Linedef: 0, Direction: 1},
			},
			SubSectors: []mapdata.SubSector{{FirstSeg: 0, SegCount: 1}},
		},
	}

	sec, ok := g.subSectorSectorIndex(0)
	if !ok {
		t.Fatal("expected one-sided reverse subsector lookup to succeed")
	}
	if sec != 0 {
		t.Fatalf("sector index=%d want=0", sec)
	}
}

func TestSubSectorSectorIndex_PrefersFirstSeg(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{}, {}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
			Linedefs: []mapdata.Linedef{
				{SideNum: [2]int16{0, -1}},
				{SideNum: [2]int16{1, -1}},
			},
			Segs: []mapdata.Seg{
				{Linedef: 0, Direction: 0},
				{Linedef: 1, Direction: 0},
			},
			SubSectors: []mapdata.SubSector{{FirstSeg: 0, SegCount: 2}},
		},
	}

	sec, ok := g.subSectorSectorIndex(0)
	if !ok {
		t.Fatal("expected subsector sector lookup to succeed")
	}
	if sec != 0 {
		t.Fatalf("sector index=%d want=0", sec)
	}
}
