package doomruntime

import "testing"

func TestVerticalFocalLength_SourcePortGeometryAspectCorrectionAppliesPixelAspect(t *testing.T) {
	g := &game{geometryAspectY: doomPixelAspect}

	got := g.verticalFocalLength(100)
	want := 100 * doomPixelAspect
	if got != want {
		t.Fatalf("verticalFocalLength=%v want %v", got, want)
	}
}

func TestVerticalFocalLength_StaysUnchangedWhenGeometryAspectCorrectionDisabled(t *testing.T) {
	tests := []struct {
		name string
		g    *game
	}{
		{name: "nil", g: nil},
		{name: "faithful", g: &game{geometryAspectY: 1}},
		{name: "sourceport disabled", g: &game{geometryAspectY: 1}},
	}

	for _, tc := range tests {
		if got := tc.g.verticalFocalLength(100); got != 100 {
			t.Fatalf("%s: verticalFocalLength=%v want 100", tc.name, got)
		}
	}
}
