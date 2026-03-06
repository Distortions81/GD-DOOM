package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestLinedefDecisionPseudo3DIgnoresMappedGate(t *testing.T) {
	g := &game{
		parity: automapParityState{reveal: revealNormal, iddt: 0},
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}},
		},
	}
	ld := mapdata.Linedef{
		Flags:   0, // not mapped
		SideNum: [2]int16{0, -1},
	}
	d := g.linedefDecisionPseudo3D(ld)
	if !d.visible {
		t.Fatal("pseudo3d should not hide line due to automap mapped status")
	}
}

func TestVisibleSegIndicesPseudo3D_KeepsLightOnlyPortalSplitter(t *testing.T) {
	prev := doomSectorLighting
	doomSectorLighting = true
	t.Cleanup(func() { doomSectorLighting = prev })

	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 96},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 192},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Linedefs: []mapdata.Linedef{
				{V1: 0, V2: 1, SideNum: [2]int16{0, 1}},
			},
			Vertexes: []mapdata.Vertex{
				{X: -32, Y: 16},
				{X: 32, Y: 16},
			},
			Segs: []mapdata.Seg{
				{StartVertex: 0, EndVertex: 1, Linedef: 0, Direction: 0},
			},
			SubSectors: []mapdata.SubSector{
				{FirstSeg: 0, SegCount: 1},
			},
			Nodes: []mapdata.Node{
				{
					BBoxR:   [4]int16{64, -64, -64, 64},
					BBoxL:   [4]int16{64, -64, -64, 64},
					ChildID: [2]uint16{0x8000, 0x8000},
				},
			},
		},
		viewW: 320,
		viewH: 200,
	}
	g.visibleSectorSeen = make([]int, len(g.m.Sectors))
	g.visibleSubSectorSeen = make([]int, len(g.m.SubSectors))

	got := g.visibleSegIndicesPseudo3D()
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("light-only two-sided portal splitter should remain visible, got=%v", got)
	}
}
