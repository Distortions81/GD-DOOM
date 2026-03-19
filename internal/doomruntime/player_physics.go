package doomruntime

import (
	"fmt"
	"os"
	"slices"
)

type slideIntercept struct {
	frac int64
	line int
}

const doomMaxThingRadius = 32 * fracUnit

func (g *game) updatePlayer(cmd moveCmd) {
	if g.isDead {
		return
	}

	if cmd.turnRaw != 0 {
		g.p.angle += uint32(cmd.turnRaw)
	}
	if cmd.turn != 0 {
		g.turnHeld++
		turn := angleTurn[0]
		if g.turnHeld < slowTurnTics {
			turn = angleTurn[2]
		} else if cmd.run {
			turn = angleTurn[1]
		}
		turnSpeed := g.opts.KeyboardTurnSpeed
		if turnSpeed == 0 {
			turnSpeed = 1
		}
		turn = uint32(float64(turn) * turnSpeed)
		if turn == 0 {
			turn = 1
		}
		if cmd.turn < 0 {
			g.p.angle -= turn
		} else {
			g.p.angle += turn
		}
	} else {
		g.turnHeld = 0
	}

	onground := g.p.z <= g.p.floorz
	if cmd.forward != 0 && onground && g.p.reactionTime == 0 {
		g.thrust(g.p.angle, cmd.forward*2048)
	}
	if cmd.side != 0 && onground && g.p.reactionTime == 0 {
		g.thrust(g.p.angle-0x40000000, cmd.side*2048)
	}
}

func (g *game) tickPlayerBody() {
	prevX := g.p.x
	prevY := g.p.y
	if g.isDead {
		g.p.momx = 0
		g.p.momy = 0
		g.p.momz = 0
		return
	}
	g.xyMovement()
	g.zMovement()
	g.checkWalkSpecialLines(prevX, prevY, g.p.x, g.p.y)
}

func (g *game) tickGameplayWorld() {
	g.tickThinkers()
	g.tickWorldLogic()
}

func (g *game) tickThinkers() {
	g.tickPlayerBody()
	g.tickFloors()
	g.tickPlats()
	g.tickCeilings()
	g.tickDoors()
	if !g.isDead {
		g.processThingPickups()
	}
	g.tickBossBrainSpecials()
	g.tickProjectiles()
	g.tickProjectileImpacts()
	g.tickMonsters()
	g.tickHitscanPuffs()
}

func (g *game) runGameplayTic(cmd moveCmd, usePressed, fireHeld bool) {
	g.currentMoveCmd = cmd
	g.setAttackHeld(fireHeld)
	g.updatePlayer(cmd)
	g.tickPlayerViewHeight()
	g.tickPlayerSpecialSector()
	if usePressed {
		g.handleUse()
	}
	g.tickWeaponFire()
	g.tickPlayerCounters()
	g.tickGameplayWorld()
}

func (g *game) tickPlayerSpecialSector() {
	if g == nil {
		return
	}
	g.trackSecrets()
	if want := os.Getenv("GD_DEBUG_WORLD_RNG_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				sec := g.sectorAt(g.p.x, g.p.y)
				special := int16(-1)
				if g.m != nil && sec >= 0 && sec < len(g.m.Sectors) {
					special = g.m.Sectors[sec].Special
				}
				fmt.Printf("world-player-debug tic=%d world=%d sec=%d special=%d suit=%d health=%d\n",
					g.demoTick-1, g.worldTic, sec, special, g.inventory.RadSuitTics, g.stats.Health)
			}
		}
	}
	g.applySectorHazardDamage()
}

func (g *game) tickPlayerCounters() {
	if g == nil {
		return
	}
	if g.playerPainStateTics > 0 {
		g.playerPainStateTics--
		if g.playerPainStateTics == 0 {
			switch g.playerPainStatePhase {
			case 1:
				g.playerPainStatePhase = 2
				g.playerPainStateTics = 4
			default:
				g.playerPainStatePhase = 0
			}
		}
	}
	if g.p.reactionTime > 0 {
		g.p.reactionTime--
	}
	if g.inventory.InvulnTics > 0 {
		g.inventory.InvulnTics--
	}
	if g.inventory.InvisTics > 0 {
		g.inventory.InvisTics--
	}
	if g.inventory.RadSuitTics > 0 {
		g.inventory.RadSuitTics--
	}
	if g.inventory.LightAmpTics > 0 {
		g.inventory.LightAmpTics--
	}
}

func (g *game) tickDoors() {
	setDoorCeiling := func(sec int, z int64) {
		if sec < 0 || sec >= len(g.sectorCeil) {
			return
		}
		if g.sectorCeil[sec] == z {
			return
		}
		g.sectorCeil[sec] = z
		g.markDynamicSectorPlaneCacheDirty(sec)
		if sec >= 0 && sec < len(g.m.Sectors) {
			g.m.Sectors[sec].CeilingHeight = int16(z >> fracBits)
		}
	}
	doorWouldCrushPlayer := func(sec int, nextCeil int64) bool {
		if g == nil || sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
			return false
		}
		oldCeil := g.sectorCeil[sec]
		if nextCeil >= oldCeil {
			return false
		}
		oldMapCeil := int16(0)
		if g.m != nil && sec < len(g.m.Sectors) {
			oldMapCeil = g.m.Sectors[sec].CeilingHeight
			g.m.Sectors[sec].CeilingHeight = int16(nextCeil >> fracBits)
		}
		g.sectorCeil[sec] = nextCeil
		tmfloor, tmceil, _, ok := g.checkPositionFor(g.p.x, g.p.y, false)
		g.sectorCeil[sec] = oldCeil
		if g.m != nil && sec < len(g.m.Sectors) {
			g.m.Sectors[sec].CeilingHeight = oldMapCeil
		}
		if !ok {
			return true
		}
		if tmceil-tmfloor < playerHeight {
			return true
		}
		playerTop := g.p.z + playerHeight
		return tmceil-g.p.z < playerHeight || tmceil < playerTop
	}
	for sec, d := range g.doors {
		switch d.direction {
		case 0:
			d.topCountdown--
			if d.topCountdown <= 0 {
				switch d.typ {
				case doorBlazeRaise, doorNormal:
					d.direction = -1
					g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
				case doorClose30ThenOpen:
					d.direction = 1
					g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
				}
			}
		case 2:
			d.topCountdown--
			if d.topCountdown <= 0 && d.typ == doorRaiseIn5Mins {
				d.direction = 1
				d.typ = doorNormal
				g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
			}
		case -1:
			next := g.sectorCeil[sec] - d.speed
			if doorWouldCrushPlayer(sec, next) {
				switch d.typ {
				case doorBlazeClose, doorClose:
					// Vanilla close-only doors keep trying to close, but do not
					// advance through blocking actors.
				default:
					d.direction = 1
					g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
				}
				continue
			}
			if next <= g.sectorFloor[sec] {
				setDoorCeiling(sec, g.sectorFloor[sec])
				switch d.typ {
				case doorBlazeRaise, doorBlazeClose, doorNormal, doorClose:
					delete(g.doors, sec)
				case doorClose30ThenOpen:
					d.direction = 0
					d.topCountdown = 35 * 30
				}
			} else {
				setDoorCeiling(sec, next)
			}
		case 1:
			next := g.sectorCeil[sec] + d.speed
			if next >= d.topHeight {
				setDoorCeiling(sec, d.topHeight)
				switch d.typ {
				case doorBlazeRaise, doorNormal:
					d.direction = 0
					d.topCountdown = d.topWait
				case doorClose30ThenOpen, doorBlazeOpen, doorOpen:
					delete(g.doors, sec)
				}
			} else {
				setDoorCeiling(sec, next)
			}
		}
	}
}

func (g *game) thrust(angle uint32, move int64) {
	g.p.momx += fixedMul(move, doomFineCosine(angle))
	g.p.momy += fixedMul(move, doomFineSineAtAngle(angle))
}

func (g *game) xyMovement() {
	if g.p.momx == 0 && g.p.momy == 0 {
		return
	}
	g.p.momx = clamp(g.p.momx, -maxMove, maxMove)
	g.p.momy = clamp(g.p.momy, -maxMove, maxMove)
	g.debugPlayerMove(fmt.Sprintf("xy start mom=(%d,%d)", g.p.momx, g.p.momy), g.p.x, g.p.y)

	xmove := g.p.momx
	ymove := g.p.momy
	for {
		var ptryx, ptryy int64
		if xmove > maxMove/2 || ymove > maxMove/2 {
			ptryx = g.p.x + (xmove / 2)
			ptryy = g.p.y + (ymove / 2)
			g.debugPlayerMove(fmt.Sprintf("xy split step=(%d,%d) remain_before=(%d,%d)", ptryx, ptryy, xmove, ymove), ptryx, ptryy)
			xmove >>= 1
			ymove >>= 1
		} else {
			ptryx = g.p.x + xmove
			ptryy = g.p.y + ymove
			g.debugPlayerMove(fmt.Sprintf("xy final step=(%d,%d) remain=(%d,%d)", ptryx, ptryy, xmove, ymove), ptryx, ptryy)
			xmove = 0
			ymove = 0
		}
		if !g.tryMove(ptryx, ptryy) {
			g.slideMove()
		}
		if xmove == 0 && ymove == 0 {
			break
		}
	}

	if g.p.z > g.p.floorz {
		return
	}

	if g.p.momx > -stopSpeed && g.p.momx < stopSpeed &&
		g.p.momy > -stopSpeed && g.p.momy < stopSpeed &&
		g.currentMoveCmd.forward == 0 && g.currentMoveCmd.side == 0 {
		g.p.momx = 0
		g.p.momy = 0
	} else {
		g.p.momx = fixedMul(g.p.momx, friction)
		g.p.momy = fixedMul(g.p.momy, friction)
	}
	g.debugPlayerMove(fmt.Sprintf("xy end pos=(%d,%d) mom=(%d,%d)", g.p.x, g.p.y, g.p.momx, g.p.momy), g.p.x, g.p.y)
}

func (g *game) tryMove(x, y int64) bool {
	tmfloor, tmceil, tmdrop, ok := g.checkPositionFor(x, y, false)
	if !ok {
		g.debugPlayerMove("tryMove blocked", x, y)
		return false
	}
	if tmceil-tmfloor < playerHeight {
		g.debugPlayerMove("tryMove low ceiling", x, y)
		return false
	}
	if tmceil-g.p.z < playerHeight {
		g.debugPlayerMove("tryMove ceiling clip", x, y)
		return false
	}
	if tmfloor-g.p.z > stepHeight {
		g.debugPlayerMove("tryMove high step", x, y)
		return false
	}
	_ = tmdrop

	g.p.floorz = tmfloor
	g.p.ceilz = tmceil
	g.p.x = x
	g.p.y = y
	return true
}

func (g *game) zMovement() {
	if g == nil {
		return
	}
	if g.p.viewHeight == 0 {
		g.p.viewHeight = playerViewHeight
	}

	if g.p.z < g.p.floorz {
		g.p.viewHeight -= g.p.floorz - g.p.z
		g.p.deltaViewHeight = (playerViewHeight - g.p.viewHeight) >> 3
	}

	g.p.z += g.p.momz

	if g.p.z <= g.p.floorz {
		if g.p.momz < 0 {
			if g.p.momz < -playerGravity*8 {
				g.p.deltaViewHeight = g.p.momz >> 3
				g.emitSoundEvent(soundEventOof)
			}
			g.p.momz = 0
		}
		g.p.z = g.p.floorz
	} else {
		if g.p.momz == 0 {
			g.p.momz = -playerGravity * 2
		} else {
			g.p.momz -= playerGravity
		}
	}

	if g.p.z+playerHeight > g.p.ceilz {
		if g.p.momz > 0 {
			g.p.momz = 0
		}
		g.p.z = g.p.ceilz - playerHeight
		if g.p.z < g.p.floorz {
			g.p.z = g.p.floorz
		}
	}
}

func (g *game) checkPosition(x, y int64) (int64, int64, int64, bool) {
	return g.checkPositionFor(x, y, false)
}

func (g *game) checkPositionFor(x, y int64, blockMonsterLines bool) (int64, int64, int64, bool) {
	return g.checkPositionForActor(x, y, playerRadius, blockMonsterLines, -1, false)
}

func (g *game) checkPositionForActor(x, y, radius int64, blockMonsterLines bool, moverThingIdx int, moverIsMonster bool) (int64, int64, int64, bool) {
	tmboxTop := y + radius
	tmboxBottom := y - radius
	tmboxRight := x + radius
	tmboxLeft := x - radius
	probeEnabled := g.debugPlayerProbeActive()

	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		if probeEnabled {
			g.debugPlayerProbe(fmt.Sprintf("sector invalid sec=%d bbox=[t=%d b=%d r=%d l=%d]", sec, tmboxTop, tmboxBottom, tmboxRight, tmboxLeft), x, y)
		}
		return 0, 0, 0, false
	}
	tmfloor := g.sectorFloor[sec]
	tmceil := g.sectorCeil[sec]
	tmdrop := tmfloor
	if probeEnabled {
		g.debugPlayerProbe(fmt.Sprintf("start sec=%d floor=%d ceil=%d bbox=[t=%d b=%d r=%d l=%d]", sec, tmfloor, tmceil, tmboxTop, tmboxBottom, tmboxRight, tmboxLeft), x, y)
	}

	if g.actorBlockedByThings(x, y, radius, moverThingIdx, moverIsMonster) {
		g.debugPlayerProbe("blocked by thing", x, y)
		return 0, 0, 0, false
	}

	g.validCount++

	xl := int((tmboxLeft - g.bmapOriginX) >> (fracBits + 7))
	xh := int((tmboxRight - g.bmapOriginX) >> (fracBits + 7))
	yl := int((tmboxBottom - g.bmapOriginY) >> (fracBits + 7))
	yh := int((tmboxTop - g.bmapOriginY) >> (fracBits + 7))

	processPhysLine := func(physIdx int) bool {
		if physIdx < 0 || physIdx >= len(g.lines) {
			return true
		}
		if physIdx >= len(g.lineValid) {
			g.lineValid = append(g.lineValid, make([]int, physIdx-len(g.lineValid)+1)...)
		}
		if g.lineValid[physIdx] == g.validCount {
			return true
		}
		g.lineValid[physIdx] = g.validCount
		ld := g.lines[physIdx]
		if tmboxRight <= ld.bbox[3] || tmboxLeft >= ld.bbox[2] || tmboxTop <= ld.bbox[1] || tmboxBottom >= ld.bbox[0] {
			return true
		}
		box := [4]int64{tmboxTop, tmboxBottom, tmboxRight, tmboxLeft}
		if g.boxOnLineSide(box, ld) != -1 {
			return true
		}
		frontSec := -1
		backSec := -1
		if ld.sideNum0 >= 0 && int(ld.sideNum0) < len(g.m.Sidedefs) {
			frontSec = int(g.m.Sidedefs[int(ld.sideNum0)].Sector)
		}
		if ld.sideNum1 >= 0 && int(ld.sideNum1) < len(g.m.Sidedefs) {
			backSec = int(g.m.Sidedefs[int(ld.sideNum1)].Sector)
		}
		if probeEnabled {
			g.debugPlayerProbe(fmt.Sprintf("touch line=%d flags=0x%04x front=%d back=%d bbox=[%d %d %d %d]", ld.idx, ld.flags, frontSec, backSec, ld.bbox[0], ld.bbox[1], ld.bbox[2], ld.bbox[3]), x, y)
		}

		if ld.sideNum1 < 0 {
			if probeEnabled {
				g.debugPlayerProbe(fmt.Sprintf("block line=%d reason=onesided floor=%d ceil=%d drop=%d", ld.idx, tmfloor, tmceil, tmdrop), x, y)
			}
			return false
		}
		if (ld.flags & mlBlocking) != 0 {
			if probeEnabled {
				g.debugPlayerProbe(fmt.Sprintf("block line=%d reason=blocking floor=%d ceil=%d drop=%d", ld.idx, tmfloor, tmceil, tmdrop), x, y)
			}
			return false
		}
		if blockMonsterLines && (ld.flags&mlBlockMonsters) != 0 {
			if probeEnabled {
				g.debugPlayerProbe(fmt.Sprintf("block line=%d reason=blockmonsters floor=%d ceil=%d drop=%d", ld.idx, tmfloor, tmceil, tmdrop), x, y)
			}
			return false
		}

		opentop, openbottom, lowfloor, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			if probeEnabled {
				g.debugPlayerProbe(fmt.Sprintf("block line=%d reason=openrange floor=%d ceil=%d drop=%d openbottom=%d opentop=%d", ld.idx, tmfloor, tmceil, tmdrop, openbottom, opentop), x, y)
			}
			return false
		}
		if probeEnabled {
			g.debugPlayerProbe(fmt.Sprintf("open line=%d openbottom=%d opentop=%d openrange=%d lowfloor=%d", ld.idx, openbottom, opentop, openrange, lowfloor), x, y)
		}
		if opentop < tmceil {
			tmceil = opentop
		}
		if openbottom > tmfloor {
			tmfloor = openbottom
		}
		if lowfloor < tmdrop {
			tmdrop = lowfloor
		}
		return true
	}
	iter := func(lineIdx int) bool {
		if lineIdx < 0 || lineIdx >= len(g.physForLine) {
			return true
		}
		return processPhysLine(g.physForLine[lineIdx])
	}

	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		for bx := xl; bx <= xh; bx++ {
			for by := yl; by <= yh; by++ {
				if probeEnabled {
					g.debugPlayerProbe(fmt.Sprintf("scan block bx=%d by=%d", bx, by), x, y)
				}
				if !g.blockLinesIterator(bx, by, iter) {
					return 0, 0, 0, false
				}
			}
		}
	} else {
		for i := range g.lines {
			if !processPhysLine(i) {
				return 0, 0, 0, false
			}
		}
	}
	if probeEnabled {
		g.debugPlayerProbe(fmt.Sprintf("ok floor=%d ceil=%d drop=%d", tmfloor, tmceil, tmdrop), x, y)
	}
	return tmfloor, tmceil, tmdrop, true
}

func (g *game) debugPlayerProbeActive() bool {
	if g == nil || !g.debugPlayerProbeEnabled {
		return false
	}
	return g.demoTick-1 == g.debugPlayerProbeTic || g.worldTic == g.debugPlayerProbeTic
}

func (g *game) debugPlayerProbe(msg string, x, y int64) {
	if !g.debugPlayerProbeActive() {
		return
	}
	fmt.Printf("player-probe-debug tic=%d world=%d msg=%s pos=(%d,%d) player=(%d,%d) mom=(%d,%d)\n",
		g.demoTick-1, g.worldTic, msg, x, y, g.p.x, g.p.y, g.p.momx, g.p.momy)
}

func actorsOverlapXY(ax, ay, aradius, bx, by, bradius int64) bool {
	blockdist := aradius + bradius
	return abs(ax-bx) < blockdist && abs(ay-by) < blockdist
}

func (g *game) actorBlockedByThings(x, y, radius int64, moverThingIdx int, moverIsMonster bool) bool {
	if g == nil || g.m == nil {
		return false
	}
	probeEnabled := g.debugPlayerProbeActive()
	if moverIsMonster && !g.isDead && actorsOverlapXY(x, y, radius, g.p.x, g.p.y, playerRadius) {
		if probeEnabled {
			g.debugPlayerProbe(fmt.Sprintf("block thing=player type=player pos=(%d,%d) radius=%d", g.p.x, g.p.y, playerRadius), x, y)
		}
		return true
	}
	visitThing := func(i int) bool {
		if i < 0 || i >= len(g.m.Things) {
			return false
		}
		th := g.m.Things[i]
		if i == moverThingIdx {
			return false
		}
		if !g.thingBlocksInSession(i) {
			return false
		}
		tx, ty := g.thingPosFixed(i, th)
		if isMonster(th.Type) {
			if i < 0 || i >= len(g.thingHP) || g.thingHP[i] <= 0 {
				return false
			}
			if actorsOverlapXY(x, y, radius, tx, ty, monsterRadius(th.Type)) {
				if probeEnabled {
					g.debugPlayerProbe(fmt.Sprintf("block thing=%d type=%d pos=(%d,%d) radius=%d kind=monster", i, th.Type, tx, ty, monsterRadius(th.Type)), x, y)
				}
				return true
			}
		}
		if !thingTypeBlocksActorMovement(th.Type, moverIsMonster) {
			return false
		}
		tx, ty = g.thingPosFixed(i, th)
		if actorsOverlapXY(x, y, radius, tx, ty, thingTypeRadius(th.Type)) {
			if probeEnabled {
				g.debugPlayerProbe(fmt.Sprintf("block thing=%d type=%d pos=(%d,%d) radius=%d kind=solid", i, th.Type, tx, ty, thingTypeRadius(th.Type)), x, y)
			}
			return true
		}
		return false
	}

	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		left := int((x - radius - g.bmapOriginX - doomMaxThingRadius) >> (fracBits + 7))
		right := int((x + radius - g.bmapOriginX + doomMaxThingRadius) >> (fracBits + 7))
		bottom := int((y - radius - g.bmapOriginY - doomMaxThingRadius) >> (fracBits + 7))
		top := int((y + radius - g.bmapOriginY + doomMaxThingRadius) >> (fracBits + 7))
		if len(g.thingBlockLinks) != g.bmapWidth*g.bmapHeight {
			g.rebuildThingBlockmap()
		}
		for by := bottom; by <= top; by++ {
			for bx := left; bx <= right; bx++ {
				if bx < 0 || by < 0 || bx >= g.bmapWidth || by >= g.bmapHeight {
					continue
				}
				cell := by*g.bmapWidth + bx
				for i := g.thingBlockLinks[cell]; i >= 0; i = g.thingBlockNext[i] {
					if visitThing(i) {
						return true
					}
				}
			}
		}
		return false
	}

	for i := range g.m.Things {
		if visitThing(i) {
			return true
		}
	}
	return false
}

var doomSolidMapThingTypes = map[int16]struct{}{
	25:   {},
	26:   {},
	27:   {},
	28:   {},
	29:   {},
	30:   {},
	31:   {},
	32:   {},
	33:   {},
	35:   {},
	36:   {},
	37:   {},
	41:   {},
	42:   {},
	44:   {},
	45:   {},
	46:   {},
	47:   {},
	48:   {},
	49:   {},
	50:   {},
	51:   {},
	52:   {},
	53:   {},
	54:   {},
	55:   {},
	56:   {},
	57:   {},
	70:   {},
	73:   {},
	74:   {},
	75:   {},
	76:   {},
	77:   {},
	78:   {},
	85:   {},
	86:   {},
	88:   {},
	2028: {},
	2035: {},
}

func thingTypeBlocksActorMovement(typ int16, moverIsMonster bool) bool {
	if _, ok := doomSolidMapThingTypes[typ]; ok {
		return true
	}
	_ = moverIsMonster
	return false
}

func thingTypeRadius(typ int16) int64 {
	if info, ok := demoTraceThingInfoForType(typ); ok && info.radius > 0 {
		return info.radius
	}
	return 20 * fracUnit
}

func (g *game) blockLinesIterator(x, y int, fn func(int) bool) bool {
	if x < 0 || y < 0 || x >= g.bmapWidth || y >= g.bmapHeight {
		return true
	}
	idx := y*g.bmapWidth + x
	if idx < 0 || idx >= len(g.m.BlockMap.Cells) {
		return true
	}
	cell := g.m.BlockMap.Cells[idx]
	start := 0
	// Doom blocklists carry a leading 0 sentinel before linedef numbers.
	if len(cell) > 0 && cell[0] == 0 {
		start = 1
	}
	for _, lineWord := range cell[start:] {
		if !fn(int(lineWord)) {
			return false
		}
	}
	return true
}

func (g *game) lineOpening(ld physLine) (int64, int64, int64, int64) {
	if ld.sideNum1 < 0 || int(ld.sideNum0) >= len(g.m.Sidedefs) || int(ld.sideNum1) >= len(g.m.Sidedefs) {
		return 0, 0, 0, 0
	}
	fidx := g.m.Sidedefs[int(ld.sideNum0)].Sector
	bidx := g.m.Sidedefs[int(ld.sideNum1)].Sector
	if int(fidx) >= len(g.m.Sectors) || int(bidx) >= len(g.m.Sectors) {
		return 0, 0, 0, 0
	}
	frontCeil := g.sectorCeil[fidx]
	backCeil := g.sectorCeil[bidx]
	frontFloor := g.sectorFloor[fidx]
	backFloor := g.sectorFloor[bidx]
	opentop := min64(frontCeil, backCeil)
	openbottom := max64(frontFloor, backFloor)
	lowfloor := min64(frontFloor, backFloor)
	return opentop, openbottom, lowfloor, opentop - openbottom
}

func (g *game) boxOnLineSide(box [4]int64, ld physLine) int {
	var p1, p2 int
	switch ld.slope {
	case slopeHorizontal:
		p1 = b2i(box[0] > ld.y1)
		p2 = b2i(box[1] > ld.y1)
		if ld.dx < 0 {
			p1 ^= 1
			p2 ^= 1
		}
	case slopeVertical:
		p1 = b2i(box[2] < ld.x1)
		p2 = b2i(box[3] < ld.x1)
		if ld.dy < 0 {
			p1 ^= 1
			p2 ^= 1
		}
	case slopePositive:
		p1 = g.pointOnLineSide(box[3], box[0], ld)
		p2 = g.pointOnLineSide(box[2], box[1], ld)
	default:
		p1 = g.pointOnLineSide(box[2], box[0], ld)
		p2 = g.pointOnLineSide(box[3], box[1], ld)
	}
	if p1 == p2 {
		return p1
	}
	return -1
}

func (g *game) pointOnLineSide(x, y int64, line physLine) int {
	return pointOnDivlineSide(x, y, divline{x: line.x1, y: line.y1, dx: line.dx, dy: line.dy})
}

func (g *game) slideMove() {
	hitcount := 0
	for {
		hitcount++
		if hitcount == 3 {
			if !g.tryMove(g.p.x, g.p.y+g.p.momy) {
				_ = g.tryMove(g.p.x+g.p.momx, g.p.y)
			}
			return
		}

		var leadx, leady, trailx, traily int64
		if g.p.momx > 0 {
			leadx = g.p.x + playerRadius
			trailx = g.p.x - playerRadius
		} else {
			leadx = g.p.x - playerRadius
			trailx = g.p.x + playerRadius
		}
		if g.p.momy > 0 {
			leady = g.p.y + playerRadius
			traily = g.p.y - playerRadius
		} else {
			leady = g.p.y - playerRadius
			traily = g.p.y + playerRadius
		}

		bestFrac := int64(fracUnit + 1)
		bestLine := -1
		for _, tc := range [][4]int64{{leadx, leady, leadx + g.p.momx, leady + g.p.momy}, {trailx, leady, trailx + g.p.momx, leady + g.p.momy}, {leadx, traily, leadx + g.p.momx, traily + g.p.momy}} {
			f, li, ok := g.firstBlockingIntercept(tc[0], tc[1], tc[2], tc[3])
			if ok && f < bestFrac {
				bestFrac = f
				bestLine = li
			}
		}

		if bestLine < 0 {
			g.debugPlayerMove("slideMove no best line", g.p.x+g.p.momx, g.p.y+g.p.momy)
			if !g.tryMove(g.p.x, g.p.y+g.p.momy) {
				_ = g.tryMove(g.p.x+g.p.momx, g.p.y)
			}
			return
		}
		g.debugPlayerMove(fmt.Sprintf("slideMove bestLine=%d bestFrac=%d", bestLine, bestFrac), g.p.x, g.p.y)

		bestFracFixed := bestFrac
		bestFracFixed -= 0x800
		if bestFracFixed > 0 {
			newx := fixedMul(g.p.momx, bestFracFixed)
			newy := fixedMul(g.p.momy, bestFracFixed)
			if !g.tryMove(g.p.x+newx, g.p.y+newy) {
				if !g.tryMove(g.p.x, g.p.y+g.p.momy) {
					_ = g.tryMove(g.p.x+g.p.momx, g.p.y)
				}
				return
			}
		}

		restFixed := fracUnit - (bestFracFixed + 0x800)
		if restFixed <= 0 {
			return
		}
		if restFixed > fracUnit {
			restFixed = fracUnit
		}
		tmxmove := fixedMul(g.p.momx, restFixed)
		tmymove := fixedMul(g.p.momy, restFixed)
		tmxmove, tmymove = g.hitSlideLine(g.lines[bestLine], tmxmove, tmymove)
		g.p.momx = tmxmove
		g.p.momy = tmymove
		if g.tryMove(g.p.x+tmxmove, g.p.y+tmymove) {
			return
		}
	}
}

func (g *game) debugPlayerMove(msg string, x, y int64) {
	if g == nil || os.Getenv("GD_DEBUG_PLAYER_MOVE_TIC") == "" {
		return
	}
	var want int
	if _, err := fmt.Sscanf(os.Getenv("GD_DEBUG_PLAYER_MOVE_TIC"), "%d", &want); err != nil {
		return
	}
	if g.demoTick-1 != want {
		return
	}
	fmt.Printf("player-move-debug tic=%d world=%d msg=%s from=(%d,%d) to=(%d,%d) angle=%d mom=(%d,%d)\n",
		g.demoTick-1, g.worldTic, msg, g.p.x, g.p.y, x, y, g.p.angle, g.p.momx, g.p.momy)
}

func (g *game) firstBlockingIntercept(x1, y1, x2, y2 int64) (int64, int, bool) {
	intercepts := make([]slideIntercept, 0, 16)
	trace := divline{x: x1, y: y1, dx: x2 - x1, dy: y2 - y1}
	for i, ld := range g.lines {
		lineDL := divline{x: ld.x1, y: ld.y1, dx: ld.dx, dy: ld.dy}
		if doomPointOnDivlineSide(ld.x1, ld.y1, trace) == doomPointOnDivlineSide(ld.x2, ld.y2, trace) {
			continue
		}
		if doomPointOnDivlineSide(x1, y1, lineDL) == doomPointOnDivlineSide(x2, y2, lineDL) {
			continue
		}
		frac := interceptVector(trace, lineDL)
		if frac < 0 || frac > fracUnit {
			continue
		}
		intercepts = append(intercepts, slideIntercept{frac: frac, line: i})
	}
	slices.SortFunc(intercepts, func(a, b slideIntercept) int {
		if a.frac < b.frac {
			return -1
		}
		if a.frac > b.frac {
			return 1
		}
		return 0
	})

	for _, it := range intercepts {
		ld := g.lines[it.line]
		if (ld.flags&mlTwoSided) == 0 || ld.sideNum1 < 0 {
			if g.pointOnLineSide(g.p.x, g.p.y, ld) == 1 {
				continue
			}
			return it.frac, it.line, true
		}
		opentop, openbottom, _, openrange := g.lineOpening(ld)
		if openrange < playerHeight {
			return it.frac, it.line, true
		}
		if opentop-g.p.z < playerHeight {
			return it.frac, it.line, true
		}
		if openbottom-g.p.z > stepHeight {
			return it.frac, it.line, true
		}
	}
	return 0, 0, false
}

func (g *game) hitSlideLine(ld physLine, tmxmove, tmymove int64) (int64, int64) {
	if ld.slope == slopeHorizontal {
		return tmxmove, 0
	}
	if ld.slope == slopeVertical {
		return 0, tmymove
	}
	if ld.dx == 0 && ld.dy == 0 {
		return 0, 0
	}
	lineAngle := vectorToAngle(ld.dx, ld.dy)
	if g.pointOnLineSide(g.p.x, g.p.y, ld) == 1 {
		lineAngle += statusAng180
	}
	moveAngle := vectorToAngle(tmxmove, tmymove)
	deltaAngle := moveAngle - lineAngle
	if deltaAngle > statusAng180 {
		deltaAngle += statusAng180
	}
	moveLen := approxDistance(tmxmove, tmymove)
	newLen := fixedMul(moveLen, doomFineCosine(deltaAngle))
	return fixedMul(newLen, doomFineCosine(lineAngle)), fixedMul(newLen, doomFineSineAtAngle(lineAngle))
}

func (g *game) sectorAt(x, y int64) int {
	if len(g.m.Nodes) == 0 {
		if len(g.m.Sectors) == 0 {
			return -1
		}
		return 0
	}
	child := uint16(len(g.m.Nodes) - 1)
	for {
		if child&0x8000 != 0 {
			ss := int(child & 0x7fff)
			if ss < 0 || ss >= len(g.m.SubSectors) {
				return -1
			}
			if ss < len(g.subSectorSec) {
				if sec := g.subSectorSec[ss]; sec >= 0 && sec < len(g.m.Sectors) {
					return sec
				}
			}
			s := g.m.SubSectors[ss]
			if int(s.FirstSeg) >= len(g.m.Segs) {
				return -1
			}
			seg := g.m.Segs[s.FirstSeg]
			if int(seg.Linedef) >= len(g.m.Linedefs) {
				return -1
			}
			ld := g.m.Linedefs[seg.Linedef]
			side := int(seg.Direction)
			if side < 0 || side > 1 {
				side = 0
			}
			sideNum := ld.SideNum[side]
			if sideNum < 0 || int(sideNum) >= len(g.m.Sidedefs) {
				return -1
			}
			sec := g.m.Sidedefs[int(sideNum)].Sector
			if int(sec) >= len(g.m.Sectors) {
				return -1
			}
			return int(sec)
		}
		ni := int(child)
		if ni < 0 || ni >= len(g.m.Nodes) {
			return -1
		}
		n := g.m.Nodes[ni]
		dl := divline{
			x:  int64(n.X) << fracBits,
			y:  int64(n.Y) << fracBits,
			dx: int64(n.DX) << fracBits,
			dy: int64(n.DY) << fracBits,
		}
		side := pointOnDivlineSide(x, y, dl)
		child = n.ChildID[side]
	}
}
