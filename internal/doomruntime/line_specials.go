package doomruntime

import (
	"fmt"
	"os"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

const (
	floorMoveSpeed    = fracUnit
	floorTurboSpeed   = 4 * fracUnit
	platWaitTics      = 3 * doomTicsPerSecond
	platMoveSpeed     = fracUnit
	ceilingMoveSpeed  = fracUnit
	stairBuild8Speed  = fracUnit / 4
	stairTurbo16Speed = 4 * fracUnit
	teleportThingType = 14
)

type floorFinishAction uint8

const (
	floorFinishNone floorFinishAction = iota
	floorFinishSetTexture
)

type floorThinker struct {
	sector        int
	direction     int
	speed         int64
	destHeight    int64
	finish        floorFinishAction
	finishFlat    string
	finishSpecial int16
}

type platStatus uint8

const (
	platStatusUp platStatus = iota
	platStatusDown
	platStatusWaiting
	platStatusInStasis
)

type platType uint8

const (
	platTypeDownWaitUpStay platType = iota
	platTypeRaiseToNearestAndChange
	platTypePerpetualRaise
)

type platThinker struct {
	sector        int
	typ           platType
	status        platStatus
	oldStatus     platStatus
	speed         int64
	low           int64
	high          int64
	wait          int
	count         int
	finishFlat    string
	finishSpecial int16
}

type ceilingThinker struct {
	sector       int
	action       mapdata.CeilingAction
	direction    int
	oldDirection int
	speed        int64
	topHeight    int64
	bottomHeight int64
	crush        bool
}

func lineSpecialSupported(info mapdata.LineSpecialInfo) bool {
	return info.Exit != mapdata.ExitNone ||
		info.Door != nil ||
		info.Floor != nil ||
		info.Plat != nil ||
		info.Stairs != nil ||
		info.Light != nil ||
		info.Teleport != nil ||
		info.Ceiling != nil ||
		info.Combo != "" ||
		info.Donut
}

func (g *game) activateNonDoorLineSpecial(lineIdx int, side int, info mapdata.LineSpecialInfo, actorIdx int, isPlayer bool) bool {
	switch {
	case info.Floor != nil:
		return g.activateFloorLine(lineIdx, *info.Floor)
	case info.Plat != nil:
		return g.activatePlatLine(lineIdx, *info.Plat)
	case info.Stairs != nil:
		return g.activateStairsLine(lineIdx, *info.Stairs)
	case info.Light != nil:
		return g.activateLightLine(lineIdx, *info.Light)
	case info.Teleport != nil:
		return g.activateTeleportLine(lineIdx, side, *info.Teleport, actorIdx, isPlayer)
	case info.Ceiling != nil:
		return g.activateCeilingLine(lineIdx, *info.Ceiling)
	case info.Combo != "":
		return g.activateComboLine(lineIdx, info.Combo)
	case info.Donut:
		return g.activateDonutLine(lineIdx)
	default:
		return false
	}
}

func (g *game) activateShootLineSpecial(lineIdx int, info mapdata.LineSpecialInfo) bool {
	if !lineSpecialSupported(info) || info.Trigger != mapdata.TriggerShoot {
		return false
	}
	if info.Exit != mapdata.ExitNone {
		return g.handleExitSpecial(lineIdx, uint16(info.Special), mapdata.TriggerShoot)
	}
	if info.Door != nil {
		return g.evDoDoorTagged(lineIdx, info)
	}
	return g.activateNonDoorLineSpecial(lineIdx, 0, info, -1, true)
}

func (g *game) taggedSectorsForLine(lineIdx int) []int {
	if g == nil || g.m == nil || lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
		return nil
	}
	return g.taggedSectorsForTag(g.m.Linedefs[lineIdx].Tag)
}

func (g *game) taggedSectorsForTag(tag uint16) []int {
	if g == nil || g.m == nil || tag == 0 {
		return nil
	}
	out := make([]int, 0, 4)
	for sec := range g.m.Sectors {
		if g.m.Sectors[sec].Tag >= 0 && uint16(g.m.Sectors[sec].Tag) == tag {
			out = append(out, sec)
		}
	}
	return out
}

func (g *game) activateTaggedFloor(tag uint16, action mapdata.FloorAction) bool {
	targets := g.taggedSectorsForTag(tag)
	if len(targets) == 0 {
		return false
	}
	if g.floors == nil {
		g.floors = make(map[int]*floorThinker)
	}
	activated := false
	for _, sec := range targets {
		if g.sectorHasActiveMover(sec) {
			continue
		}
		ft := &floorThinker{sector: sec}
		switch action {
		case mapdata.FloorRaiseToTexture:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
		case mapdata.FloorLowerToLowest:
			ft.direction = -1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findLowestFloorSurrounding(sec)
		default:
			continue
		}
		if ft.direction > 0 && ft.destHeight > g.sectorCeil[sec] {
			ft.destHeight = g.sectorCeil[sec]
		}
		g.floors[sec] = ft
		activated = true
	}
	return activated
}

func (g *game) frontSectorForLine(lineIdx int) (int, bool) {
	if g == nil || g.m == nil || lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
		return -1, false
	}
	side := g.m.Linedefs[lineIdx].SideNum[0]
	if side < 0 || int(side) >= len(g.m.Sidedefs) {
		return -1, false
	}
	sec := int(g.m.Sidedefs[int(side)].Sector)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return -1, false
	}
	return sec, true
}

func (g *game) sectorHasActiveMover(sec int) bool {
	if sec < 0 {
		return false
	}
	return (g.doors != nil && g.doors[sec] != nil) ||
		(g.floors != nil && g.floors[sec] != nil) ||
		(g.plats != nil && g.plats[sec] != nil) ||
		(g.ceilings != nil && g.ceilings[sec] != nil)
}

func (g *game) playerTouchesSector(sec int) bool {
	if g == nil || sec < 0 {
		return false
	}
	samples := [][2]int64{
		{0, 0},
		{playerRadius, 0},
		{-playerRadius, 0},
		{0, playerRadius},
		{0, -playerRadius},
		{playerRadius, playerRadius},
		{playerRadius, -playerRadius},
		{-playerRadius, playerRadius},
		{-playerRadius, -playerRadius},
	}
	for _, s := range samples {
		if g.sectorAt(g.p.x+s[0], g.p.y+s[1]) == sec {
			return true
		}
	}
	return false
}

func (g *game) setSectorFloorHeight(sec int, z int64) {
	if g == nil || sec < 0 || sec >= len(g.sectorFloor) {
		return
	}
	old := g.sectorFloor[sec]
	oldPlayerFloor := g.p.floorz
	if old == z {
		return
	}
	g.sectorFloor[sec] = z
	g.markDynamicSectorPlaneCacheDirty(sec)
	if sec < len(g.m.Sectors) {
		g.m.Sectors[sec].FloorHeight = int16(z >> fracBits)
	}
	g.heightClipAroundSector(sec, oldPlayerFloor)
}

func (g *game) setSectorCeilingHeight(sec int, z int64) {
	if g == nil || sec < 0 || sec >= len(g.sectorCeil) {
		return
	}
	oldPlayerFloor := g.p.floorz
	if g.sectorCeil[sec] == z {
		return
	}
	g.sectorCeil[sec] = z
	g.markDynamicSectorPlaneCacheDirty(sec)
	if sec < len(g.m.Sectors) {
		g.m.Sectors[sec].CeilingHeight = int16(z >> fracBits)
	}
	g.heightClipAroundSector(sec, oldPlayerFloor)
}

func (g *game) heightClipAroundSector(sec int, oldPlayerFloor int64) {
	if g == nil || g.m == nil || sec < 0 {
		return
	}
	sectors := map[int]struct{}{sec: {}}
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		switch {
		case ok0 && ok1 && s0 == sec:
			sectors[s1] = struct{}{}
		case ok0 && ok1 && s1 == sec:
			sectors[s0] = struct{}{}
		}
	}
	playerClipped := false
	for affected := range sectors {
		if !playerClipped && g.playerTouchesSector(affected) {
			g.heightClipPlayer(oldPlayerFloor)
			playerClipped = true
		}
		g.heightClipThingsInSector(affected)
	}
}

func (g *game) heightClipPlayer(oldFloorz int64) bool {
	if g == nil {
		return false
	}
	onFloor := g.p.z == oldFloorz
	tmfloor, tmceil, _, ok := g.checkPositionFor(g.p.x, g.p.y, false)
	if !ok {
		return false
	}
	g.p.floorz = tmfloor
	g.p.ceilz = tmceil
	if onFloor {
		g.p.z = g.p.floorz
	} else if g.p.z+playerHeight > g.p.ceilz {
		g.p.z = g.p.ceilz - playerHeight
	}
	if g.p.ceilz-g.p.floorz < playerHeight {
		return false
	}
	return true
}

func thingCollisionRadius(typ int16) int64 {
	if info, ok := demoTraceThingInfoForType(typ); ok && info.radius > 0 {
		return info.radius
	}
	if isMonster(typ) {
		return monsterRadius(typ)
	}
	return 20 * fracUnit
}

func (g *game) thingTouchesSector(sec, i int, th mapdata.Thing) bool {
	if g == nil || g.m == nil || sec < 0 {
		return false
	}
	x, y := g.thingPosFixed(i, th)
	if g.sectorAt(x, y) == sec {
		return true
	}
	radius := thingCollisionRadius(th.Type)
	box := [4]int64{y + radius, y - radius, x + radius, x - radius}
	for _, ld := range g.lines {
		front, back := g.physLineSectors(ld)
		if front != sec && back != sec {
			continue
		}
		if box[3] >= ld.bbox[2] || box[2] <= ld.bbox[3] || box[1] >= ld.bbox[0] || box[0] <= ld.bbox[1] {
			continue
		}
		if g.boxOnLineSide(box, ld) == -1 {
			return true
		}
	}
	return false
}

func (g *game) heightClipThingsInSector(sec int) {
	if g == nil || g.m == nil {
		return
	}
	for i, th := range g.m.Things {
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if !g.thingTouchesSector(sec, i, th) {
			continue
		}
		g.heightClipThing(i, th)
	}
}

func (g *game) heightClipThing(i int, th mapdata.Thing) bool {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return false
	}
	x, y := g.thingPosFixed(i, th)
	radius := thingCollisionRadius(th.Type)
	oldZ, oldFloorZ, _ := g.thingSupportState(i, th)
	tmfloor, tmceil, _, ok := g.checkPositionForActor(x, y, radius, isMonster(th.Type), i, isMonster(th.Type))
	if !ok {
		return false
	}
	z := oldZ
	if z == oldFloorZ {
		z = tmfloor
	} else {
		height := g.thingCurrentHeight(i, th)
		if z+height > tmceil {
			z = tmceil - height
		}
	}
	if want := os.Getenv("GD_DEBUG_SUPPORT_TIC"); want != "" && os.Getenv("GD_DEBUG_SUPPORT_IDX") == fmt.Sprint(i) {
		if fmt.Sprint(g.demoTick-1) == want || fmt.Sprint(g.worldTic) == want {
			fmt.Printf("support-debug phase=heightclip tic=%d world=%d idx=%d type=%d x=%d y=%d oldz=%d oldfloor=%d tmfloor=%d tmceil=%d newz=%d sec=%d\n",
				g.demoTick-1, g.worldTic, i, th.Type, x, y, oldZ, oldFloorZ, tmfloor, tmceil, z, g.sectorAt(x, y))
		}
	}
	g.setThingSupportState(i, z, tmfloor, tmceil)
	return tmceil-tmfloor >= g.thingCurrentHeight(i, th)
}


func (g *game) findLowestFloorSurrounding(sec int) int64 {
	lowest := g.sectorFloor[sec]
	found := false
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		switch {
		case ok0 && ok1 && s0 == sec:
			if !found || g.sectorFloor[s1] < lowest {
				lowest = g.sectorFloor[s1]
			}
			found = true
		case ok0 && ok1 && s1 == sec:
			if !found || g.sectorFloor[s0] < lowest {
				lowest = g.sectorFloor[s0]
			}
			found = true
		}
	}
	return lowest
}

func (g *game) findHighestFloorSurrounding(sec int) int64 {
	highest := g.sectorFloor[sec]
	found := false
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		switch {
		case ok0 && ok1 && s0 == sec:
			if !found || g.sectorFloor[s1] > highest {
				highest = g.sectorFloor[s1]
			}
			found = true
		case ok0 && ok1 && s1 == sec:
			if !found || g.sectorFloor[s0] > highest {
				highest = g.sectorFloor[s0]
			}
			found = true
		}
	}
	if !found {
		return g.sectorFloor[sec]
	}
	return highest
}

func (g *game) findLowestCeilingSurrounding(sec int) int64 {
	lowest := int64(1<<62 - 1)
	found := false
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		switch {
		case ok0 && ok1 && s0 == sec:
			if !found || g.sectorCeil[s1] < lowest {
				lowest = g.sectorCeil[s1]
			}
			found = true
		case ok0 && ok1 && s1 == sec:
			if !found || g.sectorCeil[s0] < lowest {
				lowest = g.sectorCeil[s0]
			}
			found = true
		}
	}
	if !found {
		return g.sectorCeil[sec]
	}
	return lowest
}

func (g *game) findHighestCeilingSurrounding(sec int) int64 {
	highest := g.sectorCeil[sec]
	found := false
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		switch {
		case ok0 && ok1 && s0 == sec:
			if !found || g.sectorCeil[s1] > highest {
				highest = g.sectorCeil[s1]
			}
			found = true
		case ok0 && ok1 && s1 == sec:
			if !found || g.sectorCeil[s0] > highest {
				highest = g.sectorCeil[s0]
			}
			found = true
		}
	}
	if !found {
		return g.sectorCeil[sec]
	}
	return highest
}

func (g *game) findNextHighestFloor(sec int, current int64) int64 {
	var next int64
	found := false
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		if !ok0 || !ok1 {
			continue
		}
		var other int
		switch {
		case s0 == sec:
			other = s1
		case s1 == sec:
			other = s0
		default:
			continue
		}
		h := g.sectorFloor[other]
		if h <= current {
			continue
		}
		if !found || h < next {
			next = h
			found = true
		}
	}
	if !found {
		return current
	}
	return next
}

func (g *game) findMinSurroundingLight(sec int, maxLight int16) int16 {
	minLight := maxLight
	found := false
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		switch {
		case ok0 && ok1 && s0 == sec:
			if !found || g.m.Sectors[s1].Light < minLight {
				minLight = g.m.Sectors[s1].Light
			}
			found = true
		case ok0 && ok1 && s1 == sec:
			if !found || g.m.Sectors[s0].Light < minLight {
				minLight = g.m.Sectors[s0].Light
			}
			found = true
		}
	}
	if !found {
		return maxLight
	}
	return minLight
}

func (g *game) sectorIndexFromSidedef(side int16) (int, bool) {
	if side < 0 || int(side) >= len(g.m.Sidedefs) {
		return -1, false
	}
	sec := int(g.m.Sidedefs[int(side)].Sector)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return -1, false
	}
	return sec, true
}

func (g *game) activateFloorLine(lineIdx int, info mapdata.FloorInfo) bool {
	targets := g.taggedSectorsForLine(lineIdx)
	if len(targets) == 0 {
		return false
	}
	frontSec, _ := g.frontSectorForLine(lineIdx)
	if g.floors == nil {
		g.floors = make(map[int]*floorThinker)
	}
	activated := false
	for _, sec := range targets {
		if g.sectorHasActiveMover(sec) {
			continue
		}
		ft := &floorThinker{sector: sec}
		switch info.Action {
		case mapdata.FloorRaise:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.lowestSurroundingCeiling(sec)
		case mapdata.FloorRaiseToNearest:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findNextHighestFloor(sec, g.sectorFloor[sec])
		case mapdata.FloorLower:
			ft.direction = -1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findHighestFloorSurrounding(sec)
		case mapdata.FloorLowerAndChange:
			ft.direction = -1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findLowestFloorSurrounding(sec)
		case mapdata.FloorRaiseCrush:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findLowestCeilingSurrounding(sec) - 8*fracUnit
		case mapdata.FloorRaise24:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
		case mapdata.FloorRaise24AndChange:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
			ft.finish = floorFinishSetTexture
			if frontSec >= 0 {
				ft.finishFlat = g.m.Sectors[frontSec].FloorPic
				ft.finishSpecial = g.m.Sectors[frontSec].Special
			}
		case mapdata.FloorRaiseToTexture:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
		case mapdata.FloorLowerToLowest:
			ft.direction = -1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findLowestFloorSurrounding(sec)
		case mapdata.FloorTurboLower:
			ft.direction = -1
			ft.speed = floorTurboSpeed
			ft.destHeight = g.findHighestFloorSurrounding(sec)
			if ft.destHeight != g.sectorFloor[sec] {
				ft.destHeight += 8 * fracUnit
			}
		case mapdata.FloorRaiseTurbo:
			ft.direction = 1
			ft.speed = floorTurboSpeed
			ft.destHeight = g.findNextHighestFloor(sec, g.sectorFloor[sec])
		case mapdata.FloorRaise512:
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 512*fracUnit
		default:
			continue
		}
		if ft.direction > 0 && ft.destHeight > g.sectorCeil[sec] {
			ft.destHeight = g.sectorCeil[sec]
		}
		g.floors[sec] = ft
		activated = true
	}
	return activated
}

func (g *game) activatePlatLine(lineIdx int, info mapdata.PlatInfo) bool {
	targets := g.taggedSectorsForLine(lineIdx)
	if len(targets) == 0 {
		return false
	}
	if g.plats == nil {
		g.plats = make(map[int]*platThinker)
	}
	frontSec, _ := g.frontSectorForLine(lineIdx)
	activated := false
	for _, sec := range targets {
		if g.sectorHasActiveMover(sec) {
			continue
		}
		pt := &platThinker{sector: sec}
		switch info.Action {
		case mapdata.PlatRaiseToNearestAndChange:
			pt.typ = platTypeRaiseToNearestAndChange
			pt.status = platStatusUp
			pt.oldStatus = platStatusInStasis
			pt.speed = platMoveSpeed / 2
			pt.low = g.sectorFloor[sec]
			pt.high = g.findNextHighestFloor(sec, g.sectorFloor[sec])
			pt.wait = 0
			if frontSec >= 0 {
				pt.finishFlat = g.m.Sectors[frontSec].FloorPic
				pt.finishSpecial = 0
			}
		case mapdata.PlatRaiseAndChange24:
			pt.typ = platTypeRaiseToNearestAndChange
			pt.status = platStatusUp
			pt.oldStatus = platStatusInStasis
			pt.speed = platMoveSpeed / 2
			pt.low = g.sectorFloor[sec]
			pt.high = g.sectorFloor[sec] + 24*fracUnit
			if frontSec >= 0 {
				pt.finishFlat = g.m.Sectors[frontSec].FloorPic
				pt.finishSpecial = 0
			}
		case mapdata.PlatRaiseAndChange32:
			pt.typ = platTypeRaiseToNearestAndChange
			pt.status = platStatusUp
			pt.oldStatus = platStatusInStasis
			pt.speed = platMoveSpeed / 2
			pt.low = g.sectorFloor[sec]
			pt.high = g.sectorFloor[sec] + 32*fracUnit
			if frontSec >= 0 {
				pt.finishFlat = g.m.Sectors[frontSec].FloorPic
				pt.finishSpecial = 0
			}
		case mapdata.PlatDownWaitUpStay:
			pt.typ = platTypeDownWaitUpStay
			pt.status = platStatusDown
			pt.speed = 4 * platMoveSpeed
			pt.low = g.findLowestFloorSurrounding(sec)
			if pt.low > g.sectorFloor[sec] {
				pt.low = g.sectorFloor[sec]
			}
			pt.high = g.sectorFloor[sec]
			pt.wait = platWaitTics
			pt.count = pt.wait
		case mapdata.PlatBlazeDownWaitUpStay:
			pt.typ = platTypeDownWaitUpStay
			pt.status = platStatusDown
			pt.speed = 8 * platMoveSpeed
			pt.low = g.findLowestFloorSurrounding(sec)
			if pt.low > g.sectorFloor[sec] {
				pt.low = g.sectorFloor[sec]
			}
			pt.high = g.sectorFloor[sec]
			pt.wait = platWaitTics
			pt.count = pt.wait
		case mapdata.PlatPerpetualRaise:
			if g.activateInStasisPlats(g.m.Linedefs[lineIdx].Tag) {
				activated = true
				continue
			}
			pt.typ = platTypePerpetualRaise
			pt.speed = platMoveSpeed
			pt.low = g.findLowestFloorSurrounding(sec)
			if pt.low > g.sectorFloor[sec] {
				pt.low = g.sectorFloor[sec]
			}
			pt.high = g.findHighestFloorSurrounding(sec)
			if pt.high < g.sectorFloor[sec] {
				pt.high = g.sectorFloor[sec]
			}
			pt.wait = platWaitTics
			if doomrand.PRandom()&1 == 0 {
				pt.status = platStatusUp
			} else {
				pt.status = platStatusDown
			}
		case mapdata.PlatStop:
			if g.stopTaggedPlats(g.m.Linedefs[lineIdx].Tag) {
				activated = true
			}
			continue
		default:
			continue
		}
		g.plats[sec] = pt
		activated = true
	}
	return activated
}

func (g *game) activateStairsLine(lineIdx int, info mapdata.StairsInfo) bool {
	targets := g.taggedSectorsForLine(lineIdx)
	if len(targets) == 0 {
		return false
	}
	if g.floors == nil {
		g.floors = make(map[int]*floorThinker)
	}
	var stepSize, speed int64
	switch info.Action {
	case mapdata.StairsBuild8:
		stepSize = 8 * fracUnit
		speed = stairBuild8Speed
	case mapdata.StairsTurbo16:
		stepSize = 16 * fracUnit
		speed = stairTurbo16Speed
	default:
		return false
	}
	activated := false
	for _, start := range targets {
		if g.sectorHasActiveMover(start) {
			continue
		}
		texture := g.m.Sectors[start].FloorPic
		sec := start
		height := g.sectorFloor[sec] + stepSize
		visited := map[int]struct{}{start: {}}
		for {
			if !g.sectorHasActiveMover(sec) {
				g.floors[sec] = &floorThinker{
					sector:     sec,
					direction:  1,
					speed:      speed,
					destHeight: height,
				}
				activated = true
			}
			next, ok := g.nextStairSector(sec, texture, visited)
			if !ok {
				break
			}
			visited[next] = struct{}{}
			sec = next
			height += stepSize
		}
	}
	return activated
}

func (g *game) nextStairSector(sec int, texture string, visited map[int]struct{}) (int, bool) {
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		if !ok0 || !ok1 {
			continue
		}
		if s0 != sec {
			continue
		}
		if g.m.Sectors[s1].FloorPic != texture {
			continue
		}
		if _, seen := visited[s1]; seen {
			continue
		}
		return s1, true
	}
	return -1, false
}

func (g *game) activateLightLine(lineIdx int, info mapdata.LightInfo) bool {
	targets := g.taggedSectorsForLine(lineIdx)
	if len(targets) == 0 {
		return false
	}
	activated := false
	for _, sec := range targets {
		switch info.Action {
		case mapdata.LightVeryDark:
			g.m.Sectors[sec].Light = 35
		case mapdata.LightBrightestNeighbor:
			bright := g.m.Sectors[sec].Light
			for _, ld := range g.m.Linedefs {
				s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
				s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
				if !ok0 || !ok1 {
					continue
				}
				switch {
				case s0 == sec && g.m.Sectors[s1].Light > bright:
					bright = g.m.Sectors[s1].Light
				case s1 == sec && g.m.Sectors[s0].Light > bright:
					bright = g.m.Sectors[s0].Light
				}
			}
			g.m.Sectors[sec].Light = bright
		case mapdata.LightFullBright:
			g.m.Sectors[sec].Light = 255
		case mapdata.LightTurnTagOff:
			g.m.Sectors[sec].Light = g.findMinSurroundingLight(sec, g.m.Sectors[sec].Light)
		case mapdata.LightStartStrobing:
			if sec >= 0 && sec < len(g.sectorLightFx) && g.sectorLightFx[sec].kind != sectorLightEffectNone {
				continue
			}
			g.spawnSectorStrobeFlash(sec, sectorLightSlowDark, false)
		default:
			continue
		}
		g.refreshSectorPlaneCacheLighting()
		activated = true
	}
	return activated
}

func (g *game) activateTeleportLine(lineIdx int, side int, info mapdata.TeleportInfo, actorIdx int, isPlayer bool) bool {
	if side == 1 || !info.UsesTag {
		return false
	}
	if info.MonsterOnly && isPlayer {
		return false
	}
	if !isPlayer && (g == nil || g.m == nil || actorIdx < 0 || actorIdx >= len(g.m.Things)) {
		return false
	}
	line := g.m.Linedefs[lineIdx]
	actorLabel := "player"
	actorType := int16(0)
	actorX := g.p.x
	actorY := g.p.y
	actorZ := g.p.z
	actorFloorZ := g.p.floorz
	actorCeilZ := g.p.ceilz
	actorAngle := g.p.angle
	actorRadius := int64(playerRadius)
	actorHeight := int64(playerHeight)
	blockMonsterLines := false
	moverIsMonster := false
	if !isPlayer {
		actor := g.m.Things[actorIdx]
		actorLabel = "thing"
		actorType = actor.Type
		actorX, actorY = g.thingPosFixed(actorIdx, actor)
		actorZ, actorFloorZ, actorCeilZ = g.thingSupportState(actorIdx, actor)
		actorAngle = g.thingWorldAngle(actorIdx, actor)
		actorRadius = thingTypeRadius(actor.Type)
		actorHeight = g.thingCurrentHeight(actorIdx, actor)
		blockMonsterLines = true
		moverIsMonster = true
	}
	for i, th := range g.m.Things {
		if th.Type != teleportThingType {
			continue
		}
		// Check if this is a missile (skip missiles for teleport)
		// In Doom, missiles are typically things with specific types or flags
		// Missiles should not be teleported according to original Doom behavior
		if int(th.Flags)&0x10000 != 0 { // MF_MISSILE flag check (0x10000 = 65536)
			continue
		}
		sec := g.thingSectorCached(i, th)
		if sec < 0 || sec >= len(g.m.Sectors) {
			continue
		}
		if g.m.Sectors[sec].Tag < 0 || uint16(g.m.Sectors[sec].Tag) != line.Tag {
			continue
		}
		tx, ty := g.thingPosFixed(i, th)
		tmfloor, tmceil, _, ok := g.checkPositionForActor(tx, ty, actorRadius, blockMonsterLines, actorIdx, moverIsMonster)
		if !ok {
			return false
		}
		if tmceil-tmfloor < actorHeight {
			return false
		}
		destSec := g.sectorAt(tx, ty)
		if destSec < 0 || destSec >= len(g.sectorFloor) || destSec >= len(g.sectorCeil) {
			return false
		}
		destAngle := thingDegToWorldAngle(th.Angle)
		destFogX := tx + fixedMul(20*fracUnit, doomFineCosine(destAngle))
		destFogY := ty + fixedMul(20*fracUnit, doomFineSineAtAngle(destAngle))
		if isPlayer {
			g.p.x = tx
			g.p.y = ty
			g.p.floorz = tmfloor
			g.p.ceilz = tmceil
			g.p.z = tmfloor
			g.p.momz = 0
			g.p.viewHeight = playerViewHeight
			g.p.deltaViewHeight = 0
			g.playerViewZ = g.p.z + g.p.viewHeight
			g.p.momx = 0
			g.p.momy = 0
			g.p.reactionTime = 18
			g.p.angle = destAngle
		} else {
			g.setThingPosFixed(actorIdx, tx, ty)
			g.setThingSupportState(actorIdx, tmfloor, tmfloor, tmceil)
			g.setThingWorldAngle(actorIdx, destAngle)
			if actorIdx >= 0 && actorIdx < len(g.thingMoveDir) {
				g.thingMoveDir[actorIdx] = monsterDirNoDir
			}
			if actorIdx >= 0 && actorIdx < len(g.thingMoveCount) {
				g.thingMoveCount[actorIdx] = 0
			}
		}
		g.spawnTeleportFog(actorX, actorY, actorZ)
		g.spawnTeleportFog(destFogX, destFogY, tmfloor)
		g.emitSoundEventAt(soundEventTeleport, actorX, actorY)
		g.emitSoundEventAt(soundEventTeleport, destFogX, destFogY)
		if g.opts.DebugEvents {
			triggerX1, triggerY1, triggerX2, triggerY2, triggerCX, triggerCY := g.teleportTriggerDebugPos(lineIdx)
			fmt.Printf(
				"teleport tic=%d activator=%s actor_idx=%d actor_type=%d entity_pos=(%.2f,%.2f,%.2f) entity_floor=%.2f entity_ceil=%.2f entity_angle_from=%d entity_angle_to=%d line=%d special=%d tag=%d side=%d trigger_line=(%.2f,%.2f)->(%.2f,%.2f) trigger_pos=(%.2f,%.2f) source_pos=(%.2f,%.2f,%.2f) dest_pos=(%.2f,%.2f,%.2f) dest_floor=%.2f dest_ceil=%.2f dest_sector=%d dest_thing_idx=%d dest_thing_type=%d dest_thing_angle=%d dest_fog_pos=(%.2f,%.2f,%.2f)\n",
				g.worldTic,
				actorLabel,
				actorIdx,
				actorType,
				fixedToDebugFloat(actorX),
				fixedToDebugFloat(actorY),
				fixedToDebugFloat(actorZ),
				fixedToDebugFloat(actorFloorZ),
				fixedToDebugFloat(actorCeilZ),
				worldAngleToThingDeg(actorAngle),
				worldAngleToThingDeg(destAngle),
				lineIdx,
				line.Special,
				line.Tag,
				side,
				fixedToDebugFloat(triggerX1),
				fixedToDebugFloat(triggerY1),
				fixedToDebugFloat(triggerX2),
				fixedToDebugFloat(triggerY2),
				fixedToDebugFloat(triggerCX),
				fixedToDebugFloat(triggerCY),
				fixedToDebugFloat(actorX),
				fixedToDebugFloat(actorY),
				fixedToDebugFloat(actorZ),
				fixedToDebugFloat(tx),
				fixedToDebugFloat(ty),
				fixedToDebugFloat(tmfloor),
				fixedToDebugFloat(tmfloor),
				fixedToDebugFloat(tmceil),
				destSec,
				i,
				th.Type,
				th.Angle,
				fixedToDebugFloat(destFogX),
				fixedToDebugFloat(destFogY),
				fixedToDebugFloat(tmfloor),
			)
		}
		return true
	}
	return false
}

func (g *game) teleportTriggerDebugPos(lineIdx int) (x1, y1, x2, y2, cx, cy int64) {
	if g == nil || lineIdx < 0 || lineIdx >= len(g.lines) {
		return 0, 0, 0, 0, 0, 0
	}
	line := g.lines[lineIdx]
	x1, y1 = line.x1, line.y1
	x2, y2 = line.x2, line.y2
	cx = (x1 + x2) / 2
	cy = (y1 + y2) / 2
	return x1, y1, x2, y2, cx, cy
}

func fixedToDebugFloat(v int64) float64 {
	return float64(v) / fracUnit
}

func (g *game) activateCeilingLine(lineIdx int, info mapdata.CeilingInfo) bool {
	targets := g.taggedSectorsForLine(lineIdx)
	if len(targets) == 0 {
		return false
	}
	if g.ceilings == nil {
		g.ceilings = make(map[int]*ceilingThinker)
	}
	activated := false
	for _, sec := range targets {
		if existing := g.ceilings[sec]; existing != nil {
			switch info.Action {
			case mapdata.CeilingCrushRaise, mapdata.CeilingFastCrushRaise, mapdata.CeilingSilentCrushRaise:
				if existing.direction == 0 {
					existing.direction = existing.oldDirection
					activated = true
				}
			case mapdata.CeilingCrushStop:
				existing.oldDirection = existing.direction
				existing.direction = 0
				activated = true
			}
			continue
		}
		if g.sectorHasActiveMover(sec) && info.Action != mapdata.CeilingCrushStop {
			continue
		}
		ct := &ceilingThinker{sector: sec, action: info.Action, speed: ceilingMoveSpeed}
		switch info.Action {
		case mapdata.CeilingLowerToFloor:
			ct.direction = -1
			ct.bottomHeight = g.sectorFloor[sec]
		case mapdata.CeilingCrushRaise:
			ct.direction = -1
			ct.crush = true
			ct.topHeight = g.sectorCeil[sec]
			ct.bottomHeight = g.sectorFloor[sec] + 8*fracUnit
		case mapdata.CeilingLowerAndCrush:
			ct.direction = -1
			ct.crush = true
			ct.bottomHeight = g.sectorFloor[sec] + 8*fracUnit
		case mapdata.CeilingFastCrushRaise:
			ct.direction = -1
			ct.crush = true
			ct.speed = 2 * ceilingMoveSpeed
			ct.topHeight = g.sectorCeil[sec]
			ct.bottomHeight = g.sectorFloor[sec] + 8*fracUnit
		case mapdata.CeilingSilentCrushRaise:
			ct.direction = -1
			ct.crush = true
			ct.topHeight = g.sectorCeil[sec]
			ct.bottomHeight = g.sectorFloor[sec] + 8*fracUnit
		case mapdata.CeilingRaiseToHighest:
			ct.direction = 1
			ct.topHeight = g.findHighestCeilingSurrounding(sec)
		case mapdata.CeilingCrushStop:
			continue
		default:
			continue
		}
		g.ceilings[sec] = ct
		activated = true
	}
	return activated
}

func (g *game) activateComboLine(lineIdx int, action mapdata.ComboAction) bool {
	switch action {
	case mapdata.ComboRaiseCeilingLowerFloor:
		return g.activateCeilingLine(lineIdx, mapdata.CeilingInfo{Action: mapdata.CeilingRaiseToHighest, UsesTag: true}) ||
			g.activateFloorLine(lineIdx, mapdata.FloorInfo{Action: mapdata.FloorLowerToLowest, UsesTag: true})
	default:
		return false
	}
}

func (g *game) activateInStasisPlats(tag uint16) bool {
	activated := false
	for _, pt := range g.plats {
		if pt == nil || pt.status != platStatusInStasis {
			continue
		}
		sec := pt.sector
		if sec >= 0 && sec < len(g.m.Sectors) && g.m.Sectors[sec].Tag >= 0 && uint16(g.m.Sectors[sec].Tag) == tag {
			pt.status = pt.oldStatus
			activated = true
		}
	}
	return activated
}

func (g *game) stopTaggedPlats(tag uint16) bool {
	stopped := false
	for _, pt := range g.plats {
		if pt == nil || pt.status == platStatusInStasis {
			continue
		}
		sec := pt.sector
		if sec >= 0 && sec < len(g.m.Sectors) && g.m.Sectors[sec].Tag >= 0 && uint16(g.m.Sectors[sec].Tag) == tag {
			pt.oldStatus = pt.status
			pt.status = platStatusInStasis
			stopped = true
		}
	}
	return stopped
}

func (g *game) activateDonutLine(lineIdx int) bool {
	targets := g.taggedSectorsForLine(lineIdx)
	if len(targets) == 0 {
		return false
	}
	if g.floors == nil {
		g.floors = make(map[int]*floorThinker)
	}
	activated := false
	for _, s1 := range targets {
		if g.sectorHasActiveMover(s1) {
			continue
		}
		s2, s3, ok := g.findDonutSectors(s1)
		if !ok || g.sectorHasActiveMover(s2) {
			continue
		}
		dest := g.sectorFloor[s3]
		g.floors[s2] = &floorThinker{
			sector:        s2,
			direction:     1,
			speed:         floorMoveSpeed / 2,
			destHeight:    dest,
			finish:        floorFinishSetTexture,
			finishFlat:    g.m.Sectors[s3].FloorPic,
			finishSpecial: 0,
		}
		g.floors[s1] = &floorThinker{
			sector:     s1,
			direction:  -1,
			speed:      floorMoveSpeed / 2,
			destHeight: dest,
		}
		activated = true
	}
	return activated
}

func (g *game) findDonutSectors(s1 int) (int, int, bool) {
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1b, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		if !ok0 || !ok1 {
			continue
		}
		var s2 int
		switch {
		case s0 == s1:
			s2 = s1b
		case s1b == s1:
			s2 = s0
		default:
			continue
		}
		for _, ld2 := range g.m.Linedefs {
			a0, oka0 := g.sectorIndexFromSidedef(ld2.SideNum[0])
			a1, oka1 := g.sectorIndexFromSidedef(ld2.SideNum[1])
			if !oka0 || !oka1 {
				continue
			}
			if a0 == s2 && a1 != s1 {
				return s2, a1, true
			}
			if a1 == s2 && a0 != s1 {
				return s2, a0, true
			}
		}
	}
	return -1, -1, false
}

func (g *game) tickFloors() {
	for sec, ft := range g.floors {
		cur := g.sectorFloor[sec]
		next := cur + int64(ft.direction)*ft.speed
		done := false
		if ft.direction < 0 {
			if next <= ft.destHeight {
				next = ft.destHeight
				done = true
			}
		} else {
			if next >= ft.destHeight {
				next = ft.destHeight
				done = true
			}
		}
		g.setSectorFloorHeight(sec, next)
		if done {
			if ft.finish == floorFinishSetTexture {
				g.m.Sectors[sec].FloorPic = ft.finishFlat
				g.m.Sectors[sec].Special = ft.finishSpecial
				g.markDynamicSectorPlaneCacheDirty(sec)
			}
			delete(g.floors, sec)
		}
	}
}

func (g *game) tickPlats() {
	for sec, pt := range g.plats {
		switch pt.status {
		case platStatusUp:
			next := g.sectorFloor[sec] + pt.speed
			if next >= pt.high {
				next = pt.high
				g.setSectorFloorHeight(sec, next)
				if pt.typ == platTypeRaiseToNearestAndChange {
					if pt.finishFlat != "" {
						g.m.Sectors[sec].FloorPic = pt.finishFlat
					}
					g.m.Sectors[sec].Special = pt.finishSpecial
					g.markDynamicSectorPlaneCacheDirty(sec)
					delete(g.plats, sec)
				} else {
					pt.status = platStatusWaiting
					pt.count = pt.wait
				}
				continue
			}
			g.setSectorFloorHeight(sec, next)
		case platStatusDown:
			next := g.sectorFloor[sec] - pt.speed
			if next <= pt.low {
				next = pt.low
				g.setSectorFloorHeight(sec, next)
				pt.status = platStatusWaiting
				pt.count = pt.wait
				continue
			}
			g.setSectorFloorHeight(sec, next)
		case platStatusWaiting:
			pt.count--
			if pt.count > 0 {
				continue
			}
			if g.sectorFloor[sec] == pt.low {
				pt.status = platStatusUp
			} else {
				pt.status = platStatusDown
			}
		case platStatusInStasis:
			continue
		}
	}
}

func (g *game) tickCeilings() {
	for sec, ct := range g.ceilings {
		cur := g.sectorCeil[sec]
		switch ct.direction {
		case -1:
			next := cur - ct.speed
			if next <= ct.bottomHeight {
				next = ct.bottomHeight
				g.setSectorCeilingHeight(sec, next)
				if ct.action == mapdata.CeilingCrushRaise || ct.action == mapdata.CeilingFastCrushRaise || ct.action == mapdata.CeilingSilentCrushRaise {
					ct.direction = 1
				} else {
					delete(g.ceilings, sec)
				}
				continue
			}
			g.setSectorCeilingHeight(sec, next)
		case 1:
			next := cur + ct.speed
			if next >= ct.topHeight {
				next = ct.topHeight
				g.setSectorCeilingHeight(sec, next)
				delete(g.ceilings, sec)
				continue
			}
			g.setSectorCeilingHeight(sec, next)
		case 0:
			continue
		}
	}
}
