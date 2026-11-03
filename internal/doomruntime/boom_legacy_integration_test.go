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

type legacyMoverCase struct {
	Position                                                    int
	Map                                                         int
	Name                                                        string
	Special                                                     uint16
	Use, Actor, Activator, Side, Tag, Busy, Target, Tics, Retry int
	Floors, Ceilings                                            [3]int64
	Types                                                       [3]int16
}

func TestBoomLegacyMoverCapture(t *testing.T) {
	path := os.Getenv("GD_LEGACY_CASES")
	if path == "" {
		t.Skip("run scripts/verify_boom_legacy.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []legacyMoverCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_LEGACY_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(os.Getenv("GD_LEGACY_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprintln(out, "name,tic,remaining,f0,c0,flat0,type0,flags0,f1,c1,flat1,type1,flags1,f2,c2,flat2,type2,flags2,z,health,factive,cactive,switch,button")
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	for _, c := range cases {
		m, err := mapdata.LoadMap(wf, mapdata.MapName(fmt.Sprintf("MAP%02d", c.Map)))
		if err != nil {
			t.Fatal(err)
		}
		g := newGame(m, Options{SkillLevel: 3, Headless: true})
		for i := range m.Sectors {
			g.sectorFloor[i] = c.Floors[i]
			g.sectorCeil[i] = c.Ceilings[i]
			m.Sectors[i].FloorHeight = int16(c.Floors[i] >> fracBits)
			m.Sectors[i].CeilingHeight = int16(c.Ceilings[i] >> fracBits)
			m.Sectors[i].Special = c.Types[i]
			m.Sectors[i].Tag = 0
		}
		m.Sectors[c.Target].Tag = 7
		m.Linedefs[2].Tag = uint16(c.Tag)
		m.Linedefs[2].Special = c.Special
		g.lineSpecial[2] = c.Special
		for actor := 1; actor <= 2; actor++ {
			x := int64(-320 * fracUnit)
			if c.Target == 0 {
				x = 320 * fracUnit
			}
			if c.Actor == actor {
				x = 0
				if c.Target == 0 {
					x = -256 * fracUnit
				}
			}
			if c.Actor == actor && c.Position != 0 {
				x = -120 * fracUnit
				if c.Position == 2 {
					x = 120 * fracUnit
				}
			}
			y := int64(-48 * fracUnit)
			if actor == 2 {
				y = 48 * fracUnit
			}
			floor, ceil, ok := g.teleportDestinationHeights(x, y)
			if !ok {
				t.Fatal("bad actor position")
			}
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
			old := m.Linedefs[2].Tag
			m.Linedefs[2].Tag = 7
			switch c.Busy {
			case 1:
				g.activateFloorLine(2, mapdata.FloorInfo{Action: mapdata.FloorLowerToNearest, UsesTag: true})
			case 2:
				g.activateCeilingLine(2, mapdata.CeilingInfo{Action: mapdata.CeilingLowerToFloor, UsesTag: true})
			case 3:
				g.activateElevator(2, mapdata.ElevatorUp)
			}
			m.Linedefs[2].Tag = old
		}
		for tic := 0; tic <= c.Tics; tic++ {
			if tic == 0 || tic == 1 || tic == c.Retry {
				if g.lineSpecial[2] != 0 {
					if c.Use != 0 {
						g.useSpecialLineForActorIndex(2, c.Side, c.Activator == 0, 1)
					} else {
						prev, cur := int64(-129*fracUnit), int64(-127*fracUnit)
						if c.Map == 2 {
							prev, cur = -385*fracUnit, -383*fracUnit
						}
						if c.Side == 1 {
							prev, cur = cur, prev
						}
						g.checkWalkSpecialLinesForActorWithCandidates(prev, 0, cur, 0, 1, c.Activator == 0, []int{2})
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
			fmt.Fprintf(out, "%s,%d,%d", c.Name, tic, g.lineSpecial[2])
			for i := range m.Sectors {
				fmt.Fprintf(out, ",%d,%d,%s,%d,%d", g.sectorFloor[i], g.sectorCeil[i], m.Sectors[i].FloorPic, m.Sectors[i].Special, m.Sectors[i].Special&0x380)
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
			fmt.Fprintf(out, ",%d,%d,%d,%d,%d,%d\n", z, health, b(g.floorSlotBusy(c.Target)), b(g.ceilingSlotBusy(c.Target)), b(m.Sidedefs[m.Linedefs[2].SideNum[0]].Top == "SW2BRCOM"), timer)
		}
	}
}
