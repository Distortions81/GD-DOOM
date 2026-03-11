package doomruntime

import (
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

const (
	barrelThingType  int16 = 2035
	barrelStateBAR1        = 944
	barrelStateBAR2        = 945
	barrelStateBEXP        = 946
	barrelStateBEXP2       = 947
	barrelStateBEXP3       = 948
	barrelStateBEXP4       = 949
	barrelStateBEXP5       = 950
)

var (
	barrelSpawnStateTics = [...]int{6, 6}
	barrelDeathStateTics = [...]int{5, 5, 5, 10, 10}
	barrelSpawnSprites   = [...]string{"BAR1A0", "BAR1B0"}
	barrelDeathSprites   = [...]string{"BEXPA0", "BEXPB0", "BEXPC0", "BEXPD0", "BEXPE0"}
)

func isBarrelThingType(typ int16) bool {
	return typ == barrelThingType
}

func thingTypeIsShootable(typ int16) bool {
	return isMonster(typ) || isBarrelThingType(typ)
}

func thingTypeNoBlood(typ int16) bool {
	return isBarrelThingType(typ)
}

func shootableThingSpawnHealth(typ int16) int {
	if !thingTypeIsShootable(typ) {
		return 0
	}
	if info, ok := demoTraceThingInfoForType(typ); ok && info.health > 0 {
		return info.health
	}
	return monsterSpawnHealth(typ)
}

func (g *game) thingCurrentHeight(i int, th mapdata.Thing) int64 {
	if info, ok := demoTraceThingInfoForType(th.Type); ok && info.height > 0 {
		if isBarrelThingType(th.Type) && i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
			return info.height >> 2
		}
		return info.height
	}
	if isMonster(th.Type) {
		return monsterHeight(th.Type)
	}
	return 16 * fracUnit
}

func barrelRandomizedDeathStartTics() int {
	tics := 5 - (doomrand.PRandom() & 3)
	if tics < 1 {
		tics = 1
	}
	return tics
}

func barrelTotalDeathTics(firstTics int) int {
	total := firstTics
	for _, tics := range barrelDeathStateTics[1:] {
		total += tics
	}
	return total
}

func (g *game) tickBarrel(i int, th mapdata.Thing) {
	if g == nil || i < 0 || g.m == nil || i >= len(g.m.Things) {
		return
	}
	if i < len(g.thingCollected) && g.thingCollected[i] {
		return
	}
	if i < len(g.thingDead) && g.thingDead[i] {
		g.tickBarrelDeathState(i, th)
		return
	}
	g.tickBarrelSpawnState(i)
}

func (g *game) tickBarrelSpawnState(i int) {
	if g == nil || i < 0 {
		return
	}
	if i >= len(g.thingState) || i >= len(g.thingStatePhase) || i >= len(g.thingStateTics) {
		return
	}
	g.thingState[i] = monsterStateSpawn
	if g.thingStateTics[i] <= 0 {
		phase := g.thingStatePhase[i] & 1
		g.thingStateTics[i] = barrelSpawnStateTics[phase]
	}
	g.thingStateTics[i]--
	if g.thingStateTics[i] > 0 {
		return
	}
	g.thingStatePhase[i] = (g.thingStatePhase[i] + 1) & 1
	g.thingStateTics[i] = barrelSpawnStateTics[g.thingStatePhase[i]]
}

func (g *game) tickBarrelDeathState(i int, th mapdata.Thing) {
	if g == nil || i < 0 {
		return
	}
	if i >= len(g.thingStatePhase) || i >= len(g.thingStateTics) {
		return
	}
	if g.thingStateTics[i] > 0 {
		g.thingStateTics[i]--
	}
	if g.thingStateTics[i] > 0 {
		return
	}
	switch g.thingStatePhase[i] {
	case 0:
		g.thingStatePhase[i] = 1
		g.thingStateTics[i] = barrelDeathStateTics[1]
		x, y := g.thingPosFixed(i, th)
		g.emitSoundEventAt(soundEventBarrelExplode, x, y)
	case 1:
		g.thingStatePhase[i] = 2
		g.thingStateTics[i] = barrelDeathStateTics[2]
	case 2:
		g.thingStatePhase[i] = 3
		g.thingStateTics[i] = barrelDeathStateTics[3]
		g.radiusAttackFromThing(i, 128)
	case 3:
		g.thingStatePhase[i] = 4
		g.thingStateTics[i] = barrelDeathStateTics[4]
	default:
		g.thingStateTics[i] = 0
		if i >= 0 && i < len(g.thingCollected) {
			g.thingCollected[i] = true
		}
		if i >= 0 && i < len(g.thingSupportValid) {
			g.thingSupportValid[i] = false
		}
	}
}

func (g *game) damageShootableThing(thingIdx int, damage int) {
	if g == nil || g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) || damage <= 0 {
		return
	}
	typ := g.m.Things[thingIdx].Type
	switch {
	case isMonster(typ):
		g.damageMonster(thingIdx, damage)
	case isBarrelThingType(typ):
		g.damageBarrel(thingIdx, damage)
	}
}

func (g *game) damageBarrel(thingIdx int, damage int) {
	if g == nil || g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) || damage <= 0 {
		return
	}
	if thingIdx >= len(g.thingHP) || thingIdx >= len(g.thingDead) || thingIdx >= len(g.thingState) || thingIdx >= len(g.thingStatePhase) || thingIdx >= len(g.thingStateTics) || thingIdx >= len(g.thingDeathTics) {
		return
	}
	if g.thingDead[thingIdx] || g.thingHP[thingIdx] <= 0 {
		return
	}
	g.thingHP[thingIdx] -= damage
	if g.thingHP[thingIdx] > 0 {
		return
	}
	g.thingHP[thingIdx] = 0
	g.thingDead[thingIdx] = true
	g.thingState[thingIdx] = monsterStateDeath
	g.thingStatePhase[thingIdx] = 0
	g.thingStateTics[thingIdx] = barrelRandomizedDeathStartTics()
	g.thingDeathTics[thingIdx] = barrelTotalDeathTics(g.thingStateTics[thingIdx])
}

func (g *game) radiusAttackFromThing(spotIdx int, damage int) {
	if g == nil || g.m == nil || spotIdx < 0 || spotIdx >= len(g.m.Things) || damage <= 0 {
		return
	}
	spot := g.m.Things[spotIdx]
	sx, sy := g.thingPosFixed(spotIdx, spot)
	sz, _, _ := g.thingSupportState(spotIdx, spot)
	sheight := g.thingCurrentHeight(spotIdx, spot)
	dist := int64(damage)*fracUnit + doomMaxThingRadius

	if !g.isDead {
		dx := abs(g.p.x - sx)
		dy := abs(g.p.y - sy)
		playerDist := dx
		if dy > playerDist {
			playerDist = dy
		}
		playerDist = (playerDist - playerRadius) >> fracBits
		if playerDist < 0 {
			playerDist = 0
		}
		if playerDist < int64(damage) && g.actorHasLOS(g.p.x, g.p.y, g.p.z, playerHeight, sx, sy, sz, sheight) {
			g.damagePlayerFrom(damage-int(playerDist), "Explosion", sx, sy, true)
		}
	}

	seen := make([]bool, len(g.m.Things))
	visitThing := func(i int) {
		if i < 0 || i >= len(g.m.Things) || seen[i] || i == spotIdx {
			return
		}
		seen[i] = true
		if i < len(g.thingCollected) && g.thingCollected[i] {
			return
		}
		th := g.m.Things[i]
		if !thingTypeIsShootable(th.Type) {
			return
		}
		if i >= len(g.thingHP) || g.thingHP[i] <= 0 {
			return
		}
		if th.Type == 16 || th.Type == 7 {
			return
		}
		tx, ty := g.thingPosFixed(i, th)
		radius := thingTypeRadius(th.Type)
		dx := abs(tx - sx)
		dy := abs(ty - sy)
		tdist := dx
		if dy > tdist {
			tdist = dy
		}
		tdist = (tdist - radius) >> fracBits
		if tdist < 0 {
			tdist = 0
		}
		if tdist >= int64(damage) {
			return
		}
		tz, _, _ := g.thingSupportState(i, th)
		theight := g.thingCurrentHeight(i, th)
		if !g.actorHasLOS(tx, ty, tz, theight, sx, sy, sz, sheight) {
			return
		}
		g.damageShootableThing(i, damage-int(tdist))
	}

	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		left := int((sx - dist - g.bmapOriginX) >> (fracBits + 7))
		right := int((sx + dist - g.bmapOriginX) >> (fracBits + 7))
		bottom := int((sy - dist - g.bmapOriginY) >> (fracBits + 7))
		top := int((sy + dist - g.bmapOriginY) >> (fracBits + 7))
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
					visitThing(i)
				}
			}
		}
		return
	}
	for i := range g.m.Things {
		visitThing(i)
	}
}

func (g *game) barrelSpriteNameScaled(i int, tickUnits, unitsPerTic int) string {
	if g == nil || i < 0 || i >= len(g.m.Things) {
		return g.worldThingSpriteNameScaled(barrelThingType, tickUnits, unitsPerTic)
	}
	if i < len(g.thingDead) && g.thingDead[i] {
		phase := 0
		if i < len(g.thingStatePhase) {
			phase = g.thingStatePhase[i]
		}
		if phase < 0 {
			phase = 0
		}
		if phase >= len(barrelDeathSprites) {
			phase = len(barrelDeathSprites) - 1
		}
		return barrelDeathSprites[phase]
	}
	phase := 0
	if i < len(g.thingStatePhase) {
		phase = g.thingStatePhase[i] & 1
	}
	return barrelSpawnSprites[phase]
}

func (g *game) runtimeWorldThingSpriteNameScaled(i int, th mapdata.Thing, tickUnits, unitsPerTic int) string {
	if isBarrelThingType(th.Type) {
		return g.barrelSpriteNameScaled(i, tickUnits, unitsPerTic)
	}
	return g.worldThingSpriteNameScaled(th.Type, tickUnits, unitsPerTic)
}
