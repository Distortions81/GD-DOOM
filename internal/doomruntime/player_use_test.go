package doomruntime

import "testing"

func TestCollectUseLineInterceptsPreservesInsertionOrderOnTie(t *testing.T) {
	g := &game{
		lines: []physLine{
			{idx: 10, x1: 64 * fracUnit, y1: -8 * fracUnit, x2: 64 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit},
			{idx: 11, x1: 64 * fracUnit, y1: -8 * fracUnit, x2: 64 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit},
		},
	}
	intercepts := g.collectUseLineIntercepts(0, 0, 128*fracUnit, 0)
	if len(intercepts) != 2 {
		t.Fatalf("intercepts=%d want=2", len(intercepts))
	}
	if intercepts[0].line != 10 {
		t.Fatalf("first intercept line=%d want=10", intercepts[0].line)
	}
	if intercepts[1].line != 11 {
		t.Fatalf("second intercept line=%d want=11", intercepts[1].line)
	}
}

func TestPeekUseTargetLine_IgnoresInterceptsBeyondUseRangeLikeDoom(t *testing.T) {
	g := &game{
		lines:       []physLine{{idx: 0, x1: 66 * fracUnit, y1: -8 * fracUnit, x2: 66 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit, sideNum1: 1}},
		physForLine: []int{0},
		lineSpecial: []uint16{31},
		p:           player{angle: 0},
	}
	if got, tr := g.peekUseTargetLine(); got != -1 || tr != useTraceNone {
		t.Fatalf("peekUseTargetLine beyond range=(%d,%v) want (-1,%v)", got, tr, useTraceNone)
	}
}

func TestPeekUseTargetLine_UsesSpecialWithinUseRange(t *testing.T) {
	g := &game{
		lines:       []physLine{{idx: 0, x1: 63 * fracUnit, y1: -8 * fracUnit, x2: 63 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit, sideNum1: 1}},
		physForLine: []int{0},
		lineSpecial: []uint16{31},
		p:           player{angle: 0},
	}
	if got, tr := g.peekUseTargetLine(); got != 0 || tr != useTraceSpecial {
		t.Fatalf("peekUseTargetLine within range=(%d,%v) want (0,%v)", got, tr, useTraceSpecial)
	}
}
