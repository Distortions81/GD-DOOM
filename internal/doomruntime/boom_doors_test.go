package doomruntime

import (
	"gddoom/internal/mapdata"
	"testing"
)

func TestBoomGeneralizedDoorParameters(t *testing.T) {
	for kind := uint16(0); kind < 4; kind++ {
		for speed := uint16(0); speed < 4; speed++ {
			for delay := uint16(0); delay < 4; delay++ {
				special := uint16(0x3c00) | (kind << 5) | (speed << 3) | (delay << 8)
				g := boomFloorGame(special)
				if !g.activateDoorLine(0, mapdata.LookupLineSpecial(special), true) {
					t.Fatalf("door %x did not activate", special)
				}
				d := g.doors[1]
				if d.speed != vDoorSpeed<<speed || d.topWait != [...]int{35, 140, 315, 1050}[delay] {
					t.Fatalf("door %x parameters=%+v", special, d)
				}
				if kind == 2 && d.typ != doorGenCloseWaitOpen {
					t.Fatal("generalized close/wait/open used legacy wait semantics")
				}
			}
		}
	}
}

func TestBoomGeneralizedDoorWaitDoesNotChangeLegacySaves(t *testing.T) {
	for _, gen := range []bool{false, true} {
		g := boomFloorGame(0)
		g.sectorCeil[1] = g.sectorFloor[1]
		d := &doorThinker{sector: 1, typ: doorClose30ThenOpen, direction: -1, speed: 2 * fracUnit, topWait: 4 * 35}
		if gen {
			d.typ = doorGenCloseWaitOpen
			d.topWait = 35
		}
		g.doors = map[int]*doorThinker{1: d}
		g.tickDoor(1, d)
		want := 1050
		if gen {
			want = 35
		}
		if d.direction != 0 || d.topCountdown != want {
			t.Fatalf("generalized=%v wait=%d want=%d", gen, d.topCountdown, want)
		}
	}
}
