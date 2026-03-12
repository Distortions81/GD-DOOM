package scene

import "testing"

func TestWallDepthColumnOccludesPoint(t *testing.T) {
	col := WallDepthColumn{DepthQ: 100, Top: 10, Bottom: 20}
	if !WallDepthColumnOccludesPoint(col, 15, 101) {
		t.Fatal("expected deeper point inside range to be occluded")
	}
	if WallDepthColumnOccludesPoint(col, 9, 101) {
		t.Fatal("point outside visible wall slice should not be occluded")
	}
	if WallDepthColumnOccludesPoint(col, 15, 100) {
		t.Fatal("equal depth should not occlude")
	}
}

func TestWallDepthColumnOccludesBBox(t *testing.T) {
	col := WallDepthColumn{DepthQ: 100, Top: 10, Bottom: 20}
	if !WallDepthColumnOccludesBBox(col, 12, 18, 101) {
		t.Fatal("bbox fully inside wall slice should be occluded")
	}
	if WallDepthColumnOccludesBBox(col, 8, 18, 101) {
		t.Fatal("bbox partially outside wall slice should remain visible")
	}
}

func TestWallDepthColumnHasAnyOccluder(t *testing.T) {
	col := WallDepthColumn{DepthQ: 100, Top: 10, Bottom: 20}
	if !WallDepthColumnHasAnyOccluder(col, 8, 18, 101) {
		t.Fatal("overlapping bbox should report some occluder")
	}
	if WallDepthColumnHasAnyOccluder(col, 0, 5, 101) {
		t.Fatal("disjoint bbox should not report occluder")
	}

	closed := WallDepthColumn{DepthQ: 100, Closed: true}
	if !WallDepthColumnHasAnyOccluder(closed, 0, 5, 101) {
		t.Fatal("closed wall column should occlude entire column")
	}
}
