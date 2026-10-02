package doomruntime

import "testing"

func TestGPUCutoutPassesInterleaveFuzzAndReuseStorage(t *testing.T) {
	r := &gpuRenderer{orderedCutouts: true}
	back := r.opaqueCutoutCommands()
	if r.opaqueCutoutCommands() != back {
		t.Fatal("adjacent opaque draws should share a pass")
	}
	r.startFuzzCutout()
	r.opaqueCutoutCommands()
	r.startFuzzCutout()
	r.startFuzzCutout()
	r.opaqueCutoutCommands()
	want := []bool{false, true, false, true, true, false}
	if r.cutoutPassesUsed != len(want) {
		t.Fatalf("passes=%d want %d", r.cutoutPassesUsed, len(want))
	}
	for i, fuzz := range want {
		if r.cutoutPasses[i].fuzz != fuzz {
			t.Fatalf("pass %d fuzz=%t want %t", i, r.cutoutPasses[i].fuzz, fuzz)
		}
	}
	storage := &r.cutoutPasses[0]
	r.cutoutPassesUsed = 0
	r.opaqueCutoutCommands()
	if &r.cutoutPasses[0] != storage {
		t.Fatal("frame reset should reuse pass storage")
	}
}

func TestPainterOrderBypassesCoverage(t *testing.T) {
	g := &game{viewW: 8, viewH: 8, cutoutCoverageBits: []uint64{^uint64(0)}, cutoutPainterOrder: true}
	if g.cutoutCoveredAtIndex(10) || g.cutoutMaskRectFullyCovered(0, 7, 0, 7) {
		t.Fatal("farther sprites must not prevent nearer painter-order draws")
	}
	spans := g.appendUncoveredCutoutSpans(8, 0, 7, nil)
	if len(spans) != 1 || spans[0] != (solidSpan{L: 0, R: 7}) {
		t.Fatalf("painter spans=%v", spans)
	}
	g.cutoutCoverageBits[0] = 0
	g.markCutoutCoveredAtIndex(10)
	g.markCutoutRowSpanCovered(8, 0, 7)
	if g.cutoutCoverageBits[0] != 0 {
		t.Fatal("painter-order draws should not write front-to-back coverage")
	}
}
