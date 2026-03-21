package doomruntime

import (
	"fmt"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"os"
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
	return typ == barrelThingType || typ == 30
}

func thingTypeIsShootable(typ int16) bool {
	return isMonster(typ) || isBarrelThingType(typ) || typ == 88
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

func (g *game) debugRadiusAttackEnabled(sx, sy int64) bool {
	if g == nil {
		return false
	}
	want := os.Getenv("GD_DEBUG_RADIUS_TIC")
	if want == "" {
		return false
	}
	var tic int
	if _, err := fmt.Sscanf(want, "%d", &tic); err != nil {
		return false
	}
	if g.demoTick-1 != tic && g.worldTic != tic {
		return false
	}
	if pos := os.Getenv("GD_DEBUG_RADIUS_POS"); pos != "" {
		var wantX, wantY int64
		if _, err := fmt.Sscanf(pos, "%d,%d", &wantX, &wantY); err != nil {
			return false
		}
		if sx != wantX || sy != wantY {
			return false
		}
	}
	return true
}

func (g *game) tickBarrel(i int, th mapdata.Thing) {
	if g == nil || i < 0 || g.m == nil || i >= len(g.m.Things) {
		return
	}
	if i < len(g.thingCollected) && g.thingCollected[i] {
		return
	}
	g.tickMonsterMomentum(i, th)
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
	g.damageShootableThingFrom(thingIdx, damage, true, -1, 0, 0, false)
}

func (g *game) damageShootableThingFrom(thingIdx int, damage int, sourcePlayer bool, sourceThing int, inflictorX, inflictorY int64, hasInflictor bool) {
	if g == nil || g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) || damage <= 0 {
		return
	}
	if want := os.Getenv("GD_DEBUG_BARREL_DAMAGE_TIC"); want != "" && os.Getenv("GD_DEBUG_BARREL_DAMAGE_IDX") == fmt.Sprint(thingIdx) {
		if fmt.Sprint(g.demoTick-1) == want || fmt.Sprint(g.worldTic) == want {
			rnd, prnd := doomrand.State()
			fmt.Printf("barrel-damage-debug tic=%d world=%d idx=%d type=%d damage=%d hp_before=%d source_player=%t source_thing=%d rnd=%d prnd=%d\n",
				g.demoTick-1, g.worldTic, thingIdx, g.m.Things[thingIdx].Type, damage, g.thingHP[thingIdx], sourcePlayer, sourceThing, rnd, prnd)
		}
	}
	typ := g.m.Things[thingIdx].Type
	switch {
	case isMonster(typ):
		g.damageMonsterFrom(thingIdx, damage, sourcePlayer, sourceThing, inflictorX, inflictorY, hasInflictor)
	case isBarrelThingType(typ):
		g.damageBarrelFrom(thingIdx, damage, sourcePlayer, sourceThing, inflictorX, inflictorY, hasInflictor)
	}
}

func (g *game) damageBarrel(thingIdx int, damage int) {
	g.damageBarrelFrom(thingIdx, damage, true, -1, 0, 0, false)
}

func (g *game) damageBarrelFrom(thingIdx int, damage int, sourcePlayer bool, sourceThing int, inflictorX, inflictorY int64, hasInflictor bool) {
	if g == nil || g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) || damage <= 0 {
		return
	}
	if thingIdx >= len(g.thingHP) || thingIdx >= len(g.thingDead) || thingIdx >= len(g.thingState) || thingIdx >= len(g.thingStatePhase) || thingIdx >= len(g.thingStateTics) || thingIdx >= len(g.thingDeathTics) {
		return
	}
	if g.thingDead[thingIdx] || g.thingHP[thingIdx] <= 0 {
		return
	}
	g.applyMonsterDamageThrust(thingIdx, damage, sourcePlayer, sourceThing, inflictorX, inflictorY, hasInflictor)
	g.thingHP[thingIdx] -= damage
	if g.thingHP[thingIdx] > 0 {
		_ = doomrand.PRandom()
		if thingIdx >= 0 && thingIdx < len(g.thingReactionTics) {
			g.thingReactionTics[thingIdx] = 0
		}
		g.maybeRetargetMonsterAfterDamage(thingIdx, g.m.Things[thingIdx].Type, sourcePlayer, sourceThing)
		return
	}
	g.thingDead[thingIdx] = true
	g.thingState[thingIdx] = monsterStateDeath
	g.thingStatePhase[thingIdx] = 0
	if want := os.Getenv("GD_DEBUG_BARREL_KILL_TIC"); want != "" {
		var tic int
		if _, err := fmt.Sscanf(want, "%d", &tic); err == nil && (g.demoTick-1 == tic || g.worldTic == tic) {
			rnd, prnd := doomrand.State()
			x, y := g.thingPosFixed(thingIdx, g.m.Things[thingIdx])
			fmt.Printf("gd-barrel-kill-debug tic=%d world=%d idx=%d pos=(%d,%d) tics_before=%d prnd_before=%d rnd_before=%d\n",
				g.demoTick-1, g.worldTic, thingIdx, x, y, g.thingStateTics[thingIdx], prnd, rnd)
		}
	}
	g.thingStateTics[thingIdx] = barrelRandomizedDeathStartTics()
	if want := os.Getenv("GD_DEBUG_BARREL_KILL_TIC"); want != "" {
		var tic int
		if _, err := fmt.Sscanf(want, "%d", &tic); err == nil && (g.demoTick-1 == tic || g.worldTic == tic) {
			rnd, prnd := doomrand.State()
			x, y := g.thingPosFixed(thingIdx, g.m.Things[thingIdx])
			fmt.Printf("gd-barrel-kill-debug tic=%d world=%d idx=%d pos=(%d,%d) tics_after=%d prnd_after=%d rnd_after=%d\n",
				g.demoTick-1, g.worldTic, thingIdx, x, y, g.thingStateTics[thingIdx], prnd, rnd)
		}
	}
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
	sourcePlayer := false
	sourceThing := -1
	if spotIdx >= 0 && spotIdx < len(g.thingTargetPlayer) && g.thingTargetPlayer[spotIdx] {
		sourcePlayer = true
	} else if spotIdx >= 0 && spotIdx < len(g.thingTargetIdx) {
		sourceThing = g.thingTargetIdx[spotIdx]
	}
	g.radiusAttackAt(sx, sy, sz, sheight, spotIdx, damage, "Explosion", sourcePlayer, sourceThing)
}

func (g *game) radiusAttackAt(sx, sy, sz, sheight int64, ignoreThing int, damage int, msg string, sourcePlayer bool, sourceThing int) {
	if g == nil || damage <= 0 {
		return
	}
	debugRadius := g.debugRadiusAttackEnabled(sx, sy)
	debugVisit := 0
	dist := int64(damage)*fracUnit + doomMaxThingRadius
	if g.m == nil {
		return
	}

	seen := make([]bool, len(g.m.Things))
	playerSeen := false
	playerCell := -1
	if !g.isDead && playerHeight > 0 {
		playerCell = g.thingBlockmapCellFor(g.p.x, g.p.y)
	}
	visitPlayer := func() {
		if playerSeen || g.isDead || playerHeight <= 0 || g.stats.Health <= 0 {
			return
		}
		playerSeen = true
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
		hasLOS := false
		damageToPlayer := 0
		if playerDist < int64(damage) {
			hasLOS = g.actorHasLOS(g.p.x, g.p.y, g.p.z, playerHeight, sx, sy, sz, sheight)
			if hasLOS {
				damageToPlayer = damage - int(playerDist)
			}
		}
		if debugRadius {
			rndBefore, prndBefore := doomrand.State()
			fmt.Printf("gd-radius-debug tic=%d world=%d visit=%d player dist=%d damage=%d los=%t health=%d armor=%d prnd_before=%d rnd_before=%d\n",
				g.demoTick-1, g.worldTic, debugVisit, playerDist, damageToPlayer, hasLOS,
				g.stats.Health, g.stats.Armor, prndBefore, rndBefore)
		}
		debugVisit++
		if damageToPlayer <= 0 {
			return
		}
		g.damagePlayerFrom(damageToPlayer, msg, sx, sy, true)
		if debugRadius {
			rndAfter, prndAfter := doomrand.State()
			fmt.Printf("gd-radius-debug tic=%d world=%d player health_after=%d armor_after=%d prnd_after=%d rnd_after=%d\n",
				g.demoTick-1, g.worldTic, g.stats.Health, g.stats.Armor, prndAfter, rndAfter)
		}
	}
	visitThing := func(i int) {
		if i < 0 || i >= len(g.m.Things) || seen[i] || i == ignoreThing {
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
		hasLOS := g.actorHasLOS(tx, ty, tz, theight, sx, sy, sz, sheight)
		if debugRadius {
			rndBefore, prndBefore := doomrand.State()
			fmt.Printf("gd-radius-debug tic=%d world=%d visit=%d idx=%d type=%d pos=(%d,%d,%d) dist=%d los=%t hp_before=%d state=%d tics=%d prnd_before=%d rnd_before=%d\n",
				g.demoTick-1, g.worldTic, debugVisit, i, th.Type, tx, ty, tz, tdist, hasLOS,
				g.thingHP[i], func() monsterThinkState {
					if i < len(g.thingState) {
						return g.thingState[i]
					}
					return 0
				}(), func() int {
					if i < len(g.thingStateTics) {
						return g.thingStateTics[i]
					}
					return 0
				}(), prndBefore, rndBefore)
		}
		debugVisit++
		if !hasLOS {
			return
		}
		rndBefore, prndBefore := doomrand.State()
		g.damageShootableThingFrom(i, damage-int(tdist), sourcePlayer, sourceThing, sx, sy, true)
		if debugRadius {
			rndAfter, prndAfter := doomrand.State()
			fmt.Printf("gd-radius-debug tic=%d world=%d idx=%d hp_after=%d dead=%t prnd_after=%d rnd_after=%d\n",
				g.demoTick-1, g.worldTic, i, g.thingHP[i],
				i < len(g.thingDead) && g.thingDead[i], prndAfter, rndAfter)
			_ = rndBefore
			_ = prndBefore
		}
	}

	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		seen := make(map[int]struct{})
		left := int((sx - dist - g.bmapOriginX) >> (fracBits + 7))
		right := int((sx + dist - g.bmapOriginX) >> (fracBits + 7))
		bottom := int((sy - dist - g.bmapOriginY) >> (fracBits + 7))
		top := int((sy + dist - g.bmapOriginY) >> (fracBits + 7))
		if len(g.thingBlockCells) != g.bmapWidth*g.bmapHeight {
			g.rebuildThingBlockmap()
		}
		for by := bottom; by <= top; by++ {
			for bx := left; bx <= right; bx++ {
				if bx < 0 || by < 0 || bx >= g.bmapWidth || by >= g.bmapHeight {
					continue
				}
				cell := by*g.bmapWidth + bx
				if cell == playerCell {
					visitPlayer()
				}
				for _, i := range g.thingBlockCells[cell] {
					if _, ok := seen[i]; ok {
						continue
					}
					seen[i] = struct{}{}
					visitThing(i)
				}
			}
		}
		return
	}
	visitPlayer()
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
	if ref, ok := g.runtimeWorldThingSpriteRef(i, th, tickUnits, unitsPerTic); ok && ref != nil {
		return ref.key
	}
	if isBarrelThingType(th.Type) {
		return g.barrelSpriteNameScaled(i, tickUnits, unitsPerTic)
	}
	return g.worldThingSpriteNameScaled(th.Type, tickUnits, unitsPerTic)
}

func (g *game) runtimeWorldThingSpriteRef(i int, th mapdata.Thing, tickUnits, unitsPerTic int) (*spriteRenderRef, bool) {
	if isBarrelThingType(th.Type) {
		return g.spriteRenderRef(g.barrelSpriteNameScaled(i, tickUnits, unitsPerTic))
	}
	anim := thingAnimRefState{}
	if i >= 0 && i < len(g.thingWorldAnimRef) {
		anim = g.thingWorldAnimRef[i]
	} else {
		anim = g.worldThingAnimRefs(th.Type)
	}
	if len(anim.refs) == 0 {
		return nil, false
	}
	if anim.staticRef != nil {
		return anim.staticRef, true
	}
	return pickThingAnimRef(anim, tickUnits, unitsPerTic), true
}
