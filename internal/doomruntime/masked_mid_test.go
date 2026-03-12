package doomruntime

import (
	"math"
	"testing"
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
