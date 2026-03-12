package gameplay

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestCloneMapForRestartDeepCopiesMutableSlices(t *testing.T) {
	src := &mapdata.Map{
		Name:     "E1M1",
		Things:   []mapdata.Thing{{Type: 1}},
		Vertexes: []mapdata.Vertex{{X: 1}},
		Sectors:  []mapdata.Sector{{Light: 160}},
		Reject:   []byte{1, 2, 3},
		BlockMap: &mapdata.BlockMap{Offsets: []uint16{1}, Cells: [][]int16{{2, 3}}},
	}

	dup := CloneMapForRestart(src)
	if dup == nil || dup == src {
		t.Fatal("clone should return a distinct map")
	}

	dup.Sectors[0].Light = 32
	dup.Reject[0] = 9
	dup.BlockMap.Offsets[0] = 7
	dup.BlockMap.Cells[0][0] = 8

	if src.Sectors[0].Light != 160 {
		t.Fatalf("source sector light mutated: got %d want 160", src.Sectors[0].Light)
	}
	if src.Reject[0] != 1 {
		t.Fatalf("source reject mutated: got %d want 1", src.Reject[0])
	}
	if src.BlockMap.Offsets[0] != 1 {
		t.Fatalf("source blockmap offset mutated: got %d want 1", src.BlockMap.Offsets[0])
	}
	if src.BlockMap.Cells[0][0] != 2 {
		t.Fatalf("source blockmap cell mutated: got %d want 2", src.BlockMap.Cells[0][0])
	}
}

func TestClampHelpers(t *testing.T) {
	if got := ClampDetailLevel(99, false, 3, 5); got != 2 {
		t.Fatalf("faithful detail clamp=%d want 2", got)
	}
	if got := ClampDetailLevel(99, true, 3, 5); got != 4 {
		t.Fatalf("source port detail clamp=%d want 4", got)
	}
	if got := ClampGamma(99, 7); got != 6 {
		t.Fatalf("gamma clamp=%d want 6", got)
	}
	if got := ClampIDDT(99); got != 2 {
		t.Fatalf("iddt clamp=%d want 2", got)
	}
	if got := ClampVolume(-1); got != 0 {
		t.Fatalf("volume clamp=%v want 0", got)
	}
	if got := ClampVolume(2); got != 1 {
		t.Fatalf("volume clamp=%v want 1", got)
	}
	if got := ClampOPLVolume(99, 4); got != 4 {
		t.Fatalf("opl clamp=%v want 4", got)
	}
	if got := NormalizeReveal(99, false, 1, 2); got != 1 {
		t.Fatalf("faithful reveal=%d want 1", got)
	}
	if got := NormalizeReveal(99, true, 1, 2); got != 2 {
		t.Fatalf("source port reveal=%d want 2", got)
	}
}

func TestRestartMapForRespawn(t *testing.T) {
	cur := &mapdata.Map{Name: "E1M1", Sectors: []mapdata.Sector{{Light: 32}}}
	pristine := &mapdata.Map{Name: "E1M1", Sectors: []mapdata.Sector{{Light: 160}}}

	if got := RestartMapForRespawn(cur, pristine, false); got != cur {
		t.Fatal("multiplayer respawn should keep current map")
	}

	got := RestartMapForRespawn(cur, pristine, true)
	if got == nil || got == cur {
		t.Fatal("single-player respawn should clone pristine map")
	}
	if got.Sectors[0].Light != 160 {
		t.Fatalf("single-player respawn light=%d want 160", got.Sectors[0].Light)
	}
}
