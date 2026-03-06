package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestClassifyWallPortal_SkyHeightDeltaMarksCeiling(t *testing.T) {
	front := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "F_SKY1",
		Light:         160,
	}
	back := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 64,
		CeilingPic:    "F_SKY1",
		Light:         160,
	}

	got := classifyWallPortal(front, back, 41)
	if got.topWall {
		t.Fatal("sky portal should suppress upper wall")
	}
	if !got.markCeiling {
		t.Fatal("sky portal with ceiling delta should still mark ceiling for sky masking")
	}
	if got.solidWall {
		t.Fatal("open sky portal should not be treated as solid wall")
	}
}

func TestClassifyWallPortal_IdenticalNonSkyCanSkipCeilingMark(t *testing.T) {
	front := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}
	back := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}

	got := classifyWallPortal(front, back, 41)
	if got.markCeiling {
		t.Fatal("identical non-sky ceiling portal should not force ceiling mark")
	}
}

func TestWallSegPrepass_KeepsLightOnlyPortalSplitter(t *testing.T) {
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
		},
		viewW: 320,
		viewH: 200,
	}

	prev := doomSectorLighting
	doomSectorLighting = true
	t.Cleanup(func() { doomSectorLighting = prev })

	pp := g.buildWallSegPrepassSingle(0, 0, 0, 1, 0, doomFocalLength(g.viewW), 2)
	if !pp.ok {
		t.Fatalf("light-only two-sided portal splitter should survive prepass culling, got reason=%q", pp.logReason)
	}
}
