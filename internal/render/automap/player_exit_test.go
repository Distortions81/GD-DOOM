package automap

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
				idx:   0,
				x1:    0,
				y1:    100,
				x2:    0,
				y2:    -100,
				dx:    0,
				dy:    -200,
				slope: slopeVertical,
			},
		},
	}
	g.checkWalkSpecialLines(-10, 0, 10, 0)
	if !g.levelExitRequested {
		t.Fatal("walk exit should trigger on line crossing")
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
