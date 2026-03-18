package doomruntime

import "testing"

func TestEnsureFloorVisplaneForRange_ReusesWhenNoOverlapConflict(t *testing.T) {
	g := &game{viewW: 32, viewH: 20}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 160}

	pl1, created := g.ensureFloorVisplaneForRange(key, 2, 5)
	if pl1 == nil || !created {
		t.Fatal("expected first visplane allocation")
	}
	pl2, created := g.ensureFloorVisplaneForRange(key, 6, 9)
	if pl2 != pl1 {
		t.Fatal("expected visplane reuse")
	}
	if created {
		t.Fatal("reuse should report created=false")
	}
	if got := len(g.floorPlanes[key]); got != 1 {
		t.Fatalf("visplane count=%d want=1", got)
	}
}

func TestEnsureFloorVisplaneForRange_SplitsOnOverlapConflict(t *testing.T) {
	g := &game{viewW: 32, viewH: 20}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 160}

	pl1, created := g.ensureFloorVisplaneForRange(key, 2, 6)
	if pl1 == nil || !created {
		t.Fatal("expected first visplane allocation")
	}
	if ok := markFloorColumnRange(pl1, 4, 3, 10, g.floorClip, g.ceilingClip); !ok {
		t.Fatal("expected column mark")
	}
	pl2, created := g.ensureFloorVisplaneForRange(key, 4, 8)
	if pl2 == nil || !created {
		t.Fatal("expected split visplane allocation")
	}
	if pl2 == pl1 {
		t.Fatal("expected a new visplane on overlap conflict")
	}
	if got := len(g.floorPlanes[key]); got != 2 {
		t.Fatalf("visplane count=%d want=2", got)
	}
}
