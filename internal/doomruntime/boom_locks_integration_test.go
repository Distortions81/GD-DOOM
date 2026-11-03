//go:build integration

package doomruntime

import (
	"bufio"
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"os"
	"strings"
	"testing"
)

// The external verifier compares dispatcher outcomes with unmodified DSDA code.
// Two attempts per case check one-shot consumption and busy repeat rejection.
func TestBoomLocksCapture(t *testing.T) {
	path := os.Getenv("GD_LOCK_WAD")
	if path == "" {
		t.Skip("run scripts/verify_boom_locks.py")
	}
	wf, err := wad.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "MAP01")
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(m, Options{SkillLevel: 3})
	if len(m.Things) != 2 || m.Things[1].Type != 3001 {
		t.Fatal("requires real imp fixture")
	}
	f, err := os.Create(os.Getenv("GD_LOCK_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprintln(out, "special,mask,monster,phase,allowed,active,remaining,speed,wait,top,direction,kind,message")
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	for special := uint16(0x3800); special <= 0x3bff; special++ {
		for mask := 0; mask < 64; mask++ {
			for monster := 0; monster < 2; monster++ {
				g.inventory = lockInventory(mask)
				g.doors = nil
				g.extraDoors = nil
				g.recycledDoors = nil
				g.delayedSwitchReverts = nil
				g.lineSpecial[2] = special
				g.m.Linedefs[2].Special = special
				g.soundQueue = nil
				g.soundQueueOrigin = nil
				info := mapdata.LookupLineSpecial(special)
				allowed := info.Door.CanActivate(g.inventory.keys())
				for phase := 0; phase < 2; phase++ {
					g.useText = ""
					if g.lineSpecial[2] != 0 {
						switch special & 7 {
						case 0, 1:
							g.checkWalkSpecialLinesForActorWithCandidates(-fracUnit, 0, fracUnit, 0, 1, monster == 0, []int{2})
						case 4, 5:
							g.shootSpecialLine(2, monster == 0)
						default:
							g.useSpecialLineForActorIndex(2, 0, monster == 0, 1)
						}
					}
					if strings.HasPrefix(g.useText, "USE:") {
						g.useText = ""
					}
					var speed, top int64
					var wait, direction, kind int
					d := g.activeDoorThinker(1)
					if d != nil {
						speed = d.speed
						top = d.topHeight
						wait = d.topWait
						direction = d.direction
						if d.typ == doorOpen || d.typ == doorBlazeOpen {
							kind = 1
						}
					}
					fmt.Fprintf(out, "%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%s\n", special, mask, monster, phase, b(allowed), b(d != nil), g.lineSpecial[2], speed, wait, top, direction, kind, g.useText)
				}
			}
		}
	}
}
