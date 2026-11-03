//go:build integration

package doomruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"os"
	"sort"
	"strings"
	"testing"
)

type manualDoorCase struct {
	Name                                                                  string
	Special, Second                                                       uint16
	Kind, Age, Actor, Activator, Side, Secret, Keys, Tics, At2, At3, Late int
	Floors, Ceilings                                                      [3]int64
}

func tickManualProbeMovers(g *game) {
	g.prunePendingDoors()
	for order := int64(0); ; {
		ref, ok := g.nextWorldThinkerAfter(order)
		if !ok {
			return
		}
		order = ref.order
		switch ref.kind {
		case worldThinkerDoor, worldThinkerCeiling, worldThinkerFloor, worldThinkerPlat:
			g.tickWorldThinker(ref)
		}
	}
}
func manualProbeKind(g *game, ceiling bool) int {
	if !ceiling {
		if ft := g.floors[1]; ft != nil {
			if ft.elevator {
				return 5
			}
			return 3
		}
		if g.plats[1] != nil {
			return 4
		}
		return 0
	}
	owner, tracked := g.ceilingOwners[1]
	if tracked && owner == 0 {
		return 0
	}
	if d := g.doors[1]; d != nil && !d.pendingRemove && (!tracked || owner == d.order) {
		return 1
	}
	if c := g.ceilings[1]; c != nil && (!tracked || owner == c.order) {
		return 2
	}
	if f := g.floors[1]; f != nil && f.elevator && (!tracked || owner == f.order) {
		return 5
	}
	return 0
}
func manualProbeProfiles(g *game) [5]string {
	type item struct {
		order int64
		kind  int
		text  string
	}
	var items []item
	addDoor := func(d *doorThinker) {
		if d == nil || d.pendingRemove {
			return
		}
		kind := 1
		switch d.typ {
		case doorNormal, doorBlazeRaise:
			kind = 0
		case doorClose30ThenOpen, doorGenCloseWaitOpen:
			kind = 2
		case doorClose, doorBlazeClose:
			kind = 3
		}
		items = append(items, item{d.order, 1, fmt.Sprintf("%d:%d:%d:%d:%d:%d:%d;", d.sector, kind, d.direction, d.speed, d.topHeight, d.topWait, d.topCountdown)})
	}
	for _, d := range g.doors {
		addDoor(d)
	}
	for _, d := range g.extraDoors {
		addDoor(d)
	}
	for _, c := range g.ceilings {
		items = append(items, item{c.order, 2, fmt.Sprintf("%d:%d:%d:%d:%d:%d;", c.sector, c.direction, c.speed, c.topHeight, c.bottomHeight, c.oldSpeed)})
	}
	for _, f := range g.floors {
		if f.elevator {
			items = append(items, item{f.order, 5, fmt.Sprintf("%d:%d:%d:%d:%d;", f.sector, f.direction, f.speed, f.destHeight, f.ceilingDestHeight)})
		} else {
			items = append(items, item{f.order, 3, fmt.Sprintf("%d:%d:%d:%d;", f.sector, f.direction, f.speed, f.destHeight)})
		}
	}
	for _, p := range g.plats {
		old := platStatus(0)
		if p.status == platStatusInStasis {
			old = p.oldStatus
		}
		items = append(items, item{p.order, 4, fmt.Sprintf("%d:%d:%d:%d:%d:%d:%d:%d;", p.sector, p.status, p.speed, p.low, p.high, p.wait, p.count, old)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].order < items[j].order })
	var result [5]string
	for _, it := range items {
		result[it.kind-1] += it.text
	}
	return result
}
func TestBoomManualDoorCapture(t *testing.T) {
	path := os.Getenv("GD_MANUAL_CASES")
	if path == "" {
		t.Skip("run scripts/verify_boom_manual.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []manualDoorCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_MANUAL_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(os.Getenv("GD_MANUAL_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprintln(out, "name,tic,line1,line2,f0,c0,f1,c1,f2,c2,fowner,cowner,doors,ceilings,floors,plats,elevators,z,health,lock0,lock1,lock2")
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
			m.Sectors[i].FloorPic = "FLOOR0_1"
			m.Sectors[i].Tag = 0
		}
		m.Sectors[1].Tag = 7
		if c.Keys != 0 {
			g.inventory = lockInventory(63)
		}
		for actor := 1; actor <= 2; actor++ {
			x := int64(-1024 * fracUnit)
			if c.Actor == actor {
				x = 0
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
			} else {
				g.setThingPosFixed(1, x, y)
				g.setThingSupportState(1, floor, floor, ceil)
			}
		}
		switch c.Kind {
		case 1, 2, 3, 4:
			g.activatePlatLine(2, mapdata.PlatInfo{Action: mapdata.PlatDownWaitUpStay, UsesTag: true})
		case 5:
			g.activateElevator(2, mapdata.ElevatorUp)
		case 6:
			g.activateElevator(2, mapdata.ElevatorDown)
		case 7, 8, 9:
			special := uint16(0x2f93)
			if c.Kind == 8 {
				special = 0x2f83
			}
			g.activateGeneralizedCrusher(2, *mapdata.LookupLineSpecial(special).Ceiling)
		case 10, 11:
			g.activateGeneralizedStairs(2, *mapdata.LookupLineSpecial(0x3103).Stairs)
		case 12:
			g.activateFloorLine(2, mapdata.FloorInfo{Action: mapdata.FloorLowerToNearest, UsesTag: true})
		case 13:
			g.activateCeilingLine(2, mapdata.CeilingInfo{Action: mapdata.CeilingLowerToFloor, UsesTag: true})
		case 14, 16:
			g.activateDoorSectors([]int{1}, mapdata.DoorRaise)
		case 15:
			g.activateDoorSectors([]int{1}, mapdata.DoorClose)
		case 17:
			g.activateDoorSectors([]int{1}, mapdata.DoorOpen)
		case 18:
			m.Linedefs[2].Special = 0x3c07
			g.activateGeneralizedDoor(2, mapdata.LookupLineSpecial(0x3c07), true)
		}
		if c.Kind == 4 {
			g.stopTaggedPlats(7)
		}
		if c.Kind == 9 {
			g.stopBoomCeilings(7)
		}
		for i := 0; i < c.Age; i++ {
			g.worldTic = i + 1
			tickManualProbeMovers(g)
			g.tickDelayedSwitchReverts()
		}
		m.Linedefs[2].Special = c.Special
		g.lineSpecial[2] = c.Special
		m.Linedefs[10].Special = c.Second
		g.lineSpecial[10] = c.Second
		if c.Secret != 0 {
			m.Linedefs[2].Flags |= mlSecret
			m.Linedefs[10].Flags |= mlSecret
		}
		for tic := 0; tic <= c.Tics; tic++ {
			if tic == c.Late {
				g.activateGeneralizedCrusher(2, *mapdata.LookupLineSpecial(0x2f93).Ceiling)
			}
			if tic == 0 {
				g.useSpecialLineForActorIndex(2, c.Side, c.Activator == 0, 1)
			}
			if (tic == c.At2 || tic == c.At3) && g.lineSpecial[10] != 0 {
				g.useSpecialLineForActorIndex(10, c.Side, c.Activator == 0, 1)
			}
			if tic > 0 {
				g.worldTic = c.Age + tic
				tickManualProbeMovers(g)
				g.tickDelayedSwitchReverts()
			}
			fmt.Fprintf(out, "%s,%d,%d,%d", c.Name, tic, g.lineSpecial[2], g.lineSpecial[10])
			for i := 0; i < 3; i++ {
				fmt.Fprintf(out, ",%d,%d", g.sectorFloor[i], g.sectorCeil[i])
			}
			profiles := manualProbeProfiles(g)
			fmt.Fprintf(out, ",%d,%d,%s", manualProbeKind(g, false), manualProbeKind(g, true), strings.Join(profiles[:], ","))
			var z int64
			health := 0
			if c.Actor == 1 {
				z = g.p.z
				health = g.stats.Health
			} else if c.Actor == 2 {
				z, _, _ = g.thingSupportState(1, m.Things[1])
				health = g.thingHP[1]
			}
			var locks [3]int
			for i := range locks {
				if i < len(g.boomStairs) {
					locks[i] = g.boomStairs[i].Lock
				}
			}
			fmt.Fprintf(out, ",%d,%d,%d,%d,%d\n", z, health, locks[0], locks[1], locks[2])
		}
	}
}
