package scene

import "testing"

func TestAddSpanAndCoverage(t *testing.T) {
	var spans []ScreenSpan
	spans = AddSpan(spans, 10, 20)
	spans = AddSpan(spans, 30, 40)
	spans = AddSpan(spans, 21, 29)

	if len(spans) != 1 {
		t.Fatalf("expected one merged span, got %d", len(spans))
	}
	if spans[0] != (ScreenSpan{L: 10, R: 40}) {
		t.Fatalf("unexpected merged span: %+v", spans[0])
	}
	if !SpanFullyCovered(spans, 12, 38) {
		t.Fatal("expected interior range to be fully covered")
	}
	if SpanFullyCovered(spans, 0, 12) {
		t.Fatal("did not expect partially outside range to be fully covered")
	}
}

func TestClipRangeAgainstSpans(t *testing.T) {
	covered := []ScreenSpan{{L: 10, R: 20}, {L: 30, R: 40}}
	out := ClipRangeAgainstSpans(0, 50, covered, nil)
	if len(out) != 3 {
		t.Fatalf("range count=%d want=3", len(out))
	}
	if out[0] != (ScreenSpan{L: 0, R: 9}) {
		t.Fatalf("out[0]=%+v want=0..9", out[0])
	}
	if out[1] != (ScreenSpan{L: 21, R: 29}) {
		t.Fatalf("out[1]=%+v want=21..29", out[1])
	}
	if out[2] != (ScreenSpan{L: 41, R: 50}) {
		t.Fatalf("out[2]=%+v want=41..50", out[2])
	}
}

func TestAddSpanInPlace(t *testing.T) {
	spans := []ScreenSpan{{L: 10, R: 20}, {L: 30, R: 40}}
	spans = AddSpanInPlace(spans, 21, 29)
	if len(spans) != 1 {
		t.Fatalf("expected one merged span, got %d", len(spans))
	}
	if spans[0] != (ScreenSpan{L: 10, R: 40}) {
		t.Fatalf("unexpected merged span: %+v", spans[0])
	}

	spans = []ScreenSpan{{L: 10, R: 20}, {L: 30, R: 40}}
	spans = AddSpanInPlace(spans, 24, 26)
	if len(spans) != 3 {
		t.Fatalf("expected inserted span, got %d", len(spans))
	}
	if spans[1] != (ScreenSpan{L: 24, R: 26}) {
		t.Fatalf("unexpected inserted span: %+v", spans[1])
	}
}

func TestAddSpanInPlaceAllocFreeWithCapacity(t *testing.T) {
	spans := make([]ScreenSpan, 2, 4)
	base0 := ScreenSpan{L: 10, R: 20}
	base1 := ScreenSpan{L: 30, R: 40}

	allocs := testing.AllocsPerRun(1000, func() {
		spans[0] = base0
		spans[1] = base1
		work := spans[:2]
		work = AddSpanInPlace(work, 24, 26)
		if len(work) != 3 {
			t.Fatalf("len=%d want=3", len(work))
		}
	})
	if allocs != 0 {
		t.Fatalf("AddSpanInPlace allocs=%v want 0", allocs)
	}
}
