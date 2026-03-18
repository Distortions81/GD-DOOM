package doomruntime

import "testing"

func TestResetFloorVisplaneFrameInitializesClipBuffers(t *testing.T) {
	g := &game{viewW: 4, viewH: 3}
	g.resetFloorVisplaneFrame()
	if len(g.floorClip) != 4 || len(g.ceilingClip) != 4 {
		t.Fatalf("clip len mismatch floor=%d ceil=%d", len(g.floorClip), len(g.ceilingClip))
	}
	for i := 0; i < 4; i++ {
		if g.floorClip[i] != 3 {
			t.Fatalf("floorClip[%d]=%d want 3", i, g.floorClip[i])
		}
		if g.ceilingClip[i] != floorUnset {
			t.Fatalf("ceilingClip[%d]=%d want %d", i, g.ceilingClip[i], floorUnset)
		}
	}
	if len(g.floorSpans) != 0 {
		t.Fatalf("expected empty spans, got %d", len(g.floorSpans))
	}
}

func TestFloorVisplaneForKeyAndResetSetsSentinels(t *testing.T) {
	g := &game{viewW: 5, viewH: 4}
	g.resetFloorVisplaneFrame()
	key := floorPlaneKey{flat: "FLOOR0_1", floorH: 0, light: 128}
	pl := g.floorVisplaneForKey(key)
	if got, want := len(pl.top), 7; got != want {
		t.Fatalf("top len=%d want=%d", got, want)
	}
	if got, want := len(pl.bottom), 7; got != want {
		t.Fatalf("bottom len=%d want=%d", got, want)
	}
	for i := range pl.top {
		if pl.top[i] != floorUnset || pl.bottom[i] != floorUnset {
			t.Fatalf("sentinel init failed at i=%d", i)
		}
	}
	pl.minX = 1
	pl.maxX = 3
	pl.top[2] = 9
	pl.bottom[2] = 12

	g.resetFloorVisplaneFrame()
	if pl.minX != 5 || pl.maxX != -1 {
		t.Fatalf("plane bounds reset mismatch min=%d max=%d", pl.minX, pl.maxX)
	}
	for i := range pl.top {
		if pl.top[i] != floorUnset || pl.bottom[i] != floorUnset {
			t.Fatalf("sentinel reset failed at i=%d", i)
		}
	}
}
