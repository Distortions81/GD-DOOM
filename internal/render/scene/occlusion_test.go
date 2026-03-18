package scene

import "testing"

func TestQuadTriMaybeVisible(t *testing.T) {
	pointOccluded := func(x, y int) bool {
		return !(x == 5 && y == 5)
	}
	if !QuadTriMaybeVisible(0, 10, 0, 10, pointOccluded) {
		t.Fatal("expected center opening to keep quad visible")
	}
}

func TestTriangleOcclusionState(t *testing.T) {
	visiblePoint := func(x, y int) bool {
		return !(x == 1 && y == 1)
	}
	if got := TriangleOcclusionState(0, 0, 2, 0, 0, 2, 10, 10, visiblePoint, func(x0, x1, y0, y1 int) bool {
		return true
	}); got != 0 {
		t.Fatalf("state=%d want visible", got)
	}

	alwaysOccluded := func(x, y int) bool { return true }
	if got := TriangleOcclusionState(0, 0, 2, 0, 0, 2, 10, 10, alwaysOccluded, func(x0, x1, y0, y1 int) bool {
		return true
	}); got != 2 {
		t.Fatalf("state=%d want fully occluded", got)
	}
	if got := TriangleOcclusionState(0, 0, 2, 0, 0, 2, 10, 10, alwaysOccluded, func(x0, x1, y0, y1 int) bool {
		return false
	}); got != 1 {
		t.Fatalf("state=%d want maybe occluded", got)
	}
}

func TestTriangleOcclusionStateInView(t *testing.T) {
	alwaysOccluded := func(x, y int) bool { return true }
	if got := TriangleOcclusionStateInView(-10, -10, -8, -10, -10, -8, 10, 10, alwaysOccluded, func(x0, x1, y0, y1 int) bool {
		return true
	}); got != 0 {
		t.Fatalf("state=%d want visible when no sample is on-screen", got)
	}
	if got := TriangleOcclusionStateInView(0, 0, 2, 0, 0, 2, 10, 10, alwaysOccluded, func(x0, x1, y0, y1 int) bool {
		return true
	}); got != 2 {
		t.Fatalf("state=%d want fully occluded", got)
	}
}
