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

func TestBuildWallPrepass_RejectsBehind(t *testing.T) {
	got := BuildWallPrepass(1, -4, 0, 1.5, 5, 8, 320, 160, 2)
	if got.OK {
		t.Fatal("expected reject")
	}
	if got.LogReason != "BEHIND" {
		t.Fatalf("reason=%q want BEHIND", got.LogReason)
	}
}

func TestBuildWallPrepass_RejectsBackface(t *testing.T) {
	got := BuildWallPrepass(4, -2, 0, 8, 2, 8, 320, 160, 2)
	if got.OK {
		t.Fatal("expected reject")
	}
	if got.LogReason != "BACKFACE" {
		t.Fatalf("reason=%q want BACKFACE", got.LogReason)
	}
}

func TestBuildWallPrepass_ProjectsVisibleSegment(t *testing.T) {
	got := BuildWallPrepass(4, 2, 1, 8, -2, 9, 320, 160, 2)
	if !got.OK {
		t.Fatalf("reason=%q want projected segment", got.LogReason)
	}
	if got.Projection.MinX > got.Projection.MaxX {
		t.Fatalf("range=%d..%d invalid", got.Projection.MinX, got.Projection.MaxX)
	}
	if got.LogReason != "" {
		t.Fatalf("log reason=%q want empty", got.LogReason)
	}
}

func TestBuildWallPrepassFromWorld_MatchesCameraSpaceBuild(t *testing.T) {
	input := WallPrepassWorldInput{
		X1W: 4, Y1W: 2, U1: 1,
		X2W: 8, Y2W: -2, U2: 9,
	}
	got := BuildWallPrepassFromWorld(input, 0, 0, 1, 0, 320, 160, 2)
	want := BuildWallPrepass(4, 2, 1, 8, -2, 9, 320, 160, 2)
	if got.OK != want.OK {
		t.Fatalf("ok=%v want %v", got.OK, want.OK)
	}
	if got.LogReason != want.LogReason {
		t.Fatalf("reason=%q want %q", got.LogReason, want.LogReason)
	}
	if got.Projection != want.Projection {
		t.Fatalf("projection=%+v want %+v", got.Projection, want.Projection)
	}
}

func TestNewWallPrepassWorldInput_ReversesUForBackSide(t *testing.T) {
	got := NewWallPrepassWorldInput(1, 2, 3, 4, 10, 6, 1)
	if got.U1 != 10 {
		t.Fatalf("u1=%f want 10", got.U1)
	}
	if got.U2 != 4 {
		t.Fatalf("u2=%f want 4", got.U2)
	}
}

func TestMaskedMidSeg_CarriesProjectionPayload(t *testing.T) {
	proj, status := ProjectWallSegment(4, -2, 1, 8, 2, 9, 320, 160)
	if status != WallProjectionOK {
		t.Fatalf("status=%v want ok", status)
	}
	got := MaskedMidSeg{
		Dist:       6,
		X0:         proj.MinX,
		X1:         proj.MaxX,
		Projection: proj,
		WorldHigh:  48,
		WorldLow:   16,
		TexUOff:    3,
		TexMid:     20,
	}
	if got.Projection != proj {
		t.Fatalf("projection=%+v want %+v", got.Projection, proj)
	}
	if got.X0 != proj.MinX || got.X1 != proj.MaxX {
		t.Fatalf("x range=%d..%d want %d..%d", got.X0, got.X1, proj.MinX, proj.MaxX)
	}
}

func TestWallProjectionStepper_MatchesProjectedWallSampleAtX(t *testing.T) {
	proj, status := ProjectWallSegment(4, -2, 1, 8, 2, 9, 320, 160)
	if status != WallProjectionOK {
		t.Fatalf("status=%v want ok", status)
	}
	stepper := NewWallProjectionStepper(proj, proj.MinX)
	for x := proj.MinX; x <= proj.MaxX; x++ {
		gotDepth, gotTexU, gotOK := stepper.Sample()
		wantDepth, wantTexU, wantOK := ProjectedWallSampleAtX(proj, x)
		if gotOK != wantOK {
			t.Fatalf("x=%d ok=%v want %v", x, gotOK, wantOK)
		}
		if gotOK {
			if math.Abs(gotDepth-wantDepth) > 1e-9 {
				t.Fatalf("x=%d depth=%f want %f", x, gotDepth, wantDepth)
			}
			if math.Abs(gotTexU-wantTexU) > 1e-9 {
				t.Fatalf("x=%d texU=%f want %f", x, gotTexU, wantTexU)
			}
		}
		stepper.Next()
	}
}
