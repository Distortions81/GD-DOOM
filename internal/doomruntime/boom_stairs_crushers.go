package doomruntime

import "gddoom/internal/mapdata"

type boomStairState struct{ Lock, Prev, Next int }

func isGeneralizedStairOrCrusher(info mapdata.LineSpecialInfo) bool {
	return info.Stairs != nil && info.Stairs.Generalized != nil || info.Ceiling != nil && info.Ceiling.Crusher != nil
}
func generalizedStairOrCrusherAllowsMonsters(info mapdata.LineSpecialInfo) bool {
	if info.Stairs != nil && info.Stairs.Generalized != nil {
		return info.Stairs.Generalized.Monsters
	}
	return info.Ceiling != nil && info.Ceiling.Crusher != nil && info.Ceiling.Crusher.Monsters
}
func validBoomStairs(states []boomStairState, n int) bool {
	if len(states) == 0 {
		return true
	}
	if len(states) != n {
		return false
	}
	for i, s := range states {
		if s.Lock < -2 || s.Lock > 0 || s.Prev < -1 || s.Prev >= n || s.Next < -1 || s.Next >= n || s.Prev == i || s.Next == i {
			return false
		}
	}
	return true
}
func (g *game) initBoomStairStates() {
	if len(g.boomStairs) == len(g.m.Sectors) {
		return
	}
	g.boomStairs = make([]boomStairState, len(g.m.Sectors))
	for i := range g.boomStairs {
		g.boomStairs[i].Prev = -1
		g.boomStairs[i].Next = -1
	}
}
func (g *game) activateGeneralizedStairs(line int, info mapdata.StairsInfo) bool {
	g.initBoomStairStates()
	if g.floors == nil {
		g.floors = make(map[int]*floorThinker)
	}
	gen := info.Generalized
	direction := -1
	if gen.Up {
		direction = 1
	}
	step := [...]int64{4, 8, 16, 24}[gen.Step] * fracUnit * int64(direction)
	speed := [...]int64{fracUnit / 4, fracUnit / 2, 2 * fracUnit, 4 * fracUnit}[gen.Speed]
	active := false
	for _, start := range g.generalizedMoverTargets(line, info.UsesTag) {
		if g.floorSlotBusy(start) || g.boomStairs[start].Lock != 0 {
			continue
		}
		active = true
		texture := g.m.Sectors[start].FloorPic
		sec := start
		height := g.sectorFloor[sec] + step
		g.boomStairs[sec] = boomStairState{Lock: -2, Prev: -1, Next: -1}
		for {
			g.floors[sec] = &floorThinker{order: g.allocThinkerOrder(), sector: sec, direction: direction, speed: speed, destHeight: height}
			next := -1
			for _, ld := range g.m.Linedefs {
				front, ok0 := g.sectorIndexFromSidedef(ld.SideNum[0])
				back, ok1 := g.sectorIndexFromSidedef(ld.SideNum[1])
				if !ok0 || !ok1 || front != sec || !gen.IgnoreTexture && g.m.Sectors[back].FloorPic != texture {
					continue
				}
				oldBoom := g.opts.DemoScript != nil && g.opts.DemoScript.Header.Version == 201
				if oldBoom {
					height += step
				}
				if g.floorSlotBusy(back) || g.boomStairs[back].Lock != 0 {
					continue
				}
				if !oldBoom {
					height += step
				}
				next = back
				break
			}
			if next < 0 {
				break
			}
			g.boomStairs[sec].Next = next
			g.boomStairs[next] = boomStairState{Lock: -2, Prev: sec, Next: -1}
			sec = next
		}
	}
	if active {
		g.lineSpecial[line] ^= 256
	}
	return active
}
func (g *game) finishBoomStair(sec int) {
	if sec < 0 || sec >= len(g.boomStairs) || g.boomStairs[sec].Lock != -2 {
		return
	}
	g.boomStairs[sec].Lock = -1
	first := sec
	for steps := 0; g.boomStairs[first].Prev != -1; steps++ {
		if steps >= len(g.boomStairs) {
			return
		}
		first = g.boomStairs[first].Prev
	}
	chain := make([]int, 0, 8)
	for current := first; current != -1; current = g.boomStairs[current].Next {
		if len(chain) >= len(g.boomStairs) || g.boomStairs[current].Lock == -2 {
			return
		}
		chain = append(chain, current)
	}
	for _, i := range chain {
		g.boomStairs[i].Lock = 0
	}
}
func (g *game) ceilingTag(ct *ceilingThinker) uint16 {
	if ct.hasTriggerTag {
		return ct.triggerTag
	}
	return uint16(g.m.Sectors[ct.sector].Tag)
}
func (g *game) stopBoomCeilings(tag uint16) bool {
	active := false
	for _, ct := range g.ceilings {
		if ct != nil && g.ceilingTag(ct) == tag && ct.direction != 0 {
			ct.oldDirection = ct.direction
			ct.direction = 0
			active = true
		}
	}
	return active
}
func (g *game) activateGeneralizedCrusher(line int, info mapdata.CeilingInfo) bool {
	active := false
	tag := g.m.Linedefs[line].Tag
	for _, ct := range g.ceilings {
		if ct != nil && g.ceilingTag(ct) == tag && ct.direction == 0 {
			ct.direction = ct.oldDirection
			active = true
		}
	}
	if g.ceilings == nil {
		g.ceilings = make(map[int]*ceilingThinker)
	}
	for _, sec := range g.generalizedMoverTargets(line, info.UsesTag) {
		if g.ceilingSlotBusy(sec) {
			continue
		}
		action := mapdata.CeilingGenCrusher
		if info.Crusher.Silent {
			action = mapdata.CeilingGenSilentCrusher
		}
		speed := int64(fracUnit) << info.Crusher.Speed
		g.ceilings[sec] = &ceilingThinker{order: g.allocThinkerOrder(), sector: sec, action: action, direction: -1, speed: speed, oldSpeed: speed, topHeight: g.sectorCeil[sec], bottomHeight: g.sectorFloor[sec] + 8*fracUnit, crush: true, triggerTag: uint16(g.m.Sectors[sec].Tag), hasTriggerTag: true}
		active = true
	}
	return active
}
func (g *game) tickGeneralizedCrusher(sec int, ct *ceilingThinker) {
	if ct.direction == 0 {
		return
	}
	cur := g.sectorCeil[sec]
	dest := ct.topHeight
	if ct.direction < 0 {
		dest = max64(ct.bottomHeight, g.sectorFloor[sec])
	}
	next := cur + int64(ct.direction)*ct.speed
	past := ct.direction < 0 && next < dest || ct.direction > 0 && next > dest
	if past {
		next = dest
	}
	blocked := g.setSectorCeilingHeightWithCrush(sec, next, ct.direction < 0)
	if past {
		if blocked {
			g.setSectorCeilingHeightWithCrush(sec, cur, ct.direction < 0)
		}
		if ct.direction < 0 {
			if ct.oldSpeed < 3*fracUnit {
				ct.speed = ct.oldSpeed
			}
			ct.direction = 1
		} else {
			ct.direction = -1
		}
	} else if blocked && ct.direction < 0 && ct.oldSpeed < 3*fracUnit {
		ct.speed = fracUnit / 8
	}
}
