package scene

import "testing"

func TestFloorSpriteTop(t *testing.T) {
	if got := FloorSpriteTop(16, 100); got != 84 {
		t.Fatalf("top=%f want 84", got)
	}
}

func TestClampedSpriteBounds(t *testing.T) {
	x0, x1, y0, y1, ok := ClampedSpriteBounds(-2, -3, 8, 10, 1, 6, 10, 8)
	if !ok {
		t.Fatal("expected visible bounds")
	}
	if x0 != 0 || x1 != 5 || y0 != 1 || y1 != 6 {
		t.Fatalf("got %d..%d %d..%d", x0, x1, y0, y1)
	}
}

func TestSpritePatchBounds_FloorAnchored(t *testing.T) {
	x0, x1, y0, y1, ok := SpritePatchBounds(50, 100, 16, 8, 16, 4, 0, 199, 320, 200, true)
	if !ok {
		t.Fatal("expected bounds")
	}
	if y1 != 99 {
		t.Fatalf("bottom=%d want 99", y1)
	}
	if y0 != 84 {
		t.Fatalf("top=%d want 84", y0)
	}
	if x0 >= x1 {
		t.Fatalf("invalid x range %d..%d", x0, x1)
	}
}

func TestProjectileFallbackBounds(t *testing.T) {
	x0, x1, y0, y1, ok := ProjectileFallbackBounds(50, 100, 20, 0, 199, 320, 200)
	if !ok {
		t.Fatal("expected fallback bounds")
	}
	if x0 != 40 || y0 != 80 || x1 != 59 || y1 != 99 {
		t.Fatalf("got %d..%d %d..%d", x0, x1, y0, y1)
	}
}

func TestOpaqueRectScreenBounds(t *testing.T) {
	x0, x1, y0, y1, ok := OpaqueRectScreenBounds(1, 2, 3, 4, 10, 20, 2, 0, 199, 320, 200)
	if !ok {
		t.Fatal("expected bounds")
	}
	if x0 != 12 || x1 != 17 || y0 != 24 || y1 != 29 {
		t.Fatalf("got %d..%d %d..%d", x0, x1, y0, y1)
	}
}

func TestSpritePatchBoundsFromScale(t *testing.T) {
	x0, x1, y0, y1, ok := SpritePatchBoundsFromScale(50, 60, 2, 8, 10, 4, 3, 0, 199, 320, 200)
	if !ok {
		t.Fatal("expected bounds")
	}
	if x0 != 42 || x1 != 57 || y0 != 54 || y1 != 73 {
		t.Fatalf("got %d..%d %d..%d", x0, x1, y0, y1)
	}
}

func TestCircleScreenBounds(t *testing.T) {
	x0, x1, y0, y1, ok := CircleScreenBounds(50, 60, 10, 0, 199, 320, 200)
	if !ok {
		t.Fatal("expected bounds")
	}
	if x0 != 40 || x1 != 59 || y0 != 50 || y1 != 69 {
		t.Fatalf("got %d..%d %d..%d", x0, x1, y0, y1)
	}
}
