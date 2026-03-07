package automap

import "gddoom/internal/mapdata"

const (
	floorMoveSpeed    = fracUnit
	floorTurboSpeed   = 4 * fracUnit
	platWaitTics      = 3 * doomTicsPerSecond
	platMoveSpeed     = fracUnit
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
)

type platType uint8

const (
	platTypeDownWaitUpStay platType = iota
	platTypeRaiseToNearestAndChange
)

type platThinker struct {
	sector        int
	typ           platType
	status        platStatus
	speed         int64
	low           int64
	high          int64
	wait          int
	count         int
	finishFlat    string
	finishSpecial int16
}

func lineSpecialSupported(info mapdata.LineSpecialInfo) bool {
	return info.Exit != mapdata.ExitNone ||
		info.Door != nil ||
		info.Floor != nil ||
		info.Plat != nil ||
		info.Stairs != nil ||
		info.Light != nil ||
		info.Teleport != nil ||
		info.Donut
}

func (g *game) activateNonDoorLineSpecial(lineIdx int, side int, info mapdata.LineSpecialInfo) bool {
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
		return g.activateTeleportLine(lineIdx, side, *info.Teleport)
	case info.Donut:
		return g.activateDonutLine(lineIdx)
	default:
		return false
	}
}

func (g *game) taggedSectorsForLine(lineIdx int) []int {
	if g == nil || g.m == nil || lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
		return nil
	}
	tag := g.m.Linedefs[lineIdx].Tag
	if tag == 0 {
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
		(g.plats != nil && g.plats[sec] != nil)
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
	if old == z {
		return
	}
	g.sectorFloor[sec] = z
	g.markDynamicSectorPlaneCacheDirty(sec)
	if sec < len(g.m.Sectors) {
		g.m.Sectors[sec].FloorHeight = int16(z >> fracBits)
	}
	if g.playerTouchesSector(sec) {
		onFloor := g.p.z == old
		if onFloor || g.p.floorz == old {
			g.p.floorz = z
			if onFloor || g.p.z < z {
				g.p.z = z
			}
		}
		if g.p.z < z {
			g.p.z = z
		}
		g.p.floorz = z
	}
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
			pt.speed = platMoveSpeed / 2
			pt.high = g.findNextHighestFloor(sec, g.sectorFloor[sec])
			pt.wait = 0
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
			next, ok := g.nextStairSector(sec, texture)
			if !ok {
				break
			}
			sec = next
			height += stepSize
		}
	}
	return activated
}

func (g *game) nextStairSector(sec int, texture string) (int, bool) {
	for _, ld := range g.m.Linedefs {
		s0, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
		s1, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
		if !ok0 || !ok1 {
			continue
		}
		if s0 == sec && g.m.Sectors[s1].FloorPic == texture {
			return s1, true
		}
		if s1 == sec && g.m.Sectors[s0].FloorPic == texture {
			return s0, true
		}
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
			g.spawnSectorStrobeFlash(sec, sectorLightSlowDark, false)
		default:
			continue
		}
		g.refreshSectorPlaneCacheLighting()
		activated = true
	}
	return activated
}

func (g *game) activateTeleportLine(lineIdx int, side int, info mapdata.TeleportInfo) bool {
	if side == 1 || !info.UsesTag {
		return false
	}
	line := g.m.Linedefs[lineIdx]
	for i, th := range g.m.Things {
		if th.Type != teleportThingType {
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
		destSec := g.sectorAt(tx, ty)
		if destSec < 0 || destSec >= len(g.sectorFloor) || destSec >= len(g.sectorCeil) {
			return false
		}
		floorZ := g.sectorFloor[destSec]
		ceilZ := g.sectorCeil[destSec]
		if ceilZ-floorZ < playerHeight {
			return false
		}
		g.p.x = tx
		g.p.y = ty
		g.p.floorz = floorZ
		g.p.ceilz = ceilZ
		g.p.z = floorZ
		g.playerViewZ = g.p.z + 41*fracUnit
		g.p.momx = 0
		g.p.momy = 0
		g.p.angle = thingDegToWorldAngle(th.Angle)
		return true
	}
	return false
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
		}
	}
}
