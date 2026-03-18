package doomruntime

import (
	"math"
	"testing"

	"gddoom/internal/render/scene"
)

func TestMaskedMidUsesPortalOverlapNotBackSectorExtents(t *testing.T) {
	worldTop := 87.0
	worldBottom := -41.0
	worldHigh := 215.0
	worldLow := -105.0

	maskedWorldHigh := math.Min(worldTop, worldHigh)
	maskedWorldLow := math.Max(worldBottom, worldLow)

	if maskedWorldHigh != worldTop {
		t.Fatalf("maskedWorldHigh=%v want %v", maskedWorldHigh, worldTop)
	}
	if maskedWorldLow != worldBottom {
		t.Fatalf("maskedWorldLow=%v want %v", maskedWorldLow, worldBottom)
	}
}

func TestMaskedMidEnvelopeStepper_MatchesPerColumnProjection(t *testing.T) {
	proj, status := scene.ProjectWallSegment(20, -2, 0, 80, 2, 1, 320, 160)
	if status != scene.WallProjectionOK {
		t.Fatalf("status=%v want ok", status)
	}
	worldHigh := 48.0
	worldLow := -16.0
	focal := 160.0
	halfH := 100.0
	stepper := newMaskedMidEnvelopeStepper(proj, proj.MinX, worldHigh, worldLow, focal, halfH)
	for x := proj.MinX; x <= proj.MaxX; x++ {
		gotY0, gotY1, gotOK := stepper.Sample()
		depth, _, wantOK := scene.ProjectedWallSampleAtX(proj, x)
		if gotOK != wantOK {
			t.Fatalf("x=%d ok=%v want %v", x, gotOK, wantOK)
		}
		if gotOK {
			wantY0 := int(math.Ceil(halfH - (worldHigh/depth)*focal))
			wantY1 := int(math.Floor(halfH - (worldLow/depth)*focal))
			if gotY0 != wantY0 || gotY1 != wantY1 {
				t.Fatalf("x=%d y=(%d,%d) want (%d,%d)", x, gotY0, gotY1, wantY0, wantY1)
			}
		}
		stepper.Next()
	}
}
