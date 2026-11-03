//go:build integration

package doomruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"os"
	"testing"
)

type generalizedMoverCase struct {
	Name                                                                                                string
	Special                                                                                             uint16
	Actor, Activator, Side, Tag, Busy, Secret, Position, Target, Tics, Retry, Late, Stop, Resume, Retag int
	Floors, Ceilings                                                                                    [3]int64
	Flats                                                                                               [3]string
}

func TestBoomGeneralizedMoverCapture(t *testing.T) {
	path := os.Getenv("GD_GEN_CASES")
	if path == "" {
		t.Skip("run scripts/verify_boom_generalized.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []generalizedMoverCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_GEN_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(os.Getenv("GD_GEN_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	soundTrace := newSectorSoundTrace(t)
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprint(out, "name,tic,remaining")
	for i := 0; i < 3; i++ {
		fmt.Fprintf(out, ",f%d,c%d,lock%d,prev%d,next%d,fa%d,ca%d,fs%d,fd%d,dir%d,cs%d,cd%d,orig%d,silent%d", i, i, i, i, i, i, i, i, i, i, i, i, i, i)
	}
	fmt.Fprintln(out, ",z,health,switch,button")
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	for _, c := range cases {
		m, err := mapdata.LoadMap(wf, "MAP01")
		if err != nil {
			t.Fatal(err)
		}
		g := newGame(m, Options{SkillLevel: 3, Headless: true})
		for i := range m.Sectors {
			g.sectorFloor[i] = c.Floors[i]
			g.sectorCeil[i] = c.Ceilings[i]
			m.Sectors[i].FloorHeight = int16(c.Floors[i] >> fracBits)
			m.Sectors[i].CeilingHeight = int16(c.Ceilings[i] >> fracBits)
			m.Sectors[i].FloorPic = c.Flats[i]
			m.Sectors[i].Tag = 0
		}
		m.Sectors[c.Target].Tag = 7
		m.Linedefs[2].Tag = uint16(c.Tag)
		m.Linedefs[2].Special = c.Special
		g.lineSpecial[2] = c.Special
		if c.Secret != 0 {
			m.Linedefs[2].Flags |= mlSecret
		}
		for actor := 1; actor <= 2; actor++ {
			x := int64(-1024 * fracUnit)
			if c.Actor == actor {
				x = 0
				if c.Position == 1 {
					x = -120 * fracUnit
				}
				if c.Position == 2 {
					x = 120 * fracUnit
				}
			}
			y := int64(-48 * fracUnit)
			if actor == 2 {
				y = 48 * fracUnit
			}
			floor, ceil, _ := g.teleportDestinationHeights(x, y)
			if actor == 1 {
				g.setPlayerPosFixed(x, y)
				g.p.z = floor
				g.p.floorz = floor
				g.p.ceilz = ceil
				if c.Actor == actor && c.Position != 0 {
					g.heightClipPlayer(floor)
				}
			} else {
				g.setThingPosFixed(1, x, y)
				g.setThingSupportState(1, floor, floor, ceil)
				if c.Actor == actor && c.Position != 0 {
					g.heightClipThing(1, m.Things[1])
				}
			}
		}
		if c.Busy != 0 {
			sec := c.Target
			if c.Busy == 4 {
				sec = 1
			}
			old := m.Linedefs[2].Tag
			oldSect := m.Sectors[sec].Tag
			m.Linedefs[2].Tag = 9
			m.Sectors[sec].Tag = 9
			switch c.Busy {
			case 1, 4:
				g.activateFloorLine(2, mapdata.FloorInfo{Action: mapdata.FloorRaise24, UsesTag: true})
				if c.Busy == 4 {
					g.floors[sec].speed = fracUnit / 4
				}
			case 2:
				g.activateCeilingLine(2, mapdata.CeilingInfo{Action: mapdata.CeilingLowerToFloor, UsesTag: true})
			case 3:
				g.activateElevator(2, mapdata.ElevatorUp)
			}
			m.Linedefs[2].Tag = old
			m.Sectors[sec].Tag = oldSect
		}
		for tic := 0; tic <= c.Tics; tic++ {
			if tic == c.Stop {
				if c.Retag != 0 {
					m.Sectors[c.Target].Tag = 9
				}
				g.stopBoomCeilings(7)
			}
			if tic == c.Resume {
				oldSpecial, oldTag := g.lineSpecial[2], m.Linedefs[2].Tag
				g.lineSpecial[2] = c.Special&^7 | 3
				m.Linedefs[2].Tag = 7
				g.activateGeneralizedCrusher(2, *mapdata.LookupLineSpecial(g.lineSpecial[2]).Ceiling)
				g.lineSpecial[2] = oldSpecial
				m.Linedefs[2].Tag = oldTag
			}
			if tic == 0 || tic == 1 || tic == c.Retry || tic == c.Late {
				if special := g.lineSpecial[2]; special != 0 {
					switch special & 7 {
					case 0, 1:
						prev, cur := int64(-129*fracUnit), int64(-127*fracUnit)
						if c.Side == 1 {
							prev, cur = cur, prev
						}
						g.checkWalkSpecialLinesForActorWithCandidates(prev, 0, cur, 0, 1, c.Activator == 0, []int{2})
					case 4, 5:
						g.shootSpecialLine(2, c.Activator == 0)
					default:
						g.useSpecialLineForActorIndex(2, c.Side, c.Activator == 0, 1)
					}
				}
			}
			if tic > 0 {
				g.worldTic = tic
				for order := int64(0); ; {
					ref, ok := g.nextWorldThinkerAfter(order)
					if !ok {
						break
					}
					order = ref.order
					switch ref.kind {
					case worldThinkerFloor, worldThinkerCeiling, worldThinkerPlat, worldThinkerDoor:
						g.tickWorldThinker(ref)
					}
				}
				g.tickDelayedSwitchReverts()
			}
			soundTrace.frame(g, c.Name, tic)
			fmt.Fprintf(out, "%s,%d,%d", c.Name, tic, g.lineSpecial[2])
			for i := 0; i < 3; i++ {
				st := boomStairState{Prev: -1, Next: -1}
				if i < len(g.boomStairs) {
					st = g.boomStairs[i]
				}
				var fs, fd, cs, orig int64
				dir, cd, silent := 0, 0, 0
				if ft := g.floors[i]; ft != nil && !ft.elevator {
					fs = ft.speed
					fd = ft.destHeight
					dir = ft.direction
				}
				if ct := g.ceilings[i]; ct != nil {
					cs = ct.speed
					cd = ct.direction
					orig = ct.oldSpeed
					silent = b(ct.action == mapdata.CeilingGenSilentCrusher)
				}
				fmt.Fprintf(out, ",%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d", g.sectorFloor[i], g.sectorCeil[i], st.Lock, st.Prev, st.Next, b(g.floorSlotBusy(i)), b(g.ceilingSlotBusy(i)), fs, fd, dir, cs, cd, orig, silent)
			}
			var z int64
			health := 0
			if c.Actor == 1 {
				z = g.p.z
				health = g.stats.Health
			} else if c.Actor == 2 {
				z, _, _ = g.thingSupportState(1, m.Things[1])
				health = g.thingHP[1]
			}
			timer := 0
			for _, button := range g.delayedSwitchReverts {
				timer = button.tics
			}
			fmt.Fprintf(out, ",%d,%d,%d,%d\n", z, health, b(m.Sidedefs[m.Linedefs[2].SideNum[0]].Top == "SW2BRCOM"), timer)
		}
	}
}
