package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestNewGameStartsInWalkModeEvenWhenStartInMapMode(t *testing.T) {
	g := newGame(&mapdata.Map{
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 0},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}, Options{StartInMapMode: true})
	if g.mode != viewWalk {
		t.Fatalf("mode=%v want=%v", g.mode, viewWalk)
	}
}
