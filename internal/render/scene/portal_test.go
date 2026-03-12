package scene

import "testing"

func TestClassifyWallPortal_SkyHeightDeltaMarksCeiling(t *testing.T) {
	got := ClassifyWallPortal(WallPortalInput{
		FrontFloor: 0, FrontCeil: 128, BackFloor: 0, BackCeil: 64, EyeZ: 41,
		FrontLight: 160, BackLight: 160, BackExists: true,
		IsFrontCeilingSky: true, IsBackCeilingSky: true,
	})
	if got.TopWall {
		t.Fatal("sky portal should suppress upper wall")
	}
	if !got.MarkCeiling {
		t.Fatal("sky portal with ceiling delta should still mark ceiling")
	}
	if got.SolidWall {
		t.Fatal("open sky portal should not be solid")
	}
}

func TestClassifyWallPortal_IdenticalNonSkyCanSkipCeilingMark(t *testing.T) {
	got := ClassifyWallPortal(WallPortalInput{
		FrontFloor: 0, FrontCeil: 128, BackFloor: 0, BackCeil: 128, EyeZ: 41,
		FrontCeilingFlat: "CEIL1_1", BackCeilingFlat: "CEIL1_1",
		FrontLight: 160, BackLight: 160, BackExists: true,
	})
	if got.MarkCeiling {
		t.Fatal("identical non-sky ceiling portal should not force ceiling mark")
	}
}

func TestClassifyWallPortal_PartialDoorFromRoomSideProducesUpperWall(t *testing.T) {
	got := ClassifyWallPortal(WallPortalInput{
		FrontFloor: 0, FrontCeil: 128, BackFloor: 0, BackCeil: 72, EyeZ: 41,
		FrontCeilingFlat: "CEIL1_1", BackCeilingFlat: "CEIL1_1",
		FrontLight: 160, BackLight: 160, BackExists: true,
	})
	if !got.TopWall {
		t.Fatal("room->partial-door portal should produce an upper wall slice")
	}
	if got.BottomWall {
		t.Fatal("room->partial-door portal should not produce a lower wall slice")
	}
	if got.SolidWall {
		t.Fatal("room->partial-door portal should stay open")
	}
}
