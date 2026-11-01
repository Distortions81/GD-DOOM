package doomruntime

import "gddoom/internal/mapdata"

const (
	boomScrollFloor uint8 = iota
	boomScrollCeiling
	boomScrollWall
	boomScrollCarry
)

// Setup determines the immutable fields; the remainder is authoritative state.
// Fixed values wrap as signed 16.16, matching Boom's control_scroll_t.
type boomScrollerState struct {
	Order                                int64
	Kind                                 uint8
	Target, Control                      int
	DX, DY                               int64
	Accel                                bool
	LastHeight, VX, VY, OffsetX, OffsetY int64
}

func boomFixed(v int64) int64 { return int64(int32(v)) }
func boomScrollAdd(a *boomScrollVector, x, y int64) {
	a.x = boomFixed(a.x + x)
	a.y = boomFixed(a.y + y)
}
func (g *game) boomLineDelta(line mapdata.Linedef) (int64, int64) {
	a, b := g.m.Vertexes[line.V1], g.m.Vertexes[line.V2]
	return boomFixed(floorFixed(b.X) - floorFixed(a.X)), boomFixed(floorFixed(b.Y) - floorFixed(a.Y))
}

// Boom's original Add_WallScroller arithmetic, including fixed-point rounding.
func boomWallScrollVector(dx, dy, lx, ly int64) (int64, int64) {
	x, y := abs(lx), abs(ly)
	if y > x {
		x, y = y, x
	}
	if x == 0 {
		return 0, 0
	}
	angle := doomTanToAngle[min(len(doomTanToAngle)-1, int(fixedDiv(y, x)>>5))] + doomAng90
	d := fixedDiv(x, doomFineSineAtAngle(angle))
	a := boomFixed(fixedMul(dy, ly) + fixedMul(dx, lx))
	b := boomFixed(fixedMul(dx, ly) - fixedMul(dy, lx))
	return boomFixed(-fixedDiv(a, d)), boomFixed(-fixedDiv(b, d))
}
func (g *game) initBoomScrollers() {
	g.boomCarries = nil
	g.boomScrollers = nil
	n := len(g.m.Sectors)
	g.boomFloorScroll = make([]boomScrollVector, n)
	g.boomCeilingScroll = make([]boomScrollVector, n)
	g.boomFloorOffset = make([]boomScrollVector, n)
	g.boomCeilingOffset = make([]boomScrollVector, n)
	g.boomWallScroll = make([]boomScrollVector, len(g.m.Sidedefs))
	g.boomWallOffset = make([]boomScrollVector, len(g.m.Sidedefs))
	linesByTag := make(map[uint16][]int)
	for i, line := range g.m.Linedefs {
		linesByTag[line.Tag] = append(linesByTag[line.Tag], i)
	}
	for i, line := range g.m.Linedefs {
		special := line.Special
		control := -1
		accel := false
		if special >= 245 && special <= 249 {
			special += 5
			control = g.boomScrollControl(line)
		} else if special >= 214 && special <= 218 {
			special += 36
			control = g.boomScrollControl(line)
			accel = true
		}
		if special < 250 || special > 255 {
			continue
		}
		if (line.Special >= 245 && line.Special <= 249 || line.Special >= 214 && line.Special <= 218) && control < 0 {
			continue
		}
		dx, dy := g.boomLineDelta(line)
		dx >>= 5
		dy >>= 5
		add := func(kind uint8, target int, x, y int64) {
			if control < 0 {
				switch kind {
				case boomScrollFloor:
					boomScrollAdd(&g.boomFloorScroll[target], x, y)
				case boomScrollCeiling:
					boomScrollAdd(&g.boomCeilingScroll[target], x, y)
				case boomScrollWall:
					boomScrollAdd(&g.boomWallScroll[target], x, y)
				case boomScrollCarry:
					g.boomCarries = append(g.boomCarries, boomCarryThinker{g.allocThinkerOrder(), target, boomScrollVector{x, y}})
				}
				return
			}
			g.boomScrollers = append(g.boomScrollers, boomScrollerState{Order: g.allocThinkerOrder(), Kind: kind, Target: target, Control: control, DX: x, DY: y, Accel: accel, LastHeight: boomFixed(g.sectorFloor[control] + g.sectorCeil[control])})
		}
		if special == 254 {
			for _, target := range linesByTag[line.Tag] {
				if target == i {
					continue
				}
				wall := g.m.Linedefs[target]
				side := int(wall.SideNum[0])
				if side < 0 || side >= len(g.m.Sidedefs) {
					continue
				}
				lx, ly := g.boomLineDelta(wall)
				x, y := boomWallScrollVector(dx, dy, lx, ly)
				add(boomScrollWall, side, x, y)
			}
			continue
		}
		if special == 255 {
			side := int(line.SideNum[0])
			if side < 0 || side >= len(g.m.Sidedefs) {
				continue
			}
			s := g.m.Sidedefs[side]
			add(boomScrollWall, side, -int64(s.TextureOffset)*fracUnit, int64(s.RowOffset)*fracUnit)
			continue
		}
		targets := g.taggedSectorsForTag(line.Tag)
		// Combined floor/carry creates all plane thinkers before the carry thinkers.
		for _, sec := range targets {
			if special == 250 {
				add(boomScrollCeiling, sec, -dx, dy)
			}
			if special == 251 || special == 253 {
				add(boomScrollFloor, sec, -dx, dy)
			}
		}
		if special == 252 || special == 253 {
			for _, sec := range targets {
				add(boomScrollCarry, sec, boomFixed(fixedMul(dx, 6144)), boomFixed(fixedMul(dy, 6144)))
			}
		}
	}
}
func (g *game) boomScrollControl(line mapdata.Linedef) int {
	side := int(line.SideNum[0])
	if side < 0 || side >= len(g.m.Sidedefs) {
		return -1
	}
	return int(g.m.Sidedefs[side].Sector)
}
func (g *game) tickBoomScroller(i int) {
	s := &g.boomScrollers[i]
	height := boomFixed(g.sectorFloor[s.Control] + g.sectorCeil[s.Control])
	delta := boomFixed(height - s.LastHeight)
	s.LastHeight = height
	dx, dy := boomFixed(fixedMul(s.DX, delta)), boomFixed(fixedMul(s.DY, delta))
	if s.Accel {
		s.VX = boomFixed(s.VX + dx)
		s.VY = boomFixed(s.VY + dy)
	} else {
		s.VX, s.VY = dx, dy
	}
	if s.Kind == boomScrollCarry {
		g.tickBoomCarry(boomCarryThinker{sector: s.Target, vector: boomScrollVector{s.VX, s.VY}})
		return
	}
	s.OffsetX = boomFixed(s.OffsetX + s.VX)
	s.OffsetY = boomFixed(s.OffsetY + s.VY)
	var dest *boomScrollVector
	switch s.Kind {
	case boomScrollFloor:
		dest = &g.boomFloorOffset[s.Target]
	case boomScrollCeiling:
		dest = &g.boomCeilingOffset[s.Target]
	case boomScrollWall:
		dest = &g.boomWallOffset[s.Target]
	}
	boomScrollAdd(dest, s.VX, s.VY)
}
func (g *game) rebuildBoomScrollOffsets() {
	clear(g.boomFloorOffset)
	clear(g.boomCeilingOffset)
	clear(g.boomWallOffset)
	for _, s := range g.boomScrollers {
		var dest *boomScrollVector
		switch s.Kind {
		case boomScrollFloor:
			if s.Target < len(g.boomFloorOffset) {
				dest = &g.boomFloorOffset[s.Target]
			}
		case boomScrollCeiling:
			if s.Target < len(g.boomCeilingOffset) {
				dest = &g.boomCeilingOffset[s.Target]
			}
		case boomScrollWall:
			if s.Target < len(g.boomWallOffset) {
				dest = &g.boomWallOffset[s.Target]
			}
		}
		if dest != nil {
			boomScrollAdd(dest, s.OffsetX, s.OffsetY)
		}
	}
}
func (g *game) boomWallOffsets(side int) boomScrollVector {
	if side < 0 || side >= len(g.boomWallScroll) {
		return boomScrollVector{}
	}
	v := g.boomWallScroll[side]
	off := g.boomWallOffset[side]
	return boomScrollVector{boomFixed(v.x*int64(g.worldTic) + off.x), boomFixed(v.y*int64(g.worldTic) + off.y)}
}
