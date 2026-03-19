package doomruntime

import (
	"fmt"
	"math"
	"os"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

const (
	monsterWakeRange     = 1024 * fracUnit
	monsterMeleeRange    = 64 * fracUnit
	monsterAttackRange   = 1024 * fracUnit
	monsterAttackTics    = 35
	monsterBaseThreshold = 100

	monsterDiagFrac = 47000
)

type monsterMoveDir uint8

type monsterThinkState uint8

const (
	monsterDirEast monsterMoveDir = iota
	monsterDirNorthEast
	monsterDirNorth
	monsterDirNorthWest
	monsterDirWest
	monsterDirSouthWest
	monsterDirSouth
	monsterDirSouthEast
	monsterDirNoDir
)

const (
	monsterStateSpawn monsterThinkState = iota
	monsterStateSee
	monsterStatePain
	monsterStateAttack
	monsterStateDeath
)

var (
	monsterOpposite = [9]monsterMoveDir{
		monsterDirWest,
		monsterDirSouthWest,
		monsterDirSouth,
		monsterDirSouthEast,
		monsterDirEast,
		monsterDirNorthEast,
		monsterDirNorth,
		monsterDirNorthWest,
		monsterDirNoDir,
	}
	monsterDiags = [4]monsterMoveDir{
		monsterDirNorthWest,
		monsterDirNorthEast,
		monsterDirSouthWest,
		monsterDirSouthEast,
	}
	monsterXSpeed = [8]int64{
		fracUnit,
		monsterDiagFrac,
		0,
		-monsterDiagFrac,
		-fracUnit,
		-monsterDiagFrac,
		0,
		monsterDiagFrac,
	}
	monsterYSpeed = [8]int64{
		0,
		monsterDiagFrac,
		fracUnit,
		monsterDiagFrac,
		0,
		-monsterDiagFrac,
		-fracUnit,
		-monsterDiagFrac,
	}
)

func (g *game) tickMonsters() {
	if g.m == nil {
		return
	}
	g.ensureMonsterAIState()
	for i, th := range g.m.Things {
		if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
			continue
		}
		if i >= 0 && i < len(g.thingDead) && g.thingDead[i] && i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0 {
			g.thingDeathTics[i]--
		}
		if isBarrelThingType(th.Type) {
			g.tickBarrel(i, th)
			continue
		}
		if !isMonster(th.Type) || g.thingHP[i] <= 0 {
			continue
		}
		if th.Type == 88 {
			continue
		}
		g.tickMonsterMomentum(i, th)
		if !g.monsterHasTarget(i) {
			g.clearMonsterTargetState(i)
		}
		tx, ty := g.thingPosFixed(i, th)
		targetX, targetY := int64(0), int64(0)
		dist := int64(0)
		if px, py, _, _, _, ok := g.monsterTargetPos(i); ok {
			targetX, targetY = px, py
			dist = doomApproxDistance(targetX-tx, targetY-ty)
		}

		if i >= 0 && i < len(g.thingAttackTics) && g.thingAttackTics[i] > 0 {
			if g.tickMonsterAttackState(i, th.Type, tx, ty, targetX, targetY, dist) {
				continue
			}
		}
		resumedFromPain := false
		if i >= 0 && i < len(g.thingPainTics) && g.thingPainTics[i] > 0 {
			g.thingPainTics[i]--
			if i >= 0 && i < len(g.thingStateTics) && g.thingState[i] == monsterStatePain {
				g.thingStateTics[i] = g.thingPainTics[i]
			}
			if i >= 0 && i < len(g.thingAttackFireTics) {
				g.thingAttackFireTics[i] = -1
			}
			if g.thingPainTics[i] > 0 {
				continue
			}
			g.resetMonsterIdleOrChaseState(i, th.Type)
			resumedFromPain = true
		}
		if !resumedFromPain && i >= 0 && i < len(g.thingState) && (g.thingState[i] == monsterStatePain || g.thingState[i] == monsterStateAttack) {
			g.resetMonsterIdleOrChaseState(i, th.Type)
		}

		if !resumedFromPain && !g.monsterAdvanceThinkState(i, th.Type, tx, ty, targetX, targetY, dist) {
			continue
		}
		// A sleeping monster can acquire a target while advancing out of its
		// spawn state. Recompute chase inputs immediately so the wake-up tic
		// uses the actual target position instead of stale zero values.
		targetX, targetY = 0, 0
		dist = 0
		if px, py, _, _, _, ok := g.monsterTargetPos(i); ok {
			targetX, targetY = px, py
			dist = doomApproxDistance(targetX-tx, targetY-ty)
		}
		if i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
			g.thingReactionTics[i]--
		}
		if i >= 0 && i < len(g.thingThreshold) && g.thingThreshold[i] > 0 {
			if !g.monsterHasTarget(i) {
				g.thingThreshold[i] = 0
			} else {
				g.thingThreshold[i]--
			}
		}
		g.monsterTurnTowardMoveDir(i)

		// Doom A_Chase: prevent consecutive missile attacks.
		if g.thingJustAtk[i] {
			g.thingJustAtk[i] = false
			g.monsterPickNewChaseDir(i, th.Type, targetX, targetY)
			continue
		}

		if g.monsterCanMeleeTarget(i, th.Type, dist, tx, ty, targetX, targetY) {
			g.faceMonsterToward(i, tx, ty, targetX, targetY)
			if g.startMonsterAttackState(i, th.Type, false) {
				continue
			}
		}

		if g.monsterCanTryMissileNow(i) && g.monsterCheckMissileRange(i, th.Type, dist, tx, ty, targetX, targetY) {
			g.faceMonsterToward(i, tx, ty, targetX, targetY)
			if g.startMonsterAttackState(i, th.Type, true) {
				continue
			}
		}
		if th.Type == 64 && g.archvileTryRaiseCorpse(i) {
			continue
		}

		g.thingMoveCount[i]--
		if g.thingMoveCount[i] < 0 || !g.monsterMoveInDir(i, th.Type, g.thingMoveDir[i]) {
			g.monsterPickNewChaseDir(i, th.Type, targetX, targetY)
		}
		g.setMonsterThinkState(i, th.Type, monsterStateSee, g.monsterSeeStateTicsForPhase(i, th.Type))
		ax, ay := tx, ty
		if i >= 0 && g.m != nil && i < len(g.m.Things) {
			ax, ay = g.thingPosFixed(i, g.m.Things[i])
		}
		g.emitMonsterActiveSound(i, th.Type, ax, ay)
	}
}

func (g *game) setMonsterThinkState(i int, typ int16, state monsterThinkState, tics int) {
	if i < 0 || i >= len(g.thingState) || i >= len(g.thingStateTics) {
		return
	}
	if tics < 0 {
		tics = 0
	}
	g.thingState[i] = state
	g.thingStateTics[i] = tics
}

func (g *game) monsterAdvanceThinkState(i int, typ int16, tx, ty, px, py, dist int64) bool {
	if i < 0 || i >= len(g.thingState) || i >= len(g.thingStateTics) {
		return true
	}
	if g.thingStateTics[i] > 0 {
		g.thingStateTics[i]--
		if g.thingStateTics[i] > 0 {
			return false
		}
	}
	switch g.thingState[i] {
	case monsterStateSpawn:
		if _, wake := g.monsterAcquireSectorSoundTarget(i, tx, ty); wake || g.monsterLookForPlayer(i, false, tx, ty) {
			g.thingAggro[i] = true
			g.setMonsterTargetPlayer(i)
			g.emitMonsterSeeSound(i, typ, tx, ty)
			if i >= 0 && i < len(g.thingStatePhase) {
				g.thingStatePhase[i] = 0
			}
			g.setMonsterThinkState(i, typ, monsterStateSee, g.monsterSeeStateTicsForPhase(i, typ))
			return true
		}
		if i >= 0 && i < len(g.thingStatePhase) {
			count := len(monsterSpawnFrameTics(typ))
			if count < 1 {
				count = 1
			}
			g.thingStatePhase[i] = (g.thingStatePhase[i] + 1) % count
		}
		g.setMonsterThinkState(i, typ, monsterStateSpawn, g.monsterSpawnStateTicsForPhase(i, typ))
		return false
	case monsterStateSee:
		if !g.monsterHasTarget(i) {
			if g.monsterLookForPlayer(i, true, tx, ty) {
				g.thingAggro[i] = true
				g.setMonsterTargetPlayer(i)
				g.setMonsterThinkState(i, typ, monsterStateSee, g.monsterSeeStateTicsForPhase(i, typ))
				return false
			}
			g.setMonsterThinkState(i, typ, monsterStateSpawn, g.monsterSpawnStateTicsForPhase(i, typ))
			return false
		}
		if i >= 0 && i < len(g.thingStatePhase) {
			count := len(monsterSeeFrameTics(typ, g.fastMonstersActive()))
			if count < 1 {
				count = 1
			}
			g.thingStatePhase[i] = (g.thingStatePhase[i] + 1) % count
		}
		return true
	case monsterStateDeath:
		// Death animation: just decrement tics, don't advance state
		if i >= 0 && i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0 {
			g.thingDeathTics[i]--
		}
		return true
	default:
		return true
	}
}

func (g *game) monsterIdleOrChaseState(i int) monsterThinkState {
	if i >= 0 && i < len(g.thingAggro) && g.thingAggro[i] {
		return monsterStateSee
	}
	return monsterStateSpawn
}

func (g *game) monsterTargetAlive() bool {
	return g != nil && !g.isDead
}

func (g *game) clearMonsterTargetState(i int) {
	if g == nil || i < 0 {
		return
	}
	if i < len(g.thingAggro) {
		g.thingAggro[i] = false
	}
	if i < len(g.thingJustAtk) {
		g.thingJustAtk[i] = false
	}
	if i < len(g.thingJustHit) {
		g.thingJustHit[i] = false
	}
	if i < len(g.thingTargetPlayer) {
		g.thingTargetPlayer[i] = false
	}
	if i < len(g.thingTargetIdx) {
		g.thingTargetIdx[i] = -1
	}
	if i < len(g.thingThreshold) {
		g.thingThreshold[i] = 0
	}
}

func (g *game) setMonsterTargetPlayer(i int) {
	if g == nil || i < 0 {
		return
	}
	if i < len(g.thingTargetPlayer) {
		g.thingTargetPlayer[i] = true
	}
	if i < len(g.thingTargetIdx) {
		g.thingTargetIdx[i] = -1
	}
}

func (g *game) setMonsterTargetThing(i, targetIdx int) {
	if g == nil || i < 0 {
		return
	}
	if i < len(g.thingTargetPlayer) {
		g.thingTargetPlayer[i] = false
	}
	if i < len(g.thingTargetIdx) {
		g.thingTargetIdx[i] = targetIdx
	}
}

func (g *game) monsterTargetThingIdx(i int) (int, bool) {
	if g == nil || i < 0 || i >= len(g.thingTargetPlayer) || i >= len(g.thingTargetIdx) {
		return -1, false
	}
	if g.thingTargetPlayer[i] {
		return -1, false
	}
	idx := g.thingTargetIdx[i]
	if idx < 0 || g.m == nil || idx >= len(g.m.Things) {
		return -1, false
	}
	return idx, true
}

func (g *game) monsterHasTarget(i int) bool {
	if g == nil || i < 0 {
		return false
	}
	if i >= len(g.thingTargetPlayer) || i >= len(g.thingTargetIdx) || (i < len(g.thingAggro) && g.thingAggro[i] && !g.thingTargetPlayer[i] && g.thingTargetIdx[i] < 0) {
		return g.monsterTargetAlive()
	}
	if i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] {
		return g.monsterTargetAlive()
	}
	if idx, ok := g.monsterTargetThingIdx(i); ok {
		return idx < len(g.thingHP) && g.thingHP[idx] > 0 && (idx >= len(g.thingCollected) || !g.thingCollected[idx])
	}
	return false
}

func (g *game) monsterTargetPos(i int) (x, y, z, height, radius int64, ok bool) {
	if g == nil || i < 0 {
		return 0, 0, 0, 0, 0, false
	}
	if i >= len(g.thingTargetPlayer) || i >= len(g.thingTargetIdx) || (i < len(g.thingAggro) && g.thingAggro[i] && !g.thingTargetPlayer[i] && g.thingTargetIdx[i] < 0) {
		if !g.monsterTargetAlive() {
			return 0, 0, 0, 0, 0, false
		}
		return g.p.x, g.p.y, g.p.z, playerHeight, playerRadius, true
	}
	if i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] {
		if !g.monsterTargetAlive() {
			return 0, 0, 0, 0, 0, false
		}
		return g.p.x, g.p.y, g.p.z, playerHeight, playerRadius, true
	}
	targetIdx, ok := g.monsterTargetThingIdx(i)
	if !ok {
		return 0, 0, 0, 0, 0, false
	}
	th := g.m.Things[targetIdx]
	x, y = g.thingPosFixed(targetIdx, th)
	z, _, _ = g.thingSupportState(targetIdx, th)
	height = g.thingCurrentHeight(targetIdx, th)
	radius = thingTypeRadius(th.Type)
	return x, y, z, height, radius, true
}

func (g *game) monsterHasLOSTarget(i int, typ int16, x, y int64) bool {
	if g == nil || i < 0 {
		return false
	}
	if i >= len(g.thingTargetPlayer) || i >= len(g.thingTargetIdx) || (i < len(g.thingAggro) && g.thingAggro[i] && !g.thingTargetPlayer[i] && g.thingTargetIdx[i] < 0) {
		return g.monsterHasLOSPlayerAt(i, typ, x, y)
	}
	tx, ty, tz, height, _, ok := g.monsterTargetPos(i)
	if !ok {
		return false
	}
	if i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] {
		return g.monsterHasLOSPlayerAt(i, typ, x, y)
	}
	z, _, _ := g.monsterSupportHeights(i, g.m.Things[i])
	return g.actorHasLOS(x, y, z, monsterHeight(typ), tx, ty, tz, height)
}

func (g *game) monsterIdleOrChaseTics(i int, typ int16) int {
	if i >= 0 && i < len(g.thingAggro) && g.thingAggro[i] {
		return g.monsterSeeStateTicsForPhase(i, typ)
	}
	return g.monsterSpawnStateTicsForPhase(i, typ)
}

func (g *game) monsterSpawnStateTicsForPhase(i int, typ int16) int {
	phase := 0
	if i >= 0 && i < len(g.thingStatePhase) {
		phase = g.thingStatePhase[i]
	}
	return monsterSpawnStateTicsAtPhase(typ, phase)
}

func (g *game) monsterSeeStateTicsForPhase(i int, typ int16) int {
	phase := 0
	if i >= 0 && i < len(g.thingStatePhase) {
		phase = g.thingStatePhase[i]
	}
	return monsterSeeStateTicsAtPhase(typ, phase, g.fastMonstersActive())
}

func (g *game) resetMonsterIdleOrChaseState(i int, typ int16) {
	if i >= 0 && i < len(g.thingStatePhase) {
		g.thingStatePhase[i] = 0
	}
	g.setMonsterThinkState(i, typ, g.monsterIdleOrChaseState(i), g.monsterIdleOrChaseTics(i, typ))
}

func (g *game) emitMonsterSeeSound(i int, typ int16, x, y int64) {
	ev, fullVolume := monsterSeeSoundEvent(typ)
	if ev < 0 {
		return
	}
	if fullVolume {
		g.emitSoundEvent(ev)
		return
	}
	g.emitSoundEventAt(ev, x, y)
}

func (g *game) emitMonsterActiveSound(i int, typ int16, x, y int64) {
	ev := monsterActiveSoundEvent(typ)
	if ev < 0 {
		return
	}
	if !shouldEmitMonsterActiveSound(doomrand.PRandom()) {
		return
	}
	g.emitSoundEventAt(ev, x, y)
}

func shouldEmitMonsterActiveSound(r int) bool {
	return r >= 0 && r < 3
}

func monsterSeeSoundEvent(typ int16) (soundEvent, bool) {
	switch typ {
	case 3004, 9, 65:
		switch doomrand.PRandom() % 3 {
		case 1:
			return soundEventMonsterSeePosit2, false
		case 2:
			return soundEventMonsterSeePosit3, false
		default:
			return soundEventMonsterSeePosit1, false
		}
	case 3001:
		if doomrand.PRandom()%2 != 0 {
			return soundEventMonsterSeeImp2, false
		}
		return soundEventMonsterSeeImp1, false
	case 3002, 58:
		return soundEventMonsterSeeDemon, false
	case 3005:
		return soundEventMonsterSeeCaco, false
	case 3003:
		return soundEventMonsterSeeBaron, false
	case 69:
		return soundEventMonsterSeeKnight, false
	case 7:
		return soundEventMonsterSeeSpider, true
	case 68:
		return soundEventMonsterSeeArachnotron, false
	case 16:
		return soundEventMonsterSeeCyber, true
	case 71:
		return soundEventMonsterSeePainElemental, false
	case 84:
		return soundEventMonsterSeeWolfSS, false
	case 64:
		return soundEventMonsterSeeArchvile, false
	case 66:
		return soundEventMonsterSeeRevenant, false
	default:
		return -1, false
	}
}

func monsterActiveSoundEvent(typ int16) soundEvent {
	switch typ {
	case 3004, 9, 65, 84:
		return soundEventMonsterActivePosit
	case 3001:
		return soundEventMonsterActiveImp
	case 3002, 58, 3005, 3003, 69, 3006, 7, 16, 71:
		return soundEventMonsterActiveDemon
	case 67:
		return soundEventMonsterActivePosit
	case 68:
		return soundEventMonsterActiveArachnotron
	case 64:
		return soundEventMonsterActiveArchvile
	case 66:
		return soundEventMonsterActiveRevenant
	default:
		return -1
	}
}

func (g *game) ensureMonsterAIState() {
	if g.m == nil {
		return
	}
	n := len(g.m.Things)
	if len(g.thingTargetPlayer) != n {
		old := g.thingTargetPlayer
		g.thingTargetPlayer = make([]bool, n)
		copy(g.thingTargetPlayer, old)
	}
	if len(g.thingTargetIdx) != n {
		old := g.thingTargetIdx
		g.thingTargetIdx = make([]int, n)
		for i := range g.thingTargetIdx {
			g.thingTargetIdx[i] = -1
		}
		copy(g.thingTargetIdx, old)
	}
	if len(g.thingThreshold) != n {
		old := g.thingThreshold
		g.thingThreshold = make([]int, n)
		copy(g.thingThreshold, old)
	}
	if len(g.thingMoveDir) != n {
		old := g.thingMoveDir
		g.thingMoveDir = make([]monsterMoveDir, n)
		copy(g.thingMoveDir, old)
	}
	if len(g.thingMoveCount) != n {
		old := g.thingMoveCount
		g.thingMoveCount = make([]int, n)
		copy(g.thingMoveCount, old)
	}
	if len(g.thingJustAtk) != n {
		old := g.thingJustAtk
		g.thingJustAtk = make([]bool, n)
		copy(g.thingJustAtk, old)
	}
	if len(g.thingJustHit) != n {
		old := g.thingJustHit
		g.thingJustHit = make([]bool, n)
		copy(g.thingJustHit, old)
	}
	if len(g.thingReactionTics) != n {
		old := g.thingReactionTics
		g.thingReactionTics = make([]int, n)
		copy(g.thingReactionTics, old)
	}
	if len(g.thingWakeTics) != n {
		old := g.thingWakeTics
		g.thingWakeTics = make([]int, n)
		copy(g.thingWakeTics, old)
	}
	if len(g.thingLastLook) != n {
		old := g.thingLastLook
		g.thingLastLook = make([]int, n)
		copy(g.thingLastLook, old)
	}
	if len(g.thingDead) != n {
		old := g.thingDead
		g.thingDead = make([]bool, n)
		copy(g.thingDead, old)
	}
	if len(g.thingDropped) != n {
		old := g.thingDropped
		g.thingDropped = make([]bool, n)
		copy(g.thingDropped, old)
	}
	if len(g.thingX) != n {
		old := g.thingX
		g.thingX = make([]int64, n)
		copy(g.thingX, old)
		for i := len(old); i < n; i++ {
			g.thingX[i] = int64(g.m.Things[i].X) << fracBits
		}
	}
	if len(g.thingY) != n {
		old := g.thingY
		g.thingY = make([]int64, n)
		copy(g.thingY, old)
		for i := len(old); i < n; i++ {
			g.thingY[i] = int64(g.m.Things[i].Y) << fracBits
		}
	}
	if len(g.thingDeathTics) != n {
		old := g.thingDeathTics
		g.thingDeathTics = make([]int, n)
		copy(g.thingDeathTics, old)
	}
	if len(g.thingAttackTics) != n {
		old := g.thingAttackTics
		g.thingAttackTics = make([]int, n)
		copy(g.thingAttackTics, old)
	}
	if len(g.thingAttackPhase) != n {
		old := g.thingAttackPhase
		g.thingAttackPhase = make([]int, n)
		copy(g.thingAttackPhase, old)
	}
	if len(g.thingAttackFireTics) != n {
		old := g.thingAttackFireTics
		g.thingAttackFireTics = make([]int, n)
		for i := range g.thingAttackFireTics {
			g.thingAttackFireTics[i] = -1
		}
		copy(g.thingAttackFireTics, old)
	}
	if len(g.thingPainTics) != n {
		old := g.thingPainTics
		g.thingPainTics = make([]int, n)
		copy(g.thingPainTics, old)
	}
	if len(g.thingThinkWait) != n {
		old := g.thingThinkWait
		g.thingThinkWait = make([]int, n)
		copy(g.thingThinkWait, old)
	}
	if len(g.thingState) != n {
		old := g.thingState
		g.thingState = make([]monsterThinkState, n)
		copy(g.thingState, old)
		for i := len(old); i < n; i++ {
			g.thingState[i] = g.monsterIdleOrChaseState(i)
		}
	}
	if len(g.thingStateTics) != n {
		old := g.thingStateTics
		g.thingStateTics = make([]int, n)
		copy(g.thingStateTics, old)
	}
	if len(g.thingStatePhase) != n {
		old := g.thingStatePhase
		g.thingStatePhase = make([]int, n)
		copy(g.thingStatePhase, old)
	}
	if len(g.thingMomX) != n {
		old := g.thingMomX
		g.thingMomX = make([]int64, n)
		copy(g.thingMomX, old)
	}
	if len(g.thingMomY) != n {
		old := g.thingMomY
		g.thingMomY = make([]int64, n)
		copy(g.thingMomY, old)
	}
	if len(g.thingMomZ) != n {
		old := g.thingMomZ
		g.thingMomZ = make([]int64, n)
		copy(g.thingMomZ, old)
	}
}

func (g *game) tickMonsterMomentum(i int, th mapdata.Thing) {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	if i >= len(g.thingMomX) || i >= len(g.thingMomY) || i >= len(g.thingMomZ) {
		return
	}
	momx := g.thingMomX[i]
	momy := g.thingMomY[i]
	momz := g.thingMomZ[i]
	if momx == 0 && momy == 0 && momz == 0 {
		return
	}
	momx = clamp(momx, -maxMove, maxMove)
	momy = clamp(momy, -maxMove, maxMove)

	tx, ty := g.thingPosFixed(i, th)
	nx := tx + momx
	ny := ty + momy
	if tmfloor, tmceil, ok := g.tryMoveProbeMonster(i, th.Type, nx, ny); ok {
		g.setThingPosFixed(i, nx, ny)
		g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
	} else {
		momx = 0
		momy = 0
	}

	if i < len(g.thingZState) && i < len(g.thingFloorState) && g.thingZState[i] > g.thingFloorState[i] {
		g.thingMomX[i] = momx
		g.thingMomY[i] = momy
		g.thingMomZ[i] = momz
		return
	}
	if momx > -stopSpeed && momx < stopSpeed && momy > -stopSpeed && momy < stopSpeed {
		g.thingMomX[i] = 0
		g.thingMomY[i] = 0
		g.thingMomZ[i] = 0
		return
	}
	g.thingMomX[i] = fixedMul(momx, friction)
	g.thingMomY[i] = fixedMul(momy, friction)
	g.thingMomZ[i] = momz
}

func (g *game) setThingMomentum(i int, momx, momy, momz int64) {
	if g == nil || i < 0 {
		return
	}
	if i >= len(g.thingMomX) {
		g.thingMomX = append(g.thingMomX, make([]int64, i-len(g.thingMomX)+1)...)
	}
	if i >= len(g.thingMomY) {
		g.thingMomY = append(g.thingMomY, make([]int64, i-len(g.thingMomY)+1)...)
	}
	if i >= len(g.thingMomZ) {
		g.thingMomZ = append(g.thingMomZ, make([]int64, i-len(g.thingMomZ)+1)...)
	}
	g.thingMomX[i] = momx
	g.thingMomY[i] = momy
	g.thingMomZ[i] = momz
}

func monsterPainChance(typ int16) int {
	switch typ {
	case 3004: // zombieman
		return 200
	case 9: // shotgun guy
		return 170
	case 3001: // imp
		return 200
	case 3002, 58: // demon/spectre
		return 180
	case 3006: // lost soul
		return 256
	case 3005: // cacodemon
		return 128
	case 3003, 69: // baron/hell knight
		return 50
	case 64: // arch-vile
		return 10
	case 66: // revenant
		return 100
	case 67: // mancubus
		return 80
	case 16: // cyberdemon
		return 20
	case 7: // spider mastermind
		return 40
	case 68: // arachnotron
		return 128
	case 71: // pain elemental
		return 128
	case 84: // wolf ss
		return 170
	default:
		return 100
	}
}

func monsterPainDurationTics(typ int16) int {
	if total := monsterPainAnimTotalTics(typ); total > 0 {
		return total
	}
	switch typ {
	case 16:
		return 10
	case 7:
		return 8
	default:
		return 6
	}
}

func (g *game) startMonsterAttackAnim(i int, typ int16) {
	if i < 0 || i >= len(g.thingAttackTics) {
		return
	}
	total := monsterAttackStateTotalTics(typ)
	if total <= 0 {
		total = monsterAttackAnimTotalTics(typ)
	}
	if total <= 0 {
		g.thingAttackTics[i] = 0
		if i >= 0 && i < len(g.thingState) && i < len(g.thingStateTics) {
			g.resetMonsterIdleOrChaseState(i, typ)
		}
		return
	}
	g.thingAttackTics[i] = total
	if i >= 0 && i < len(g.thingAttackPhase) {
		g.thingAttackPhase[i] = 0
	}
	if i >= 0 && i < len(g.thingState) && i < len(g.thingStateTics) {
		g.thingState[i] = monsterStateAttack
		if monsterUsesExplicitAttackFrames(typ) {
			g.thingStateTics[i] = monsterAttackFrameDuration(typ, 0)
		} else {
			g.thingStateTics[i] = total
		}
	}
}

func (g *game) startMonsterAttackState(i int, typ int16, missile bool) bool {
	if i < 0 || g.m == nil || i >= len(g.m.Things) {
		return false
	}
	if !missile {
		if ev := monsterAttackStateEntrySoundEvent(typ); ev >= 0 {
			tx, ty := g.thingPosFixed(i, g.m.Things[i])
			g.emitSoundEventAt(ev, tx, ty)
		}
	}
	g.startMonsterAttackAnim(i, typ)
	if monsterUsesExplicitAttackFrames(typ) {
		if i >= 0 && i < len(g.thingAttackFireTics) {
			g.thingAttackFireTics[i] = -1
		}
		tx, ty := g.thingPosFixed(i, g.m.Things[i])
		dist := doomApproxDistance(g.p.x-tx, g.p.y-ty)
		g.runMonsterAttackPhaseEntry(i, typ, 0, tx, ty, g.p.x, g.p.y, dist)
		if missile && i >= 0 && i < len(g.thingJustAtk) {
			g.thingJustAtk[i] = true
		}
		return true
	}
	if i < 0 || i >= len(g.thingAttackFireTics) {
		// Fallback for malformed state in tests.
		tx := int64(g.m.Things[i].X) << fracBits
		ty := int64(g.m.Things[i].Y) << fracBits
		dist := doomApproxDistance(g.p.x-tx, g.p.y-ty)
		return g.monsterAttack(i, typ, dist)
	}
	delay := monsterAttackFireDelayTics(typ)
	g.thingAttackFireTics[i] = delay
	if delay <= 0 {
		tx := int64(g.m.Things[i].X) << fracBits
		ty := int64(g.m.Things[i].Y) << fracBits
		dist := doomApproxDistance(g.p.x-tx, g.p.y-ty)
		if !g.monsterAttack(i, typ, dist) {
			g.thingAttackTics[i] = 0
			g.thingAttackFireTics[i] = -1
			return false
		}
		g.thingAttackFireTics[i] = -1
	}
	if missile && i >= 0 && i < len(g.thingJustAtk) {
		g.thingJustAtk[i] = true
	}
	return true
}

func (g *game) monsterCanTryMissileNow(i int) bool {
	// Doom A_Chase gate: in non-fast modes, missile attacks only when movecount is 0.
	if g.fastMonstersActive() {
		return true
	}
	if i < 0 || i >= len(g.thingMoveCount) {
		return true
	}
	// Match Doom's `if (actor->movecount) goto nomissile;`
	// Any non-zero value (including negative) blocks missile attacks.
	return g.thingMoveCount[i] == 0
}

func monsterAttackFireDelayTics(typ int16) int {
	switch typ {
	case 3004: // zombieman
		return 10
	case 9: // sergeant
		return 16
	case 3001: // imp
		return 16
	case 3002, 58: // demon/spectre
		return 16
	case 3005: // cacodemon
		return 10
	case 3003, 69: // baron/knight
		return 16
	case 16: // cyberdemon
		return 6
	case 7: // spider mastermind
		return 20
	case 3006: // lost soul
		return 0
	default:
		return 0
	}
}

func monsterAttackStateTotalTics(typ int16) int {
	switch typ {
	case 3004: // zombieman
		return 26
	case 9: // sergeant
		return 24
	case 3001: // imp
		return 22
	case 3002, 58: // demon/spectre
		return 24
	case 3005: // cacodemon
		return 15
	case 3003, 69: // baron/knight
		return 24
	case 16: // cyberdemon
		return 66
	case 7: // spider mastermind (single volley cycle)
		return 29
	case 3006: // lost soul
		return 10
	default:
		return 0
	}
}

func monsterUsesExplicitAttackFrames(typ int16) bool {
	switch typ {
	case 3004, 9, 3001, 3002, 58, 3005, 3003, 69, 7, 16, 64, 66, 67, 68, 71, 84: // core roster plus explicit advanced attacks
		return true
	default:
		return false
	}
}

func monsterAttackFrameDuration(typ int16, phase int) int {
	frameTics := monsterAttackFrameTics(typ)
	if phase < 0 || phase >= len(frameTics) {
		return 0
	}
	return frameTics[phase]
}

func (g *game) runMonsterAttackPhaseEntry(i int, typ int16, phase int, tx, ty, px, py, dist int64) {
	switch typ {
	case 3004: // zombieman
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 1:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 9: // shotgun guy
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 1:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3001: // imp
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3002, 58: // demon/spectre
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3005: // cacodemon
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3003, 69: // baron/hell knight
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 64: // arch-vile
		switch phase {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 9:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 66: // revenant missile
		switch phase {
		case 0, 1, 3:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 67: // mancubus
		switch phase {
		case 0, 2, 3, 5, 6, 8, 9:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 1, 4, 7:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 71: // pain elemental
		switch phase {
		case 0, 1, 2:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 3:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 84: // wolf ss
		switch phase {
		case 0, 1, 3:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 2, 4:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 68: // arachnotron
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 1:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 16: // cyberdemon
		switch phase {
		case 0, 2, 4:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 1, 3, 5:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 7: // spider mastermind
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, px, py)
		case 1, 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	}
}

func (g *game) tickMonsterAttackState(i int, typ int16, tx, ty, px, py, dist int64) bool {
	if i < 0 || i >= len(g.thingAttackTics) {
		return false
	}
	if monsterUsesExplicitAttackFrames(typ) {
		g.thingAttackTics[i]--
		if i >= 0 && i < len(g.thingStateTics) {
			g.thingStateTics[i]--
			if g.thingStateTics[i] <= 0 {
				nextPhase := g.thingAttackPhase[i] + 1
				if nextPhase >= len(monsterAttackFrameTics(typ)) {
					g.thingAttackTics[i] = 0
					if i >= 0 && i < len(g.thingState) && i < len(g.thingStateTics) {
						g.resetMonsterIdleOrChaseState(i, typ)
					}
					return true
				}
				g.thingAttackPhase[i] = nextPhase
				g.thingStateTics[i] = monsterAttackFrameDuration(typ, nextPhase)
				g.runMonsterAttackPhaseEntry(i, typ, nextPhase, tx, ty, px, py, dist)
			}
		}
		return true
	}

	nextAttackTics := g.thingAttackTics[i] - 1
	g.debugMonsterAttack(i, "attack-tick", nextAttackTics)
	if i >= 0 && i < len(g.thingAttackFireTics) && g.thingAttackFireTics[i] >= 0 {
		if g.thingAttackFireTics[i] > 0 {
			g.thingAttackFireTics[i]--
		}
		if g.thingAttackFireTics[i] == 0 {
			g.faceMonsterToward(i, tx, ty, px, py)
			_ = g.monsterAttack(i, typ, dist)
			g.thingAttackFireTics[i] = -1
		}
	}
	g.thingAttackTics[i]--
	if i >= 0 && i < len(g.thingStateTics) && g.thingState[i] == monsterStateAttack {
		g.thingStateTics[i] = g.thingAttackTics[i]
	}
	return true
}

func demoTraceMonsterAttackState(typ int16, phase int) (int, bool) {
	switch typ {
	case 3004:
		if phase >= 0 && phase <= 2 {
			return 184 + phase, true
		}
	case 9:
		if phase >= 0 && phase <= 2 {
			return 217 + phase, true
		}
	case 3001:
		if phase >= 0 && phase <= 2 {
			return 452 + phase, true
		}
	case 3002, 58:
		if phase >= 0 && phase <= 2 {
			return 485 + phase, true
		}
	case 3005:
		if phase >= 0 && phase <= 2 {
			return 504 + phase, true
		}
	case 3003:
		if phase >= 0 && phase <= 2 {
			return 537 + phase, true
		}
	case 69:
		if phase >= 0 && phase <= 2 {
			return 566 + phase, true
		}
	}
	return 0, false
}

func monsterLookInterval(typ int16) int {
	if info, ok := demoTraceThingInfoForType(typ); ok && info.spawnTics > 0 {
		return info.spawnTics
	}
	return 1
}

func monsterSpawnStateTicsAtPhase(typ int16, phase int) int {
	tics := monsterSpawnFrameTics(typ)
	if len(tics) == 0 {
		wait := monsterLookInterval(typ)
		if wait < 1 {
			wait = 1
		}
		return wait
	}
	if phase < 0 || phase >= len(tics) {
		phase = 0
	}
	if tics[phase] < 1 {
		return 1
	}
	return tics[phase]
}

func monsterSpawnStateTics(typ int16) int {
	return monsterSpawnStateTicsAtPhase(typ, 0)
}

func monsterSeeStateTicsAtPhase(typ int16, phase int, fast bool) int {
	tics := monsterSeeFrameTics(typ, fast)
	if len(tics) == 0 {
		switch typ {
		case 3004, 84, 67:
			if fast {
				return 2
			}
			return 4
		case 9:
			if fast {
				return 2
			}
			return 3
		case 3002, 58, 64, 66:
			return 2
		case 3006:
			return 6
		case 65, 3001, 3003, 3005, 7, 16, 68, 69, 71:
			return 3
		default:
			return 3
		}
	}
	if phase < 0 || phase >= len(tics) {
		phase = 0
	}
	if tics[phase] < 1 {
		return 1
	}
	return tics[phase]
}

func monsterSeeStateTics(typ int16, fast bool) int {
	return monsterSeeStateTicsAtPhase(typ, 0, fast)
}

func monsterReactionTimeTics(typ int16) int {
	switch typ {
	case 3004, 9, 3001, 3002, 3006, 3005, 3003, 16, 7, 58, 64, 65, 66, 67, 68, 69, 71, 84:
		return 8
	default:
		return 0
	}
}

func (g *game) monsterCanMelee(typ int16, dist, tx, ty, px, py int64) bool {
	if !g.monsterTargetAlive() {
		return false
	}
	if !monsterHasMeleeAttack(typ) {
		return false
	}
	if dist >= monsterMeleeRange-20*fracUnit+playerRadius {
		return false
	}
	return g.monsterHasLOSPlayer(typ, tx, ty)
}

func (g *game) monsterCanMeleeTarget(i int, typ int16, dist, tx, ty, px, py int64) bool {
	if !g.monsterHasTarget(i) {
		return false
	}
	if !monsterHasMeleeAttack(typ) {
		return false
	}
	_, _, _, _, radius, ok := g.monsterTargetPos(i)
	if !ok {
		return false
	}
	if dist >= monsterMeleeRange-20*fracUnit+radius {
		return false
	}
	return g.monsterHasLOSTarget(i, typ, tx, ty)
}

func (g *game) monsterCheckMissileRange(i int, typ int16, dist, tx, ty, px, py int64) bool {
	if !g.monsterHasTarget(i) {
		return false
	}
	if isMeleeOnlyMonster(typ) {
		return false
	}
	if !g.monsterHasLOSTarget(i, typ, tx, ty) {
		return false
	}
	if i >= 0 && i < len(g.thingJustHit) && g.thingJustHit[i] {
		g.thingJustHit[i] = false
		return true
	}
	if i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
		return false
	}

	d := int((dist - 64*fracUnit) >> fracBits)
	if !monsterHasMeleeAttack(typ) {
		d -= 128
	}

	switch typ {
	case 64: // archvile
		if d > 14*64 {
			return false
		}
	case 66: // revenant
		if d < 196 {
			return false
		}
		d >>= 1
	}

	if typ == 16 || typ == 7 || typ == 3006 {
		d >>= 1
	}
	if d < 0 {
		d = 0
	}
	if d > 200 {
		d = 200
	}
	if typ == 16 && d > 160 {
		d = 160
	}
	return doomrand.PRandom() >= d
}

func (g *game) monsterPickNewChaseDir(i int, typ int16, targetX, targetY int64) {
	if g.m == nil || i < 0 || i >= len(g.m.Things) || i >= len(g.thingMoveDir) {
		return
	}
	tx, ty := g.thingPosFixed(i, g.m.Things[i])
	olddir := g.thingMoveDir[i]
	if olddir > monsterDirNoDir {
		olddir = monsterDirNoDir
	}
	turnaround := monsterOpposite[olddir]

	deltax := targetX - tx
	deltay := targetY - ty

	d1 := monsterDirNoDir
	d2 := monsterDirNoDir
	if deltax > 10*fracUnit {
		d1 = monsterDirEast
	} else if deltax < -10*fracUnit {
		d1 = monsterDirWest
	}
	if deltay < -10*fracUnit {
		d2 = monsterDirSouth
	} else if deltay > 10*fracUnit {
		d2 = monsterDirNorth
	}

	if d1 != monsterDirNoDir && d2 != monsterDirNoDir {
		diag := monsterDiags[(b2i(deltay < 0)<<1)+b2i(deltax > 0)]
		g.debugMonsterChase(i, fmt.Sprintf("diag candidate=%d turnaround=%d", diag, turnaround))
		if diag != turnaround && g.monsterTryWalk(i, typ, diag) {
			g.debugMonsterChase(i, fmt.Sprintf("diag success dir=%d", diag))
			return
		}
	}

	if doomrand.PRandom() > 200 || abs(deltay) > abs(deltax) {
		d1, d2 = d2, d1
		g.debugMonsterChase(i, fmt.Sprintf("swap d1=%d d2=%d", d1, d2))
	}

	if d1 == turnaround {
		d1 = monsterDirNoDir
	}
	if d2 == turnaround {
		d2 = monsterDirNoDir
	}

	if d1 != monsterDirNoDir && g.monsterTryWalk(i, typ, d1) {
		g.debugMonsterChase(i, fmt.Sprintf("d1 success dir=%d", d1))
		return
	}
	if d2 != monsterDirNoDir && g.monsterTryWalk(i, typ, d2) {
		g.debugMonsterChase(i, fmt.Sprintf("d2 success dir=%d", d2))
		return
	}

	if olddir != monsterDirNoDir && g.monsterTryWalk(i, typ, olddir) {
		g.debugMonsterChase(i, fmt.Sprintf("olddir success dir=%d", olddir))
		return
	}

	if (doomrand.PRandom() & 1) != 0 {
		for dir := int(monsterDirEast); dir <= int(monsterDirSouthEast); dir++ {
			d := monsterMoveDir(dir)
			if d != turnaround && g.monsterTryWalk(i, typ, d) {
				g.debugMonsterChase(i, fmt.Sprintf("scan success dir=%d", d))
				return
			}
		}
	} else {
		for dir := int(monsterDirSouthEast); dir >= int(monsterDirEast); dir-- {
			d := monsterMoveDir(dir)
			if d != turnaround && g.monsterTryWalk(i, typ, d) {
				g.debugMonsterChase(i, fmt.Sprintf("reverse scan success dir=%d", d))
				return
			}
		}
	}

	if turnaround != monsterDirNoDir && g.monsterTryWalk(i, typ, turnaround) {
		g.debugMonsterChase(i, fmt.Sprintf("turnaround success dir=%d", turnaround))
		return
	}
	g.thingMoveDir[i] = monsterDirNoDir
}

func (g *game) monsterTryWalk(i int, typ int16, dir monsterMoveDir) bool {
	if i < 0 || i >= len(g.thingMoveDir) {
		return false
	}
	g.thingMoveDir[i] = dir
	g.debugMonsterChase(i, fmt.Sprintf("trywalk dir=%d", dir))
	if !g.monsterMoveInDir(i, typ, dir) {
		g.debugMonsterChase(i, fmt.Sprintf("trywalk blocked dir=%d", dir))
		return false
	}
	if i >= 0 && i < len(g.thingMoveCount) {
		g.thingMoveCount[i] = doomrand.PRandom() & 15
		g.debugMonsterChase(i, fmt.Sprintf("trywalk moved dir=%d movecount=%d", dir, g.thingMoveCount[i]))
	}
	return true
}

func (g *game) debugMonsterChase(i int, msg string) {
	if g == nil || os.Getenv("GD_DEBUG_MONSTER_CHASE") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(os.Getenv("GD_DEBUG_MONSTER_CHASE"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantIdx >= 0 && i != wantIdx {
		return
	}
	if g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	tx, ty := int64(0), int64(0)
	if g.m != nil && i >= 0 && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	fmt.Printf("monster-chase-debug tic=%d world=%d idx=%d type=%d msg=%s pos=(%d,%d) movedir=%d movecount=%d angle=%d target=(%d,%d)\n",
		g.demoTick-1, g.worldTic, i, g.m.Things[i].Type, msg, tx, ty, g.thingMoveDir[i], g.thingMoveCount[i], g.thingWorldAngle(i, g.m.Things[i]), g.p.x, g.p.y)
}

func (g *game) debugMonsterMove(i int, msg string) {
	if g == nil || os.Getenv("GD_DEBUG_MONSTER_MOVE") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(os.Getenv("GD_DEBUG_MONSTER_MOVE"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantIdx >= 0 && i != wantIdx {
		return
	}
	if g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	tx, ty := int64(0), int64(0)
	if g.m != nil && i >= 0 && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	fmt.Printf("monster-move-debug tic=%d world=%d idx=%d type=%d msg=%s pos=(%d,%d) movedir=%d movecount=%d angle=%d\n",
		g.demoTick-1, g.worldTic, i, g.m.Things[i].Type, msg, tx, ty, g.thingMoveDir[i], g.thingMoveCount[i], g.thingWorldAngle(i, g.m.Things[i]))
}

func (g *game) monsterMoveInDir(i int, typ int16, dir monsterMoveDir) bool {
	if g.m == nil || i < 0 || i >= len(g.m.Things) {
		return false
	}
	if dir >= monsterDirNoDir {
		return false
	}
	step := monsterMoveStep(typ, g.fastMonstersActive())
	dx := fixedMul(step, monsterXSpeed[dir])
	dy := fixedMul(step, monsterYSpeed[dir])
	if dx == 0 && dy == 0 {
		return false
	}

	x, y := g.thingPosFixed(i, g.m.Things[i])
	nx := x + dx
	ny := y + dy
	g.debugMonsterMove(i, fmt.Sprintf("move dir=%d from=(%d,%d) to=(%d,%d)", dir, x, y, nx, ny))
	tmfloor, tmceil, ok := g.tryMoveProbeMonster(i, typ, nx, ny)
	if !ok {
		g.debugMonsterMove(i, fmt.Sprintf("move blocked dir=%d", dir))
		if i >= 0 && i < len(g.thingMoveDir) {
			// Doom P_Move clears movedir before trying usable blocking
			// specials, so a successful door-use path leaves DI_NODIR.
			g.thingMoveDir[i] = monsterDirNoDir
		}
		return g.monsterUseBlockingSpecialLines(i, nx, ny)
	}
	prevX, prevY := x, y
	g.setThingPosFixed(i, nx, ny)
	g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
	g.checkWalkSpecialLinesForActor(prevX, prevY, nx, ny, i, false)
	g.debugMonsterMove(i, fmt.Sprintf("move success dir=%d", dir))
	return true
}

func (g *game) monsterTurnTowardMoveDir(i int) {
	if g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	dir := g.thingMoveDir[i]
	if dir >= monsterDirNoDir {
		return
	}
	angle := g.thingWorldAngle(i, g.m.Things[i]) & (7 << 29)
	delta := int32(angle - (uint32(dir) << 29))
	if delta > 0 {
		angle -= statusAng45
	} else if delta < 0 {
		angle += statusAng45
	}
	g.debugMonsterAngle(i, "turn-movedir", angle)
	g.setThingWorldAngle(i, angle)
}

func (g *game) monsterAttack(i int, typ int16, dist int64) bool {
	if !g.monsterHasTarget(i) {
		return false
	}
	meleeOnly := isMeleeOnlyMonster(typ)
	var sx, sy int64
	if i >= 0 && g.m != nil && i < len(g.m.Things) {
		sx, sy = g.thingPosFixed(i, g.m.Things[i])
	}
	targetX, targetY, _, _, _, ok := g.monsterTargetPos(i)
	if !ok {
		return false
	}
	if monsterAttackCallsFaceTarget(typ) {
		g.faceMonsterToward(i, sx, sy, targetX, targetY)
	}
	if dist <= monsterMeleeRange && monsterHasMeleeAttack(typ) {
		damage := monsterMeleeDamage(typ)
		if damage > 0 {
			if ev := monsterMeleeAttackSoundEvent(typ); ev >= 0 {
				g.emitSoundEventAt(ev, sx, sy)
			}
			g.damageMonsterTarget(i, damage, "Monster hit you", sx, sy)
			return true
		}
	}
	if meleeOnly {
		return false
	}
	if typ == 3004 {
		// Zombieman: single bullet with Doom-style spread and chance to miss.
		g.emitSoundEventAt(soundEventShootPistol, sx, sy)
		g.monsterHitscanAttack(i, typ, sx, sy, 1)
		return true
	}
	if typ == 9 {
		// Sergeant: 3 pellets.
		g.emitSoundEventAt(soundEventShootShotgun, sx, sy)
		g.monsterHitscanAttack(i, typ, sx, sy, 3)
		return true
	}
	if typ == 65 {
		// Chaingunner uses Doom's A_CPosAttack, which starts sfx_shotgn.
		g.emitSoundEventAt(soundEventShootShotgun, sx, sy)
		g.monsterHitscanAttack(i, typ, sx, sy, 1)
		return true
	}
	if typ == 84 {
		// WolfSS uses the chaingunner-style single hitscan attack action.
		g.emitSoundEventAt(soundEventShootShotgun, sx, sy)
		g.monsterHitscanAttack(i, typ, sx, sy, 1)
		return true
	}
	if typ == 7 {
		// Spider mastermind repeats the shotgun-guy action in its attack sequence.
		g.emitSoundEventAt(soundEventShootShotgun, sx, sy)
		g.monsterHitscanAttack(i, typ, sx, sy, 3)
		return true
	}
	if typ == 67 {
		const (
			fatSpread     uint32 = 0x08000000
			fatSpreadHalf uint32 = 0x04000000
		)
		phase := 0
		if i >= 0 && i < len(g.thingAttackPhase) {
			phase = g.thingAttackPhase[i]
		}
		switch phase {
		case 1:
			if !g.spawnMonsterProjectileAngleOffset(i, typ, fatSpread) {
				return false
			}
			if !g.spawnMonsterProjectileAngleOffset(i, typ, fatSpread*2) {
				return false
			}
		case 4:
			if !g.spawnMonsterProjectileAngleOffset(i, typ, ^fatSpread+1) {
				return false
			}
			if !g.spawnMonsterProjectileAngleOffset(i, typ, ^(fatSpread*2)+1) {
				return false
			}
		case 7:
			if !g.spawnMonsterProjectileAngleOffset(i, typ, ^fatSpreadHalf+1) {
				return false
			}
			if !g.spawnMonsterProjectileAngleOffset(i, typ, fatSpreadHalf) {
				return false
			}
		default:
			return false
		}
		return true
	}
	if typ == 71 {
		if i < 0 || g.m == nil || i >= len(g.m.Things) {
			return false
		}
		return g.spawnPainLostSoul(i, g.thingWorldAngle(i, g.m.Things[i]))
	}
	if typ == 64 {
		if !g.monsterHasLOSTarget(i, typ, sx, sy) {
			return false
		}
		g.damageMonsterTarget(i, 20, "Arch-Vile blast", sx, sy)
		if i >= len(g.thingTargetPlayer) || i >= len(g.thingTargetIdx) || g.thingTargetPlayer[i] || g.thingTargetIdx[i] < 0 {
			g.p.momz = 10 * fracUnit
		}
		return true
	}
	if usesMonsterProjectile(typ) {
		return g.spawnMonsterProjectile(i, typ)
	}
	damage := monsterRangedDamage(typ)
	if damage <= 0 {
		return false
	}
	g.emitSoundEventAt(soundEventShootPistol, sx, sy)
	g.damageMonsterTarget(i, damage, "Monster shot you", sx, sy)
	return true
}

func (g *game) monsterAimAngleToTarget(i int, sx, sy int64) uint32 {
	tx, ty, _, _, _, ok := g.monsterTargetPos(i)
	if !ok {
		return 0
	}
	angle := angleToThing(sx, sy, tx, ty)
	if i >= 0 && i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] && g.playerInvisible() {
		angle += uint32(int32(doomrand.PRandom()-doomrand.PRandom()) << 21)
	}
	return angle
}

func (g *game) damageMonsterTarget(i, damage int, msg string, attackerX, attackerY int64) {
	if g == nil || i < 0 {
		return
	}
	if i >= len(g.thingTargetPlayer) || i >= len(g.thingTargetIdx) || (i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i]) {
		g.damagePlayerFrom(damage, msg, attackerX, attackerY, true)
		return
	}
	targetIdx, ok := g.monsterTargetThingIdx(i)
	if !ok {
		g.damagePlayerFrom(damage, msg, attackerX, attackerY, true)
		return
	}
	g.damageShootableThingFrom(targetIdx, damage, false, i)
}

func (g *game) countActiveThingType(typ int16) int {
	if g == nil || g.m == nil {
		return 0
	}
	count := 0
	for i, th := range g.m.Things {
		if th.Type != typ {
			continue
		}
		if i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		count++
	}
	return count
}

func (g *game) spawnPainLostSoul(sourceIdx int, angle uint32) bool {
	if g == nil || g.m == nil || sourceIdx < 0 || sourceIdx >= len(g.m.Things) {
		return false
	}
	if g.countActiveThingType(3006) > 20 {
		return false
	}
	src := g.m.Things[sourceIdx]
	sx, sy := g.thingPosFixed(sourceIdx, src)
	sz, _, _ := g.thingSupportState(sourceIdx, src)
	prestep := 4*fracUnit + (3*(monsterRadius(src.Type)+monsterRadius(3006)))/2
	x := sx + fixedMul(prestep, doomFineCosine(angle))
	y := sy + fixedMul(prestep, doomFineSineAtAngle(angle))
	z := sz + 8*fracUnit
	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
		return false
	}
	tmfloor, tmceil := g.sectorFloor[sec], g.sectorCeil[sec]
	idx := g.appendRuntimeThing(mapdata.Thing{
		X:     int16(x >> fracBits),
		Y:     int16(y >> fracBits),
		Angle: worldAngleToThingDeg(angle),
		Type:  3006,
	}, false)
	if idx < 0 {
		return false
	}
	g.setThingPosFixed(idx, x, y)
	g.setThingSupportState(idx, z, tmfloor, tmceil)
	g.setThingWorldAngle(idx, angle)
	g.thingHP[idx] = monsterSpawnHealth(3006)
	g.thingAggro[idx] = true
	g.thingReactionTics[idx] = 0
	g.thingState[idx] = monsterStateSee
	g.thingStatePhase[idx] = 0
	g.thingStateTics[idx] = monsterSeeStateTics(3006, g.fastMonstersActive())
	return true
}

func monsterCanBeResurrected(typ int16) bool {
	if !isMonster(typ) || !monsterLeavesCorpse(typ) {
		return false
	}
	switch typ {
	case 64, 7, 16:
		return false
	default:
		return true
	}
}

func (g *game) archvileTryRaiseCorpse(vileIdx int) bool {
	if g == nil || g.m == nil || vileIdx < 0 || vileIdx >= len(g.m.Things) {
		return false
	}
	vile := g.m.Things[vileIdx]
	vx, vy := g.thingPosFixed(vileIdx, vile)
	for corpseIdx, th := range g.m.Things {
		if corpseIdx == vileIdx || corpseIdx >= len(g.thingDead) || !g.thingDead[corpseIdx] {
			continue
		}
		if corpseIdx < len(g.thingCollected) && g.thingCollected[corpseIdx] {
			continue
		}
		if !monsterCanBeResurrected(th.Type) {
			continue
		}
		cx, cy := g.thingPosFixed(corpseIdx, th)
		if doomApproxDistance(cx-vx, cy-vy) > 64*fracUnit {
			continue
		}
		if corpseIdx < len(g.thingHP) {
			g.thingHP[corpseIdx] = monsterSpawnHealth(th.Type)
		}
		if corpseIdx < len(g.thingDead) {
			g.thingDead[corpseIdx] = false
		}
		if corpseIdx < len(g.thingDeathTics) {
			g.thingDeathTics[corpseIdx] = 0
		}
		if corpseIdx < len(g.thingPainTics) {
			g.thingPainTics[corpseIdx] = 0
		}
		if corpseIdx < len(g.thingAttackTics) {
			g.thingAttackTics[corpseIdx] = 0
		}
		if corpseIdx < len(g.thingAttackFireTics) {
			g.thingAttackFireTics[corpseIdx] = -1
		}
		if corpseIdx < len(g.thingState) {
			g.thingState[corpseIdx] = monsterStateSee
		}
		if corpseIdx < len(g.thingStatePhase) {
			g.thingStatePhase[corpseIdx] = 0
		}
		if corpseIdx < len(g.thingStateTics) {
			g.thingStateTics[corpseIdx] = monsterSeeStateTics(th.Type, g.fastMonstersActive())
		}
		if corpseIdx < len(g.thingJustAtk) {
			g.thingJustAtk[corpseIdx] = false
		}
		if corpseIdx < len(g.thingJustHit) {
			g.thingJustHit[corpseIdx] = false
		}
		if corpseIdx < len(g.thingAggro) {
			g.thingAggro[corpseIdx] = true
		}
		if corpseIdx < len(g.thingReactionTics) {
			g.thingReactionTics[corpseIdx] = 0
		}
		return true
	}
	return false
}

func monsterAttackCallsFaceTarget(typ int16) bool {
	switch typ {
	case 3004, 9, 84, 65: // zombieman, sergeant, ss, chaingunner
		return true
	case 3001, 3002, 58, 3005, 3006: // imp, demon/spectre, caco, lost soul
		return true
	case 16, 68, 7: // cyberdemon, arachnotron, spider mastermind
		return true
	default:
		return false
	}
}

func monsterMeleeAttackSoundEvent(typ int16) soundEvent {
	switch typ {
	case 3001, 3003, 69:
		return soundEventMonsterAttackClaw
	case 3002, 58:
		return -1
	case 3006:
		return soundEventMonsterAttackSkull
	default:
		return -1
	}
}

func monsterAttackStateEntrySoundEvent(typ int16) soundEvent {
	switch typ {
	case 3002, 58:
		return soundEventMonsterAttackSgt
	case 64:
		return soundEventMonsterAttackArchvile
	case 67:
		return soundEventMonsterAttackMancubus
	default:
		return -1
	}
}

func (g *game) monsterHitscanAttack(i int, typ int16, sx, sy int64, pellets int) {
	if pellets <= 0 {
		return
	}
	baseAngle := g.monsterAimAngleToTarget(i, sx, sy)
	actor := g.monsterLineAttackActor(i, typ)
	slope, ok := g.aimLineAttack(actor, baseAngle, monsterAttackRange)
	if !ok {
		slope = 0
	}
	for pellet := 0; pellet < pellets; pellet++ {
		angle := addDoomAngleSpread(baseAngle, doomMonsterSpreadShift)
		damage := 3 * (1 + doomrand.PRandom()%5)
		outcome := g.lineAttackTrace(actor, angle, monsterAttackRange, slope, true)
		g.applyLineAttackOutcome(actor, outcome, damage)
	}
}

func monsterMoveStep(typ int16, fast bool) int64 {
	scale := int64(1)
	if fast {
		scale = 2
	}
	switch typ {
	case 3004, 9, 3001, 84, 65:
		return 8 * fracUnit * scale
	case 3002, 58:
		return 10 * fracUnit * scale
	case 3005, 3003, 69:
		return 8 * fracUnit * scale
	case 66:
		return 10 * fracUnit * scale
	case 16:
		return 16 * fracUnit * scale
	case 7, 68:
		return 12 * fracUnit * scale
	case 67, 71:
		return 8 * fracUnit * scale
	case 64:
		return 15 * fracUnit * scale
	case 3006:
		return 8 * fracUnit * scale
	default:
		return 8 * fracUnit * scale
	}
}

func monsterLeavesCorpse(typ int16) bool {
	switch typ {
	case 3006: // lost soul
		return false
	default:
		return true
	}
}

func (g *game) monsterVisibleAfterDeath(i int, typ int16) bool {
	if g == nil || i < 0 || i >= len(g.thingDead) || !g.thingDead[i] {
		return true
	}
	if monsterLeavesCorpse(typ) {
		return true
	}
	return i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0
}

func monsterAttackCooldown(typ int16, fast bool) int {
	scale := 1
	if fast {
		scale = 2
	}
	switch typ {
	case 9:
		base := 22 + doomPRandomN(10)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	case 3004, 65, 84:
		base := 28 + doomPRandomN(12)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	case 3002, 3006, 58:
		base := 18 + doomPRandomN(8)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	default:
		base := monsterAttackTics + doomPRandomN(10)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	}
}

func (g *game) fastMonstersActive() bool {
	return g.opts.FastMonsters || g.opts.SkillLevel == 5
}

func isMeleeOnlyMonster(typ int16) bool {
	switch typ {
	case 3002, 3006, 58:
		return true
	default:
		return false
	}
}

func monsterHasMeleeAttack(typ int16) bool {
	switch typ {
	case 3001, 3002, 3003, 3006, 58, 66, 69:
		return true
	default:
		return false
	}
}

func monsterMeleeDamage(typ int16) int {
	switch typ {
	case 3002, 58: // demon/spectre
		return 4 * (1 + doomPRandomN(10))
	case 3006: // lost soul
		return 3 * (1 + doomPRandomN(8))
	case 3001: // imp
		return 3 * (1 + doomPRandomN(8))
	case 3003, 69: // baron/hell knight
		return 10 * (1 + doomPRandomN(8))
	case 66: // revenant
		return 6 * (1 + doomPRandomN(10))
	default:
		return 3 * (1 + doomPRandomN(8))
	}
}

func monsterRangedDamage(typ int16) int {
	switch typ {
	case 3004, 84: // zombieman/wolfenstein-ss hitscan-like
		return 3 * (1 + doomPRandomN(5))
	case 65: // chaingunner (single burst approximation)
		return 3 * (1 + doomPRandomN(5))
	case 9: // sergeant pellets
		pellets := 3
		dmg := 0
		for p := 0; p < pellets; p++ {
			dmg += 3 * (1 + doomPRandomN(5))
		}
		return dmg
	case 3001: // imp fireball
		return 3 * (1 + doomPRandomN(8))
	case 3005: // caco ball
		return 5 * (1 + doomPRandomN(8))
	case 3003, 69: // baron/hell knight ball
		return 8 * (1 + doomPRandomN(8))
	case 16: // rocket-like
		return 20 + doomPRandomN(60)
	case 66: // revenant tracer-like
		return 10 * (1 + doomPRandomN(8))
	case 67, 68: // mancubus/arachnotron approximation
		return 5 * (1 + doomPRandomN(8))
	case 64: // archvile flame approximation
		return 20 + doomPRandomN(20)
	case 7: // spider mastermind chaingun-like burst approximation
		dmg := 0
		for i := 0; i < 3; i++ {
			dmg += 3 * (1 + doomPRandomN(5))
		}
		return dmg
	default:
		return 3 * (1 + doomPRandomN(8))
	}
}

func (g *game) actorHasLOS(ax, ay, az, aheight, bx, by, bz, bheight int64) bool {
	if g == nil {
		return false
	}
	dx := bx - ax
	dy := by - ay
	totalDist := math.Hypot(float64(dx), float64(dy))
	if totalDist <= 0 {
		return true
	}

	sightZStart := az + aheight - (aheight >> 2)
	topSlope := float64((bz+bheight)-sightZStart) / totalDist
	bottomSlope := float64(bz-sightZStart) / totalDist

	intercepts := g.losInterceptScratch[:0]
	for i, ld := range g.lines {
		frac, ok := segmentIntersectFrac(ax, ay, bx, by, ld.x1, ld.y1, ld.x2, ld.y2)
		if !ok || frac <= 0 || frac >= 1 {
			continue
		}
		intercepts = insertInterceptOrdered(intercepts, intercept{frac: frac, line: i})
	}
	g.losInterceptScratch = intercepts[:0]
	if len(intercepts) == 0 {
		return true
	}

	for _, it := range intercepts {
		ld := g.lines[it.line]
		if (ld.flags&mlTwoSided) == 0 || ld.sideNum1 < 0 {
			return false
		}
		front, back := g.physLineSectors(ld)
		if front < 0 || back < 0 || front >= len(g.sectorFloor) || back >= len(g.sectorFloor) || front >= len(g.sectorCeil) || back >= len(g.sectorCeil) {
			return false
		}
		if g.sectorFloor[front] == g.sectorFloor[back] && g.sectorCeil[front] == g.sectorCeil[back] {
			continue
		}
		opentop, openbottom, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			return false
		}
		dist := totalDist * it.frac
		if dist <= 0 {
			continue
		}
		if g.sectorFloor[front] != g.sectorFloor[back] {
			slope := float64(openbottom-sightZStart) / dist
			if slope > bottomSlope {
				bottomSlope = slope
			}
		}
		if g.sectorCeil[front] != g.sectorCeil[back] {
			slope := float64(opentop-sightZStart) / dist
			if slope < topSlope {
				topSlope = slope
			}
		}
		if topSlope <= bottomSlope {
			return false
		}
	}
	return true
}

func insertInterceptOrdered(intercepts []intercept, next intercept) []intercept {
	idx := len(intercepts)
	for idx > 0 && next.frac < intercepts[idx-1].frac {
		idx--
	}
	intercepts = append(intercepts, intercept{})
	copy(intercepts[idx+1:], intercepts[idx:])
	intercepts[idx] = next
	return intercepts
}

func (g *game) playerHasLOSMonster(i int, th mapdata.Thing) bool {
	if g == nil || g.m == nil || len(g.sectorFloor) == 0 {
		return true
	}
	tx, ty := g.thingPosFixed(i, th)
	tz, _, _ := g.monsterSupportHeights(i, th)
	return g.actorHasLOS(g.p.x, g.p.y, g.p.z, playerHeight, tx, ty, tz, monsterHeight(th.Type))
}

func (g *game) monsterHasLOSPlayer(typ int16, x, y int64) bool {
	return g.monsterHasLOSPlayerAt(-1, typ, x, y)
}

func (g *game) monsterHasLOSPlayerAt(i int, typ int16, x, y int64) bool {
	if g == nil || g.m == nil || len(g.sectorFloor) == 0 {
		return true
	}
	if typ == 0 {
		typ = 3004
	}
	z := g.thingFloorZ(x, y)
	if i >= 0 && i < len(g.m.Things) {
		z, _, _ = g.monsterSupportHeights(i, g.m.Things[i])
	} else {
		for i, th := range g.m.Things {
			tx, ty := g.thingPosFixed(i, th)
			if tx != x || ty != y || th.Type != typ {
				continue
			}
			z, _, _ = g.monsterSupportHeights(i, th)
			break
		}
	}
	return g.actorHasLOS(x, y, z, monsterHeight(typ), g.p.x, g.p.y, g.p.z, playerHeight)
}

func (g *game) monsterSupportHeights(i int, th mapdata.Thing) (int64, int64, int64) {
	return g.thingSupportState(i, th)
}

func (g *game) monsterHeardPlayer(i int, tx, ty int64) bool {
	_, wake := g.monsterAcquireSectorSoundTarget(i, tx, ty)
	return wake
}

func (g *game) monsterAcquireSectorSoundTarget(i int, tx, ty int64) (hasSoundTarget bool, wake bool) {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return false, false
	}
	sec := g.sectorAt(tx, ty)
	if sec < 0 || sec >= len(g.sectorSoundTarget) || !g.sectorSoundTarget[sec] {
		return false, false
	}
	g.setMonsterTargetPlayer(i)
	if int(g.m.Things[i].Flags)&thingFlagAmbush != 0 {
		return true, g.monsterHasLOSPlayerAt(i, g.m.Things[i].Type, tx, ty)
	}
	return true, true
}

func (g *game) monsterLookForPlayer(i int, allAround bool, tx, ty int64) bool {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) || g.isDead {
		return false
	}
	look := 0
	if i >= 0 && i < len(g.thingLastLook) {
		look = g.thingLastLook[i] & 3
	}

	stop := (look - 1) & 3
	count := 0
	for {
		if g.monsterPlayerSlotActive(look) {
			count++
			if count > 2 || look == stop {
				if i >= 0 && i < len(g.thingLastLook) {
					g.thingLastLook[i] = look
				}
				return false
			}
			if !g.monsterHasLOSPlayerAt(i, g.m.Things[i].Type, tx, ty) {
				look = (look + 1) & 3
				continue
			}
			if !allAround {
				angleToPlayer := math.Atan2(float64(g.p.y-ty), float64(g.p.x-tx)) * (180.0 / math.Pi)
				if angleToPlayer < 0 {
					angleToPlayer += 360
				}
				actorAngle := float64(g.thingWorldAngle(i, g.m.Things[i])) * (360.0 / 4294967296.0)
				delta := angleToPlayer - actorAngle
				for delta < 0 {
					delta += 360
				}
				for delta >= 360 {
					delta -= 360
				}
				if delta > 90 && delta < 270 {
					dist := hypotFixed(g.p.x-tx, g.p.y-ty)
					if dist > monsterMeleeRange {
						look = (look + 1) & 3
						continue
					}
				}
			}
			if i >= 0 && i < len(g.thingLastLook) {
				g.thingLastLook[i] = look
			}
			g.setMonsterTargetPlayer(i)
			return true
		}
		look = (look + 1) & 3
	}
}

func (g *game) monsterPlayerSlotActive(slot int) bool {
	if g == nil {
		return false
	}
	activeSlot := g.localSlot - 1
	if activeSlot < 0 || activeSlot >= 4 {
		activeSlot = 0
	}
	return slot >= 0 && slot < 4 && slot == activeSlot
}

func (g *game) propagateNoiseAlertFrom(x, y int64) {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		return
	}
	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return
	}
	best := make([]int, len(g.m.Sectors))
	for i := range best {
		best[i] = -1
	}
	g.propagateSectorNoise(sec, 0, best)
}

func (g *game) propagateSectorNoise(sec int, soundBlocks int, best []int) {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return
	}
	traversed := soundBlocks + 1
	if sec < len(best) && best[sec] != -1 && best[sec] <= traversed {
		return
	}
	if sec < len(best) {
		best[sec] = traversed
	}
	if sec < len(g.sectorSoundTarget) {
		g.sectorSoundTarget[sec] = true
	}
	for _, ld := range g.lines {
		front, back := g.physLineSectors(ld)
		if front != sec && back != sec {
			continue
		}
		if back < 0 {
			continue
		}
		_, _, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			continue
		}
		other := front
		if other == sec {
			other = back
		}
		if other < 0 || other >= len(g.m.Sectors) {
			continue
		}
		if (ld.flags & lineSoundBlock) != 0 {
			if soundBlocks == 0 {
				g.propagateSectorNoise(other, 1, best)
			}
			continue
		}
		g.propagateSectorNoise(other, soundBlocks, best)
	}
}

func (g *game) physLineSectors(ld physLine) (int, int) {
	if g == nil || g.m == nil {
		return -1, -1
	}
	front := -1
	back := -1
	if ld.sideNum0 >= 0 && int(ld.sideNum0) < len(g.m.Sidedefs) {
		front = int(g.m.Sidedefs[int(ld.sideNum0)].Sector)
	}
	if ld.sideNum1 >= 0 && int(ld.sideNum1) < len(g.m.Sidedefs) {
		back = int(g.m.Sidedefs[int(ld.sideNum1)].Sector)
	}
	return front, back
}

func (g *game) moveMonsterToward(i int, typ int16, x, y, tx, ty, step int64) {
	ang := math.Atan2(float64(ty-y), float64(tx-x))
	g.faceMonsterToward(i, x, y, tx, ty)
	if typ == 3001 {
		// Imps in Doom don't steer perfectly every tic; add small random drift.
		switch doomPRandomN(5) {
		case 0:
			ang += math.Pi / 8
		case 1:
			ang -= math.Pi / 8
		}
	}
	dx := int64(math.Cos(ang) * float64(step))
	dy := int64(math.Sin(ang) * float64(step))
	nx := x + dx
	ny := y + dy
	if tmfloor, tmceil, ok := g.tryMoveProbeMonster(i, typ, nx, ny); ok {
		g.setThingPosFixed(i, nx, ny)
		g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
		return
	}
	if tmfloor, tmceil, ok := g.tryMoveProbeMonster(i, typ, x+dx, y); ok {
		g.setThingPosFixed(i, x+dx, y)
		g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
		return
	}
	if tmfloor, tmceil, ok := g.tryMoveProbeMonster(i, typ, x, y+dy); ok {
		g.setThingPosFixed(i, x, y+dy)
		g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
	}
}

func (g *game) faceMonsterToward(i int, fromX, fromY, toX, toY int64) {
	if g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	if fromX == toX && fromY == toY {
		return
	}
	angle := doomPointToAngle2(fromX, fromY, toX, toY)
	g.debugMonsterAngle(i, "face-target", angle)
	g.setThingWorldAngle(i, angle)
}

func (g *game) debugMonsterAngle(i int, src string, angle uint32) {
	if g == nil || os.Getenv("GD_DEBUG_MONSTER_ANGLE") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(os.Getenv("GD_DEBUG_MONSTER_ANGLE"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantIdx >= 0 && i != wantIdx {
		return
	}
	if g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	tx, ty := int64(0), int64(0)
	if g.m != nil && i >= 0 && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	fmt.Printf("monster-angle-debug tic=%d world=%d idx=%d type=%d src=%s angle=%d deg=%d pos=(%d,%d) movedir=%d movecount=%d target=(%d,%d)\n",
		g.demoTick-1, g.worldTic, i, g.m.Things[i].Type, src, angle, worldAngleToThingDeg(angle), tx, ty, g.thingMoveDir[i], g.thingMoveCount[i], g.p.x, g.p.y)
}

func (g *game) debugMonsterAttack(i int, src string, nextAttackTics int) {
	if g == nil || os.Getenv("GD_DEBUG_MONSTER_ATTACK") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(os.Getenv("GD_DEBUG_MONSTER_ATTACK"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantIdx >= 0 && i != wantIdx {
		return
	}
	if g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	tx, ty := int64(0), int64(0)
	if g.m != nil && i >= 0 && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	fmt.Printf("monster-attack-debug tic=%d world=%d idx=%d type=%d src=%s next_attack_tics=%d attack_tics=%d fire_tics=%d pos=(%d,%d) target=(%d,%d)\n",
		g.demoTick-1, g.worldTic, i, g.m.Things[i].Type, src, nextAttackTics, g.thingAttackTics[i], g.thingAttackFireTics[i], tx, ty, g.p.x, g.p.y)
}

func (g *game) tryMoveProbe(x, y int64) bool {
	if g.m == nil || len(g.m.Sectors) == 0 {
		return false
	}
	_, _, _, ok := g.checkPositionForActor(x, y, 20*fracUnit, true, -1, true)
	return ok
}

func (g *game) tryMoveProbeMonster(i int, typ int16, x, y int64) (int64, int64, bool) {
	if g.m == nil || len(g.m.Sectors) == 0 || i < 0 || i >= len(g.m.Things) {
		return 0, 0, false
	}
	tmfloor, tmceil, tmdrop, ok := g.checkPositionForActor(x, y, monsterRadius(typ), true, i, true)
	g.debugMonsterMove(i, fmt.Sprintf("probe to=(%d,%d) ok=%v floor=%d ceil=%d drop=%d", x, y, ok, tmfloor, tmceil, tmdrop))
	if !ok {
		return 0, 0, false
	}
	height := monsterHeight(typ)
	z, _, _ := g.thingSupportState(i, g.m.Things[i])
	if tmceil-tmfloor < height {
		return 0, 0, false
	}
	if tmceil-z < height {
		return 0, 0, false
	}
	if tmfloor-z > stepHeight {
		return 0, 0, false
	}
	if !monsterCanDropOff(typ) && !monsterCanFloat(typ) && tmfloor-tmdrop > stepHeight {
		return 0, 0, false
	}
	return tmfloor, tmceil, true
}

func (g *game) blockedSpecialLinesForMonsterMove(i int, x, y int64) []int {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return nil
	}
	radius := monsterRadius(g.m.Things[i].Type)
	tmboxTop := y + radius
	tmboxBottom := y - radius
	tmboxRight := x + radius
	tmboxLeft := x - radius
	lines := make([]int, 0, 4)

	g.validCount++
	processPhysLine := func(physIdx int) {
		if physIdx < 0 || physIdx >= len(g.lines) {
			return
		}
		if g.lineValid[physIdx] == g.validCount {
			return
		}
		g.lineValid[physIdx] = g.validCount
		ld := g.lines[physIdx]
		if tmboxRight <= ld.bbox[3] || tmboxLeft >= ld.bbox[2] || tmboxTop <= ld.bbox[1] || tmboxBottom >= ld.bbox[0] {
			return
		}
		box := [4]int64{tmboxTop, tmboxBottom, tmboxRight, tmboxLeft}
		if g.boxOnLineSide(box, ld) != -1 {
			return
		}
		blocked := false
		switch {
		case ld.sideNum1 < 0:
			blocked = true
		case (ld.flags & mlBlocking) != 0:
			blocked = true
		case (ld.flags & mlBlockMonsters) != 0:
			blocked = true
		default:
			_, _, _, openrange := g.lineOpening(ld)
			blocked = openrange <= 0
		}
		if blocked && ld.idx >= 0 && ld.idx < len(g.lineSpecial) && g.lineSpecial[ld.idx] != 0 {
			lines = append(lines, ld.idx)
		}
	}

	iter := func(lineIdx int) bool {
		if lineIdx < 0 || lineIdx >= len(g.physForLine) {
			return true
		}
		processPhysLine(g.physForLine[lineIdx])
		return true
	}

	xl := int((tmboxLeft - g.bmapOriginX) >> (fracBits + 7))
	xh := int((tmboxRight - g.bmapOriginX) >> (fracBits + 7))
	yl := int((tmboxBottom - g.bmapOriginY) >> (fracBits + 7))
	yh := int((tmboxTop - g.bmapOriginY) >> (fracBits + 7))
	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		for bx := xl; bx <= xh; bx++ {
			for by := yl; by <= yh; by++ {
				_ = g.blockLinesIterator(bx, by, iter)
			}
		}
	} else {
		for physIdx := range g.lines {
			processPhysLine(physIdx)
		}
	}
	return lines
}

func (g *game) monsterUseBlockingSpecialLines(i int, x, y int64) bool {
	for _, lineIdx := range g.blockedSpecialLinesForMonsterMove(i, x, y) {
		if g.useSpecialLineForActor(lineIdx, 0, false) {
			return true
		}
	}
	return false
}

func monsterCanFloat(typ int16) bool {
	switch typ {
	case 3005, 3006, 71:
		return true
	default:
		return false
	}
}

func monsterCanDropOff(typ int16) bool {
	switch typ {
	case 3005, 3006, 71:
		return true
	default:
		return false
	}
}

func monsterRadius(typ int16) int64 {
	switch typ {
	case 3004, 9, 65, 84, 3001, 64, 66:
		return 20 * fracUnit
	case 3002, 58:
		return 30 * fracUnit
	case 3005, 71:
		return 31 * fracUnit
	case 3003, 69:
		return 24 * fracUnit
	case 3006:
		return 16 * fracUnit
	case 7:
		return 128 * fracUnit
	case 68:
		return 64 * fracUnit
	case 16:
		return 40 * fracUnit
	case 67:
		return 48 * fracUnit
	default:
		return 20 * fracUnit
	}
}

func monsterHeight(typ int16) int64 {
	switch typ {
	case 3003, 69, 67, 68:
		return 64 * fracUnit
	case 16:
		return 110 * fracUnit
	case 7:
		return 100 * fracUnit
	default:
		return 56 * fracUnit
	}
}

func hypotFixed(dx, dy int64) int64 {
	return int64(math.Hypot(float64(dx), float64(dy)))
}

func doomApproxDistance(dx, dy int64) int64 {
	dx = abs(dx)
	dy = abs(dy)
	if dx < dy {
		dx, dy = dy, dx
	}
	return dx + dy/2
}

func doomPRandomN(n int) int {
	if n <= 0 {
		return 0
	}
	return doomrand.PRandom() % n
}
