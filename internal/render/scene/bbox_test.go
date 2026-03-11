package scene

import "testing"

func TestClampBBoxToView(t *testing.T) {
	x0, x1, y0, y1, ok := ClampBBoxToView(-2, 12, -3, 8, 10, 6)
	if !ok {
		t.Fatal("expected clamped bbox")
	}
	if x0 != 0 || x1 != 9 || y0 != 0 || y1 != 5 {
		t.Fatalf("got %d..%d %d..%d", x0, x1, y0, y1)
	}
}

func TestBBoxFullyOccluded(t *testing.T) {
	got := BBoxFullyOccluded(0, 3, 0, 2, 10, 10, func(x, y0, y1 int) bool {
		return x != 2
	})
	if got {
		t.Fatal("expected one visible column to keep bbox visible")
	}
	got = BBoxFullyOccluded(0, 3, 0, 2, 10, 10, func(x, y0, y1 int) bool {
		return true
	})
	if !got {
		t.Fatal("expected fully occluded bbox")
	}
}

func TestBBoxHasAnyOccluder(t *testing.T) {
	got := BBoxHasAnyOccluder(0, 3, 0, 2, 10, 10, func(x, y0, y1 int) bool {
		return x == 2
	})
	if !got {
		t.Fatal("expected occluding column")
	}
	got = BBoxHasAnyOccluder(0, 3, 0, 2, 10, 10, func(x, y0, y1 int) bool {
		return false
	})
	if got {
		t.Fatal("expected no occluder")
	}
}
