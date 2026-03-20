package doomruntime

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview"
)

const (
	doomTicsPerSecond = 35

	fracBits = 16
	fracUnit = 1 << fracBits

	playerRadius        = 16 * fracUnit
	playerHeight        = 56 * fracUnit
	playerViewHeight    = 41 * fracUnit
	playerViewHeightMin = playerViewHeight / 2
	playerGravity       = fracUnit
	maxMove             = 30 * fracUnit
	stopSpeed           = 0x1000
	friction            = 0xe800
	stepHeight          = 24 * fracUnit
	useRange            = 64 * fracUnit
	vDoorSpeed          = 2 * fracUnit
	vDoorWaitTic        = 150
	slowTurnTics        = 6
	switchResetTics     = 35

	mlBlocking      = 0x0001
	mlBlockMonsters = 0x0002
	mlTwoSided      = 0x0004
)

var (
	forwardMove = [2]int64{0x19, 0x32}
	sideMove    = [2]int64{0x18, 0x28}
	angleTurn   = [3]uint32{640 << 16, 1280 << 16, 320 << 16}
)

type viewMode int

const (
	viewMap viewMode = iota
	viewWalk
)

type player struct {
	x               int64
	y               int64
	z               int64
	floorz          int64
	ceilz           int64
	angle           uint32
	momx            int64
	momy            int64
	momz            int64
	reactionTime    int
	viewHeight      int64
	deltaViewHeight int64
}

type moveCmd struct {
	forward    int64
	side       int64
	turn       int
	turnRaw    int64
	run        bool
	weaponSlot int
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

type doorType int

const (
	doorNormal doorType = iota
	doorClose
	doorClose30ThenOpen
	doorBlazeRaise
	doorBlazeOpen
	doorBlazeClose
	doorOpen
	doorRaiseIn5Mins
)

type doorThinker struct {
	order        int64
	sector       int
	typ          doorType
	direction    int
	topHeight    int64
	topWait      int
	topCountdown int
	speed        int64
}

func spawnPlayer(m *mapdata.Map, requestedSlot int) (player, int, []playerStart, int) {
	starts := collectPlayerStarts(m)
	if s, ok := chooseSpawnStart(starts, requestedSlot); ok {
		return player{x: s.x, y: s.y, z: 0, floorz: 0, ceilz: 128 * fracUnit, angle: s.angle, viewHeight: playerViewHeight}, s.slot, starts, s.index
	}
	b := mapBounds(m)
	return player{x: int64(((b.minX + b.maxX) / 2) * fracUnit), y: int64(((b.minY + b.maxY) / 2) * fracUnit), ceilz: 128 * fracUnit, viewHeight: playerViewHeight}, 1, starts, -1
}

func (g *game) initPhysics() {
	g.lines = buildPhysLines(g.m)
	g.mapVisibleLines = buildMapVisibleLines(g.lines)
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
	g.doors = make(map[int]*doorThinker)
	sec := g.sectorAt(g.p.x, g.p.y)
	if sec >= 0 && sec < len(g.m.Sectors) {
		g.p.floorz = int64(g.m.Sectors[sec].FloorHeight) << fracBits
		g.p.ceilz = int64(g.m.Sectors[sec].CeilingHeight) << fracBits
		g.p.z = g.p.floorz
	}
	if g.p.viewHeight == 0 {
		g.p.viewHeight = playerViewHeight
	}
}

func buildMapVisibleLines(lines []physLine) []mapview.Line {
	out := make([]mapview.Line, 0, len(lines))
	for _, line := range lines {
		out = append(out, mapview.Line{
			Index: line.idx,
			BBox:  line.bbox,
		})
	}
	return out
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
