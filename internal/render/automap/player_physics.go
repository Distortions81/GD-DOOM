package automap

import (
	"math"
	"sort"
)

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
	if cmd.forward != 0 && onground {
		g.thrust(g.p.angle, cmd.forward*2048)
	}
	if cmd.side != 0 && onground {
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
	g.tickProjectiles()
	g.tickProjectileImpacts()
	g.tickMonsters()
}

func (g *game) runGameplayTic(cmd moveCmd, usePressed, fireHeld bool) {
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
	g.applySectorHazardDamage()
}

func (g *game) tickPlayerCounters() {
	if g == nil {
		return
	}
	if g.inventory.RadSuitTics > 0 {
		g.inventory.RadSuitTics--
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
	playerTouchesSector := func(sec int) bool {
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
	doorWouldCrushPlayer := func(sec int, nextCeil int64) bool {
		if g == nil || sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
			return false
		}
		if !playerTouchesSector(sec) {
			return false
		}
		playerTop := g.p.z + playerHeight
		floorZ := g.sectorFloor[sec]
		return nextCeil-floorZ < playerHeight || nextCeil < playerTop
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

	xmove := g.p.momx
	ymove := g.p.momy
	for {
		var ptryx, ptryy int64
		if abs(xmove) > maxMove/2 || abs(ymove) > maxMove/2 {
			ptryx = g.p.x + (xmove >> 1)
			ptryy = g.p.y + (ymove >> 1)
			xmove >>= 1
			ymove >>= 1
		} else {
			ptryx = g.p.x + xmove
			ptryy = g.p.y + ymove
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

	if g.p.momx > -stopSpeed && g.p.momx < stopSpeed && g.p.momy > -stopSpeed && g.p.momy < stopSpeed {
		g.p.momx = 0
		g.p.momy = 0
	} else {
		g.p.momx = fixedMul(g.p.momx, friction)
		g.p.momy = fixedMul(g.p.momy, friction)
	}
}

func (g *game) tryMove(x, y int64) bool {
	tmfloor, tmceil, tmdrop, ok := g.checkPositionFor(x, y, false)
	if !ok {
		return false
	}
	if tmceil-tmfloor < playerHeight {
		return false
	}
	if tmceil-g.p.z < playerHeight {
		return false
	}
	if tmfloor-g.p.z > stepHeight {
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

	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return 0, 0, 0, false
	}
	tmfloor := g.sectorFloor[sec]
	tmceil := g.sectorCeil[sec]
	tmdrop := tmfloor

	if g.actorBlockedByThings(x, y, radius, moverThingIdx, moverIsMonster) {
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

		if ld.sideNum1 < 0 {
			return false
		}
		if (ld.flags & mlBlocking) != 0 {
			return false
		}
		if blockMonsterLines && (ld.flags&mlBlockMonsters) != 0 {
			return false
		}

		opentop, openbottom, lowfloor, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			return false
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
	return tmfloor, tmceil, tmdrop, true
}

func actorsOverlapXY(ax, ay, aradius, bx, by, bradius int64) bool {
	blockdist := aradius + bradius
	return abs(ax-bx) < blockdist && abs(ay-by) < blockdist
}

func (g *game) actorBlockedByThings(x, y, radius int64, moverThingIdx int, moverIsMonster bool) bool {
	if g == nil || g.m == nil {
		return false
	}
	if moverIsMonster && !g.isDead && actorsOverlapXY(x, y, radius, g.p.x, g.p.y, playerRadius) {
		return true
	}
	for i, th := range g.m.Things {
		if i == moverThingIdx {
			continue
		}
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if !isMonster(th.Type) {
			continue
		}
		if i < 0 || i >= len(g.thingHP) || g.thingHP[i] <= 0 {
			continue
		}
		if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
			continue
		}
		tx, ty := g.thingPosFixed(i, th)
		if actorsOverlapXY(x, y, radius, tx, ty, monsterRadius(th.Type)) {
			return true
		}
	}
	return false
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
	if line.dx == 0 {
		if x <= line.x1 {
			if line.dy > 0 {
				return 1
			}
			return 0
		}
		if line.dy < 0 {
			return 1
		}
		return 0
	}
	if line.dy == 0 {
		if y <= line.y1 {
			if line.dx < 0 {
				return 1
			}
			return 0
		}
		if line.dx > 0 {
			return 1
		}
		return 0
	}
	dx := x - line.x1
	dy := y - line.y1
	left := fixedMul(line.dy>>fracBits, dx)
	right := fixedMul(dy, line.dx>>fracBits)
	if right < left {
		return 0
	}
	return 1
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

		bestFrac := 2.0
		bestLine := -1
		for _, tc := range [][4]int64{{leadx, leady, leadx + g.p.momx, leady + g.p.momy}, {trailx, leady, trailx + g.p.momx, leady + g.p.momy}, {leadx, traily, leadx + g.p.momx, traily + g.p.momy}} {
			f, li, ok := g.firstBlockingIntercept(tc[0], tc[1], tc[2], tc[3])
			if ok && f < bestFrac {
				bestFrac = f
				bestLine = li
			}
		}

		if bestLine < 0 {
			if !g.tryMove(g.p.x, g.p.y+g.p.momy) {
				_ = g.tryMove(g.p.x+g.p.momx, g.p.y)
			}
			return
		}

		bestFrac -= float64(0x800) / float64(fracUnit)
		if bestFrac > 0 {
			newx := fixedMul(g.p.momx, int64(bestFrac*fracUnit))
			newy := fixedMul(g.p.momy, int64(bestFrac*fracUnit))
			if !g.tryMove(g.p.x+newx, g.p.y+newy) {
				if !g.tryMove(g.p.x, g.p.y+g.p.momy) {
					_ = g.tryMove(g.p.x+g.p.momx, g.p.y)
				}
				return
			}
		}

		rest := 1.0 - (bestFrac + float64(0x800)/float64(fracUnit))
		if rest <= 0 {
			return
		}
		if rest > 1 {
			rest = 1
		}
		tmxmove := fixedMul(g.p.momx, int64(rest*fracUnit))
		tmymove := fixedMul(g.p.momy, int64(rest*fracUnit))
		tmxmove, tmymove = g.hitSlideLine(g.lines[bestLine], tmxmove, tmymove)
		g.p.momx = tmxmove
		g.p.momy = tmymove
		if g.tryMove(g.p.x+tmxmove, g.p.y+tmymove) {
			return
		}
	}
}

func (g *game) firstBlockingIntercept(x1, y1, x2, y2 int64) (float64, int, bool) {
	intercepts := make([]intercept, 0, 16)
	for i, ld := range g.lines {
		frac, ok := segmentIntersectFrac(x1, y1, x2, y2, ld.x1, ld.y1, ld.x2, ld.y2)
		if !ok {
			continue
		}
		intercepts = append(intercepts, intercept{frac: frac, line: i})
	}
	sort.Slice(intercepts, func(i, j int) bool { return intercepts[i].frac < intercepts[j].frac })

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
	ll := math.Hypot(float64(ld.dx), float64(ld.dy))
	if ll == 0 {
		return 0, 0
	}
	dx := float64(ld.dx) / ll
	dy := float64(ld.dy) / ll
	dot := float64(tmxmove)*dx + float64(tmymove)*dy
	return int64(dot * dx), int64(dot * dy)
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
