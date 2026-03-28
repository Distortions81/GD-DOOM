package doomruntime

import (
	"fmt"
	"math"

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
	order         int64
	sector        int
	typ           int
	crush         bool
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
	platTypeBlazeDownWaitUpStay
)

type platThinker struct {
	order         int64
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
	order        int64
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
		ft := &floorThinker{order: g.allocThinkerOrder(), sector: sec}
		switch action {
		case mapdata.FloorRaiseToTexture:
			ft.typ = 5
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
		case mapdata.FloorLowerToLowest:
			ft.typ = 1
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
		if want := runtimeDebugEnv("GD_DEBUG_SECTOR_ACTIVATE"); want != "" {
			var wantSec int
			if _, err := fmt.Sscanf(want, "%d", &wantSec); err == nil && sec == wantSec {
				fmt.Printf("sector-activate-debug tic=%d world=%d kind=tagged-floor sec=%d action=%v tag=%d dir=%d speed=%d dest=%d\n",
					g.demoTick-1, g.worldTic, sec, action, tag, ft.direction, ft.speed, ft.destHeight)
			}
		}
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
	if want := runtimeDebugEnv("GD_DEBUG_FLOOR_TIC"); want != "" {
		var tic int
		if _, err := fmt.Sscanf(want, "%d", &tic); err == nil && (g.demoTick-1 == tic || g.worldTic == tic) {
			fmt.Printf("floor-move-debug tic=%d world=%d sec=%d old=%d new=%d\n", g.demoTick-1, g.worldTic, sec, old, z)
		}
	}
	g.sectorFloor[sec] = z
	g.markDynamicSectorPlaneCacheDirty(sec)
	if sec < len(g.m.Sectors) {
		g.m.Sectors[sec].FloorHeight = int16(z >> fracBits)
	}
	g.heightClipAroundSector(sec, oldPlayerFloor)
}

func (g *game) sectorMoveWouldBlockLiveActor(sec int, newFloor, newCeil int64) bool {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
		return false
	}
	oldFloor := g.sectorFloor[sec]
	oldCeil := g.sectorCeil[sec]
	oldPlayerZ := g.p.z
	oldPlayerFloor := g.p.floorz
	oldPlayerCeil := g.p.ceilz
	g.sectorFloor[sec] = newFloor
	g.sectorCeil[sec] = newCeil
	defer func() {
		g.sectorFloor[sec] = oldFloor
		g.sectorCeil[sec] = oldCeil
		g.p.z = oldPlayerZ
		g.p.floorz = oldPlayerFloor
		g.p.ceilz = oldPlayerCeil
	}()

	if g.actorTouchesSector(sec, g.p.x, g.p.y, playerRadius) && !g.heightClipPlayer(oldPlayerFloor) {
		return true
	}
	for i, th := range g.m.Things {
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
			continue
		}
		if i >= 0 && i < len(g.thingDropped) && g.thingDropped[i] {
			continue
		}
		if !thingTypeIsShootable(th.Type) || !g.thingTouchesSector(sec, i, th) {
			continue
		}
		oldZ, oldThingFloor, oldThingCeil := g.thingSupportState(i, th)
		oldValid := i >= 0 && i < len(g.thingSupportValid) && g.thingSupportValid[i]
		if !g.heightClipThing(i, th) {
			g.setThingSupportState(i, oldZ, oldThingFloor, oldThingCeil)
			if i >= 0 && i < len(g.thingSupportValid) {
				g.thingSupportValid[i] = oldValid
			}
			return true
		}
		g.setThingSupportState(i, oldZ, oldThingFloor, oldThingCeil)
		if i >= 0 && i < len(g.thingSupportValid) {
			g.thingSupportValid[i] = oldValid
		}
	}
	return false
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
	left, right, bottom, top, ok := g.sectorBlockBox(sec)
	if want := runtimeDebugEnv("GD_DEBUG_HEIGHTCLIP_SECTOR"); want != "" {
		var wantTic, wantSec, wantIdx int
		if _, err := fmt.Sscanf(want, "%d:%d:%d", &wantTic, &wantSec, &wantIdx); err == nil && sec == wantSec && (g.demoTick-1 == wantTic || g.worldTic == wantTic) {
			cell := -1
			if wantIdx >= 0 && wantIdx < len(g.thingBlockCell) {
				cell = g.thingBlockCell[wantIdx]
			}
			fmt.Printf("heightclip-sector-debug tic=%d world=%d sec=%d box=[l=%d r=%d b=%d t=%d] ok=%v idx=%d cell=%d pos=(%d,%d)\n",
				g.demoTick-1, g.worldTic, sec, left, right, bottom, top, ok, wantIdx, cell, g.thingX[wantIdx], g.thingY[wantIdx])
		}
	}
	playerTouches := false
	if ok {
		playerTouches = g.playerBlockCellInBox(left, right, bottom, top)
	} else {
		playerTouches = g.actorTouchesSector(sec, g.p.x, g.p.y, playerRadius)
	}
	if playerTouches {
		g.heightClipPlayer(oldPlayerFloor)
	}
	if ok && g.bmapWidth > 0 && g.bmapHeight > 0 && len(g.thingBlockCells) == g.bmapWidth*g.bmapHeight {
		seen := make(map[int]struct{})
		for bx := left; bx <= right; bx++ {
			for by := bottom; by <= top; by++ {
				cell := by*g.bmapWidth + bx
				if cell < 0 || cell >= len(g.thingBlockCells) {
					continue
				}
				for _, i := range g.thingBlockCells[cell] {
					if _, dup := seen[i]; dup {
						continue
					}
					seen[i] = struct{}{}
					if i < 0 || i >= len(g.m.Things) {
						continue
					}
					if i < len(g.thingCollected) && g.thingCollected[i] {
						continue
					}
					g.heightClipThing(i, g.m.Things[i])
				}
			}
		}
	} else {
		g.heightClipThingsInSector(sec)
	}
	g.refreshProjectileSupportInSector(sec)
}

func (g *game) sectorBlockBox(sec int) (left, right, bottom, top int, ok bool) {
	if g == nil || sec < 0 || sec >= len(g.sectorBBox) || g.bmapWidth <= 0 || g.bmapHeight <= 0 {
		return 0, 0, 0, 0, false
	}
	sb := g.sectorBBox[sec]
	if !isFinite(sb.minX) || !isFinite(sb.minY) || !isFinite(sb.maxX) || !isFinite(sb.maxY) {
		return 0, 0, 0, 0, false
	}
	minX := int64(math.Round(sb.minX)) << fracBits
	minY := int64(math.Round(sb.minY)) << fracBits
	maxX := int64(math.Round(sb.maxX)) << fracBits
	maxY := int64(math.Round(sb.maxY)) << fracBits
	const maxRadius = 32 * fracUnit
	top = int((maxY - g.bmapOriginY + maxRadius) >> (fracBits + 7))
	if top >= g.bmapHeight {
		top = g.bmapHeight - 1
	}
	bottom = int((minY - g.bmapOriginY - maxRadius) >> (fracBits + 7))
	if bottom < 0 {
		bottom = 0
	}
	right = int((maxX - g.bmapOriginX + maxRadius) >> (fracBits + 7))
	if right >= g.bmapWidth {
		right = g.bmapWidth - 1
	}
	left = int((minX - g.bmapOriginX - maxRadius) >> (fracBits + 7))
	if left < 0 {
		left = 0
	}
	if left > right || bottom > top {
		return 0, 0, 0, 0, false
	}
	return left, right, bottom, top, true
}

func (g *game) playerBlockCellInBox(left, right, bottom, top int) bool {
	if g == nil || g.bmapWidth <= 0 || g.bmapHeight <= 0 {
		return false
	}
	cell := g.thingBlockmapCellFor(g.p.x, g.p.y)
	if cell < 0 {
		return false
	}
	bx := cell % g.bmapWidth
	by := cell / g.bmapWidth
	return bx >= left && bx <= right && by >= bottom && by <= top
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

func (g *game) actorTouchesSector(sec int, x, y, radius int64) bool {
	if g == nil || g.m == nil || sec < 0 {
		return false
	}
	if g.sectorAt(x, y) == sec {
		return true
	}
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

func (g *game) thingTouchesSector(sec, i int, th mapdata.Thing) bool {
	x, y := g.thingPosFixed(i, th)
	return g.actorTouchesSector(sec, x, y, g.thingCurrentRadius(i, th))
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
	if i < len(g.thingGibbed) && g.thingGibbed[i] && i < len(g.thingGibTick) && g.thingGibTick[i] == g.worldTic {
		return true
	}
	x, y := g.thingPosFixed(i, th)
	radius := g.thingCurrentRadius(i, th)
	oldZ, oldFloorZ, oldCeilZ := g.thingSupportState(i, th)
	tmfloor, tmceil, _, ok := g.checkPositionForActor(x, y, radius, isMonster(th.Type), i, isMonster(th.Type))
	if !ok {
		if floorZ, ceilZ, found := g.subsectorFloorCeilAt(x, y); found {
			tmfloor = floorZ
			tmceil = ceilZ
		} else {
			tmfloor = oldFloorZ
			tmceil = oldCeilZ
		}
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
	if want := runtimeDebugEnv("GD_DEBUG_SUPPORT_TIC"); want != "" && runtimeDebugEnv("GD_DEBUG_SUPPORT_IDX") == fmt.Sprint(i) {
		if fmt.Sprint(g.demoTick-1) == want || fmt.Sprint(g.worldTic) == want {
			fmt.Printf("support-debug phase=heightclip tic=%d world=%d idx=%d type=%d x=%d y=%d oldz=%d oldfloor=%d tmfloor=%d tmceil=%d newz=%d sec=%d\n",
				g.demoTick-1, g.worldTic, i, th.Type, x, y, oldZ, oldFloorZ, tmfloor, tmceil, z, g.sectorAt(x, y))
		}
	}
	g.setThingSupportState(i, z, tmfloor, tmceil)
	if tmceil-tmfloor >= g.thingCurrentHeight(i, th) {
		return true
	}
	if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
		if i < len(g.thingGibbed) {
			g.thingGibbed[i] = true
		}
		if i < len(g.thingGibTick) {
			g.thingGibTick[i] = g.worldTic
		}
		g.setThingSupportState(i, z, tmfloor, tmceil)
		return true
	}
	if i >= 0 && i < len(g.thingDropped) && g.thingDropped[i] {
		if i < len(g.thingCollected) {
			g.thingCollected[i] = true
		}
		g.updateThingBlockmapIndex(i)
		return true
	}
	if !thingTypeIsShootable(th.Type) {
		return true
	}
	return false
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
		ft := &floorThinker{order: g.allocThinkerOrder(), sector: sec}
		switch info.Action {
		case mapdata.FloorRaise:
			ft.typ = 3
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.lowestSurroundingCeiling(sec)
		case mapdata.FloorRaiseToNearest:
			ft.typ = 4
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findNextHighestFloor(sec, g.sectorFloor[sec])
		case mapdata.FloorLower:
			ft.typ = 0
			ft.direction = -1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findHighestFloorSurrounding(sec)
		case mapdata.FloorLowerAndChange:
			ft.typ = 6
			ft.direction = -1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findLowestFloorSurrounding(sec)
		case mapdata.FloorRaiseCrush:
			ft.typ = 9
			ft.crush = true
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findLowestCeilingSurrounding(sec) - 8*fracUnit
		case mapdata.FloorRaise24:
			ft.typ = 7
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
		case mapdata.FloorRaise24AndChange:
			ft.typ = 8
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
			ft.finish = floorFinishSetTexture
			if frontSec >= 0 {
				ft.finishFlat = g.m.Sectors[frontSec].FloorPic
				ft.finishSpecial = g.m.Sectors[frontSec].Special
			}
		case mapdata.FloorRaiseToTexture:
			ft.typ = 5
			ft.direction = 1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.sectorFloor[sec] + 24*fracUnit
		case mapdata.FloorLowerToLowest:
			ft.typ = 1
			ft.direction = -1
			ft.speed = floorMoveSpeed
			ft.destHeight = g.findLowestFloorSurrounding(sec)
		case mapdata.FloorTurboLower:
			ft.typ = 2
			ft.direction = -1
			ft.speed = floorTurboSpeed
			ft.destHeight = g.findHighestFloorSurrounding(sec)
			if ft.destHeight != g.sectorFloor[sec] {
				ft.destHeight += 8 * fracUnit
			}
		case mapdata.FloorRaiseTurbo:
			ft.typ = 10
			ft.direction = 1
			ft.speed = floorTurboSpeed
			ft.destHeight = g.findNextHighestFloor(sec, g.sectorFloor[sec])
		case mapdata.FloorRaise512:
			ft.typ = 12
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
		if want := runtimeDebugEnv("GD_DEBUG_SECTOR_ACTIVATE"); want != "" {
			var wantSec int
			if _, err := fmt.Sscanf(want, "%d", &wantSec); err == nil && sec == wantSec {
				fmt.Printf("sector-activate-debug tic=%d world=%d kind=floor sec=%d line=%d action=%v tag=%d dir=%d speed=%d dest=%d\n",
					g.demoTick-1, g.worldTic, sec, lineIdx, info.Action, g.m.Linedefs[lineIdx].Tag, ft.direction, ft.speed, ft.destHeight)
			}
		}
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
		pt := g.allocPlatThinker(sec)
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
			g.m.Sectors[sec].Special = 0
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
			pt.oldStatus = platStatusInStasis
			pt.speed = 4 * platMoveSpeed
			pt.low = g.findLowestFloorSurrounding(sec)
			if pt.low > g.sectorFloor[sec] {
				pt.low = g.sectorFloor[sec]
			}
			pt.high = g.sectorFloor[sec]
			pt.wait = platWaitTics
		case mapdata.PlatBlazeDownWaitUpStay:
			pt.typ = platTypeBlazeDownWaitUpStay
			pt.status = platStatusDown
			pt.oldStatus = platStatusInStasis
			pt.speed = 8 * platMoveSpeed
			pt.low = g.findLowestFloorSurrounding(sec)
			if pt.low > g.sectorFloor[sec] {
				pt.low = g.sectorFloor[sec]
			}
			pt.high = g.sectorFloor[sec]
			pt.wait = platWaitTics
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
		if want := runtimeDebugEnv("GD_DEBUG_SECTOR_ACTIVATE"); want != "" {
			var wantSec int
			if _, err := fmt.Sscanf(want, "%d", &wantSec); err == nil && sec == wantSec {
				fmt.Printf("sector-activate-debug tic=%d world=%d kind=plat sec=%d line=%d action=%v tag=%d status=%d speed=%d low=%d high=%d typ=%d\n",
					g.demoTick-1, g.worldTic, sec, lineIdx, info.Action, g.m.Linedefs[lineIdx].Tag, pt.status, pt.speed, pt.low, pt.high, pt.typ)
			}
		}
		if g.platTickedThisTic {
			g.tickPlat(sec, pt)
		}
		activated = true
	}
	return activated
}

func (g *game) allocPlatThinker(sec int) *platThinker {
	if g == nil {
		return &platThinker{sector: sec}
	}
	var pt *platThinker
	if n := len(g.platFree); n > 0 {
		pt = g.platFree[n-1]
		g.platFree = g.platFree[:n-1]
	} else {
		pt = &platThinker{}
	}
	pt.order = g.allocThinkerOrder()
	pt.sector = sec
	return pt
}

func (g *game) freePlatThinker(pt *platThinker) {
	if g == nil || pt == nil {
		return
	}
	g.platFree = append(g.platFree, pt)
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
					order:      g.allocThinkerOrder(),
					sector:     sec,
					typ:        3,
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
	if debugLineTriggerEnabled(lineIdx) {
		fmt.Printf("line-trigger-debug tic=%d world=%d phase=teleport-enter line=%d side=%d player=%t uses_tag=%t monster_only=%t\n",
			g.demoTick-1, g.worldTic, lineIdx, side, isPlayer, info.UsesTag, info.MonsterOnly)
	}
	if side == 1 || !info.UsesTag {
		if debugLineTriggerEnabled(lineIdx) {
			fmt.Printf("line-trigger-debug tic=%d world=%d phase=teleport-reject line=%d side=%d player=%t\n",
				g.demoTick-1, g.worldTic, lineIdx, side, isPlayer)
		}
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
			g.setPlayerPosFixed(tx, ty)
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
			g.snapThingRenderState(actorIdx)
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
		ct := &ceilingThinker{order: g.allocThinkerOrder(), sector: sec, action: info.Action, speed: ceilingMoveSpeed}
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
		if want := runtimeDebugEnv("GD_DEBUG_SECTOR_ACTIVATE"); want != "" {
			var wantSec int
			if _, err := fmt.Sscanf(want, "%d", &wantSec); err == nil && sec == wantSec {
				fmt.Printf("sector-activate-debug tic=%d world=%d kind=ceiling sec=%d line=%d action=%v tag=%d dir=%d speed=%d top=%d bottom=%d\n",
					g.demoTick-1, g.worldTic, sec, lineIdx, info.Action, g.m.Linedefs[lineIdx].Tag, ct.direction, ct.speed, ct.topHeight, ct.bottomHeight)
			}
		}
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
			order:         g.allocThinkerOrder(),
			sector:        s2,
			typ:           11,
			direction:     1,
			speed:         floorMoveSpeed / 2,
			destHeight:    dest,
			finish:        floorFinishSetTexture,
			finishFlat:    g.m.Sectors[s3].FloorPic,
			finishSpecial: 0,
		}
		g.floors[s1] = &floorThinker{
			order:      g.allocThinkerOrder(),
			sector:     s1,
			typ:        0,
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
		g.tickFloor(sec, ft)
	}
}

func (g *game) tickFloor(sec int, ft *floorThinker) {
	if g == nil || ft == nil {
		return
	}
	if want := runtimeDebugEnv("GD_DEBUG_SECTOR_MOVER"); want != "" {
		var wantTic, wantSec int
		if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantSec); err == nil && sec == wantSec && (g.demoTick-1 == wantTic || g.worldTic == wantTic) {
			fmt.Printf("sector-mover-debug tic=%d world=%d kind=floor sec=%d cur=%d dir=%d speed=%d dest=%d order=%d\n",
				g.demoTick-1, g.worldTic, sec, g.sectorFloor[sec], ft.direction, ft.speed, ft.destHeight, ft.order)
		}
	}
	cur := g.sectorFloor[sec]
	next := cur + int64(ft.direction)*ft.speed
	done := false
	if ft.direction < 0 {
		if next < ft.destHeight {
			next = ft.destHeight
			done = true
		}
	} else {
		if next > ft.destHeight {
			next = ft.destHeight
			done = true
		}
	}
	g.setSectorFloorHeight(sec, next)
	if !done {
		return
	}
	if ft.finish == floorFinishSetTexture {
		g.m.Sectors[sec].FloorPic = ft.finishFlat
		g.m.Sectors[sec].Special = ft.finishSpecial
		g.markDynamicSectorPlaneCacheDirty(sec)
	}
	delete(g.floors, sec)
}

func (g *game) tickPlats() {
	g.platTickedThisTic = true
	for sec, pt := range g.plats {
		g.tickPlat(sec, pt)
	}
}

func (g *game) tickPlat(sec int, pt *platThinker) {
	if g == nil || pt == nil {
		return
	}
	if want := runtimeDebugEnv("GD_DEBUG_SECTOR_MOVER"); want != "" {
		var wantTic, wantSec int
		if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantSec); err == nil && sec == wantSec && (g.demoTick-1 == wantTic || g.worldTic == wantTic) {
			fmt.Printf("sector-mover-debug tic=%d world=%d kind=plat sec=%d cur=%d low=%d high=%d status=%d speed=%d wait=%d count=%d typ=%d order=%d\n",
				g.demoTick-1, g.worldTic, sec, g.sectorFloor[sec], pt.low, pt.high, pt.status, pt.speed, pt.wait, pt.count, pt.typ, pt.order)
		}
	}
	switch pt.status {
	case platStatusUp:
		next := g.sectorFloor[sec] + pt.speed
		if next <= pt.high && g.sectorMoveWouldBlockLiveActor(sec, next, g.sectorCeil[sec]) {
			pt.count = pt.wait
			pt.status = platStatusDown
			return
		}
		if next > pt.high {
			next = pt.high
			g.setSectorFloorHeight(sec, next)
			if pt.typ == platTypeRaiseToNearestAndChange || pt.typ == platTypeDownWaitUpStay || pt.typ == platTypeBlazeDownWaitUpStay {
				if pt.finishFlat != "" {
					g.m.Sectors[sec].FloorPic = pt.finishFlat
				}
				g.m.Sectors[sec].Special = pt.finishSpecial
				g.markDynamicSectorPlaneCacheDirty(sec)
				delete(g.plats, sec)
				g.freePlatThinker(pt)
			} else {
				pt.status = platStatusWaiting
				pt.count = pt.wait
			}
			return
		}
		g.setSectorFloorHeight(sec, next)
	case platStatusDown:
		next := g.sectorFloor[sec] - pt.speed
		if next < pt.low {
			next = pt.low
			g.setSectorFloorHeight(sec, next)
			pt.status = platStatusWaiting
			pt.count = pt.wait
			return
		}
		g.setSectorFloorHeight(sec, next)
	case platStatusWaiting:
		pt.count--
		if pt.count > 0 {
			return
		}
		if g.sectorFloor[sec] == pt.low {
			pt.status = platStatusUp
		} else {
			pt.status = platStatusDown
		}
	case platStatusInStasis:
		return
	}
}

func (g *game) tickCeilings() {
	for sec, ct := range g.ceilings {
		g.tickCeiling(sec, ct)
	}
}

func (g *game) tickCeiling(sec int, ct *ceilingThinker) {
	if g == nil || ct == nil {
		return
	}
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
			return
		}
		g.setSectorCeilingHeight(sec, next)
	case 1:
		next := cur + ct.speed
		if next >= ct.topHeight {
			next = ct.topHeight
			g.setSectorCeilingHeight(sec, next)
			delete(g.ceilings, sec)
			return
		}
		g.setSectorCeilingHeight(sec, next)
	case 0:
		return
	}
}
