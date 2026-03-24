package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestTmpSectorProbeE1M7(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}
	m, err := mapdata.LoadMap(wf, "E1M7")
	if err != nil {
		t.Fatalf("load map: %v", err)
	}
	g := newGame(m, Options{Width: 320, Height: 200})
	x := int64(82747520)
	y := int64(-140784640)
	ss := g.subSectorAtFixed(x, y)
	t.Logf("subsector=%d sectorAt=%d sectorForSub=%d", ss, g.sectorAt(x, y), g.sectorForSubSector(ss))
}
