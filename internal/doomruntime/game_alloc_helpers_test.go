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
