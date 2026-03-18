package doomruntime

import "testing"

func TestNormalizeFlatName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "uppercases", in: "floor0_1", want: "FLOOR0_1"},
		{name: "truncates", in: "abcdefghijk", want: "ABCDEFGH"},
		{name: "stops at nul", in: "flat\x00tail", want: "FLAT"},
	}
	for _, tt := range tests {
		if got := normalizeFlatName(tt.in); got != tt.want {
			t.Fatalf("%s: normalizeFlatName(%q)=%q want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

func TestNormalizeFlatNameCanonicalAllocFree(t *testing.T) {
	const name = "FLOOR0_1"
	allocs := testing.AllocsPerRun(1000, func() {
		if got := normalizeFlatName(name); got != name {
			t.Fatalf("normalizeFlatName(%q)=%q want %q", name, got, name)
		}
	})
	if allocs != 0 {
		t.Fatalf("normalizeFlatName canonical allocs=%v want 0", allocs)
	}
}
