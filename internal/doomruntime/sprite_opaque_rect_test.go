package doomruntime

import "testing"

func TestBuildSpriteOpaqueRects_LargestFirstThenMeaningfulExtras(t *testing.T) {
	const (
		w = 16
		h = 8
	)
	mask := make([]byte, w*h)
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				mask[y*w+x] = 1
			}
		}
	}
	fill(0, 0, 7, 5)   // 48 px, should be first
	fill(10, 0, 13, 3) // 16 px, should be second
	fill(15, 7, 15, 7) // 1 px, should be ignored as not meaningful extra

	rects := buildSpriteOpaqueRects(mask, w, h)
	if len(rects) != 2 {
		t.Fatalf("rect count=%d want 2", len(rects))
	}
	if got := rects[0]; got.minX() != 0 || got.minY() != 0 || got.maxX() != 7 || got.maxY() != 5 {
		t.Fatalf("first rect=%+v want [0,0]-[7,5]", got)
	}
	if got := rects[1]; got.minX() != 10 || got.minY() != 0 || got.maxX() != 13 || got.maxY() != 3 {
		t.Fatalf("second rect=%+v want [10,0]-[13,3]", got)
	}
}

func TestBuildSpriteOpaqueRects_DoesNotDropSetForLowCoverageAlone(t *testing.T) {
	const (
		w = 64
		h = 64
	)
	mask := make([]byte, w*h)
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				mask[y*w+x] = 1
			}
		}
	}
	fill(0, 0, 7, 7)    // 64 px largest rect
	fill(16, 0, 47, 47) // 1536 px fragmented into narrow stripes
	for y := 0; y <= 47; y++ {
		for x := 16; x <= 47; x++ {
			if (x-16)%2 == 1 {
				mask[y*w+x] = 0
			}
		}
	}

	rects := buildSpriteOpaqueRects(mask, w, h)
	if len(rects) == 0 {
		t.Fatal("want at least one worthwhile rect retained")
	}
	if got := rects[0]; got.minX() != 0 || got.minY() != 0 || got.maxX() != 7 || got.maxY() != 7 {
		t.Fatalf("first rect=%+v want largest block [0,0]-[7,7]", got)
	}
}

func TestBuildSpriteOpaqueRects_RejectsSkinnyExtras(t *testing.T) {
	const (
		w = 32
		h = 16
	)
	mask := make([]byte, w*h)
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				mask[y*w+x] = 1
			}
		}
	}
	fill(0, 0, 7, 7)
	fill(12, 0, 12, 11)
	fill(15, 0, 15, 11)

	rects := buildSpriteOpaqueRects(mask, w, h)
	if len(rects) != 1 {
		t.Fatalf("rect count=%d want 1", len(rects))
	}
	if got := rects[0]; got.minX() != 0 || got.minY() != 0 || got.maxX() != 7 || got.maxY() != 7 {
		t.Fatalf("first rect=%+v want largest block [0,0]-[7,7]", got)
	}
}

func TestBuildSpriteOpaqueRects_RetainsMoreThanThreeMeaningfulRects(t *testing.T) {
	const (
		w = 64
		h = 16
	)
	mask := make([]byte, w*h)
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				mask[y*w+x] = 1
			}
		}
	}
	fill(0, 0, 11, 15)
	fill(16, 0, 27, 15)
	fill(32, 0, 43, 15)
	fill(48, 0, 59, 15)

	rects := buildSpriteOpaqueRects(mask, w, h)
	if len(rects) != 4 {
		t.Fatalf("rect count=%d want 4", len(rects))
	}
}
