package doomruntime

import "testing"

func TestEnsurePlane3DForRange_ReusesWhenNoOverlapConflict(t *testing.T) {
	planes := make(map[plane3DKey][]*plane3DVisplane)
	key := plane3DKey{height: 0, light: 160, flatID: 1, floor: true}
	pl1, created := ensurePlane3DForRange(planes, key, 2, 5, 32)
	if pl1 == nil {
		t.Fatal("expected visplane allocation")
	}
	if !created {
		t.Fatal("expected first allocation to report created=true")
	}
	got, created := ensurePlane3DForRange(planes, key, 6, 9, 32)
	if got != pl1 {
		t.Fatal("expected existing visplane reuse")
	}
	if created {
		t.Fatal("expected reuse to report created=false")
	}
	if len(planes[key]) != 1 {
		t.Fatalf("visplane count=%d want=1", len(planes[key]))
	}
	if pl1.maxX != 9 {
		t.Fatalf("maxX=%d want=9", pl1.maxX)
	}
}

func TestEnsurePlane3DForRange_SplitsOnOverlapConflict(t *testing.T) {
	planes := make(map[plane3DKey][]*plane3DVisplane)
	key := plane3DKey{height: 0, light: 160, flatID: 1, floor: true}
	pl1, created := ensurePlane3DForRange(planes, key, 2, 6, 32)
	if pl1 == nil {
		t.Fatal("expected visplane allocation")
	}
	if !created {
		t.Fatal("expected first allocation to report created=true")
	}
	ceilingClip := make([]int, 32)
	floorClip := make([]int, 32)
	for i := range floorClip {
		ceilingClip[i] = -1
		floorClip[i] = 20
	}
	if ok := markPlane3DColumnRange(pl1, 4, 6, 10, ceilingClip, floorClip); !ok {
		t.Fatal("expected column mark")
	}
	pl2, created := ensurePlane3DForRange(planes, key, 4, 8, 32)
	if pl2 == nil {
		t.Fatal("expected second visplane allocation")
	}
	if !created {
		t.Fatal("expected overlap split to report created=true")
	}
	if pl2 == pl1 {
		t.Fatal("expected split visplane, got reuse")
	}
	if len(planes[key]) != 2 {
		t.Fatalf("visplane count=%d want=2", len(planes[key]))
	}
}

func TestMarkPlane3DColumnRange_MergesRepeatedColumnMarks(t *testing.T) {
	key := plane3DKey{height: 0, light: 160, flatID: 1, floor: true}
	pl := newPlane3DVisplane(key, 2, 6, 32)
	ceilingClip := make([]int, 32)
	floorClip := make([]int, 32)
	for i := range floorClip {
		ceilingClip[i] = -1
		floorClip[i] = 20
	}

	if ok := markPlane3DColumnRange(pl, 4, 6, 10, ceilingClip, floorClip); !ok {
		t.Fatal("expected first column mark")
	}
	if ok := markPlane3DColumnRange(pl, 4, 8, 12, ceilingClip, floorClip); !ok {
		t.Fatal("expected repeated column mark to merge")
	}

	ix := 5
	if got := int(pl.top[ix]); got != 6 {
		t.Fatalf("top=%d want=6", got)
	}
	if got := int(pl.bottom[ix]); got != 12 {
		t.Fatalf("bottom=%d want=12", got)
	}
}

func TestMakePlane3DSpans_PartialCoverageSurvivesInteriorGap(t *testing.T) {
	key := plane3DKey{height: 0, light: 160, flatID: 1, floor: true}
	pl := newPlane3DVisplane(key, 2, 6, 16)
	ceilingClip := make([]int, 16)
	floorClip := make([]int, 16)
	for i := range floorClip {
		ceilingClip[i] = -1
		floorClip[i] = 20
	}

	// Simulate a floor visible on both sides of a narrow door obstruction.
	for _, x := range []int{2, 3, 5, 6} {
		if ok := markPlane3DColumnRange(pl, x, 10, 14, ceilingClip, floorClip); !ok {
			t.Fatalf("expected column mark at x=%d", x)
		}
	}

	spans := makePlane3DSpans(pl, 20, nil)
	if len(spans) == 0 {
		t.Fatal("expected spans for partially covered floor")
	}

	var leftOK, rightOK bool
	for _, sp := range spans {
		if sp.y < 10 || sp.y > 14 {
			continue
		}
		if sp.x1 == 2 && sp.x2 == 3 {
			leftOK = true
		}
		if sp.x1 == 5 && sp.x2 == 6 {
			rightOK = true
		}
	}
	if !leftOK || !rightOK {
		t.Fatalf("expected spans on both sides of the gap, got %+v", spans)
	}
}
