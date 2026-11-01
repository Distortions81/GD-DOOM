package doomruntime

import "gddoom/internal/mapdata"

func (g *game) activateGeneralizedFloor(line int, info mapdata.FloorInfo) bool {
	gen := info.Generalized
	targets := g.taggedSectorsForLine(line)
	if !info.UsesTag {
		sec, ok := g.sectorIndexFromSidedef(g.m.Linedefs[line].SideNum[1])
		if !ok {
			return false
		}
		targets = []int{sec}
	}
	if g.floors == nil {
		g.floors = make(map[int]*floorThinker)
	}
	active := false
	for _, sec := range targets {
		// Boom owns independent floor and ceiling slots.
		if g.floors[sec] != nil || g.plats[sec] != nil {
			continue
		}
		direction := -1
		if gen.Up {
			direction = 1
		}
		ft := &floorThinker{order: g.allocThinkerOrder(), sector: sec, typ: 13, direction: direction, speed: fracUnit << gen.Speed, crush: gen.Crush}
		current := g.sectorFloor[sec]
		switch gen.Target {
		case 0:
			ft.destHeight = g.findHighestFloorSurrounding(sec)
		case 1:
			ft.destHeight = g.findLowestFloorSurrounding(sec)
		case 2:
			ft.destHeight = current
			if gen.Up {
				ft.destHeight = g.findNextHighestFloor(sec, current)
			} else {
				for _, other := range g.boomNeighborSectors(sec) {
					h := g.sectorFloor[other]
					if h < current && (ft.destHeight == current || h > ft.destHeight) {
						ft.destHeight = h
					}
				}
			}
		case 3:
			ft.destHeight = g.findLowestCeilingSurrounding(sec)
		case 4:
			ft.destHeight = g.sectorCeil[sec]
		case 5:
			height := g.boomShortestLowerTexture(sec) >> fracBits
			ft.destHeight = max64(int64(-32000), min64(int64(32000), (current>>fracBits)+int64(direction)*height)) << fracBits
		case 6:
			ft.destHeight = current + int64(direction)*24*fracUnit
		case 7:
			ft.destHeight = current + int64(direction)*32*fracUnit
		}
		if gen.Change != 0 {
			model, ok := g.frontSectorForLine(line)
			if gen.NumericModel {
				ok = false
				for _, other := range g.boomNeighborSectors(sec) {
					height := g.sectorFloor[other]
					if gen.Target == 3 || gen.Target == 4 {
						height = g.sectorCeil[other]
					}
					if height == ft.destHeight {
						model, ok = other, true
						break
					}
				}
			}
			if ok {
				ft.finish, ft.finishFlat = floorFinishSetTexture, g.m.Sectors[model].FloorPic
				switch gen.Change {
				case 1:
					ft.finishSpecial = 0
				case 2:
					ft.finish = floorFinishTextureOnly
				case 3:
					ft.finishSpecial = g.m.Sectors[model].Special
				}
			}
		}
		g.floors[sec], active = ft, true
	}
	return active
}

// Preserve linedef order: numeric-model selection uses the first match.
func (g *game) boomNeighborSectors(sec int) []int {
	out := make([]int, 0, 8)
	for _, line := range g.m.Linedefs {
		if line.Flags&mlTwoSided == 0 {
			continue
		}
		a, oka := g.sectorIndexFromSidedef(line.SideNum[0])
		b, okb := g.sectorIndexFromSidedef(line.SideNum[1])
		if !oka || !okb || a == b {
			continue
		}
		if a == sec {
			out = append(out, b)
		} else if b == sec {
			out = append(out, a)
		}
	}
	return out
}

func (g *game) boomShortestLowerTexture(sec int) int64 {
	shortest := int64(32000) * fracUnit
	for _, line := range g.m.Linedefs {
		if line.Flags&mlTwoSided == 0 {
			continue
		}
		a, oka := g.sectorIndexFromSidedef(line.SideNum[0])
		b, okb := g.sectorIndexFromSidedef(line.SideNum[1])
		if !oka || !okb || (a != sec && b != sec) {
			continue
		}
		for _, side := range line.SideNum {
			name := g.m.Sidedefs[side].Bottom
			height, ok := g.opts.WallTextureHeights[name]
			if !ok {
				if tex, found := g.opts.WallTexBank[name]; found {
					height, ok = tex.Height, true
				}
			}
			if ok && height > 0 {
				shortest = min(shortest, int64(height)*fracUnit)
			}
		}
	}
	return shortest
}
