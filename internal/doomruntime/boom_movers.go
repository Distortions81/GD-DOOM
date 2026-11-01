package doomruntime

import (
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func (g *game) generalizedMoverTargets(line int, tagged bool) []int {
	if tagged {
		return g.taggedSectorsForLine(line)
	}
	sec, ok := g.sectorIndexFromSidedef(g.m.Linedefs[line].SideNum[1])
	if !ok {
		return nil
	}
	return []int{sec}
}

func (g *game) activateGeneralizedCeiling(line int, info mapdata.CeilingInfo) bool {
	gen := info.Generalized
	if g.ceilings == nil {
		g.ceilings = make(map[int]*ceilingThinker)
	}
	active := false
	for _, sec := range g.generalizedMoverTargets(line, info.UsesTag) {
		if g.ceilings[sec] != nil || g.doors[sec] != nil {
			continue
		}
		direction := -1
		if gen.Up {
			direction = 1
		}
		ct := &ceilingThinker{order: g.allocThinkerOrder(), sector: sec, action: mapdata.CeilingGeneralized, direction: direction, speed: fracUnit << gen.Speed, crush: gen.Crush}
		current := g.sectorCeil[sec]
		dest := current
		switch gen.Target {
		case 0:
			dest = g.findHighestCeilingSurrounding(sec)
		case 1:
			dest = g.findLowestCeilingSurrounding(sec)
		case 2:
			for _, other := range g.boomNeighborSectors(sec) {
				h := g.sectorCeil[other]
				if gen.Up && h > current && (dest == current || h < dest) || !gen.Up && h < current && (dest == current || h > dest) {
					dest = h
				}
			}
		case 3:
			dest = g.findHighestFloorSurrounding(sec)
		case 4:
			dest = g.sectorFloor[sec]
		case 5:
			dest = max64(-32000, min64(32000, (current>>fracBits)+int64(direction)*(g.boomShortestUpperTexture(sec)>>fracBits))) << fracBits
		case 6:
			dest = current + int64(direction)*24*fracUnit
		case 7:
			dest = current + int64(direction)*32*fracUnit
		}
		if gen.Up {
			ct.topHeight = dest
		} else {
			ct.bottomHeight = dest
		}
		if gen.Change != 0 {
			model, ok := g.frontSectorForLine(line)
			if gen.NumericModel {
				ok = false
				for _, other := range g.boomNeighborSectors(sec) {
					height := g.sectorCeil[other]
					if gen.Target == 3 || gen.Target == 4 {
						height = g.sectorFloor[other]
					}
					if height == dest {
						model, ok = other, true
						break
					}
				}
			}
			if ok {
				ct.finish, ct.finishFlat = floorFinishSetTexture, g.m.Sectors[model].CeilingPic
				switch gen.Change {
				case 1:
					ct.finishSpecial = 0
				case 2:
					ct.finish = floorFinishTextureOnly
				case 3:
					ct.finishSpecial = g.m.Sectors[model].Special
				}
			}
		}
		g.ceilings[sec] = ct
		active = true
	}
	return active
}

func (g *game) finishCeiling(sec int, ct *ceilingThinker) {
	if ct.finish != floorFinishNone {
		g.m.Sectors[sec].CeilingPic = ct.finishFlat
		if ct.finish == floorFinishSetTexture {
			g.m.Sectors[sec].Special = ct.finishSpecial
		}
		g.markDynamicSectorPlaneCacheDirty(sec)
	}
	delete(g.ceilings, sec)
}

func (g *game) boomShortestUpperTexture(sec int) int64 {
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
			name := g.m.Sidedefs[side].Top
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

func (g *game) activateGeneralizedLift(line int, info mapdata.PlatInfo) bool {
	gen := info.Generalized
	// Boom resumes tagged perpetual platforms even for manual triggers. Resuming
	// alone does not report success and must not consume a one-shot trigger.
	if gen.Target == 3 {
		g.activateInStasisPlats(g.m.Linedefs[line].Tag)
	}
	if g.plats == nil {
		g.plats = make(map[int]*platThinker)
	}
	active := false
	for _, sec := range g.generalizedMoverTargets(line, info.UsesTag) {
		if g.plats[sec] != nil || g.floors[sec] != nil {
			continue
		}
		pt := g.allocPlatThinker(sec)
		pt.triggerTag = g.m.Linedefs[line].Tag
		pt.hasTriggerTag = true
		pt.typ = platTypeDownWaitUpStay
		pt.status = platStatusDown
		pt.oldStatus = platStatusInStasis
		current := g.sectorFloor[sec]
		pt.high = current
		pt.low = current
		switch gen.Target {
		case 0:
			pt.low = min(current, g.findLowestFloorSurrounding(sec))
		case 1:
			for _, other := range g.boomNeighborSectors(sec) {
				h := g.sectorFloor[other]
				if h < current && (pt.low == current || h > pt.low) {
					pt.low = h
				}
			}
		case 2:
			pt.low = min(current, g.findLowestCeilingSurrounding(sec))
		case 3:
			pt.typ = platTypePerpetualRaise
			pt.low = min(current, g.findLowestFloorSurrounding(sec))
			pt.high = max64(current, g.findHighestFloorSurrounding(sec))
			pt.status = platStatus(doomrand.PRandom() & 1)
		}
		pt.speed = 2 * fracUnit << gen.Speed
		pt.wait = [...]int{35, 105, 175, 350}[gen.Delay]
		pt.count = 0
		pt.finishFlat = ""
		pt.finishSpecial = 0
		pt.skipOrderedTic = false
		g.plats[sec] = pt
		active = true
		if g.platTickedThisTic && !g.orderedWorldThinkersActive {
			g.tickPlat(sec, pt)
			pt.skipOrderedTic = true
		}
	}
	return active
}
