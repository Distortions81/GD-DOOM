package doomrand

import "testing"

func TestPRandomSequenceMatchesDoom(t *testing.T) {
	r := New()
	got := []int{r.PRandom(), r.PRandom(), r.PRandom(), r.PRandom(), r.PRandom()}
	want := []int{8, 109, 220, 222, 241}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PRandom[%d]=%d want %d", i, got[i], want[i])
		}
	}
}

func TestMRandomAndPRandomHaveIndependentIndices(t *testing.T) {
	r := New()
	if r.PRandom() != 8 {
		t.Fatalf("first PRandom mismatch")
	}
	if r.MRandom() != 8 {
		t.Fatalf("first MRandom mismatch")
	}
	if r.PRandom() != 109 {
		t.Fatalf("second PRandom mismatch")
	}
	if r.MRandom() != 109 {
		t.Fatalf("second MRandom mismatch")
	}
}

func TestWrapAndClear(t *testing.T) {
	r := New()
	for i := 0; i < 256; i++ {
		_ = r.PRandom()
	}
	if got := r.PRandom(); got != 8 {
		t.Fatalf("post-wrap PRandom=%d want 8", got)
	}
	r.Clear()
	if got := r.PRandom(); got != 8 {
		t.Fatalf("post-clear PRandom=%d want 8", got)
	}
}

func TestGlobalFunctions(t *testing.T) {
	Clear()
	if got := PRandom(); got != 8 {
		t.Fatalf("global PRandom=%d want 8", got)
	}
	Clear()
	if got := MRandom(); got != 8 {
		t.Fatalf("global MRandom=%d want 8", got)
	}
}

func TestOffsetHelpersDoNotAdvanceState(t *testing.T) {
	r := New()
	rnd0, prnd0 := r.State()
	if got := r.PRandomOffset(0); got != 8 {
		t.Fatalf("PRandomOffset(0)=%d want 8", got)
	}
	if got := r.PRandomOffset(1); got != 109 {
		t.Fatalf("PRandomOffset(1)=%d want 109", got)
	}
	if got := r.MRandomOffset(0); got != 8 {
		t.Fatalf("MRandomOffset(0)=%d want 8", got)
	}
	rnd1, prnd1 := r.State()
	if rnd1 != rnd0 || prnd1 != prnd0 {
		t.Fatalf("offset helpers should not advance state: before=(%d,%d) after=(%d,%d)", rnd0, prnd0, rnd1, prnd1)
	}
	if got := r.PRandom(); got != 8 {
		t.Fatalf("PRandom after offset peek=%d want 8", got)
	}
}

func TestGlobalOffsetHelpers(t *testing.T) {
	Clear()
	rnd0, prnd0 := State()
	if got := PRandomOffset(2); got != 220 {
		t.Fatalf("PRandomOffset(2)=%d want 220", got)
	}
	if got := MRandomOffset(3); got != 222 {
		t.Fatalf("MRandomOffset(3)=%d want 222", got)
	}
	rnd1, prnd1 := State()
	if rnd1 != rnd0 || prnd1 != prnd0 {
		t.Fatalf("global offset helpers should not advance state: before=(%d,%d) after=(%d,%d)", rnd0, prnd0, rnd1, prnd1)
	}
}
