package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestDiscoverMappedLinesBySectorDepth(t *testing.T) {
	m := testMapChain()

	d0 := discoverMappedLinesBySector(m, 0, 0)
	if !d0[0] {
		t.Fatalf("expected portal line from sector 0 to be mapped at depth 0")
	}
	if d0[1] {
		t.Fatalf("did not expect second portal line mapped at depth 0")
	}

	d1 := discoverMappedLinesBySector(m, 0, 1)
	if !d1[1] {
		t.Fatalf("expected second portal line mapped at depth 1")
	}
}

func TestDiscoverMappedLinesBySectorDoesNotTraverseBlocking(t *testing.T) {
	m := testMapChain()
	m.Linedefs[0].Flags |= mlBlocking
	d := discoverMappedLinesBySector(m, 0, 3)
	if d[1] {
		t.Fatalf("expected blocked portal to prevent traversal to next sector")
	}
}

func TestDiscoverMappedLinesBySectorSkipsNeverSee(t *testing.T) {
	m := testMapChain()
	m.Linedefs[2].Flags |= lineNeverSee
	d := discoverMappedLinesBySector(m, 0, 0)
	if d[2] {
		t.Fatalf("expected never-see line not to be mapped")
	}
}

func testMapChain() *mapdata.Map {
	return &mapdata.Map{
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128}, // 0
			{FloorHeight: 0, CeilingHeight: 128}, // 1
			{FloorHeight: 0, CeilingHeight: 128}, // 2
		},
		Sidedefs: []mapdata.Sidedef{
			{Sector: 0}, // 0
			{Sector: 1}, // 1
			{Sector: 2}, // 2
		},
		Linedefs: []mapdata.Linedef{
			// sector 0 <-> sector 1 portal
			{SideNum: [2]int16{0, 1}},
			// sector 1 <-> sector 2 portal
			{SideNum: [2]int16{1, 2}},
			// one-sided wall in sector 0
			{SideNum: [2]int16{0, -1}},
		},
	}
}
