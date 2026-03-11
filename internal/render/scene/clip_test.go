package scene

import "testing"

func TestMaskedClipColumnOccludesPoint(t *testing.T) {
	spans := []MaskedClipSpan{
		{Y0: 10, Y1: 20, DepthQ: 100},
	}
	if !MaskedClipColumnOccludesPoint(spans, 15, 101) {
		t.Fatal("expected masked span to occlude deeper point")
	}
	if MaskedClipColumnOccludesPoint(spans, 9, 101) {
		t.Fatal("point outside masked span should remain visible")
	}
	if MaskedClipColumnOccludesPoint(spans, 15, 100) {
		t.Fatal("equal depth should not occlude")
	}
}

func TestMaskedClipColumnOccludesPoint_WithPortalGap(t *testing.T) {
	spans := []MaskedClipSpan{
		{OpenY0: 12, OpenY1: 24, DepthQ: 100, HasOpen: true},
	}
	if MaskedClipColumnOccludesPoint(spans, 18, 101) {
		t.Fatal("portal opening should remain visible")
	}
	if !MaskedClipColumnOccludesPoint(spans, 8, 101) {
		t.Fatal("outside portal opening should be occluded")
	}
}

func TestMaskedClipColumnHasAnyOccluder(t *testing.T) {
	closed := []MaskedClipSpan{
		{DepthQ: 100, Closed: true},
	}
	if !MaskedClipColumnHasAnyOccluder(closed, 0, 10, 101) {
		t.Fatal("closed span should occlude bbox")
	}

	gap := []MaskedClipSpan{
		{OpenY0: 12, OpenY1: 24, DepthQ: 100, HasOpen: true},
	}
	if MaskedClipColumnHasAnyOccluder(gap, 14, 20, 101) {
		t.Fatal("bbox fully inside portal opening should remain visible")
	}
	if !MaskedClipColumnHasAnyOccluder(gap, 8, 20, 101) {
		t.Fatal("bbox extending outside portal opening should be occluded")
	}
}
