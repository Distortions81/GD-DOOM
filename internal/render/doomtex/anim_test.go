package doomtex

import (
	"gddoom/internal/wad"
	"reflect"
	"testing"
)

func TestAnimationCyclesUseRelativeIndices(t *testing.T) {
	order := []string{"UNUSED", "A", "B", "C"}
	sequences := buildAnimSequences(order, []AnimDef{{StartName: "A", EndName: "C"}})
	for i, name := range order[1:] {
		for tick := 0; tick < 12; tick++ {
			want := order[1+(tick+i)%3] // P_UpdateSpecials: base + (leveltime/8 + i)%n
			if got := sequences[name][tick%3]; got != want {
				t.Fatalf("%s tick=%d got=%s want=%s", name, tick, got, want)
			}
		}
	}
}

func TestFlatAnimationCountsNestedMarkers(t *testing.T) {
	f := &wad.File{Lumps: []wad.Lump{{Name: "F_START"}, {Name: "F1_START"}, {Name: "A", Size: 4096}, {Name: "B", Size: 4096}, {Name: "C", Size: 4096}, {Name: "F1_END"}, {Name: "F_END"}}}
	got := LoadFlatAnimSequences(f, []AnimDef{{StartName: "A", EndName: "C"}})
	if !reflect.DeepEqual(got["A"], []string{"A", "B", "C"}) {
		t.Fatalf("incorrect relative phase: %v", got)
	}
}
