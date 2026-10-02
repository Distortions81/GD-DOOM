package doomruntime

import (
	"fmt"
	"testing"
)

func TestManualDoorUsesPlatformWaitAsDirection(t *testing.T) {
	for _, special := range []uint16{1, 26, 27, 28, 117} {
		for _, isPlayer := range []bool{false, true} {
			for _, wait := range []int{-1, 1, platWaitTics} {
				t.Run(fmt.Sprintf("special=%d/player=%t/wait=%d", special, isPlayer, wait), func(t *testing.T) {
					g := newDoorTimingGame(0)
					g.m.Linedefs[0].Special = special
					pt := &platThinker{order: 7, sector: 0, status: platStatusWaiting, wait: wait, count: 34}
					g.plats = map[int]*platThinker{0: pt}
					allowed := isPlayer || special == 1 || special == 117
					wantWait := wait
					if allowed {
						if wait == -1 {
							wantWait = 1
						} else if isPlayer {
							wantWait = -1
						}
					}
					if got := g.evVerticalDoor(0, isPlayer); got != allowed {
						t.Fatalf("activation=%t want=%t", got, allowed)
					}
					if pt.wait != wantWait || pt.count != 34 || pt.status != platStatusWaiting || pt.order != 7 {
						t.Fatalf("platform=%+v; want wait=%d, unchanged countdown and thinker", pt, wantWait)
					}
					if len(g.doors) != 0 || g.sectorCeil[0] != 128*fracUnit {
						t.Fatal("platform cast created a door or moved its ceiling")
					}
				})
			}
		}
	}
}

func TestManualDoorPlatformRetogglePreservesExistingCountdown(t *testing.T) {
	g := newDoorTimingGame(0)
	g.m.Linedefs[0].Special = 1
	pt := &platThinker{sector: 0, status: platStatusWaiting, wait: -1, count: -8}
	g.plats = map[int]*platThinker{0: pt}
	if !g.evVerticalDoor(0, false) || pt.wait != 1 {
		t.Fatal("monster did not reverse the aliased -1 direction")
	}
	g.tickPlat(0, pt)
	if pt.count != -9 || pt.status != platStatusWaiting {
		t.Fatalf("retoggling wait restarted the old countdown: %+v", pt)
	}
}

func TestManualOpenDoorDoesNotApplyPlatformRaiseToggle(t *testing.T) {
	for _, special := range []uint16{31, 32, 33, 34, 118} {
		g := newDoorTimingGame(0)
		g.m.Linedefs[0].Special = special
		g.lineSpecial = []uint16{special}
		pt := &platThinker{sector: 0, wait: platWaitTics}
		g.plats = map[int]*platThinker{0: pt}
		if !g.evVerticalDoor(0, true) || g.activeDoorThinker(0) == nil || pt.wait != platWaitTics {
			t.Fatalf("special=%d: open-only door did not preserve its separate behavior", special)
		}
	}
}

func TestManualDoorPrefersCurrentDoorOverRetainedPlatform(t *testing.T) {
	g := newDoorTimingGame(0)
	g.m.Linedefs[0].Special = 1
	d := &doorThinker{sector: 0, direction: -1}
	g.doors[0] = d
	pt := &platThinker{sector: 0, wait: platWaitTics}
	g.plats = map[int]*platThinker{0: pt}
	if !g.evVerticalDoor(0, true) || d.direction != 1 || pt.wait != platWaitTics {
		t.Fatal("manual activation changed the retained platform instead of the current door")
	}
}

func TestPlatformWaitResumesOnlyAtExactlyZero(t *testing.T) {
	for _, count := range []int{2, 1, 0, -1, -2147483648} {
		g := newDoorTimingGame(0)
		pt := &platThinker{sector: 0, status: platStatusWaiting, count: count, low: 0}
		g.plats = map[int]*platThinker{0: pt}
		g.tickPlat(0, pt)
		want := platStatusWaiting
		if count == 1 {
			want = platStatusUp
		}
		if pt.count != int(int32(count)-1) || pt.status != want {
			t.Fatalf("count=%d: platform=%+v want status=%d", count, pt, want)
		}
	}
}
