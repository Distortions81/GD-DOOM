package automap

import (
	"math"
	"sort"
	"strings"

	"gddoom/internal/mapdata"
)

func (g *game) handleUse() {
	if g.isDead {
		g.setHUDMessage("You are dead", 20)
		return
	}
	g.useLines()
}

func (g *game) useLines() {
	lineIdx, tr := g.peekUseTargetLine()
	if tr == useTraceNone {
		g.useText = "USE: no line"
		g.useFlash = 35
		g.emitSoundEvent(soundEventNoWay)
		return
	}
	if tr == useTraceBlocked {
		g.useText = "USE: no way"
		g.useFlash = 35
		g.emitSoundEvent(soundEventNoWay)
		return
	}
	pi := -1
	if lineIdx >= 0 && lineIdx < len(g.physForLine) {
		pi = g.physForLine[lineIdx]
	}
	if pi < 0 || pi >= len(g.lines) {
		g.useText = "USE: no line"
		g.useFlash = 35
		g.emitSoundEvent(soundEventNoWay)
		return
	}
	ld := g.lines[pi]
	side := 0
	if g.pointOnLineSide(g.p.x, g.p.y, ld) == 1 {
		side = 1
	}
	g.useSpecialLine(ld.idx, side)
}

type useTraceResult int

const (
	useTraceNone useTraceResult = iota
	useTraceBlocked
	useTraceSpecial
)

func (g *game) peekUseTargetLine() (int, useTraceResult) {
	px := g.p.x
	py := g.p.y
	ang := angleToRadians(g.p.angle)
	fx := int64(math.Cos(ang) * useRange)
	fy := int64(math.Sin(ang) * useRange)
	x2 := px + fx
	y2 := py + fy

	intercepts := make([]intercept, 0, 16)
	for _, ld := range g.lines {
		frac, ok := segmentIntersectFrac(px, py, x2, y2, ld.x1, ld.y1, ld.x2, ld.y2)
		if !ok {
			continue
		}
		intercepts = append(intercepts, intercept{frac: frac, line: ld.idx})
	}
	sortUseIntercepts(intercepts, g.lineSpecial)

	for _, in := range intercepts {
		pi := -1
		if in.line >= 0 && in.line < len(g.physForLine) {
			pi = g.physForLine[in.line]
		}
		if pi < 0 || pi >= len(g.lines) {
			continue
		}
		ld := g.lines[pi]
		special := g.lineSpecial[ld.idx]
		if special == 0 {
			_, _, _, openrange := g.lineOpening(ld)
			if openrange <= 0 {
				return -1, useTraceBlocked
			}
			continue
		}
		return ld.idx, useTraceSpecial
	}
	return -1, useTraceNone
}

func sortUseIntercepts(intercepts []intercept, lineSpecial []uint16) {
	const eps = 1e-6
	sort.SliceStable(intercepts, func(i, j int) bool {
		di := intercepts[i]
		dj := intercepts[j]
		if math.Abs(di.frac-dj.frac) <= eps {
			si := uint16(0)
			if di.line >= 0 && di.line < len(lineSpecial) {
				si = lineSpecial[di.line]
			}
			sj := uint16(0)
			if dj.line >= 0 && dj.line < len(lineSpecial) {
				sj = lineSpecial[dj.line]
			}
			if (si != 0) != (sj != 0) {
				return si != 0
			}
			// Keep tie behavior deterministic for equal-distance hits.
			return di.line < dj.line
		}
		return di.frac < dj.frac
	})
}

func (g *game) useSpecialLine(lineIdx int, side int) {
	if g.isDead {
		g.useText = "You are dead"
		g.useFlash = 20
		return
	}
	special := g.lineSpecial[lineIdx]
	if side == 1 && special != 124 {
		g.useText = "USE: back side"
		g.useFlash = 35
		g.emitSoundEvent(soundEventNoWay)
		return
	}
	if g.handleExitSpecial(lineIdx, special, mapdata.TriggerUse) {
		g.animateSwitchTexture(lineIdx, side, false)
		g.emitSoundEvent(soundEventSwitchOn)
		return
	}
	info := mapdata.LookupLineSpecial(special)
	if info.Door == nil || (info.Trigger != mapdata.TriggerManual && info.Trigger != mapdata.TriggerUse) {
		g.useText = "USE: unsupported special"
		g.useFlash = 35
		g.emitSoundEvent(soundEventNoWay)
		return
	}
	if !info.Door.CanActivate(g.inventory.keys()) {
		g.useText = "USE: locked"
		g.useFlash = 35
		g.emitSoundEvent(soundEventNoWay)
		return
	}
	activated := g.activateDoorLine(lineIdx, info)
	if activated {
		g.useText = "USE: door active"
		if shouldPlaySwitchClick(info) {
			g.animateSwitchTexture(lineIdx, side, info.Repeat)
			g.emitSoundEvent(soundEventSwitchOn)
			if info.Repeat {
				g.emitSoundEventDelayed(soundEventSwitchOff, switchResetTics)
			}
		}
	} else {
		g.useText = "USE: no change"
		g.emitSoundEvent(soundEventNoWay)
	}
	g.useFlash = 35
}

func (g *game) checkWalkSpecialLines(prevX, prevY, curX, curY int64) {
	if prevX == curX && prevY == curY {
		return
	}
	for _, ld := range g.lines {
		if ld.idx < 0 || ld.idx >= len(g.lineSpecial) {
			continue
		}
		special := g.lineSpecial[ld.idx]
		if special == 0 {
			continue
		}
		info := mapdata.LookupLineSpecial(special)
		if info.Trigger != mapdata.TriggerWalk {
			continue
		}
		if info.Exit == mapdata.ExitNone && info.Door == nil {
			continue
		}
		startSide := g.pointOnLineSide(prevX, prevY, ld)
		endSide := g.pointOnLineSide(curX, curY, ld)
		if !(startSide == 0 && endSide == 1) {
			continue
		}
		if _, ok := segmentIntersectFrac(prevX, prevY, curX, curY, ld.x1, ld.y1, ld.x2, ld.y2); !ok {
			continue
		}
		if info.Exit != mapdata.ExitNone && g.handleExitSpecial(ld.idx, special, mapdata.TriggerWalk) {
			return
		}
		if info.Door != nil && info.Door.CanActivate(g.inventory.keys()) {
			if g.activateDoorLine(ld.idx, info) {
				if !info.Repeat && ld.idx >= 0 && ld.idx < len(g.lineSpecial) {
					g.lineSpecial[ld.idx] = 0
				}
				return
			}
		}
		// Crossing a special line can consume movement intent for this tic
		// even when no action is taken.
		return
	}
}

func (g *game) handleExitSpecial(lineIdx int, special uint16, trigger mapdata.TriggerType) bool {
	if g.isDead {
		return false
	}
	info := mapdata.LookupLineSpecial(special)
	if info.Exit == mapdata.ExitNone || info.Trigger != trigger {
		return false
	}
	if !info.Repeat && lineIdx >= 0 && lineIdx < len(g.lineSpecial) {
		g.lineSpecial[lineIdx] = 0
	}
	switch info.Exit {
	case mapdata.ExitSecret:
		g.requestLevelExit(true, "Secret Exit")
	default:
		g.requestLevelExit(false, "Level Complete")
	}
	return true
}

func shouldPlaySwitchClick(info mapdata.LineSpecialInfo) bool {
	// Doom-like: use-triggered switch/button specials click; manual doors do not.
	return info.Trigger == mapdata.TriggerUse && info.Door != nil
}

func (g *game) activateDoorLine(lineIdx int, info mapdata.LineSpecialInfo) bool {
	if info.Trigger == mapdata.TriggerManual {
		return g.evVerticalDoor(lineIdx)
	}
	return g.evDoDoorTagged(lineIdx, info)
}

func (g *game) evVerticalDoor(lineIdx int) bool {
	if lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
		return false
	}
	ld := g.m.Linedefs[lineIdx]
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		return false
	}
	sec := targets[0]
	if sec < 0 || sec >= len(g.sectorCeil) {
		return false
	}

	if d := g.doors[sec]; d != nil {
		switch ld.Special {
		case 1, 26, 27, 28, 117:
			if d.direction == -1 {
				d.direction = 1
			} else {
				d.direction = -1
			}
			g.emitSoundEvent(doorMoveEvent(d.typ, d.direction))
			return true
		}
	}

	d := &doorThinker{
		sector:    sec,
		direction: 1,
		speed:     vDoorSpeed,
		topWait:   vDoorWaitTic,
		topHeight: g.lowestSurroundingCeiling(sec) - 4*fracUnit,
	}
	if d.topHeight < g.sectorFloor[sec] {
		d.topHeight = g.sectorFloor[sec]
	}
	switch ld.Special {
	case 1, 26, 27, 28:
		d.typ = doorNormal
	case 31, 32, 33, 34:
		d.typ = doorOpen
		g.lineSpecial[lineIdx] = 0
	case 117:
		d.typ = doorBlazeRaise
		d.speed = vDoorSpeed * 4
	case 118:
		d.typ = doorBlazeOpen
		d.speed = vDoorSpeed * 4
		g.lineSpecial[lineIdx] = 0
	default:
		return false
	}
	g.doors[sec] = d
	g.emitSoundEvent(doorMoveEvent(d.typ, d.direction))
	return true
}

func (g *game) evDoDoorTagged(lineIdx int, info mapdata.LineSpecialInfo) bool {
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		return false
	}
	activated := false
	for _, sec := range targets {
		if sec < 0 || sec >= len(g.sectorCeil) {
			continue
		}
		if g.doors[sec] != nil {
			continue
		}
		d := &doorThinker{
			sector:    sec,
			topWait:   vDoorWaitTic,
			speed:     vDoorSpeed,
			topHeight: g.lowestSurroundingCeiling(sec) - 4*fracUnit,
		}
		if d.topHeight < g.sectorFloor[sec] {
			d.topHeight = g.sectorFloor[sec]
		}
		switch info.Door.Action {
		case mapdata.DoorOpen:
			d.typ = doorOpen
			d.direction = 1
		case mapdata.DoorClose:
			d.typ = doorClose
			d.direction = -1
		case mapdata.DoorRaise:
			d.typ = doorNormal
			d.direction = 1
		case mapdata.DoorClose30ThenOpen:
			d.typ = doorClose30ThenOpen
			d.direction = -1
		case mapdata.DoorBlazeOpen:
			d.typ = doorBlazeOpen
			d.direction = 1
			d.speed = vDoorSpeed * 4
		case mapdata.DoorBlazeClose:
			d.typ = doorBlazeClose
			d.direction = -1
			d.speed = vDoorSpeed * 4
		case mapdata.DoorBlazeRaise:
			d.typ = doorBlazeRaise
			d.direction = 1
			d.speed = vDoorSpeed * 4
		default:
			continue
		}
		g.doors[sec] = d
		g.emitSoundEvent(doorMoveEvent(d.typ, d.direction))
		activated = true
	}
	return activated
}

func doorMoveEvent(typ doorType, direction int) soundEvent {
	if direction < 0 {
		switch typ {
		case doorBlazeRaise, doorBlazeClose:
			return soundEventBlazeClose
		default:
			return soundEventDoorClose
		}
	}
	switch typ {
	case doorBlazeRaise, doorBlazeOpen, doorBlazeClose:
		return soundEventBlazeOpen
	default:
		return soundEventDoorOpen
	}
}

func (g *game) lowestSurroundingCeiling(sector int) int64 {
	lowest := int64(1<<62 - 1)
	for _, ld := range g.lines {
		if ld.sideNum0 < 0 || ld.sideNum1 < 0 {
			continue
		}
		s0 := g.m.Sidedefs[int(ld.sideNum0)].Sector
		s1 := g.m.Sidedefs[int(ld.sideNum1)].Sector
		if int(s0) == sector {
			c := g.sectorCeil[s1]
			if c < lowest {
				lowest = c
			}
		} else if int(s1) == sector {
			c := g.sectorCeil[s0]
			if c < lowest {
				lowest = c
			}
		}
	}
	if lowest == int64(1<<62-1) {
		return g.sectorCeil[sector]
	}
	return lowest
}

func toggleSwitchTexture(name string) (string, bool) {
	base := strings.TrimSpace(name)
	if len(base) < 4 {
		return name, false
	}
	upper := strings.ToUpper(base)
	if strings.HasPrefix(upper, "SW1") {
		return "SW2" + base[3:], true
	}
	if strings.HasPrefix(upper, "SW2") {
		return "SW1" + base[3:], true
	}
	return name, false
}

func (g *game) animateSwitchTexture(lineIdx, side int, repeat bool) {
	if lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
		return
	}
	ld := g.m.Linedefs[lineIdx]
	sideDefIdx := int(ld.SideNum[0])
	if side == 1 {
		sideDefIdx = int(ld.SideNum[1])
	}
	if sideDefIdx < 0 || sideDefIdx >= len(g.m.Sidedefs) {
		return
	}
	sd := &g.m.Sidedefs[sideDefIdx]
	origTop, origBottom, origMid := sd.Top, sd.Bottom, sd.Mid
	changed := false
	if next, ok := toggleSwitchTexture(sd.Top); ok {
		sd.Top = next
		changed = true
	}
	if next, ok := toggleSwitchTexture(sd.Bottom); ok {
		sd.Bottom = next
		changed = true
	}
	if next, ok := toggleSwitchTexture(sd.Mid); ok {
		sd.Mid = next
		changed = true
	}
	if !changed || !repeat {
		return
	}
	for i := range g.delayedSwitchReverts {
		if g.delayedSwitchReverts[i].sidedef != sideDefIdx {
			continue
		}
		g.delayedSwitchReverts[i].top = origTop
		g.delayedSwitchReverts[i].bottom = origBottom
		g.delayedSwitchReverts[i].mid = origMid
		g.delayedSwitchReverts[i].tics = switchResetTics
		return
	}
	g.delayedSwitchReverts = append(g.delayedSwitchReverts, delayedSwitchTexture{
		sidedef: sideDefIdx,
		top:     origTop,
		bottom:  origBottom,
		mid:     origMid,
		tics:    switchResetTics,
	})
}
