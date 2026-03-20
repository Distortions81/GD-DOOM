package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestHandleExitSpecialSetsLevelExit(t *testing.T) {
	g := &game{
		lineSpecial: []uint16{11},
	}
	ok := g.handleExitSpecial(0, 11, mapdata.TriggerUse)
	if !ok {
		t.Fatal("handleExitSpecial() should activate for use exit line")
	}
	if !g.levelExitRequested || g.secretLevelExit {
		t.Fatalf("unexpected exit state: levelExit=%t secret=%t", g.levelExitRequested, g.secretLevelExit)
	}
	if g.lineSpecial[0] != 0 {
		t.Fatalf("line special should be consumed, got %d", g.lineSpecial[0])
	}
}

func TestCheckWalkSpecialLinesTriggersExitOnCrossing(t *testing.T) {
	g := &game{
		lineSpecial: []uint16{52},
		lines: []physLine{
			{
				idx:      0,
				x1:       0,
				y1:       100,
				x2:       0,
				y2:       -100,
				dx:       0,
				dy:       -200,
				slope:    slopeVertical,
				sideNum0: -1,
				sideNum1: -1,
			},
		},
	}
	g.checkWalkSpecialLines(-10, 0, 10, 0)
	if !g.levelExitRequested {
		t.Fatal("walk exit should trigger on line crossing")
	}
}

func TestCheckWalkSpecialLinesTriggersExitOnReverseCrossing(t *testing.T) {
	g := &game{
		lineSpecial: []uint16{52},
		lines: []physLine{
			{
				idx:      0,
				x1:       0,
				y1:       100,
				x2:       0,
				y2:       -100,
				dx:       0,
				dy:       -200,
				slope:    slopeVertical,
				sideNum0: -1,
				sideNum1: -1,
			},
		},
	}
	g.checkWalkSpecialLines(10, 0, -10, 0)
	if !g.levelExitRequested {
		t.Fatal("walk exit should trigger on reverse line crossing")
	}
}

func TestHandleExitSpecialBlockedWhenDead(t *testing.T) {
	g := &game{
		isDead:      true,
		lineSpecial: []uint16{11},
	}
	ok := g.handleExitSpecial(0, 11, mapdata.TriggerUse)
	if ok {
		t.Fatal("dead player should not activate level exit")
	}
	if g.levelExitRequested {
		t.Fatal("dead player should not request level exit")
	}
}

func TestCheckWalkSpecialLines_TriggersWalkDoorAndConsumesOneShot(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{Special: 2, Tag: 7},
			},
			Sectors: []mapdata.Sector{
				{Tag: 7, FloorHeight: 0, CeilingHeight: 128},
			},
		},
		lineSpecial: []uint16{2},
		lines: []physLine{
			{
				idx:      0,
				x1:       0,
				y1:       100,
				x2:       0,
				y2:       -100,
				dx:       0,
				dy:       -200,
				slope:    slopeVertical,
				sideNum0: -1,
				sideNum1: -1,
			},
		},
		sectorFloor: []int64{0},
		sectorCeil:  []int64{128 * fracUnit},
		doors:       make(map[int]*doorThinker),
	}

	g.checkWalkSpecialLines(-10, 0, 10, 0)
	if len(g.doors) != 1 {
		t.Fatalf("walk door special should spawn door thinker, got %d", len(g.doors))
	}
	if g.lineSpecial[0] != 0 {
		t.Fatalf("one-shot walk door special should be consumed, got %d", g.lineSpecial[0])
	}
}
