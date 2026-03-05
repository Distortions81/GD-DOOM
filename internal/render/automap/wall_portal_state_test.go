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
