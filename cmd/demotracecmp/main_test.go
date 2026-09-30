package main

import "testing"

func TestCeilingOldDirectionComparedOnlyInStasis(t *testing.T) {
	for _, direction := range []float64{-1, 0, 1} {
		left := map[string]any{"kind": "ceiling", "direction": direction, "olddirection": float64(16843009)}
		right := map[string]any{"kind": "ceiling", "direction": direction, "olddirection": float64(-1)}
		_, _, _, differs := firstDiff("root.specials[0]", left, right)
		if differs != (direction == 0) {
			t.Fatalf("direction=%v differs=%t; olddirection matters only in stasis", direction, differs)
		}
	}
}
