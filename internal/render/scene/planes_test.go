package scene

import "testing"

func TestEnsurePlaneForRange_ReusesWhenNoOverlapConflict(t *testing.T) {
	planes := make(map[PlaneKey][]*PlaneVisplane)
	key := PlaneKey{Height: 0, Light: 160, Flat: "FLOOR0_1", Floor: true}
	pl1, created := EnsurePlaneForRange(planes, key, 2, 5, 32)
	if pl1 == nil {
		t.Fatal("expected visplane allocation")
	}
	if !created {
		t.Fatal("expected first allocation to report created=true")
	}
	got, created := EnsurePlaneForRange(planes, key, 6, 9, 32)
	if got != pl1 {
		t.Fatal("expected existing visplane reuse")
	}
	if created {
		t.Fatal("expected reuse to report created=false")
	}
	if len(planes[key]) != 1 {
		t.Fatalf("visplane count=%d want=1", len(planes[key]))
	}
	if pl1.MaxX != 9 {
		t.Fatalf("maxX=%d want=9", pl1.MaxX)
	}
}

func TestEnsurePlaneForRange_SplitsOnOverlapConflict(t *testing.T) {
	planes := make(map[PlaneKey][]*PlaneVisplane)
	key := PlaneKey{Height: 0, Light: 160, Flat: "FLOOR0_1", Floor: true}
	pl1, created := EnsurePlaneForRange(planes, key, 2, 6, 32)
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
	if ok := MarkPlaneColumnRange(pl1, 4, 6, 10, ceilingClip, floorClip); !ok {
		t.Fatal("expected column mark")
	}
	pl2, created := EnsurePlaneForRange(planes, key, 4, 8, 32)
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

func TestMarkPlaneColumnRange_MergesRepeatedColumnMarks(t *testing.T) {
	key := PlaneKey{Height: 0, Light: 160, Flat: "FLOOR0_1", Floor: true}
	pl := NewPlaneVisplane(key, 2, 6, 32)
	ceilingClip := make([]int, 32)
	floorClip := make([]int, 32)
	for i := range floorClip {
		ceilingClip[i] = -1
		floorClip[i] = 20
	}

	if ok := MarkPlaneColumnRange(pl, 4, 6, 10, ceilingClip, floorClip); !ok {
		t.Fatal("expected first column mark")
	}
	if ok := MarkPlaneColumnRange(pl, 4, 8, 12, ceilingClip, floorClip); !ok {
		t.Fatal("expected repeated column mark to merge")
	}

	ix := 5
	if got := int(pl.Top[ix]); got != 6 {
		t.Fatalf("top=%d want=6", got)
	}
	if got := int(pl.Bottom[ix]); got != 12 {
		t.Fatalf("bottom=%d want=12", got)
	}
}

func TestMakePlaneSpans_PartialCoverageSurvivesInteriorGap(t *testing.T) {
	key := PlaneKey{Height: 0, Light: 160, Flat: "FLOOR0_1", Floor: true}
	pl := NewPlaneVisplane(key, 2, 6, 16)
	ceilingClip := make([]int, 16)
	floorClip := make([]int, 16)
	for i := range floorClip {
		ceilingClip[i] = -1
		floorClip[i] = 20
	}
	for _, x := range []int{2, 3, 5, 6} {
		if ok := MarkPlaneColumnRange(pl, x, 10, 14, ceilingClip, floorClip); !ok {
			t.Fatalf("expected column mark at x=%d", x)
		}
	}

	spans := MakePlaneSpans(pl, 20, nil)
	if len(spans) == 0 {
		t.Fatal("expected spans for partially covered floor")
	}

	var leftOK, rightOK bool
	for _, sp := range spans {
		if sp.Y < 10 || sp.Y > 14 {
			continue
		}
		if sp.X1 == 2 && sp.X2 == 3 {
			leftOK = true
		}
		if sp.X1 == 5 && sp.X2 == 6 {
			rightOK = true
		}
	}
	if !leftOK || !rightOK {
		t.Fatalf("expected spans on both sides of the gap, got %+v", spans)
	}
}
