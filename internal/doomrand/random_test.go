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
