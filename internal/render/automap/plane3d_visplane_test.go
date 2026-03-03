package automap

import "testing"

func TestEnsurePlane3DForRange_ReusesWhenNoOverlapConflict(t *testing.T) {
	planes := make(map[plane3DKey][]*plane3DVisplane)
	key := plane3DKey{height: 0, light: 160, flat: "FLOOR0_1", floor: true}
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
	key := plane3DKey{height: 0, light: 160, flat: "FLOOR0_1", floor: true}
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
