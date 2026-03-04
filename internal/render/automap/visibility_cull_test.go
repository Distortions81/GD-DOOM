package automap

import (
	"math"
	"testing"
)

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

func TestClipRangeAgainstSolidSpans(t *testing.T) {
	covered := []solidSpan{{l: 10, r: 20}, {l: 30, r: 40}}

	out := clipRangeAgainstSolidSpans(0, 50, covered, nil)
	if len(out) != 3 {
		t.Fatalf("range count=%d want=3", len(out))
	}
	if out[0] != (solidSpan{l: 0, r: 9}) {
		t.Fatalf("out[0]=%+v want=0..9", out[0])
	}
	if out[1] != (solidSpan{l: 21, r: 29}) {
		t.Fatalf("out[1]=%+v want=21..29", out[1])
	}
	if out[2] != (solidSpan{l: 41, r: 50}) {
		t.Fatalf("out[2]=%+v want=41..50", out[2])
	}
}

func TestClipRangeAgainstSolidSpans_ReuseScratch(t *testing.T) {
	covered := []solidSpan{{l: 5, r: 15}}
	scratch := make([]solidSpan, 0, 8)

	out := clipRangeAgainstSolidSpans(0, 20, covered, scratch)
	if len(out) != 2 {
		t.Fatalf("range count=%d want=2", len(out))
	}
	if out[0] != (solidSpan{l: 0, r: 4}) || out[1] != (solidSpan{l: 16, r: 20}) {
		t.Fatalf("unexpected ranges: %+v", out)
	}

	out = clipRangeAgainstSolidSpans(6, 14, covered, out)
	if len(out) != 0 {
		t.Fatalf("expected fully covered range, got %+v", out)
	}
}

func TestWallColumnWorkEstimate_E1M1Startup(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)

	oldCols := estimateWallColumnWork(g, false)
	newCols := estimateWallColumnWork(g, true)
	if oldCols <= 0 {
		t.Fatalf("unexpected old column count: %d", oldCols)
	}
	if newCols > oldCols {
		t.Fatalf("new column count should not exceed old count: old=%d new=%d", oldCols, newCols)
	}
	saved := oldCols - newCols
	pct := float64(saved) * 100 / float64(oldCols)
	t.Logf("wall-column-work old=%d new=%d saved=%d (%.2f%%)", oldCols, newCols, saved, pct)
}

func estimateWallColumnWork(g *game, clipPartials bool) int {
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	focal := doomFocalLength(g.viewW)
	near := 2.0

	prepass := g.buildWallSegPrepassParallel(g.visibleSegIndicesPseudo3D(), camX, camY, ca, sa, focal, near)
	solid := g.beginSolid3DFrame()
	scratch := make([]solidSpan, 0, 32)
	cols := 0

	for _, pp := range prepass {
		if !pp.ok {
			continue
		}
		if solidFullyCovered(solid, pp.minSX, pp.maxSX) {
			continue
		}
		front, back := g.segSectors(pp.segIdx)
		if front == nil {
			continue
		}
		solidWall := back == nil
		if back != nil && (back.CeilingHeight <= front.FloorHeight || back.FloorHeight >= front.CeilingHeight) {
			solidWall = true
		}

		if clipPartials {
			vis := clipRangeAgainstSolidSpans(pp.minSX, pp.maxSX, solid, scratch[:0])
			scratch = vis
			for _, r := range vis {
				cols += r.r - r.l + 1
			}
		} else {
			cols += pp.maxSX - pp.minSX + 1
		}

		if solidWall {
			solid = addSolidSpan(solid, pp.minSX, pp.maxSX)
		}
	}
	return cols
}
