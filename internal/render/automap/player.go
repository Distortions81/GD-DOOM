package automap

import (
	"math"
	"sort"

	"gddoom/internal/mapdata"
)

const (
	doomTicsPerSecond = 35

	fracBits = 16
	fracUnit = 1 << fracBits

	playerRadius = 16 * fracUnit
	playerHeight = 56 * fracUnit
	maxMove      = 30 * fracUnit
	stopSpeed    = 0x1000
	friction     = 0xe800
	stepHeight   = 24 * fracUnit
	useRange     = 64 * fracUnit

	mlBlocking = 0x0001
	mlTwoSided = 0x0004
)

var (
	forwardMove = [2]int64{0x19, 0x32}
	sideMove    = [2]int64{0x18, 0x28}
	angleTurn   = [2]uint32{640 << 16, 1280 << 16}
)

type viewMode int

const (
	viewMap viewMode = iota
	viewWalk
)

type player struct {
	x      int64
	y      int64
	z      int64
	floorz int64
	ceilz  int64
	angle  uint32
	momx   int64
	momy   int64
}

type moveCmd struct {
	forward int64
	side    int64
	turn    int
	turnRaw int64
	run     bool
}

type slopeType int

const (
	slopeHorizontal slopeType = iota
	slopeVertical
	slopePositive
	slopeNegative
)

type physLine struct {
	idx      int
	x1       int64
	y1       int64
	x2       int64
	y2       int64
	dx       int64
	dy       int64
	bbox     [4]int64
	slope    slopeType
	flags    uint16
	special  uint16
	tag      uint16
	sideNum0 int16
	sideNum1 int16
}

type intercept struct {
	frac float64
	line int
}

func spawnPlayer(m *mapdata.Map) player {
	for _, t := range m.Things {
		if t.Type == 1 {
			x := int64(t.X) << fracBits
			y := int64(t.Y) << fracBits
			return player{x: x, y: y, z: 0, floorz: 0, ceilz: 128 * fracUnit, angle: degToAngle(t.Angle)}
		}
	}
	b := mapBounds(m)
	return player{x: int64(((b.minX + b.maxX) / 2) * fracUnit), y: int64(((b.minY + b.maxY) / 2) * fracUnit), ceilz: 128 * fracUnit}
}

func (g *game) initPhysics() {
	g.lines = buildPhysLines(g.m)
	g.lineValid = make([]int, len(g.lines))
	if g.m.BlockMap != nil {
		g.bmapOriginX = int64(g.m.BlockMap.OriginX) << fracBits
		g.bmapOriginY = int64(g.m.BlockMap.OriginY) << fracBits
		g.bmapWidth = int(g.m.BlockMap.Width)
		g.bmapHeight = int(g.m.BlockMap.Height)
	}
	g.sectorFloor = make([]int64, len(g.m.Sectors))
	g.sectorCeil = make([]int64, len(g.m.Sectors))
	for i, s := range g.m.Sectors {
		g.sectorFloor[i] = int64(s.FloorHeight) << fracBits
		g.sectorCeil[i] = int64(s.CeilingHeight) << fracBits
	}
	g.lineSpecial = make([]uint16, len(g.m.Linedefs))
	for i, ld := range g.m.Linedefs {
		g.lineSpecial[i] = ld.Special
	}
	sec := g.sectorAt(g.p.x, g.p.y)
	if sec >= 0 && sec < len(g.m.Sectors) {
		g.p.floorz = int64(g.m.Sectors[sec].FloorHeight) << fracBits
		g.p.ceilz = int64(g.m.Sectors[sec].CeilingHeight) << fracBits
		g.p.z = g.p.floorz
	}
}

func buildPhysLines(m *mapdata.Map) []physLine {
	out := make([]physLine, 0, len(m.Linedefs))
	for i, ld := range m.Linedefs {
		if int(ld.V1) >= len(m.Vertexes) || int(ld.V2) >= len(m.Vertexes) {
			continue
		}
		v1 := m.Vertexes[ld.V1]
		v2 := m.Vertexes[ld.V2]
		x1 := int64(v1.X) << fracBits
		y1 := int64(v1.Y) << fracBits
		x2 := int64(v2.X) << fracBits
		y2 := int64(v2.Y) << fracBits
		dx := x2 - x1
		dy := y2 - y1
		pl := physLine{
			idx:      i,
			x1:       x1,
			y1:       y1,
			x2:       x2,
			y2:       y2,
			dx:       dx,
			dy:       dy,
			flags:    ld.Flags,
			special:  ld.Special,
			tag:      ld.Tag,
			sideNum0: ld.SideNum[0],
			sideNum1: ld.SideNum[1],
		}
		if y1 > y2 {
			pl.bbox[0] = y1
			pl.bbox[1] = y2
		} else {
			pl.bbox[0] = y2
			pl.bbox[1] = y1
		}
		if x1 > x2 {
			pl.bbox[2] = x1
			pl.bbox[3] = x2
		} else {
			pl.bbox[2] = x2
			pl.bbox[3] = x1
		}
		switch {
		case dy == 0:
			pl.slope = slopeHorizontal
		case dx == 0:
			pl.slope = slopeVertical
		case (dy > 0) == (dx > 0):
			pl.slope = slopePositive
		default:
			pl.slope = slopeNegative
		}
		out = append(out, pl)
	}
	return out
}

func (g *game) updatePlayer(cmd moveCmd) {
	if cmd.turnRaw != 0 {
		g.p.angle += uint32(cmd.turnRaw)
	}
	if cmd.turn != 0 {
		turn := angleTurn[0]
		if cmd.run {
			turn = angleTurn[1]
		}
		if cmd.turn < 0 {
			g.p.angle -= turn
		} else {
			g.p.angle += turn
		}
	}

	if cmd.forward != 0 {
		g.thrust(g.p.angle, cmd.forward*2048)
	}
	if cmd.side != 0 {
		g.thrust(g.p.angle-0x40000000, cmd.side*2048)
	}

	g.xyMovement()
}

func (g *game) thrust(angle uint32, move int64) {
	rad := angleToRadians(angle)
	g.p.momx += fixedMul(move, floatToFixed(math.Cos(rad)))
	g.p.momy += fixedMul(move, floatToFixed(math.Sin(rad)))
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

	if g.p.momx > -stopSpeed && g.p.momx < stopSpeed && g.p.momy > -stopSpeed && g.p.momy < stopSpeed {
		g.p.momx = 0
		g.p.momy = 0
	} else {
		g.p.momx = fixedMul(g.p.momx, friction)
		g.p.momy = fixedMul(g.p.momy, friction)
	}
}

func (g *game) tryMove(x, y int64) bool {
	tmfloor, tmceil, tmdrop, ok := g.checkPosition(x, y)
	if !ok {
		return false
	}
	onFloor := g.p.z == g.p.floorz
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
	if onFloor {
		g.p.z = g.p.floorz
	} else if g.p.z+playerHeight > g.p.ceilz {
		g.p.z = g.p.ceilz - playerHeight
	}
	if g.p.z < g.p.floorz {
		g.p.z = g.p.floorz
	}
	return true
}

func (g *game) checkPosition(x, y int64) (int64, int64, int64, bool) {
	tmboxTop := y + playerRadius
	tmboxBottom := y - playerRadius
	tmboxRight := x + playerRadius
	tmboxLeft := x - playerRadius

	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return 0, 0, 0, false
	}
	tmfloor := int64(g.m.Sectors[sec].FloorHeight) << fracBits
	tmceil := int64(g.m.Sectors[sec].CeilingHeight) << fracBits
	tmdrop := tmfloor

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

func (g *game) handleUse() {
	lineIdx, ok := g.firstUsableLineInFront()
	if !ok {
		g.useText = "USE: no linedef"
		g.useFlash = 35
		return
	}
	info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
	if info.Door == nil {
		g.useText = "USE: no door action"
		g.useFlash = 35
		return
	}
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		g.useText = "USE: no door target"
		g.useFlash = 35
		return
	}
	if !info.Door.CanActivate(mapdata.KeyRing{}) {
		g.useText = "USE: locked"
		g.useFlash = 35
		return
	}

	opened := 0
	for _, sec := range targets {
		if sec < 0 || sec >= len(g.sectorCeil) {
			continue
		}
		target := g.lowestSurroundingCeiling(sec) - 4*fracUnit
		if target < g.sectorFloor[sec] {
			target = g.sectorFloor[sec]
		}
		if target > g.sectorCeil[sec] {
			g.sectorCeil[sec] = target
			opened++
		}
	}
	if opened > 0 {
		if !info.Repeat {
			g.lineSpecial[lineIdx] = 0
		}
		g.useText = "USE: door opened"
	} else {
		g.useText = "USE: no change"
	}
	g.useFlash = 35
}

func (g *game) firstUsableLineInFront() (int, bool) {
	px := g.p.x
	py := g.p.y
	ang := angleToRadians(g.p.angle)
	fx := int64(math.Cos(ang) * useRange)
	fy := int64(math.Sin(ang) * useRange)
	x2 := px + fx
	y2 := py + fy

	bestT := 2.0
	best := -1
	for _, ld := range g.lines {
		sp := g.lineSpecial[ld.idx]
		if sp == 0 {
			continue
		}
		info := mapdata.LookupLineSpecial(sp)
		if info.Door == nil {
			continue
		}
		if info.Trigger != mapdata.TriggerManual && info.Trigger != mapdata.TriggerUse {
			continue
		}
		t, ok := segmentIntersectFrac(px, py, x2, y2, ld.x1, ld.y1, ld.x2, ld.y2)
		if !ok {
			continue
		}
		if t < bestT {
			bestT = t
			best = ld.idx
		}
	}
	if best < 0 {
		return 0, false
	}
	return best, true
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

type divline struct {
	x  int64
	y  int64
	dx int64
	dy int64
}

func pointOnDivlineSide(x, y int64, line divline) int {
	if line.dx == 0 {
		if x <= line.x {
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
		if y <= line.y {
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
	dx := x - line.x
	dy := y - line.y
	if ((line.dy ^ line.dx ^ dx ^ dy) & 0x80000000) != 0 {
		if ((line.dy ^ dx) & 0x80000000) != 0 {
			return 1
		}
		return 0
	}
	left := fixedMul(line.dy>>8, dx>>8)
	right := fixedMul(dy>>8, line.dx>>8)
	if right < left {
		return 0
	}
	return 1
}

func segmentIntersectFrac(ax, ay, bx, by, cx, cy, dx, dy int64) (float64, bool) {
	x1, y1 := float64(ax), float64(ay)
	x2, y2 := float64(bx), float64(by)
	x3, y3 := float64(cx), float64(cy)
	x4, y4 := float64(dx), float64(dy)
	den := (x1-x2)*(y3-y4) - (y1-y2)*(x3-x4)
	if den == 0 {
		return 0, false
	}
	t := ((x1-x3)*(y3-y4) - (y1-y3)*(x3-x4)) / den
	u := -((x1-x2)*(y1-y3) - (y1-y2)*(x1-x3)) / den
	if t < 0 || t > 1 || u < 0 || u > 1 {
		return 0, false
	}
	return t, true
}

func fixedMul(a, b int64) int64 {
	return (a * b) >> fracBits
}

func floatToFixed(v float64) int64 {
	return int64(v * fracUnit)
}

func angleToRadians(a uint32) float64 {
	return float64(a) * (2 * math.Pi / 4294967296.0)
}

func degToAngle(deg int16) uint32 {
	return uint32((float64(deg) / 360.0) * 4294967296.0)
}

func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
