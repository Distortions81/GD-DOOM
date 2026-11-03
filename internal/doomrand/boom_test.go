package doomrand

import "testing"

func TestBoomStreamsSnapshotAndVanillaIsolation(t *testing.T) {
	for _, insurance := range []bool{false, true} {
		r := New()
		r.ConfigureBoom(1993, insurance)
		s := r.Capture()
		var first [ClassCount]int
		for class := Class(0); class < ClassCount; class++ {
			first[class] = r.PRandom(class)
		}
		r.Restore(s)
		for class := Class(0); class < ClassCount; class++ {
			if r.PRandom(class) != first[class] {
				t.Fatalf("restore changed class %d", class)
			}
		}
		r.Restore(s)
		a := r.PRandom(Genlift)
		r.Restore(s)
		r.PRandom(See)
		b := r.PRandom(Genlift)
		if insurance && a != b {
			t.Fatal("insurance classes interfere")
		}
		r.Restore(s)
		r.MRandom()
		if got := r.PRandom(Genlift); got != a {
			t.Fatal("misc affected gameplay")
		}
	}
	r := New()
	r.ConfigureBoom(0xffffffff, true)
	r.Clear()
	if r.Capture().Boom || r.PRandom(Genlift) != 8 || r.PRandom(See) != 109 || r.MRandom() != 8 {
		t.Fatal("Boom mode leaked into vanilla reset")
	}
}
func TestBoomDSDAGoldenDraws(t *testing.T) {
	// DSDA-Doom 0.27.5 P_Random(pr_genlift), rngseed=1993, logic tic zero.
	r := New()
	r.ConfigureBoom(1993, false)
	want := []int{36, 10, 193, 98, 219, 53, 91, 132, 22, 212, 109, 91}
	for i, v := range want {
		if got := r.PRandom(Genlift); got != v {
			t.Fatalf("draw %d = %d, want %d", i, got, v)
		}
	}
	before := r.Capture()
	peek := r.PRandomOffset(8)
	if r.Capture() != before {
		t.Fatal("peek advanced seeded state")
	}
	value := 0
	for i := 0; i <= 8; i++ {
		value = r.PRandom()
	}
	if value != peek {
		t.Fatal("peek differs from next seeded values")
	}
}
