package doomruntime

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"gddoom/internal/mapdata"
)

func debugLineTriggerEnabled(lineIdx int) bool {
	want := strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_LINE_TRIGGER"))
	if want == "" {
		return false
	}
	for _, part := range strings.Split(want, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.Atoi(part); err == nil && n == lineIdx {
			return true
		}
	}
	return false
}

func (g *game) debugDoorActivate(format string, args ...any) {
	if g == nil {
		return
	}
	want := strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_DOOR_ACTIVATE_TIC"))
	if want == "" {
		return
	}
	if want != fmt.Sprint(g.demoTick-1) && want != fmt.Sprint(g.worldTic) {
		return
	}
	fmt.Printf("door-activate-debug tic=%d world=%d %s\n", g.demoTick-1, g.worldTic, fmt.Sprintf(format, args...))
}

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
	_ = g.useSpecialLineForActor(lineIdx, side, true)
}

func (g *game) useSpecialLineForActor(lineIdx int, side int, isPlayer bool) bool {
	if debugLineTriggerEnabled(lineIdx) {
		fmt.Printf("line-trigger-debug tic=%d world=%d phase=use-enter line=%d side=%d player=%t special=%d\n",
			g.demoTick-1, g.worldTic, lineIdx, side, isPlayer, g.lineSpecial[lineIdx])
	}
	if g.isDead {
		if isPlayer {
			g.useText = "You are dead"
			g.useFlash = 20
		}
		return false
	}
	special := g.lineSpecial[lineIdx]
	if side == 1 && special != 124 {
		if isPlayer {
			g.useText = "USE: back side"
			g.useFlash = 35
		}
		return false
	}
	if !isPlayer {
		if lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
			return false
		}
		if g.m.Linedefs[lineIdx].Flags&mlSecret != 0 {
			return false
		}
		switch special {
		case 1, 32, 33, 34:
		default:
			return false
		}
	}
	if g.handleExitSpecial(lineIdx, special, mapdata.TriggerUse) {
		if isPlayer {
			g.animateSwitchTexture(lineIdx, side, false)
			g.emitSoundEvent(soundEventSwitchExit)
		}
		return true
	}
	info := mapdata.LookupLineSpecial(special)
	if !lineSpecialSupported(info) {
		if isPlayer {
			g.useText = fmt.Sprintf("USE: unsupported special %d", special)
			g.useFlash = 35
		}
		return false
	}
	if info.Trigger != mapdata.TriggerManual && info.Trigger != mapdata.TriggerUse {
		if isPlayer {
			g.useText = "USE: no change"
			g.useFlash = 35
		}
		return false
	}
	if isPlayer && info.Door != nil && !info.Door.CanActivate(g.inventory.keys()) {
		if isPlayer {
			g.useText = "USE: locked"
			g.useFlash = 35
			g.emitSoundEvent(soundEventOof)
		}
		return false
	}
	activated := false
	if info.Door != nil {
		activated = g.activateDoorLine(lineIdx, info, isPlayer)
		if !activated && !isPlayer {
			switch special {
			case 32, 33, 34:
				// Doom's P_UseSpecialLine returns true for these monster-allowed
				// keyed manual door specials even though EV_VerticalDoor rejects
				// non-player activators and does not start the door.
				return true
			}
		}
	} else {
		activated = g.activateNonDoorLineSpecial(lineIdx, side, info, -1, true)
		if activated && !info.Repeat && lineIdx >= 0 && lineIdx < len(g.lineSpecial) {
			g.lineSpecial[lineIdx] = 0
		}
	}
	if activated {
		if debugLineTriggerEnabled(lineIdx) {
			fmt.Printf("line-trigger-debug tic=%d world=%d phase=use-activate line=%d side=%d player=%t special=%d repeat=%t\n",
				g.demoTick-1, g.worldTic, lineIdx, side, isPlayer, special, info.Repeat)
		}
		if isPlayer {
			if info.Door != nil {
				g.useText = "USE: door active"
			} else {
				g.useText = "USE: special active"
			}
		}
		if isPlayer && shouldPlaySwitchClick(info) {
			g.animateSwitchTexture(lineIdx, side, info.Repeat)
			g.emitSoundEvent(soundEventSwitchOn)
			if info.Repeat {
				g.emitSoundEventDelayed(soundEventSwitchOff, switchResetTics)
			}
		}
	} else if isPlayer {
		g.useText = "USE: no change"
	}
	if isPlayer {
		g.useFlash = 35
	}
	return activated
}

func walkSpecialAllowedForNonPlayer(special uint16) bool {
	switch special {
	case 4, 10, 39, 88, 97, 125, 126:
		return true
	default:
		return false
	}
}

func (g *game) checkWalkSpecialLines(prevX, prevY, curX, curY int64) {
	g.checkWalkSpecialLinesWithCandidates(prevX, prevY, curX, curY, nil)
}

func (g *game) checkWalkSpecialLinesForActor(prevX, prevY, curX, curY int64, actorIdx int, isPlayer bool) {
	g.checkWalkSpecialLinesForActorWithCandidates(prevX, prevY, curX, curY, actorIdx, isPlayer, nil)
}

func (g *game) checkWalkSpecialLinesWithCandidates(prevX, prevY, curX, curY int64, candidateLineIdxs []int) {
	g.checkWalkSpecialLinesForActorWithCandidates(prevX, prevY, curX, curY, -1, true, candidateLineIdxs)
}

func (g *game) checkWalkSpecialLinesForActorWithCandidates(prevX, prevY, curX, curY int64, actorIdx int, isPlayer bool, candidateLineIdxs []int) {
	if prevX == curX && prevY == curY {
		return
	}
	radius := int64(0)
	if isPlayer {
		radius = playerRadius
	} else if g != nil && g.m != nil && actorIdx >= 0 && actorIdx < len(g.m.Things) {
		radius = monsterRadius(g.m.Things[actorIdx].Type)
	}
	prevSS := -1
	curSS := -1
	if g != nil && g.m != nil && len(g.m.SubSectors) > 0 {
		prevSS = g.subSectorAtFixed(prevX, prevY)
		curSS = g.subSectorAtFixed(curX, curY)
	}
	minX, maxX := prevX, curX
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := prevY, curY
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	minX -= radius
	maxX += radius
	minY -= radius
	maxY += radius
	visit := func(ld physLine) bool {
		if ld.idx < 0 || ld.idx >= len(g.lineSpecial) {
			return false
		}
		if maxX < ld.bbox[3] || minX > ld.bbox[2] || maxY < ld.bbox[1] || minY > ld.bbox[0] {
			return false
		}
		special := g.lineSpecial[ld.idx]
		if special == 0 {
			return false
		}
		if debugLineTriggerEnabled(ld.idx) {
			fmt.Printf("line-trigger-debug tic=%d world=%d phase=walk-check line=%d prev=(%d,%d) cur=(%d,%d) player=%t special=%d\n",
				g.demoTick-1, g.worldTic, ld.idx, prevX, prevY, curX, curY, isPlayer, special)
		}
		if want := runtimeDebugEnv("GD_DEBUG_WALK_SPECIAL_TIC"); want != "" {
			if want == fmt.Sprint(g.demoTick-1) || want == fmt.Sprint(g.worldTic) {
				fmt.Fprintf(os.Stderr, "walk-special-debug tic=%d world=%d line=%d special=%d prev=(%d,%d) cur=(%d,%d)\n",
					g.demoTick-1, g.worldTic, ld.idx, special, prevX, prevY, curX, curY)
			}
		}
		info := mapdata.LookupLineSpecial(special)
		if info.Trigger != mapdata.TriggerWalk {
			return false
		}
		if !lineSpecialSupported(info) {
			return false
		}
		if !isPlayer && !walkSpecialAllowedForNonPlayer(special) {
			return false
		}
		startSide := g.pointOnLineSide(prevX, prevY, ld)
		endSide := g.pointOnLineSide(curX, curY, ld)
		if want := runtimeDebugEnv("GD_DEBUG_WALK_SPECIAL_TIC"); want != "" {
			if want == fmt.Sprint(g.demoTick-1) || want == fmt.Sprint(g.worldTic) {
				fmt.Fprintf(os.Stderr, "walk-special-debug tic=%d world=%d line=%d start=%d end=%d prevSS=%d curSS=%d\n",
					g.demoTick-1, g.worldTic, ld.idx, startSide, endSide, prevSS, curSS)
			}
		}
		if startSide == endSide {
			return false
		}
		if debugLineTriggerEnabled(ld.idx) {
			fmt.Printf("line-trigger-debug tic=%d world=%d phase=walk-cross line=%d start=%d end=%d player=%t special=%d\n",
				g.demoTick-1, g.worldTic, ld.idx, startSide, endSide, isPlayer, special)
		}
		if want := runtimeDebugEnv("GD_DEBUG_WALK_SPECIAL_TIC"); want != "" {
			if want == fmt.Sprint(g.demoTick-1) || want == fmt.Sprint(g.worldTic) {
				fmt.Fprintf(os.Stderr, "walk-special-debug tic=%d world=%d line=%d candidate repeat=%t\n",
					g.demoTick-1, g.worldTic, ld.idx, info.Repeat)
			}
		}
		if info.Exit != mapdata.ExitNone {
			if isPlayer && g.handleExitSpecial(ld.idx, special, mapdata.TriggerWalk) {
				return true
			}
			return false
		}
		if info.Door != nil {
			if !isPlayer && !info.Door.CanActivate(mapdata.KeyRing{}) {
				return false
			}
			if isPlayer && !info.Door.CanActivate(g.inventory.keys()) {
				return false
			}
			if g.activateDoorLine(ld.idx, info, isPlayer) {
				if !info.Repeat && ld.idx >= 0 && ld.idx < len(g.lineSpecial) {
					g.lineSpecial[ld.idx] = 0
				}
				return true
			}
			return false
		}
		if g.activateNonDoorLineSpecial(ld.idx, startSide, info, actorIdx, isPlayer) {
			if debugLineTriggerEnabled(ld.idx) {
				fmt.Printf("line-trigger-debug tic=%d world=%d phase=walk-activate line=%d player=%t special=%d repeat=%t\n",
					g.demoTick-1, g.worldTic, ld.idx, isPlayer, special, info.Repeat)
			}
			if !info.Repeat && ld.idx >= 0 && ld.idx < len(g.lineSpecial) {
				g.lineSpecial[ld.idx] = 0
			}
			return true
		}
		return false
	}
	if len(candidateLineIdxs) != 0 {
		for _, lineIdx := range candidateLineIdxs {
			if lineIdx < 0 || lineIdx >= len(g.physForLine) {
				continue
			}
			physIdx := g.physForLine[lineIdx]
			if physIdx < 0 || physIdx >= len(g.lines) {
				continue
			}
			if visit(g.lines[physIdx]) {
				return
			}
		}
		return
	}
	for _, ld := range g.lines {
		if visit(ld) {
			return
		}
	}
}

func (g *game) lineTouchesSubsector(lineIdx, ss int) bool {
	if g == nil || g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) {
		return false
	}
	sub := g.m.SubSectors[ss]
	firstSeg := int(sub.FirstSeg)
	segCount := int(sub.SegCount)
	if firstSeg < 0 || segCount <= 0 || firstSeg+segCount > len(g.m.Segs) {
		return false
	}
	for i := 0; i < segCount; i++ {
		if int(g.m.Segs[firstSeg+i].Linedef) == lineIdx {
			return true
		}
	}
	return false
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
	return info.Trigger == mapdata.TriggerUse && lineSpecialSupported(info)
}

func (g *game) activateDoorLine(lineIdx int, info mapdata.LineSpecialInfo, isPlayer bool) bool {
	if info.Trigger == mapdata.TriggerManual {
		return g.evVerticalDoor(lineIdx, isPlayer)
	}
	return g.evDoDoorTagged(lineIdx, info)
}

func (g *game) evVerticalDoor(lineIdx int, isPlayer bool) bool {
	if lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
		return false
	}
	ld := g.m.Linedefs[lineIdx]
	if !isPlayer {
		switch ld.Special {
		case 26, 27, 28, 32, 33, 34:
			return false
		}
	}
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		return false
	}
	sec := targets[0]
	if sec < 0 || sec >= len(g.sectorCeil) {
		return false
	}

	if d := g.doors[sec]; d != nil {
		g.debugDoorActivate("line=%d sec=%d special=%d active dir=%d typ=%d player=%t", lineIdx, sec, ld.Special, d.direction, d.typ, isPlayer)
		if d.direction == 0 && d.topCountdown > 0 {
			d.traceTopCountdown = d.topCountdown
		}
		switch ld.Special {
		case 1, 26, 27, 28, 117:
			if d.direction == -1 {
				d.direction = 1
			} else {
				if !isPlayer {
					return true
				}
				d.direction = -1
			}
			g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
			g.debugDoorActivate("line=%d sec=%d special=%d retoggle dir=%d typ=%d player=%t", lineIdx, sec, ld.Special, d.direction, d.typ, isPlayer)
			return true
		}
	}

	d := g.allocDoorThinker(sec)
	d.direction = 1
	d.speed = vDoorSpeed
	d.topWait = vDoorWaitTic
	d.topHeight = g.lowestSurroundingCeiling(sec) - 4*fracUnit
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
	g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
	g.debugDoorActivate("line=%d sec=%d special=%d spawn dir=%d typ=%d player=%t", lineIdx, sec, ld.Special, d.direction, d.typ, isPlayer)
	return true
}

func (g *game) evDoDoorTagged(lineIdx int, info mapdata.LineSpecialInfo) bool {
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil {
		return false
	}
	return g.activateDoorSectors(targets, info.Door.Action)
}

func (g *game) activateTaggedDoor(tag uint16, action mapdata.DoorAction) bool {
	return g.activateDoorSectors(g.taggedSectorsForTag(tag), action)
}

func (g *game) activateDoorSectors(targets []int, action mapdata.DoorAction) bool {
	if len(targets) == 0 {
		return false
	}
	if g.doors == nil {
		g.doors = make(map[int]*doorThinker)
	}
	activated := false
	for _, sec := range targets {
		if sec < 0 || sec >= len(g.sectorCeil) {
			continue
		}
		if g.doors[sec] != nil {
			g.debugDoorActivate("tagged sec=%d action=%v already-active", sec, action)
			continue
		}
		d := g.allocDoorThinker(sec)
		d.topWait = vDoorWaitTic
		d.speed = vDoorSpeed
		d.topHeight = g.lowestSurroundingCeiling(sec) - 4*fracUnit
		if d.topHeight < g.sectorFloor[sec] {
			d.topHeight = g.sectorFloor[sec]
		}
		switch action {
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
		g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
		g.debugDoorActivate("tagged sec=%d action=%v spawn dir=%d typ=%d", sec, action, d.direction, d.typ)
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

var doomSwitchTexturePairs = map[string]string{
	"SW1BRCOM": "SW2BRCOM",
	"SW2BRCOM": "SW1BRCOM",
	"SW1BRN1":  "SW2BRN1",
	"SW2BRN1":  "SW1BRN1",
	"SW1BRN2":  "SW2BRN2",
	"SW2BRN2":  "SW1BRN2",
	"SW1BRNGN": "SW2BRNGN",
	"SW2BRNGN": "SW1BRNGN",
	"SW1BROWN": "SW2BROWN",
	"SW2BROWN": "SW1BROWN",
	"SW1COMM":  "SW2COMM",
	"SW2COMM":  "SW1COMM",
	"SW1COMP":  "SW2COMP",
	"SW2COMP":  "SW1COMP",
	"SW1DIRT":  "SW2DIRT",
	"SW2DIRT":  "SW1DIRT",
	"SW1EXIT":  "SW2EXIT",
	"SW2EXIT":  "SW1EXIT",
	"SW1GRAY":  "SW2GRAY",
	"SW2GRAY":  "SW1GRAY",
	"SW1GRAY1": "SW2GRAY1",
	"SW2GRAY1": "SW1GRAY1",
	"SW1METAL": "SW2METAL",
	"SW2METAL": "SW1METAL",
	"SW1PIPE":  "SW2PIPE",
	"SW2PIPE":  "SW1PIPE",
	"SW1SLAD":  "SW2SLAD",
	"SW2SLAD":  "SW1SLAD",
	"SW1STARG": "SW2STARG",
	"SW2STARG": "SW1STARG",
	"SW1STON1": "SW2STON1",
	"SW2STON1": "SW1STON1",
	"SW1STON2": "SW2STON2",
	"SW2STON2": "SW1STON2",
	"SW1STONE": "SW2STONE",
	"SW2STONE": "SW1STONE",
	"SW1STRTN": "SW2STRTN",
	"SW2STRTN": "SW1STRTN",
	"SW1BLUE":  "SW2BLUE",
	"SW2BLUE":  "SW1BLUE",
	"SW1CMT":   "SW2CMT",
	"SW2CMT":   "SW1CMT",
	"SW1GARG":  "SW2GARG",
	"SW2GARG":  "SW1GARG",
	"SW1GSTON": "SW2GSTON",
	"SW2GSTON": "SW1GSTON",
	"SW1HOT":   "SW2HOT",
	"SW2HOT":   "SW1HOT",
	"SW1LION":  "SW2LION",
	"SW2LION":  "SW1LION",
	"SW1SATYR": "SW2SATYR",
	"SW2SATYR": "SW1SATYR",
	"SW1SKIN":  "SW2SKIN",
	"SW2SKIN":  "SW1SKIN",
	"SW1VINE":  "SW2VINE",
	"SW2VINE":  "SW1VINE",
	"SW1WOOD":  "SW2WOOD",
	"SW2WOOD":  "SW1WOOD",
	"SW1PANEL": "SW2PANEL",
	"SW2PANEL": "SW1PANEL",
	"SW1ROCK":  "SW2ROCK",
	"SW2ROCK":  "SW1ROCK",
	"SW1MET2":  "SW2MET2",
	"SW2MET2":  "SW1MET2",
	"SW1WDMET": "SW2WDMET",
	"SW2WDMET": "SW1WDMET",
	"SW1BRIK":  "SW2BRIK",
	"SW2BRIK":  "SW1BRIK",
	"SW1MOD1":  "SW2MOD1",
	"SW2MOD1":  "SW1MOD1",
	"SW1ZIM":   "SW2ZIM",
	"SW2ZIM":   "SW1ZIM",
	"SW1STON6": "SW2STON6",
	"SW2STON6": "SW1STON6",
	"SW1TEK":   "SW2TEK",
	"SW2TEK":   "SW1TEK",
	"SW1MARB":  "SW2MARB",
	"SW2MARB":  "SW1MARB",
	"SW1SKULL": "SW2SKULL",
	"SW2SKULL": "SW1SKULL",
}

func toggleSwitchTexture(name string) (string, bool) {
	base := strings.TrimSpace(name)
	if base == "" {
		return name, false
	}
	if next, ok := doomSwitchTexturePairs[strings.ToUpper(base)]; ok {
		return next, true
	}
	return name, false
}

func (g *game) animateSwitchTexture(lineIdx, side int, repeat bool) {
	if lineIdx < 0 || lineIdx >= len(g.m.Linedefs) {
		return
	}
	ld := g.m.Linedefs[lineIdx]
	sideDefIdx := int(ld.SideNum[0])
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
		if g.delayedSwitchReverts[i].line != lineIdx {
			continue
		}
		g.delayedSwitchReverts[i].sidedef = sideDefIdx
		g.delayedSwitchReverts[i].top = origTop
		g.delayedSwitchReverts[i].bottom = origBottom
		g.delayedSwitchReverts[i].mid = origMid
		g.delayedSwitchReverts[i].tics = switchResetTics
		return
	}
	g.delayedSwitchReverts = append(g.delayedSwitchReverts, delayedSwitchTexture{
		line:    lineIdx,
		sidedef: sideDefIdx,
		top:     origTop,
		bottom:  origBottom,
		mid:     origMid,
		tics:    switchResetTics,
	})
}
