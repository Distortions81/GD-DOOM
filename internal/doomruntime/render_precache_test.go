package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestCollectMapTextureUsageExpandsAnimatedRefs(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorPic: "NUKAGE1", CeilingPic: "F_SKY1"},
			},
			Sidedefs: []mapdata.Sidedef{
				{Mid: "BLODGR1", Top: "BRICK1", Bottom: "-"},
			},
		},
	}

	flatKeys, wallKeys := g.collectMapTextureUsage()

	for _, key := range []string{"NUKAGE1", "NUKAGE2", "NUKAGE3", "F_SKY1"} {
		if !containsString(flatKeys, key) {
			t.Fatalf("missing flat key %q in %v", key, flatKeys)
		}
	}
	for _, key := range []string{"BLODGR1", "BLODGR2", "BLODGR3", "BLODGR4", "BRICK1"} {
		if !containsString(wallKeys, key) {
			t.Fatalf("missing wall key %q in %v", key, wallKeys)
		}
	}
}

func TestInitFlatIDTableSizesPlaneFlatCaches(t *testing.T) {
	g := &game{}
	g.initFlatIDTable([]string{"FLOOR0_1", "CEIL1_1"})

	if got, want := len(g.flatIDToName), 2; got != want {
		t.Fatalf("len(flatIDToName) = %d, want %d", got, want)
	}
	if got, want := len(g.planeFlatCache32Scratch), 2; got != want {
		t.Fatalf("len(planeFlatCache32Scratch) = %d, want %d", got, want)
	}
	if got, want := len(g.planeFlatCacheIndexedScratch), 2; got != want {
		t.Fatalf("len(planeFlatCacheIndexedScratch) = %d, want %d", got, want)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
