package app

import "testing"

func TestResolveForceWASMMode(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "absent", args: nil, want: false},
		{name: "present", args: []string{"-wasm-mode"}, want: true},
		{name: "explicit true", args: []string{"-wasm-mode=true"}, want: true},
		{name: "explicit false", args: []string{"-wasm-mode=false"}, want: false},
		{name: "invalid", args: []string{"-wasm-mode=maybe"}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveForceWASMMode(tc.args); got != tc.want {
				t.Fatalf("resolveForceWASMMode(%v)=%t want %t", tc.args, got, tc.want)
			}
		})
	}
}
