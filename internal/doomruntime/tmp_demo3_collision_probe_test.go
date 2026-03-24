package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestTmpDemo3WestMoveTouchedLines(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}
	m, err := mapdata.LoadMap(wf, "E1M7")
	if err != nil {
		t.Fatalf("load map: %v", err)
	}
	g := newGame(m, Options{Width: 320, Height: 200})
	x := int64(-54536512)
	y := int64(-45279296)
	r := thingTypeRadius(1)
	box := [4]int64{y + r, y - r, x + r, x - r}
	t.Logf("bbox top=%d bottom=%d right=%d left=%d", box[0], box[1], box[2], box[3])
	for _, idx := range []int{254, 255, 257, 900, 903, 904, 905} {
		phys := g.physForLine[idx]
		ld := g.lines[phys]
		side := g.boxOnLineSide(box, ld)
		t.Logf("line=%d bbox=[%d %d %d %d] side=%d flags=0x%04x special=%d front=%d back=%d",
			idx, ld.bbox[0], ld.bbox[1], ld.bbox[2], ld.bbox[3], side, ld.flags, ld.special, secForSideProbe(m, ld.sideNum0), secForSideProbe(m, ld.sideNum1))
	}
}

func secForSideProbe(m *mapdata.Map, side int16) int {
	if side < 0 || int(side) >= len(m.Sidedefs) {
		return -1
	}
	return int(m.Sidedefs[int(side)].Sector)
}

func TestTmpDemo3WestMoveBlockmapVsFullScan(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}
	m, err := mapdata.LoadMap(wf, "E1M7")
	if err != nil {
		t.Fatalf("load map: %v", err)
	}
	x := int64(-54536512)
	y := int64(-45279296)

	gBlock := newGame(m, Options{Width: 320, Height: 200})
	_, _, _, okBlock := gBlock.checkPositionForActor(x, y, thingTypeRadius(1), true, -1, true)
	t.Logf("with blockmap ok=%t", okBlock)

	mNoBlock := *m
	mNoBlock.BlockMap = nil
	gFull := newGame(&mNoBlock, Options{Width: 320, Height: 200})
	_, _, _, okFull := gFull.checkPositionForActor(x, y, thingTypeRadius(1), true, -1, true)
	t.Logf("without blockmap ok=%t", okFull)
}
