package doomruntime

import "gddoom/internal/mapdata"

// Elevators live in the floor thinker list but reserve both plane slots.
func (g *game) floorSlotBusy(sec int) bool { return g.floors[sec] != nil || g.plats[sec] != nil }
func (g *game) ceilingSlotBusy(sec int) bool {
	if owner, tracked := g.ceilingOwners[sec]; tracked {
		return owner != 0
	}
	return g.activeDoorThinker(sec) != nil || g.ceilings[sec] != nil || g.floors[sec] != nil && g.floors[sec].elevator
}
func (g *game) legacyFloorBusy(sec int) bool {
	if g.boomMomentumDropoff {
		return g.floorSlotBusy(sec)
	}
	return g.sectorHasActiveMover(sec)
}
func (g *game) legacyCeilingBusy(sec int) bool {
	if g.boomMomentumDropoff {
		return g.ceilingSlotBusy(sec)
	}
	return g.sectorHasActiveMover(sec)
}
func (g *game) findNextLowestFloor(sec int, current int64) int64 {
	next := current
	for _, other := range g.boomNeighborSectors(sec) {
		h := g.sectorFloor[other]
		if h < current && (next == current || h > next) {
			next = h
		}
	}
	return next
}
func (g *game) activateLegacyChange(line int, numeric bool) bool {
	targets := g.taggedSectorsForLine(line)
	for _, sec := range targets {
		model, ok := g.frontSectorForLine(line)
		if numeric {
			ok = false
			for _, other := range g.boomNeighborSectors(sec) {
				if g.sectorFloor[other] == g.sectorFloor[sec] {
					model = other
					ok = true
					break
				}
			}
		}
		if ok {
			g.m.Sectors[sec].FloorPic = g.m.Sectors[model].FloorPic
			g.m.Sectors[sec].Special = g.m.Sectors[model].Special
			g.markDynamicSectorPlaneCacheDirty(sec)
		}
	}
	// EV_DoChange succeeds for a tagged sector even without a numeric model.
	return len(targets) != 0
}
func (g *game) activateElevator(line int, action mapdata.ElevatorAction) bool {
	if g.floors == nil {
		g.floors = make(map[int]*floorThinker)
	}
	active := false
	for _, sec := range g.taggedSectorsForLine(line) {
		if g.floorSlotBusy(sec) || g.ceilingSlotBusy(sec) {
			continue
		}
		current := g.sectorFloor[sec]
		dest := current
		direction := 1
		switch action {
		case mapdata.ElevatorUp:
			dest = g.findNextHighestFloor(sec, current)
		case mapdata.ElevatorDown:
			direction = -1
			dest = g.findNextLowestFloor(sec, current)
		case mapdata.ElevatorCurrent:
			model, ok := g.frontSectorForLine(line)
			if !ok {
				continue
			}
			dest = g.sectorFloor[model]
			if dest <= current {
				direction = -1
			}
		}
		g.floors[sec] = &floorThinker{order: g.allocThinkerOrder(), sector: sec, elevator: true, direction: direction, speed: 4 * fracUnit, destHeight: dest, ceilingDestHeight: g.sectorCeil[sec] + dest - current}
		g.assignTrackedCeilingSlot(sec, g.floors[sec].order)
		active = true
	}
	return active
}

type boomPlaneResult uint8

const (
	boomPlaneOK boomPlaneResult = iota
	boomPlanePast
	boomPlaneBlocked
)

// Boom moves elevator planes in order, rather than translating the room as
// one volume. A blocked leading plane suppresses the trailing plane for a tic.
func (g *game) moveElevatorPlane(sec int, ceiling bool, speed, dest int64, direction int) boomPlaneResult {
	current := g.sectorFloor[sec]
	if ceiling {
		current = g.sectorCeil[sec]
	}
	if ceiling && direction < 0 {
		dest = max64(dest, g.sectorFloor[sec])
	}
	if !ceiling && direction > 0 {
		dest = min64(dest, g.sectorCeil[sec])
	}
	next := current + int64(direction)*speed
	past := direction < 0 && next < dest || direction > 0 && next > dest
	if past {
		next = dest
	}
	move := func(height int64) bool {
		if ceiling {
			return g.setSectorCeilingHeightWithCrush(sec, height, false)
		}
		return g.setSectorFloorHeightWithCrush(sec, height, false)
	}
	blocked := move(next)
	if blocked && (past || ceiling && direction < 0 || !ceiling && direction > 0) {
		move(current)
	}
	if past {
		return boomPlanePast
	}
	if blocked && (ceiling && direction < 0 || !ceiling && direction > 0) {
		return boomPlaneBlocked
	}
	return boomPlaneOK
}
func (g *game) tickElevator(sec int, ft *floorThinker) {
	firstCeiling := ft.direction < 0
	first, second := ft.destHeight, ft.ceilingDestHeight
	if firstCeiling {
		first, second = second, first
	}
	result := g.moveElevatorPlane(sec, firstCeiling, ft.speed, first, ft.direction)
	if result != boomPlaneBlocked {
		g.moveElevatorPlane(sec, !firstCeiling, ft.speed, second, ft.direction)
	}
	g.emitSectorMoveLoop(sec)
	if result == boomPlanePast {
		g.emitSectorSound(sec, soundEventPlatStop)
		g.clearTrackedCeilingSlot(sec)
		delete(g.floors, sec)
	}
}
func (g *game) tickTogglePlat(sec int, pt *platThinker) {
	if pt.status != platStatusUp && pt.status != platStatusDown {
		return
	}
	// The intentionally reversed destinations make each toggle instant.
	direction := 1
	dest := pt.high
	if pt.status == platStatusDown {
		direction = -1
		dest = pt.low
	}
	// The labels intentionally look reversed: low is the initial ceiling and
	// high the initial floor. This guarantees an immediate past-destination move.
	result := g.moveElevatorPlane(sec, false, pt.speed, dest, direction)
	if result == boomPlanePast {
		pt.oldStatus = pt.status
		pt.status = platStatusInStasis
	}
}

// Boom requires the first pillar line to be two-sided and selects a model
// from the pool's back sectors in map order. Do not silently repair bad donuts.
func (g *game) findBoomDonutSectors(pillar int) (int, int, bool) {
	for _, line := range g.m.Linedefs {
		front, okFront := g.sectorIndexFromSidedef(line.SideNum[0])
		back, okBack := g.sectorIndexFromSidedef(line.SideNum[1])
		if (!okFront || front != pillar) && (!okBack || back != pillar) {
			continue
		}
		if !okFront || !okBack || line.Flags&mlTwoSided == 0 {
			return -1, -1, false
		}
		pool := front
		if pool == pillar {
			pool = back
		}
		for _, candidate := range g.m.Linedefs {
			a, oa := g.sectorIndexFromSidedef(candidate.SideNum[0])
			b, ob := g.sectorIndexFromSidedef(candidate.SideNum[1])
			if (oa && a == pool || ob && b == pool) && ob && b != pillar {
				return pool, b, true
			}
		}
		return -1, -1, false
	}
	return -1, -1, false
}
