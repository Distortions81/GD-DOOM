package doomruntime

import "gddoom/internal/mapdata"

type monsterRaiseSequence struct {
	state  int
	tics   []int
	frames string
}

// The raise states reverse the ordinary death poses, even for an xdeath corpse.
// These are the original info.c timings; fast mode does not shorten them.
func monsterRaiseFrames(typ int16) monsterRaiseSequence {
	switch typ {
	case 3004:
		return monsterRaiseSequence{203, []int{5, 5, 5, 5}, "KJIH"}
	case 9:
		return monsterRaiseSequence{236, []int{5, 5, 5, 5, 5}, "LKJIH"}
	case 66:
		return monsterRaiseSequence{351, []int{5, 5, 5, 5, 5, 5}, "QPONML"}
	case 67:
		return monsterRaiseSequence{398, []int{5, 5, 5, 5, 5, 5, 5, 5}, "RQPONMLK"}
	case 65:
		return monsterRaiseSequence{435, []int{5, 5, 5, 5, 5, 5, 5}, "NMLKJIH"}
	case 3001:
		return monsterRaiseSequence{470, []int{8, 8, 6, 6, 6}, "MLKJI"}
	case 3002, 58:
		return monsterRaiseSequence{496, []int{5, 5, 5, 5, 5, 5}, "NMLKJI"}
	case 3005:
		return monsterRaiseSequence{516, []int{8, 8, 8, 8, 8, 8}, "LKJIHG"}
	case 3003:
		return monsterRaiseSequence{549, []int{8, 8, 8, 8, 8, 8, 8}, "ONMLKJI"}
	case 69:
		return monsterRaiseSequence{578, []int{8, 8, 8, 8, 8, 8, 8}, "ONMLKJI"}
	case 68:
		return monsterRaiseSequence{660, []int{5, 5, 5, 5, 5, 5, 5}, "PONMLKJ"}
	case 71:
		return monsterRaiseSequence{720, []int{8, 8, 8, 8, 8, 8}, "MLKJIH"}
	case 84:
		return monsterRaiseSequence{758, []int{5, 5, 5, 5, 5}, "MLKJI"}
	default:
		return monsterRaiseSequence{}
	}
}

func (g *game) archvileTryRaiseCorpse(vileIdx int) bool {
	if g == nil || g.m == nil || vileIdx < 0 || vileIdx >= len(g.m.Things) {
		return false
	}
	g.ensureMonsterAIState()
	dir := g.thingMoveDir[vileIdx]
	if dir >= monsterDirNoDir {
		return false
	}
	vx, vy := g.thingPosFixed(vileIdx, g.m.Things[vileIdx])
	step := monsterMoveStep(64, g.fastMonstersActive())
	tryX := vx + fixedMul(step, monsterXSpeed[dir])
	tryY := vy + fixedMul(step, monsterYSpeed[dir])
	corpseIdx := -1
	visit := func(i int) bool {
		if i == vileIdx || i < 0 || i >= len(g.m.Things) || !g.thingDead[i] || g.thingCollected[i] {
			return true
		}
		if g.thingStateTics[i] != -1 {
			return true
		}
		th := g.m.Things[i]
		if len(monsterRaiseFrames(th.Type).tics) == 0 {
			return true
		}
		cx, cy := g.thingPosFixed(i, th)
		maxDist := thingTypeRadius(th.Type) + thingTypeRadius(64)
		if abs(cx-tryX) > maxDist || abs(cy-tryY) > maxDist {
			return true
		}
		// PIT_VileCheck stops corpse thrust before testing its position. The
		// check uses the corpse's existing flags/radius, including zero-sized
		// crusher gibs. P_CheckPosition does not test vertical clearance.
		g.thingMomX[i], g.thingMomY[i] = 0, 0
		_, _, _, fits := g.checkPositionForActor(cx, cy, g.thingCurrentRadius(i, th), true, i, true)
		if !fits {
			return true
		}
		corpseIdx = i
		return false
	}
	if g.bmapWidth > 0 && g.bmapHeight > 0 {
		const searchRadius = 64 * fracUnit
		left := int((tryX - g.bmapOriginX - searchRadius) >> (fracBits + 7))
		right := int((tryX - g.bmapOriginX + searchRadius) >> (fracBits + 7))
		bottom := int((tryY - g.bmapOriginY - searchRadius) >> (fracBits + 7))
		top := int((tryY - g.bmapOriginY + searchRadius) >> (fracBits + 7))
		for bx := left; bx <= right && corpseIdx < 0; bx++ {
			for by := bottom; by <= top && corpseIdx < 0; by++ {
				g.blockThingsIterator(bx, by, visit)
			}
		}
	} else {
		for i := len(g.m.Things) - 1; i >= 0 && corpseIdx < 0; i-- {
			visit(i)
		}
	}
	if corpseIdx < 0 {
		return false
	}
	cx, cy := g.thingPosFixed(corpseIdx, g.m.Things[corpseIdx])
	oldPlayer, oldTarget := g.thingTargetPlayer[vileIdx], g.thingTargetIdx[vileIdx]
	g.setMonsterTargetThing(vileIdx, corpseIdx)
	g.faceMonsterToward(vileIdx, vx, vy, cx, cy)
	g.thingTargetPlayer[vileIdx], g.thingTargetIdx[vileIdx] = oldPlayer, oldTarget
	g.thingState[vileIdx], g.thingStatePhase[vileIdx], g.thingStateTics[vileIdx] = monsterStateHeal, 0, 10
	g.emitSoundEventAt(soundEventMonsterRaise, cx, cy)
	g.startMonsterRaise(corpseIdx)
	return true
}

func (g *game) startMonsterRaise(i int) {
	seq := monsterRaiseFrames(g.m.Things[i].Type)
	g.thingDead[i], g.thingXDeath[i] = false, false
	g.thingHP[i] = monsterSpawnHealth(g.m.Things[i].Type)
	g.thingState[i], g.thingStatePhase[i], g.thingStateTics[i] = monsterStateRaise, 0, seq.tics[0]
	g.thingDoomState[i] = noDoomMonsterState
	g.thingDeathTics[i], g.thingPainTics[i], g.thingAttackTics[i] = 0, 0, 0
	g.thingAttackFireTics[i] = -1
	g.thingJustAtk[i], g.thingJustHit[i], g.thingSkullFly[i], g.thingInFloat[i], g.thingAmbush[i] = false, false, false, false, false
	g.thingAggro[i], g.thingTargetPlayer[i], g.thingTargetIdx[i] = false, false, -1
	// Original flags and full corpse height are restored without relinking or
	// resetting reactiontime, threshold, movedir, or movecount. A crushed corpse
	// keeps its zero height/radius: multiplying its height by four still gives 0.
}

func (g *game) tickMonsterRaiseOrHeal(i int, th mapdata.Thing) {
	g.tickMonsterMomentum(i, th)
	g.thingStateTics[i]--
	if g.thingStateTics[i] > 0 {
		return
	}
	frames := monsterRaiseFrames(th.Type).tics
	if g.thingState[i] == monsterStateHeal {
		frames = []int{10, 10, 10}
	}
	phase := g.thingStatePhase[i] + 1
	if phase < len(frames) {
		g.thingStatePhase[i], g.thingStateTics[i] = phase, frames[phase]
		return
	}
	if monsterUsesExactDoomStateMachine(th.Type) {
		state := monsterDoomSeeState(th.Type)
		if th.Type == 68 {
			// S_BSPI_RAISE7 enters RUN1, whose A_BabyMetal calls A_Chase.
			// The sight delay belongs to A_Look after target reacquisition.
			state = 635
		}
		g.setExactDoomMonsterState(i, th.Type, state)
		return
	}
	g.thingState[i], g.thingStatePhase[i] = monsterStateSee, 0
	g.thingStateTics[i] = g.monsterSeeStateTicsForPhase(i, th.Type)
	g.thingResumeChaseNow[i] = true
	// A raise frame enters RUN1 with its target cleared. A_Chase's direct
	// P_LookForPlayers reacquisition returns without an attack/chase retry.
	g.tickGenericMonsterStateWithAttackReacquire(i, th, false)
}
