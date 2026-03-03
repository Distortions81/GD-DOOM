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
