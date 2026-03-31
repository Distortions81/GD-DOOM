package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview/linepolicy"
)

func TestLinedefDecisionUsesParityState(t *testing.T) {
	g := &game{
		parity: automapParityState{reveal: revealAllMap, iddt: 0},
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}},
		},
	}
	ld := mapdata.Linedef{
		Flags:   0, // not mapped
		SideNum: [2]int16{0, -1},
	}

	d := g.linedefDecision(ld)
	if !d.Visible {
		t.Fatal("automap linedefDecision should expose unmapped lines in allmap parity mode")
	}
	if d.Appearance != linepolicy.AppearanceUnrevealed {
		t.Fatalf("appearance=%v want=%v", d.Appearance, linepolicy.AppearanceUnrevealed)
	}
	if d.Width != 1.2 {
		t.Fatalf("width=%v want=1.2", d.Width)
	}
}

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
	if !d.Visible {
		t.Fatal("pseudo3d should not hide line due to automap mapped status")
	}
	if d.Appearance != linepolicy.AppearanceOneSided {
		t.Fatalf("appearance=%v want=%v", d.Appearance, linepolicy.AppearanceOneSided)
	}
}

func TestLinedefDecisionPseudo3DIgnoresIDDTState(t *testing.T) {
	g := &game{
		parity: automapParityState{reveal: revealNormal, iddt: 0},
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128},
				{FloorHeight: 0, CeilingHeight: 64},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
		},
	}
	ld := mapdata.Linedef{
		Flags:   lineNeverSee,
		SideNum: [2]int16{0, 1},
	}

	withNoIDDT := g.linedefDecisionPseudo3D(ld)
	g.parity.iddt = 2
	withIDDT := g.linedefDecisionPseudo3D(ld)

	if !withNoIDDT.Visible {
		t.Fatal("pseudo3d should not hide line due to automap iddt/reveal state")
	}
	if withNoIDDT != withIDDT {
		t.Fatalf("pseudo3d visibility changed with iddt: noiddt=%+v iddt=%+v", withNoIDDT, withIDDT)
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
