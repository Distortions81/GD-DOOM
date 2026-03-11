package scene

import (
	"math"
	"testing"
)

func TestProjectedClipSegmentToNear_WithAttrInterpolates(t *testing.T) {
	f1, s1, u1 := 1.0, -10.0, 2.0
	f2, s2, u2 := 8.0, 6.0, 9.0
	gotF1, gotS1, gotU1, gotF2, gotS2, gotU2, ok := ClipSegmentToNearWithAttr(f1, s1, u1, f2, s2, u2, 2.0)
	if !ok {
		t.Fatal("expected near clip to keep segment")
	}
	if gotF1 <= 2.0 || gotF2 <= 2.0 {
		t.Fatalf("clipped depths must stay in front of near plane: %f %f", gotF1, gotF2)
	}
	if gotS1 == s1 || gotU1 == u1 {
		t.Fatal("expected clipped endpoint attributes to interpolate")
	}
	if gotS2 != s2 || gotU2 != u2 {
		t.Fatal("expected untouched endpoint to retain attributes")
	}
}

func TestProjectedWallSegment_BuildsPerspectiveData(t *testing.T) {
	proj, status := ProjectWallSegment(4, -2, 0, 8, 3, 10, 320, 160)
	if status != WallProjectionOK {
		t.Fatalf("status=%v want ok", status)
	}
	if proj.MinX > proj.MaxX {
		t.Fatalf("invalid x range: %d..%d", proj.MinX, proj.MaxX)
	}
	if proj.SX1 == proj.SX2 {
		t.Fatal("expected non-degenerate screen span")
	}
	if proj.InvDepth1 <= 0 || proj.InvDepth2 <= 0 {
		t.Fatalf("invalid inverse depths: %f %f", proj.InvDepth1, proj.InvDepth2)
	}
}

func TestProjectedWallYDepthAtX_MatchesManualInterpolation(t *testing.T) {
	proj, status := ProjectWallSegment(4, -2, 1, 8, 2, 9, 320, 160)
	if status != WallProjectionOK {
		t.Fatalf("status=%v want ok", status)
	}
	x := (proj.MinX + proj.MaxX) / 2
	y, depth, ok := ProjectedWallYDepthAtX(proj, x, 200, 24, 160)
	if !ok {
		t.Fatal("expected projected sample")
	}
	tu := (float64(x) - proj.SX1) / (proj.SX2 - proj.SX1)
	if tu < 0 {
		tu = 0
	}
	if tu > 1 {
		tu = 1
	}
	invDepth := proj.InvDepth1 + (proj.InvDepth2-proj.InvDepth1)*tu
	wantDepth := 1.0 / invDepth
	wantY := float64(200)/2 - (24/wantDepth)*160
	if math.Abs(depth-wantDepth) > 1e-9 {
		t.Fatalf("depth=%f want %f", depth, wantDepth)
	}
	if math.Abs(y-wantY) > 1e-9 {
		t.Fatalf("y=%f want %f", y, wantY)
	}
}
