package automap

import "testing"

func TestClipSegmentToNear_OneEndpointBehind(t *testing.T) {
	near := 2.0
	f1, s1 := 1.0, -10.0
	f2, s2 := 8.0, 6.0

	cf1, cs1, cf2, cs2, ok := clipSegmentToNear(f1, s1, f2, s2, near)
	if !ok {
		t.Fatal("expected segment to survive near clip")
	}
	if cf1 <= near || cf2 <= near {
		t.Fatalf("expected clipped depths > near, got f1=%f f2=%f", cf1, cf2)
	}
	if cs1 == s1 && cs2 == s2 {
		t.Fatal("expected side coordinate to change at clipped endpoint")
	}
}

func TestClipSegmentToNear_BothBehind(t *testing.T) {
	near := 2.0
	_, _, _, _, ok := clipSegmentToNear(0.5, -5, 1.9, 9, near)
	if ok {
		t.Fatal("expected segment behind near plane to be rejected")
	}
}

func TestAddSolidSpanAndCoverage(t *testing.T) {
	var spans []solidSpan
	spans = addSolidSpan(spans, 10, 20)
	spans = addSolidSpan(spans, 30, 40)
	spans = addSolidSpan(spans, 21, 29) // bridge into one merged span

	if len(spans) != 1 {
		t.Fatalf("expected one merged span, got %d", len(spans))
	}
	if spans[0].l != 10 || spans[0].r != 40 {
		t.Fatalf("unexpected merged span: %+v", spans[0])
	}

	if !solidFullyCovered(spans, 12, 38) {
		t.Fatal("expected interior range to be fully covered")
	}
	if solidFullyCovered(spans, 0, 12) {
		t.Fatal("did not expect partially outside range to be fully covered")
	}
}
