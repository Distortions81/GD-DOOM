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

	got := classifyWallPortal(front, back, 41, 0, 128, 0, 64)
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

	got := classifyWallPortal(front, back, 41, 0, 128, 0, 128)
	if got.markCeiling {
		t.Fatal("identical non-sky ceiling portal should not force ceiling mark")
	}
}

func TestClassifyWallPortal_RenderLookAheadDoesNotForceSolidWallEarly(t *testing.T) {
	front := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}
	back := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 4,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}

	got := classifyWallPortal(front, back, 41, 0, 128, 0, -2)
	if got.solidWall {
		t.Fatal("render look-ahead should not force solid wall before current tic door state is closed")
	}
}

func TestClassifyWallPortal_PartialDoorFromRoomSideProducesUpperWall(t *testing.T) {
	room := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}
	door := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 72,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}

	got := classifyWallPortal(room, door, 41, 0, 128, 0, 72)
	if !got.topWall {
		t.Fatal("room->partial-door portal should produce an upper wall slice")
	}
	if got.bottomWall {
		t.Fatal("room->partial-door portal should not produce a lower wall slice")
	}
	if got.solidWall {
		t.Fatal("room->partial-door portal should stay open")
	}
}

func TestClassifyWallPortal_PartialDoorOpeningSweepKeepsUpperWall(t *testing.T) {
	room := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}

	for h := int16(vDoorSpeed / fracUnit); h < room.CeilingHeight; h += int16(vDoorSpeed / fracUnit) {
		door := &mapdata.Sector{
			FloorHeight:   0,
			CeilingHeight: h,
			CeilingPic:    "CEIL1_1",
			Light:         160,
		}
		got := classifyWallPortal(room, door, 41, 0, 128, 0, float64(h))
		if !got.topWall {
			t.Fatalf("door height=%d: expected upper wall while door is partially open", h)
		}
		if got.bottomWall {
			t.Fatalf("door height=%d: expected no lower wall while door is partially open", h)
		}
		if got.solidWall {
			t.Fatalf("door height=%d: expected open portal while door is partially open", h)
		}
	}
}

func TestClassifyWallPortal_PartialDoorSwappedSidesLosesUpperWall(t *testing.T) {
	room := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}
	door := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 72,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}

	got := classifyWallPortal(door, room, 41, 0, 72, 0, 128)
	if got.topWall {
		t.Fatal("partial-door->room swapped portal should not report the same upper wall slice")
	}
	if got.solidWall {
		t.Fatal("swapped partial-door portal should still stay open")
	}
}

func TestClassifyWallPortal_PartialDoorOpeningSweepSwappedSidesNeverMatchesRoomSide(t *testing.T) {
	room := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}

	for h := int16(vDoorSpeed / fracUnit); h < room.CeilingHeight; h += int16(vDoorSpeed / fracUnit) {
		door := &mapdata.Sector{
			FloorHeight:   0,
			CeilingHeight: h,
			CeilingPic:    "CEIL1_1",
			Light:         160,
		}
		got := classifyWallPortal(door, room, 41, 0, float64(h), 0, 128)
		if got.topWall {
			t.Fatalf("door height=%d: swapped sides should not report the same upper wall", h)
		}
		if got.solidWall {
			t.Fatalf("door height=%d: swapped sides should remain an open portal", h)
		}
	}
}

func TestClassifyWallPortal_ThickDoorOpeningSweep_TwoFacesStayOpen(t *testing.T) {
	room := &mapdata.Sector{
		FloorHeight:   0,
		CeilingHeight: 128,
		CeilingPic:    "CEIL1_1",
		Light:         160,
	}

	for h := int16(vDoorSpeed / fracUnit); h < room.CeilingHeight; h += int16(vDoorSpeed / fracUnit) {
		door := &mapdata.Sector{
			FloorHeight:   0,
			CeilingHeight: h,
			CeilingPic:    "CEIL1_1",
			Light:         160,
		}

		frontFace := classifyWallPortal(room, door, 41, 0, 128, 0, float64(h))
		backFace := classifyWallPortal(door, room, 41, 0, float64(h), 0, 128)

		if frontFace.solidWall {
			t.Fatalf("door height=%d: front face should stay open while door is partially open", h)
		}
		if backFace.solidWall {
			t.Fatalf("door height=%d: back face should stay open while door is partially open", h)
		}
		if !frontFace.topWall {
			t.Fatalf("door height=%d: front face should carry the upper wall slice", h)
		}
		if backFace.topWall {
			t.Fatalf("door height=%d: back face should not duplicate the front-face upper wall slice", h)
		}
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
