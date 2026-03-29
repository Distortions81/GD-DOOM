package doomruntime

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

const (
	monsterWakeRange     = 1024 * fracUnit
	monsterMeleeRange    = 64 * fracUnit
	monsterAttackRange   = 2048 * fracUnit
	monsterAttackTics    = 35
	monsterBaseThreshold = 100

	monsterDiagFrac = 47000
)

type monsterMoveDir uint8

type monsterThinkState uint8

type doomMonsterAction uint8

type doomMonsterStateDef struct {
	tics   int
	next   int
	action doomMonsterAction
}

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

const (
	doomMonsterActionNone doomMonsterAction = iota
	doomMonsterActionLook
	doomMonsterActionChase
	doomMonsterActionFaceTarget
	doomMonsterActionPosAttack
	doomMonsterActionSPosAttack
	doomMonsterActionCPosAttack
	doomMonsterActionCPosRefire
	doomMonsterActionPain
	doomMonsterActionSkullAttack
)

const noDoomMonsterState = -1

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

func monsterUsesExactDoomStateMachine(typ int16) bool {
	switch typ {
	case 3004, 9, 65, 3006:
		return true
	default:
		return false
	}
}

func monsterInitialDoomState(typ int16) int {
	switch typ {
	case 3004:
		return 174
	case 9:
		return 208
	case 65:
		return 406
	case 3006:
		return 721
	default:
		return noDoomMonsterState
	}
}

func monsterDoomSeeState(typ int16) int {
	switch typ {
	case 3004:
		return 176
	case 9:
		return 210
	case 65:
		return 408
	case 3006:
		return 723
	default:
		return noDoomMonsterState
	}
}

func monsterDoomPainState(typ int16) int {
	switch typ {
	case 3004:
		return 187
	case 9:
		return 221
	case 65:
		return 420
	case 3006:
		return 729
	default:
		return noDoomMonsterState
	}
}

func monsterDoomMissileState(typ int16) int {
	switch typ {
	case 3004:
		return 184
	case 9:
		return 218
	case 65:
		return 416
	case 3006:
		return 725
	default:
		return noDoomMonsterState
	}
}

func monsterDoomStateDef(state int) (doomMonsterStateDef, bool) {
	switch state {
	case 174:
		return doomMonsterStateDef{tics: 10, next: 175, action: doomMonsterActionLook}, true
	case 175:
		return doomMonsterStateDef{tics: 10, next: 174, action: doomMonsterActionLook}, true
	case 176:
		return doomMonsterStateDef{tics: 4, next: 177, action: doomMonsterActionChase}, true
	case 177:
		return doomMonsterStateDef{tics: 4, next: 178, action: doomMonsterActionChase}, true
	case 178:
		return doomMonsterStateDef{tics: 4, next: 179, action: doomMonsterActionChase}, true
	case 179:
		return doomMonsterStateDef{tics: 4, next: 180, action: doomMonsterActionChase}, true
	case 180:
		return doomMonsterStateDef{tics: 4, next: 181, action: doomMonsterActionChase}, true
	case 181:
		return doomMonsterStateDef{tics: 4, next: 182, action: doomMonsterActionChase}, true
	case 182:
		return doomMonsterStateDef{tics: 4, next: 183, action: doomMonsterActionChase}, true
	case 183:
		return doomMonsterStateDef{tics: 4, next: 176, action: doomMonsterActionChase}, true
	case 184:
		return doomMonsterStateDef{tics: 10, next: 185, action: doomMonsterActionFaceTarget}, true
	case 185:
		return doomMonsterStateDef{tics: 8, next: 186, action: doomMonsterActionPosAttack}, true
	case 186:
		return doomMonsterStateDef{tics: 8, next: 176, action: doomMonsterActionNone}, true
	case 187:
		return doomMonsterStateDef{tics: 3, next: 188, action: doomMonsterActionNone}, true
	case 188:
		return doomMonsterStateDef{tics: 3, next: 176, action: doomMonsterActionPain}, true
	case 208:
		return doomMonsterStateDef{tics: 10, next: 209, action: doomMonsterActionLook}, true
	case 209:
		return doomMonsterStateDef{tics: 10, next: 208, action: doomMonsterActionLook}, true
	case 210:
		return doomMonsterStateDef{tics: 3, next: 211, action: doomMonsterActionChase}, true
	case 211:
		return doomMonsterStateDef{tics: 3, next: 212, action: doomMonsterActionChase}, true
	case 212:
		return doomMonsterStateDef{tics: 3, next: 213, action: doomMonsterActionChase}, true
	case 213:
		return doomMonsterStateDef{tics: 3, next: 214, action: doomMonsterActionChase}, true
	case 214:
		return doomMonsterStateDef{tics: 3, next: 215, action: doomMonsterActionChase}, true
	case 215:
		return doomMonsterStateDef{tics: 3, next: 216, action: doomMonsterActionChase}, true
	case 216:
		return doomMonsterStateDef{tics: 3, next: 217, action: doomMonsterActionChase}, true
	case 217:
		return doomMonsterStateDef{tics: 3, next: 210, action: doomMonsterActionChase}, true
	case 218:
		return doomMonsterStateDef{tics: 10, next: 219, action: doomMonsterActionFaceTarget}, true
	case 219:
		return doomMonsterStateDef{tics: 10, next: 220, action: doomMonsterActionSPosAttack}, true
	case 220:
		return doomMonsterStateDef{tics: 10, next: 210, action: doomMonsterActionNone}, true
	case 221:
		return doomMonsterStateDef{tics: 3, next: 222, action: doomMonsterActionNone}, true
	case 222:
		return doomMonsterStateDef{tics: 3, next: 210, action: doomMonsterActionPain}, true
	case 406:
		return doomMonsterStateDef{tics: 10, next: 407, action: doomMonsterActionLook}, true
	case 407:
		return doomMonsterStateDef{tics: 10, next: 406, action: doomMonsterActionLook}, true
	case 408:
		return doomMonsterStateDef{tics: 3, next: 409, action: doomMonsterActionChase}, true
	case 409:
		return doomMonsterStateDef{tics: 3, next: 410, action: doomMonsterActionChase}, true
	case 410:
		return doomMonsterStateDef{tics: 3, next: 411, action: doomMonsterActionChase}, true
	case 411:
		return doomMonsterStateDef{tics: 3, next: 412, action: doomMonsterActionChase}, true
	case 412:
		return doomMonsterStateDef{tics: 3, next: 413, action: doomMonsterActionChase}, true
	case 413:
		return doomMonsterStateDef{tics: 3, next: 414, action: doomMonsterActionChase}, true
	case 414:
		return doomMonsterStateDef{tics: 3, next: 415, action: doomMonsterActionChase}, true
	case 415:
		return doomMonsterStateDef{tics: 3, next: 408, action: doomMonsterActionChase}, true
	case 416:
		return doomMonsterStateDef{tics: 10, next: 417, action: doomMonsterActionFaceTarget}, true
	case 417:
		return doomMonsterStateDef{tics: 4, next: 418, action: doomMonsterActionCPosAttack}, true
	case 418:
		return doomMonsterStateDef{tics: 4, next: 419, action: doomMonsterActionCPosAttack}, true
	case 419:
		return doomMonsterStateDef{tics: 1, next: 417, action: doomMonsterActionCPosRefire}, true
	case 420:
		return doomMonsterStateDef{tics: 3, next: 421, action: doomMonsterActionNone}, true
	case 421:
		return doomMonsterStateDef{tics: 3, next: 408, action: doomMonsterActionPain}, true
	case 721:
		return doomMonsterStateDef{tics: 10, next: 722, action: doomMonsterActionLook}, true
	case 722:
		return doomMonsterStateDef{tics: 10, next: 721, action: doomMonsterActionLook}, true
	case 723:
		return doomMonsterStateDef{tics: 6, next: 724, action: doomMonsterActionChase}, true
	case 724:
		return doomMonsterStateDef{tics: 6, next: 723, action: doomMonsterActionChase}, true
	case 725:
		return doomMonsterStateDef{tics: 10, next: 726, action: doomMonsterActionFaceTarget}, true
	case 726:
		return doomMonsterStateDef{tics: 4, next: 727, action: doomMonsterActionSkullAttack}, true
	case 727:
		return doomMonsterStateDef{tics: 4, next: 728, action: doomMonsterActionNone}, true
	case 728:
		return doomMonsterStateDef{tics: 4, next: 727, action: doomMonsterActionNone}, true
	case 729:
		return doomMonsterStateDef{tics: 3, next: 730, action: doomMonsterActionNone}, true
	case 730:
		return doomMonsterStateDef{tics: 3, next: 723, action: doomMonsterActionPain}, true
	default:
		return doomMonsterStateDef{}, false
	}
}

func monsterDoomStateFrameLetter(state int) (byte, bool) {
	switch state {
	case 174, 176, 177, 208, 210, 211, 406, 408, 409, 721, 723:
		return 'A', true
	case 175, 209, 407, 722, 724:
		return 'B', true
	case 725, 727:
		return 'C', true
	case 726, 728:
		return 'D', true
	case 729, 730:
		return 'E', true
	case 178, 179, 212, 213, 410, 411:
		return 'B', true
	case 180, 181, 214, 215, 412, 413:
		return 'C', true
	case 182, 183, 216, 217, 414, 415:
		return 'D', true
	case 184, 186, 218, 220, 416:
		return 'E', true
	case 185, 219, 417:
		return 'F', true
	case 187, 188, 221, 222, 420, 421:
		return 'G', true
	case 418:
		return 'E', true
	case 419:
		return 'F', true
	default:
		return 0, false
	}
}

func monsterDoomCompatState(typ int16, state int) (monsterThinkState, int, int) {
	switch typ {
	case 3004:
		switch {
		case state >= 174 && state <= 175:
			return monsterStateSpawn, state - 174, 0
		case state >= 176 && state <= 183:
			return monsterStateSee, state - 176, 0
		case state >= 184 && state <= 186:
			return monsterStateAttack, 0, state - 184
		case state >= 187 && state <= 188:
			return monsterStatePain, state - 187, 0
		}
	case 9:
		switch {
		case state >= 208 && state <= 209:
			return monsterStateSpawn, state - 208, 0
		case state >= 210 && state <= 217:
			return monsterStateSee, state - 210, 0
		case state >= 218 && state <= 220:
			return monsterStateAttack, 0, state - 218
		case state >= 221 && state <= 222:
			return monsterStatePain, state - 221, 0
		}
	case 65:
		switch {
		case state >= 406 && state <= 407:
			return monsterStateSpawn, state - 406, 0
		case state >= 408 && state <= 415:
			return monsterStateSee, state - 408, 0
		case state >= 416 && state <= 419:
			return monsterStateAttack, 0, state - 416
		case state >= 420 && state <= 421:
			return monsterStatePain, state - 420, 0
		}
	case 3006:
		switch {
		case state >= 721 && state <= 722:
			return monsterStateSpawn, state - 721, 0
		case state >= 723 && state <= 724:
			return monsterStateSee, state - 723, 0
		case state >= 725 && state <= 728:
			return monsterStateAttack, 0, state - 725
		case state >= 729 && state <= 730:
			return monsterStatePain, state - 729, 0
		}
	}
	return monsterStateSee, 0, 0
}

func monsterDoomAttackRemainingTics(state int) int {
	switch state {
	case 184:
		return 26
	case 185:
		return 16
	case 186:
		return 8
	case 218:
		return 30
	case 219:
		return 20
	case 220:
		return 10
	case 416:
		return 19
	case 417:
		return 9
	case 418:
		return 5
	case 419:
		return 1
	case 725:
		return 22
	case 726:
		return 12
	case 727:
		return 8
	case 728:
		return 4
	default:
		return 0
	}
}

func monsterDoomPainRemainingTics(state int) int {
	switch state {
	case 187:
		return 6
	case 188:
		return 3
	case 221:
		return 6
	case 222:
		return 3
	case 420:
		return 6
	case 421:
		return 3
	case 729:
		return 6
	case 730:
		return 3
	default:
		return 0
	}
}

func (g *game) tickMonsters() {
	if g.m == nil {
		return
	}
	g.ensureMonsterAIState()
	for i, th := range g.m.Things {
		g.tickThingThinker(i, th)
	}
}

func (g *game) tickThingThinker(i int, th mapdata.Thing) {
	if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
		return
	}
	if isBarrelThingType(th.Type) {
		g.tickBarrel(i, th)
		return
	}
	if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
		g.tickMonsterMomentum(i, th)
		if i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0 {
			g.thingDeathTics[i]--
		}
		if i >= 0 && i < len(g.thingStateTics) && g.thingState[i] == monsterStateDeath && g.thingStateTics[i] > 0 {
			g.thingStateTics[i]--
			if g.thingStateTics[i] == 0 && i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0 {
				xdeath := i >= 0 && i < len(g.thingXDeath) && g.thingXDeath[i]
				frameTics := monsterDeathFrameTicsForMode(th.Type, xdeath)
				nextPhase := 0
				if i >= 0 && i < len(g.thingStatePhase) {
					nextPhase = g.thingStatePhase[i] + 1
				}
				if nextPhase >= 0 && nextPhase < len(frameTics) {
					if i >= 0 && i < len(g.thingStatePhase) {
						g.thingStatePhase[i] = nextPhase
					}
					g.thingStateTics[i] = frameTics[nextPhase]
					soundPhase := monsterDeathSoundActionPhase(th.Type)
					if xdeath {
						soundPhase = monsterXDeathSoundActionPhase(th.Type)
					}
					if nextPhase == soundPhase {
						if want := runtimeDebugEnv("GD_DEBUG_DEATH_SOUND_TIC"); want != "" {
							var wantTic int
							if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil && (g.demoTick-1 == wantTic || g.worldTic == wantTic) {
								px, py := g.thingPosFixed(i, th)
								fmt.Printf("monster-deathsound-debug tic=%d world=%d idx=%d type=%d phase=%d state=%d tics=%d pos=(%d,%d)\n",
									g.demoTick-1, g.worldTic, i, th.Type, nextPhase, g.thingState[i], g.thingStateTics[i], px, py)
							}
						}
						px, py := g.thingPosFixed(i, th)
						if xdeath {
							g.emitSoundEventAt(soundEventMonsterDeath, px, py)
						} else {
							g.emitSoundEventAt(monsterDeathSoundEventVariant(th.Type), px, py)
						}
					}
				}
			}
		}
		g.debugThingState(i, th, "dead")
		return
	}
	if !isMonster(th.Type) || g.thingHP[i] <= 0 {
		return
	}
	if monsterUsesExactDoomStateMachine(th.Type) {
		g.debugMonsterTick(i, "start")
		g.debugThingState(i, th, "live")
		g.tickMonsterMomentum(i, th)
		g.tickExactDoomMonster(i, th)
		g.debugMonsterTick(i, "end")
		return
	}
	if !g.monsterTargetAlive() && !g.monsterHasExplicitTarget(i) {
		g.clearMonsterTargetState(i)
		return
	}
	g.debugMonsterTick(i, "start")
	g.debugThingState(i, th, "live")
	if th.Type == 88 {
		return
	}
	g.tickMonsterMomentum(i, th)
	tx, ty := g.thingPosFixed(i, th)
	targetX, targetY := int64(0), int64(0)
	dist := int64(0)
	if px, py, _, _, _, ok := g.monsterTargetPos(i); ok {
		targetX, targetY = px, py
		dist = doomApproxDistance(targetX-tx, targetY-ty)
	}

	resumedFromAttack := false
	if i >= 0 && i < len(g.thingResumeChaseNow) && g.thingResumeChaseNow[i] {
		g.thingResumeChaseNow[i] = false
		resumedFromAttack = true
	}
	if i >= 0 && i < len(g.thingAttackTics) && g.thingAttackTics[i] > 0 {
		if g.tickMonsterAttackState(i, th.Type, tx, ty, targetX, targetY, dist) {
			return
		}
		resumedFromAttack = i >= 0 && i < len(g.thingState) && g.thingState[i] != monsterStateAttack
	}
	resumedFromPain := false
	if i >= 0 && i < len(g.thingState) && g.thingState[i] == monsterStatePain {
		if i >= 0 && i < len(g.thingAttackFireTics) {
			g.thingAttackFireTics[i] = -1
		}
		if i >= 0 && i < len(g.thingStateTics) && g.thingStateTics[i] > 0 {
			g.thingStateTics[i]--
			g.syncMonsterPainTics(i, th.Type)
			if g.thingStateTics[i] > 0 {
				return
			}
		}
		frameTics := monsterPainFrameTics(th.Type)
		nextPhase := 0
		if i >= 0 && i < len(g.thingStatePhase) {
			nextPhase = g.thingStatePhase[i] + 1
		}
		if nextPhase >= 0 && nextPhase < len(frameTics) {
			if i >= 0 && i < len(g.thingStatePhase) {
				g.thingStatePhase[i] = nextPhase
			}
			g.thingStateTics[i] = frameTics[nextPhase]
			g.syncMonsterPainTics(i, th.Type)
			if nextPhase == monsterPainActionPhase(th.Type) {
				px, py := g.thingPosFixed(i, th)
				g.emitSoundEventAt(monsterPainSoundEvent(th.Type), px, py)
			}
			return
		}
		g.clearMonsterPainState(i)
		g.resetMonsterIdleOrChaseState(i, th.Type)
		resumedFromPain = true
	}
	if !resumedFromPain && !resumedFromAttack && i >= 0 && i < len(g.thingState) && (g.thingState[i] == monsterStatePain || g.thingState[i] == monsterStateAttack) {
		g.resetMonsterIdleOrChaseState(i, th.Type)
	}

	ranStateEntryAction := false
	if resumedFromPain || resumedFromAttack {
		if stop, ranChase := g.runMonsterIdleOrChaseEntryAction(i, th.Type, tx, ty, resumedFromAttack); stop {
			return
		} else if ranChase {
			ranStateEntryAction = true
		}
	}

	if !resumedFromPain && !resumedFromAttack && !g.monsterAdvanceThinkState(i, th.Type, tx, ty, targetX, targetY, dist) {
		return
	}
	targetX, targetY = 0, 0
	dist = 0
	if px, py, _, _, _, ok := g.monsterTargetPos(i); ok {
		targetX, targetY = px, py
		dist = doomApproxDistance(targetX-tx, targetY-ty)
	}
	if !ranStateEntryAction && i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
		g.thingReactionTics[i]--
	}
	skipThresholdTick := i >= 0 && i < len(g.thingState) && g.thingState[i] == monsterStateAttack
	if !ranStateEntryAction && !skipThresholdTick && i >= 0 && i < len(g.thingThreshold) && g.thingThreshold[i] > 0 {
		if !g.monsterHasTarget(i) {
			g.thingThreshold[i] = 0
		} else {
			g.thingThreshold[i]--
		}
	}
	if !ranStateEntryAction {
		g.monsterTurnTowardMoveDir(i)
	}

	if !ranStateEntryAction && !g.monsterHasTarget(i) {
		hadJustAtk := i >= 0 && i < len(g.thingJustAtk) && g.thingJustAtk[i]
		reacquired, continueChase := g.monsterRunLostTargetChaseState(i, th.Type, tx, ty)
		if !reacquired {
			return
		}
		if resumedFromAttack && !hadJustAtk {
			return
		}
		if !continueChase {
			return
		}
		g.monsterTurnTowardMoveDir(i)
		targetX, targetY, dist = 0, 0, 0
		if px, py, _, _, _, ok := g.monsterTargetPos(i); ok {
			targetX, targetY = px, py
			dist = doomApproxDistance(targetX-tx, targetY-ty)
		}
		if hadJustAtk {
			return
		}
	}

	if !ranStateEntryAction && i >= 0 && i < len(g.thingJustAtk) && g.thingJustAtk[i] {
		g.thingJustAtk[i] = false
		g.monsterPickNewChaseDir(i, th.Type, targetX, targetY)
		return
	}

	if g.monsterCanMeleeTarget(i, th.Type, dist, tx, ty, targetX, targetY) {
		g.faceMonsterToward(i, tx, ty, targetX, targetY)
		if g.startMonsterAttackState(i, th.Type, false) {
			return
		}
	}

	if g.monsterCanTryMissileNow(i) && g.monsterCheckMissileRange(i, th.Type, dist, tx, ty, targetX, targetY) {
		g.faceMonsterToward(i, tx, ty, targetX, targetY)
		if g.startMonsterAttackState(i, th.Type, true) {
			return
		}
	}
	if th.Type == 64 && g.archvileTryRaiseCorpse(i) {
		return
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
	g.debugMonsterTick(i, "end")
}

func (g *game) debugThingState(i int, th mapdata.Thing, tag string) {
	if g.worldTic != debugThingStateTic() || i != debugThingStateIdx() {
		return
	}
	phase, state, tics := 0, 0, 0
	hp, dead := 0, false
	if i >= 0 && i < len(g.thingStatePhase) {
		phase = g.thingStatePhase[i]
	}
	if i >= 0 && i < len(g.thingState) {
		state = int(g.thingState[i])
	}
	if i >= 0 && i < len(g.thingStateTics) {
		tics = g.thingStateTics[i]
	}
	if i >= 0 && i < len(g.thingHP) {
		hp = g.thingHP[i]
	}
	if i >= 0 && i < len(g.thingDead) {
		dead = g.thingDead[i]
	}
	x, y := g.thingPosFixed(i, th)
	fmt.Fprintf(os.Stderr, "thing-debug tic=%d idx=%d tag=%s type=%d hp=%d dead=%t state=%d phase=%d tics=%d pos=(%d,%d)\n",
		g.worldTic, i, tag, th.Type, hp, dead, state, phase, tics, x, y)
}

func debugThingStateIdx() int {
	v := runtimeDebugEnv("GD_DEBUG_THING_IDX")
	if v == "" {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

func debugThingStateTic() int {
	v := runtimeDebugEnv("GD_DEBUG_THING_TIC")
	if v == "" {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

func (g *game) debugMonsterTick(i int, stage string) {
	if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_TICK") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_TICK"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantIdx >= 0 && i != wantIdx {
		return
	}
	if wantTic >= 0 && g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	tx, ty := int64(0), int64(0)
	if g.m != nil && i >= 0 && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	targetIdx := -1
	targetHP := 0
	if i >= 0 && i < len(g.thingTargetIdx) {
		targetIdx = g.thingTargetIdx[i]
		if targetIdx >= 0 && targetIdx < len(g.thingHP) {
			targetHP = g.thingHP[targetIdx]
		}
	}
	attackPhase, attackTics, fireTics := 0, 0, 0
	if i >= 0 && i < len(g.thingAttackPhase) {
		attackPhase = g.thingAttackPhase[i]
	}
	if i >= 0 && i < len(g.thingAttackTics) {
		attackTics = g.thingAttackTics[i]
	}
	if i >= 0 && i < len(g.thingAttackFireTics) {
		fireTics = g.thingAttackFireTics[i]
	}
	fmt.Printf("monster-tick-debug tic=%d world=%d idx=%d type=%d stage=%s state=%d phase=%d statetics=%d attackphase=%d attacktics=%d firetics=%d movedir=%d movecount=%d threshold=%d reaction=%d justatk=%t targetPlayer=%t targetIdx=%d targetHP=%d pos=(%d,%d) angle=%d\n",
		g.demoTick-1, g.worldTic, i, g.m.Things[i].Type, stage,
		g.thingState[i], g.thingStatePhase[i], g.thingStateTics[i], attackPhase, attackTics, fireTics, g.thingMoveDir[i], g.thingMoveCount[i],
		g.thingThreshold[i], g.thingReactionTics[i], g.thingJustAtk[i], g.thingTargetPlayer[i], targetIdx, targetHP, tx, ty, g.thingWorldAngle(i, g.m.Things[i]))
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

func monsterSeeStartPhase(typ int16) int {
	switch typ {
	case 68:
		// Doom enters S_BSPI_SIGHT before the Arachnotron loops RUN1..RUN12.
		return -1
	default:
		return 0
	}
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
		if i >= 0 && i < len(g.thingStatePhase) {
			count := len(monsterSpawnFrameTics(typ))
			if count < 1 {
				count = 1
			}
			g.thingStatePhase[i] = (g.thingStatePhase[i] + 1) % count
		}
		if g.monsterRunLookState(i, typ, tx, ty) {
			return true
		}
		g.setMonsterThinkState(i, typ, monsterStateSpawn, g.monsterSpawnStateTicsForPhase(i, typ))
		return false
	case monsterStateSee:
		if !g.monsterHasTarget(i) {
			if i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
				g.thingReactionTics[i]--
			}
			g.monsterTurnTowardMoveDir(i)
			if reacquired, _ := g.monsterRunLostTargetChaseState(i, typ, tx, ty); reacquired {
				return true
			}
			return false
		}
		if i >= 0 && i < len(g.thingStatePhase) {
			count := len(monsterSeeFrameTics(typ, g.fastMonstersActive()))
			if count < 1 {
				count = 1
			}
			if typ == 68 && g.thingStatePhase[i] < 0 {
				g.thingStatePhase[i] = 0
			} else {
				g.thingStatePhase[i] = (g.thingStatePhase[i] + 1) % count
			}
			g.thingStateTics[i] = g.monsterSeeStateTicsForPhase(i, typ)
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

func (g *game) monsterRunLookState(i int, typ int16, tx, ty int64) bool {
	if i < 0 {
		return false
	}
	if i >= 0 && i < len(g.thingThreshold) {
		g.thingThreshold[i] = 0
	}
	if _, wake := g.monsterAcquireSectorSoundTarget(i, tx, ty); wake || g.monsterLookForPlayer(i, false, tx, ty) {
		if i >= 0 && i < len(g.thingAggro) {
			g.thingAggro[i] = true
		}
		g.emitMonsterSeeSound(i, typ, tx, ty)
		if i >= 0 && i < len(g.thingStatePhase) {
			g.thingStatePhase[i] = monsterSeeStartPhase(typ)
		}
		g.setMonsterThinkState(i, typ, monsterStateSee, g.monsterSeeStateTicsForPhase(i, typ))
		return true
	}
	return false
}

func (g *game) monsterRunLostTargetChaseState(i int, typ int16, tx, ty int64) (reacquired bool, continueChase bool) {
	if i < 0 {
		return false, false
	}
	if i >= 0 && i < len(g.thingThreshold) {
		g.thingThreshold[i] = 0
	}
	if g.monsterLookForPlayer(i, true, tx, ty) {
		if i >= 0 && i < len(g.thingAggro) {
			g.thingAggro[i] = true
		}
		// Direct lost-target reacquire happens from A_Chase, so vanilla keeps the
		// current see-frame progression instead of restarting the walk animation.
		if i < len(g.thingState) && g.thingState[i] != monsterStateSee {
			if i >= 0 && i < len(g.thingStatePhase) {
				g.thingStatePhase[i] = 0
			}
			g.setMonsterThinkState(i, typ, monsterStateSee, g.monsterSeeStateTicsForPhase(i, typ))
		}
		return true, false
	}
	if i >= 0 && i < len(g.thingStatePhase) {
		g.thingStatePhase[i] = 0
	}
	g.setMonsterThinkState(i, typ, monsterStateSpawn, g.monsterSpawnStateTicsForPhase(i, typ))
	if g.monsterRunLookState(i, typ, tx, ty) {
		return true, false
	}
	return false, false
}

func (g *game) monsterIdleOrChaseState(i int) monsterThinkState {
	if g.monsterHasTarget(i) || (i >= 0 && i < len(g.thingAggro) && g.thingAggro[i]) {
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

func (g *game) monsterHasExplicitTarget(i int) bool {
	if g == nil || i < 0 {
		return false
	}
	if i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] {
		return true
	}
	return i < len(g.thingTargetIdx) && g.thingTargetIdx[i] >= 0
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

func (g *game) monsterAttackTargetPos(i int) (x, y, z, height, radius int64, ok bool) {
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
		return g.p.x, g.p.y, g.p.z, playerHeight, playerRadius, true
	}
	return g.monsterTargetPos(i)
}

func (g *game) monsterHasLOSTarget(i int, typ int16, x, y int64) bool {
	if g == nil || i < 0 {
		return false
	}
	fromSector := -1
	if g.m != nil && i < len(g.m.Things) {
		fromSector = g.thingSectorCached(i, g.m.Things[i])
	}
	if i >= len(g.thingTargetPlayer) || i >= len(g.thingTargetIdx) || (i < len(g.thingAggro) && g.thingAggro[i] && !g.thingTargetPlayer[i] && g.thingTargetIdx[i] < 0) {
		if g.sightRejected(fromSector, g.playerSector()) {
			return false
		}
		return g.monsterHasLOSPlayerAt(i, typ, x, y)
	}
	if i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] {
		if g.sightRejected(fromSector, g.playerSector()) {
			return false
		}
		return g.monsterHasLOSPlayerAt(i, typ, x, y)
	}
	targetIdx, ok := g.monsterTargetThingIdx(i)
	if !ok {
		return false
	}
	if g.sightRejected(fromSector, g.thingSectorCached(targetIdx, g.m.Things[targetIdx])) {
		return false
	}
	tx, ty, tz, height, _, ok := g.monsterTargetPos(i)
	if !ok {
		return false
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
	g.clearMonsterPainState(i)
	if i >= 0 && i < len(g.thingStatePhase) {
		g.thingStatePhase[i] = 0
	}
	g.setMonsterThinkState(i, typ, g.monsterIdleOrChaseState(i), g.monsterIdleOrChaseTics(i, typ))
}

func (g *game) runMonsterIdleOrChaseEntryAction(i int, typ int16, tx, ty int64, allowJustAttackedReacquire bool) (stop bool, ranChase bool) {
	if g == nil || i < 0 || i >= len(g.thingState) {
		return false, false
	}
	switch g.thingState[i] {
	case monsterStateSpawn:
		if g.monsterRunLookState(i, typ, tx, ty) {
			return true, false
		}
		return true, false
	case monsterStateSee:
		for pass := 0; pass < 2; pass++ {
			if i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
				g.thingReactionTics[i]--
			}
			if i < len(g.thingThreshold) && g.thingThreshold[i] > 0 {
				if !g.monsterHasTarget(i) {
					g.thingThreshold[i] = 0
				} else {
					g.thingThreshold[i]--
				}
			}
			g.monsterTurnTowardMoveDir(i)
			if !g.monsterHasTarget(i) {
				reacquired, continueChase := g.monsterRunLostTargetChaseState(i, typ, tx, ty)
				if allowJustAttackedReacquire && reacquired && i < len(g.thingJustAtk) && g.thingJustAtk[i] {
					return true, false
				}
				if !reacquired || !continueChase {
					return true, false
				}
				continue
			}
			if i < len(g.thingJustAtk) && g.thingJustAtk[i] {
				g.thingJustAtk[i] = false
				targetX, targetY := int64(0), int64(0)
				if px, py, _, _, _, ok := g.monsterTargetPos(i); ok {
					targetX, targetY = px, py
				}
				if !g.fastMonstersActive() {
					g.monsterPickNewChaseDir(i, typ, targetX, targetY)
				}
				return true, true
			}
			return false, true
		}
		return true, false
	default:
		return false, false
	}
}

func monsterPainRemainingTics(typ int16, phase, stateTics int) int {
	frameTics := monsterPainFrameTics(typ)
	if len(frameTics) == 0 {
		if stateTics > 0 {
			return stateTics
		}
		return 0
	}
	if phase < 0 {
		phase = 0
	}
	if phase >= len(frameTics) {
		return 0
	}
	remaining := max(stateTics, 0)
	for _, t := range frameTics[phase+1:] {
		if t > 0 {
			remaining += t
		}
	}
	return remaining
}

func (g *game) syncMonsterPainTics(i int, typ int16) {
	if g == nil || i < 0 || i >= len(g.thingPainTics) {
		return
	}
	if i >= len(g.thingState) || g.thingState[i] != monsterStatePain {
		g.thingPainTics[i] = 0
		return
	}
	phase := 0
	if i < len(g.thingStatePhase) {
		phase = g.thingStatePhase[i]
	}
	stateTics := 0
	if i < len(g.thingStateTics) {
		stateTics = g.thingStateTics[i]
	}
	g.thingPainTics[i] = monsterPainRemainingTics(typ, phase, stateTics)
}

func (g *game) clearMonsterPainState(i int) {
	if g == nil || i < 0 || i >= len(g.thingPainTics) {
		return
	}
	g.thingPainTics[i] = 0
}

func (g *game) emitMonsterSeeSound(i int, typ int16, x, y int64) {
	ev, fullVolume := monsterSeeSoundEvent(typ)
	if ev < 0 {
		return
	}
	if want := runtimeDebugEnv("GD_DEBUG_MONSTER_RNG_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("monster-rng-debug tic=%d world=%d idx=%d type=%d site=see-sound ev=%d rnd=%d prnd=%d pos=(%d,%d)\n",
					g.demoTick-1, g.worldTic, i, typ, ev, rnd, prnd, x, y)
			}
		}
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
	roll := doomrand.PRandom()
	if want := runtimeDebugEnv("GD_DEBUG_MONSTER_RNG_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("monster-rng-debug tic=%d world=%d idx=%d type=%d site=active-sound roll=%d rnd=%d prnd=%d pos=(%d,%d)\n",
					g.demoTick-1, g.worldTic, i, typ, roll, rnd, prnd, x, y)
			}
		}
	}
	if !shouldEmitMonsterActiveSound(roll) {
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
	if len(g.thingAmbush) != n {
		old := g.thingAmbush
		g.thingAmbush = make([]bool, n)
		copy(g.thingAmbush, old)
		for i := len(old); i < n; i++ {
			g.thingAmbush[i] = int(g.m.Things[i].Flags)&thingFlagAmbush != 0
		}
	}
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
	if len(g.thingInFloat) != n {
		old := g.thingInFloat
		g.thingInFloat = make([]bool, n)
		copy(g.thingInFloat, old)
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
	if len(g.thingDoomState) != n {
		old := g.thingDoomState
		g.thingDoomState = make([]int, n)
		for i := range g.thingDoomState {
			g.thingDoomState[i] = -1
		}
		copy(g.thingDoomState, old)
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
	if len(g.thingXDeath) != n {
		old := g.thingXDeath
		g.thingXDeath = make([]bool, n)
		copy(g.thingXDeath, old)
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
	if len(g.thingSkullFly) != n {
		old := g.thingSkullFly
		g.thingSkullFly = make([]bool, n)
		copy(g.thingSkullFly, old)
	}
	if len(g.thingResumeChaseNow) != n {
		old := g.thingResumeChaseNow
		g.thingResumeChaseNow = make([]bool, n)
		copy(g.thingResumeChaseNow, old)
	}
}

func (g *game) syncExactDoomMonsterCompatState(i int, typ int16) {
	if g == nil || i < 0 || i >= len(g.thingDoomState) {
		return
	}
	state := g.thingDoomState[i]
	if state < 0 {
		return
	}
	compatState, compatPhase, attackPhase := monsterDoomCompatState(typ, state)
	if i < len(g.thingState) {
		g.thingState[i] = compatState
	}
	if i < len(g.thingStatePhase) {
		g.thingStatePhase[i] = compatPhase
	}
	if i < len(g.thingAttackPhase) {
		g.thingAttackPhase[i] = attackPhase
	}
	if i < len(g.thingAttackTics) {
		if compatState == monsterStateAttack {
			g.thingAttackTics[i] = monsterDoomAttackRemainingTics(state)
		} else {
			g.thingAttackTics[i] = 0
		}
	}
	if i < len(g.thingPainTics) {
		if compatState == monsterStatePain {
			g.thingPainTics[i] = monsterDoomPainRemainingTics(state)
		} else {
			g.thingPainTics[i] = 0
		}
	}
	if i < len(g.thingAttackFireTics) && compatState != monsterStateAttack {
		g.thingAttackFireTics[i] = -1
	}
}

func (g *game) setExactDoomMonsterState(i int, typ int16, state int) {
	g.setExactDoomMonsterStateDepth(i, typ, state, 0)
}

func (g *game) setExactDoomMonsterStateDepth(i int, typ int16, state int, depth int) {
	if g == nil || i < 0 || depth > 16 {
		return
	}
	def, ok := monsterDoomStateDef(state)
	if !ok {
		if i < len(g.thingDoomState) {
			g.thingDoomState[i] = noDoomMonsterState
		}
		return
	}
	if i < len(g.thingDoomState) {
		g.thingDoomState[i] = state
	}
	if i < len(g.thingStateTics) {
		g.thingStateTics[i] = def.tics
	}
	g.syncExactDoomMonsterCompatState(i, typ)
	g.runExactDoomMonsterAction(i, typ, state, def.action, depth)
}

func (g *game) runExactDoomMonsterAction(i int, typ int16, state int, action doomMonsterAction, depth int) {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	tx, ty := g.thingPosFixed(i, g.m.Things[i])
	targetX, targetY := int64(0), int64(0)
	dist := int64(0)
	if px, py, _, _, _, ok := g.monsterTargetPos(i); ok {
		targetX, targetY = px, py
		dist = doomApproxDistance(targetX-tx, targetY-ty)
	}
	switch action {
	case doomMonsterActionLook:
		if _, wake := g.monsterAcquireSectorSoundTarget(i, tx, ty); wake || g.monsterLookForPlayer(i, false, tx, ty) {
			if i < len(g.thingAggro) {
				g.thingAggro[i] = true
			}
			g.emitMonsterSeeSound(i, typ, tx, ty)
			g.setExactDoomMonsterStateDepth(i, typ, monsterDoomSeeState(typ), depth+1)
		}
	case doomMonsterActionChase:
		g.runExactDoomMonsterChaseAction(i, typ, tx, ty, targetX, targetY, dist, depth)
	case doomMonsterActionFaceTarget:
		if targetX != 0 || targetY != 0 || g.monsterHasTarget(i) {
			g.faceMonsterToward(i, tx, ty, targetX, targetY)
		}
	case doomMonsterActionPosAttack:
		_ = g.monsterAttack(i, typ, dist)
	case doomMonsterActionSPosAttack:
		_ = g.monsterAttack(i, typ, dist)
	case doomMonsterActionCPosAttack:
		_ = g.monsterAttack(i, typ, dist)
	case doomMonsterActionCPosRefire:
		g.faceMonsterToward(i, tx, ty, targetX, targetY)
		if !g.chaingunnerRefireKeepsAttack(i, typ, tx, ty) {
			g.setExactDoomMonsterStateDepth(i, typ, monsterDoomSeeState(typ), depth+1)
		}
	case doomMonsterActionPain:
		g.emitSoundEventAt(monsterPainSoundEvent(typ), tx, ty)
	case doomMonsterActionSkullAttack:
		_ = g.monsterAttack(i, typ, dist)
	}
}

func (g *game) runExactDoomMonsterChaseAction(i int, typ int16, tx, ty, targetX, targetY, dist int64, depth int) {
	if g == nil {
		return
	}
	if i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
		g.thingReactionTics[i]--
	}
	if i < len(g.thingThreshold) && g.thingThreshold[i] > 0 {
		if !g.monsterHasTarget(i) {
			g.thingThreshold[i] = 0
		} else {
			g.thingThreshold[i]--
		}
	}
	g.monsterTurnTowardMoveDir(i)
	if !g.monsterHasTarget(i) {
		if g.monsterLookForPlayer(i, true, tx, ty) {
			if i < len(g.thingAggro) {
				g.thingAggro[i] = true
			}
			return
		}
		g.setExactDoomMonsterStateDepth(i, typ, monsterInitialDoomState(typ), depth+1)
		return
	}
	if i < len(g.thingJustAtk) && g.thingJustAtk[i] {
		g.thingJustAtk[i] = false
		if !g.fastMonstersActive() {
			g.monsterPickNewChaseDir(i, typ, targetX, targetY)
		}
		return
	}
	if g.monsterCanMeleeTarget(i, typ, dist, tx, ty, targetX, targetY) {
		return
	}
	if g.monsterCanTryMissileNow(i) && g.monsterCheckMissileRange(i, typ, dist, tx, ty, targetX, targetY) {
		g.setExactDoomMonsterStateDepth(i, typ, monsterDoomMissileState(typ), depth+1)
		if i < len(g.thingJustAtk) {
			g.thingJustAtk[i] = true
		}
		return
	}
	if i < len(g.thingMoveCount) {
		g.thingMoveCount[i]--
		if g.thingMoveCount[i] < 0 || !g.monsterMoveInDir(i, typ, g.thingMoveDir[i]) {
			g.monsterPickNewChaseDir(i, typ, targetX, targetY)
		}
	}
	ax, ay := tx, ty
	if g.m != nil && i < len(g.m.Things) {
		ax, ay = g.thingPosFixed(i, g.m.Things[i])
	}
	g.emitMonsterActiveSound(i, typ, ax, ay)
}

func (g *game) tickExactDoomMonster(i int, th mapdata.Thing) bool {
	if !monsterUsesExactDoomStateMachine(th.Type) {
		return false
	}
	if i < 0 || i >= len(g.thingDoomState) {
		return true
	}
	if g.thingDoomState[i] == noDoomMonsterState {
		state := monsterInitialDoomState(th.Type)
		if state != noDoomMonsterState {
			if i < len(g.thingStateTics) && g.thingStateTics[i] <= 0 {
				// Test fixtures often construct monsters without running full spawn
				// initialization. Treat that as "run the spawn state's action now"
				// while real map spawns still enter with positive randomized tics.
				g.setExactDoomMonsterState(i, th.Type, state)
				return true
			}
			g.thingDoomState[i] = state
			g.syncExactDoomMonsterCompatState(i, th.Type)
		}
	}
	if i < len(g.thingStateTics) && g.thingStateTics[i] == 0 {
		def, ok := monsterDoomStateDef(g.thingDoomState[i])
		if ok {
			g.thingStateTics[i] = def.tics
		}
	}
	if i >= len(g.thingStateTics) {
		return true
	}
	if g.thingStateTics[i] != -1 {
		g.thingStateTics[i]--
		if g.thingStateTics[i] == 0 {
			def, ok := monsterDoomStateDef(g.thingDoomState[i])
			if ok {
				g.setExactDoomMonsterState(i, th.Type, def.next)
			}
		}
	}
	g.syncExactDoomMonsterCompatState(i, th.Type)
	return true
}

func (g *game) tickMonsterMomentum(i int, th mapdata.Thing) {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	if i >= len(g.thingMomX) || i >= len(g.thingMomY) || i >= len(g.thingMomZ) {
		return
	}
	if i < len(g.thingSkullFly) && g.thingSkullFly[i] {
		g.tickSkullFlyMomentum(i, th)
		return
	}
	momx := g.thingMomX[i]
	momy := g.thingMomY[i]
	momz := g.thingMomZ[i]
	if momx == 0 && momy == 0 && momz == 0 {
		z, floorZ, ceilZ := g.thingSupportState(i, th)
		if z != floorZ {
			g.thingMomZ[i] = g.tickMonsterZMovement(i, th, z, floorZ, ceilZ, 0)
		}
		return
	}
	momx = clamp(momx, -maxMove, maxMove)
	momy = clamp(momy, -maxMove, maxMove)

	z, _, _ := g.thingSupportState(i, th)
	tx, ty := g.thingPosFixed(i, th)
	xmove := momx
	ymove := momy
	for xmove != 0 || ymove != 0 {
		nx, ny := tx, ty
		if xmove > maxMove/2 || ymove > maxMove/2 {
			nx += xmove >> 1
			ny += ymove >> 1
			xmove >>= 1
			ymove >>= 1
		} else {
			nx += xmove
			ny += ymove
			xmove = 0
			ymove = 0
		}
		if tmfloor, tmceil, _, ok := g.tryMoveProbeMonster(i, th.Type, nx, ny); ok {
			prevX, prevY := tx, ty
			g.setThingPosFixed(i, nx, ny)
			g.setThingSupportState(i, z, tmfloor, tmceil)
			g.checkWalkSpecialLinesForActor(prevX, prevY, nx, ny, i, false)
			tx, ty = nx, ny
			continue
		}
		momx = 0
		momy = 0
		break
	}

	z, floorZ, ceilZ := g.thingSupportState(i, th)
	if z > floorZ || momz != 0 {
		g.thingMomX[i] = momx
		g.thingMomY[i] = momy
		g.thingMomZ[i] = g.tickMonsterZMovement(i, th, z, floorZ, ceilZ, momz)
		return
	}
	if g.corpseShouldSkipFriction(i, th, momx, momy) {
		g.thingMomX[i] = momx
		g.thingMomY[i] = momy
		if z < floorZ {
			g.thingMomZ[i] = g.tickMonsterZMovement(i, th, z, floorZ, ceilZ, momz)
		} else {
			g.thingMomZ[i] = momz
		}
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
	if z < floorZ {
		g.thingMomZ[i] = g.tickMonsterZMovement(i, th, z, floorZ, ceilZ, momz)
		return
	}
	g.thingMomZ[i] = momz
}

func (g *game) tickMonsterZMovement(i int, th mapdata.Thing, z, floorZ, ceilZ, momz int64) int64 {
	if g == nil {
		return momz
	}
	canFloat := monsterCanFloat(th.Type) && !(i >= 0 && i < len(g.thingDead) && g.thingDead[i])
	z += momz
	height := g.thingCurrentHeight(i, th)
	if canFloat && g.monsterHasTarget(i) {
		inFloat := i >= 0 && i < len(g.thingInFloat) && g.thingInFloat[i]
		if !inFloat {
			targetX, targetY, targetZ, _, _, ok := g.monsterTargetPos(i)
			if ok {
				dist := doomApproxDistance(
					g.thingX[i]-targetX,
					g.thingY[i]-targetY,
				)
				delta := (targetZ + (height >> 1)) - z
				if delta < 0 && dist < -(delta*3) {
					z -= 4 * fracUnit
				} else if delta > 0 && dist < delta*3 {
					z += 4 * fracUnit
				}
			}
		}
	}

	if z <= floorZ {
		if momz < 0 {
			momz = 0
		}
		z = floorZ
	} else if !canFloat {
		if momz == 0 {
			momz = -2 * fracUnit
		} else {
			momz -= fracUnit
		}
	}

	if z+height > ceilZ {
		if momz > 0 {
			momz = 0
		}
		z = ceilZ - height
	}

	g.setThingSupportState(i, z, floorZ, ceilZ)
	return momz
}

func (g *game) corpseShouldSkipFriction(i int, th mapdata.Thing, momx, momy int64) bool {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return false
	}
	if i >= len(g.thingDead) || !g.thingDead[i] || !monsterLeavesCorpse(th.Type) {
		return false
	}
	if momx <= fracUnit/4 && momx >= -fracUnit/4 && momy <= fracUnit/4 && momy >= -fracUnit/4 {
		return false
	}
	if i >= len(g.thingFloorState) || i >= len(g.thingSupportValid) || !g.thingSupportValid[i] {
		return false
	}
	sec := g.thingSectorCached(i, th)
	if sec < 0 || sec >= len(g.sectorFloor) {
		return false
	}
	return g.thingFloorState[i] != g.sectorFloor[sec]
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

func (g *game) resetLostSoulCharge(i int, typ int16) {
	if g == nil || i < 0 {
		return
	}
	if i < len(g.thingSkullFly) {
		g.thingSkullFly[i] = false
	}
	if i < len(g.thingMomX) {
		g.thingMomX[i] = 0
	}
	if i < len(g.thingMomY) {
		g.thingMomY[i] = 0
	}
	if i < len(g.thingMomZ) {
		g.thingMomZ[i] = 0
	}
	if i < len(g.thingAttackTics) {
		g.thingAttackTics[i] = 0
	}
	if i < len(g.thingAttackFireTics) {
		g.thingAttackFireTics[i] = -1
	}
	if i < len(g.thingAttackPhase) {
		g.thingAttackPhase[i] = 0
	}
	if i < len(g.thingResumeChaseNow) {
		g.thingResumeChaseNow[i] = false
	}
	if monsterUsesExactDoomStateMachine(typ) {
		g.setExactDoomMonsterState(i, typ, monsterInitialDoomState(typ))
		return
	}
	if i < len(g.thingStatePhase) {
		g.thingStatePhase[i] = 0
	}
	g.setMonsterThinkState(i, typ, monsterStateSpawn, g.monsterSpawnStateTicsForPhase(i, typ))
	tx, ty := int64(0), int64(0)
	if g.m != nil && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	if g.monsterRunLookState(i, typ, tx, ty) {
		if i < len(g.thingStateTics) && g.thingStateTics[i] > 0 {
			g.thingStateTics[i]--
		}
		if i < len(g.thingResumeChaseNow) {
			g.thingResumeChaseNow[i] = true
		}
		return
	}
	if i < len(g.thingStateTics) && g.thingStateTics[i] > 0 {
		g.thingStateTics[i]--
	}
}

func (g *game) lostSoulChargeTargetAt(i int, th mapdata.Thing, x, y, z int64) (lineAttackTarget, bool) {
	if g == nil {
		return lineAttackTarget{}, false
	}
	const maxThingBlockRadius = 32 * fracUnit
	radius := monsterRadius(th.Type)
	if g.stats.Health > 0 && g.playerMobjHealth > 0 && abs(g.p.x-x) < radius+playerRadius && abs(g.p.y-y) < radius+playerRadius {
		return lineAttackTarget{kind: lineAttackTargetPlayer}, true
	}
	if g.m == nil {
		return lineAttackTarget{}, false
	}
	visitThing := func(other int) (lineAttackTarget, bool) {
		if other == i || other < 0 || other >= len(g.m.Things) {
			return lineAttackTarget{}, false
		}
		oth := g.m.Things[other]
		if other < len(g.thingCollected) && g.thingCollected[other] {
			return lineAttackTarget{}, false
		}
		if !thingTypeIsShootable(oth.Type) && !isPickupType(oth.Type) && !thingTypeBlocksActorMovement(oth.Type, true) {
			return lineAttackTarget{}, false
		}
		ox, oy := g.thingPosFixed(other, oth)
		oradius := g.thingCurrentRadius(other, oth)
		if abs(ox-x) >= radius+oradius || abs(oy-y) >= radius+oradius {
			return lineAttackTarget{}, false
		}
		return lineAttackTarget{kind: lineAttackTargetThing, idx: other}, true
	}
	if g.bmapWidth > 0 && g.bmapHeight > 0 {
		left := int((x - radius - g.bmapOriginX - maxThingBlockRadius) >> (fracBits + 7))
		right := int((x + radius - g.bmapOriginX + maxThingBlockRadius) >> (fracBits + 7))
		bottom := int((y - radius - g.bmapOriginY - maxThingBlockRadius) >> (fracBits + 7))
		top := int((y + radius - g.bmapOriginY + maxThingBlockRadius) >> (fracBits + 7))
		if left < 0 {
			left = 0
		}
		if bottom < 0 {
			bottom = 0
		}
		if right >= g.bmapWidth {
			right = g.bmapWidth - 1
		}
		if top >= g.bmapHeight {
			top = g.bmapHeight - 1
		}
		for by := bottom; by <= top; by++ {
			for bx := left; bx <= right; bx++ {
				var hit lineAttackTarget
				if !g.blockThingsIterator(bx, by, func(other int) bool {
					var ok bool
					hit, ok = visitThing(other)
					return !ok
				}) {
					return hit, true
				}
			}
		}
		return lineAttackTarget{}, false
	}
	for other := range g.m.Things {
		if target, ok := visitThing(other); ok {
			return target, true
		}
	}
	return lineAttackTarget{}, false
}

func (g *game) tickSkullFlyMomentum(i int, th mapdata.Thing) {
	momx := clamp(g.thingMomX[i], -maxMove, maxMove)
	momy := clamp(g.thingMomY[i], -maxMove, maxMove)
	momz := g.thingMomZ[i]
	debugSkull := false
	if want := runtimeDebugEnv("GD_DEBUG_SKULL_FLY"); want != "" {
		var wantTic, wantIdx int
		if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantIdx); err == nil {
			debugSkull = wantIdx == i && (g.demoTick-1 == wantTic || g.worldTic == wantTic)
		}
	}
	if momx == 0 && momy == 0 {
		if debugSkull {
			px, py := g.thingPosFixed(i, th)
			fmt.Printf("skull-fly-debug tic=%d world=%d idx=%d event=zero-momentum pos=(%d,%d) mom=(%d,%d,%d)\n",
				g.demoTick-1, g.worldTic, i, px, py, momx, momy, momz)
		}
		g.resetLostSoulCharge(i, th.Type)
		return
	}

	tx, ty := g.thingPosFixed(i, th)
	z, _, _ := g.thingSupportState(i, th)
	xmove, ymove := momx, momy
	skullActive := i < len(g.thingSkullFly) && g.thingSkullFly[i]
	for xmove != 0 || ymove != 0 {
		stepX, stepY := xmove, ymove
		if stepX > maxMove/2 || stepY > maxMove/2 {
			stepX >>= 1
			stepY >>= 1
			xmove >>= 1
			ymove >>= 1
		} else {
			xmove = 0
			ymove = 0
		}
		nx, ny := tx+stepX, ty+stepY
		if debugSkull {
			fmt.Printf("skull-fly-debug tic=%d world=%d idx=%d event=step from=(%d,%d,%d) to=(%d,%d,%d) step=(%d,%d) rem=(%d,%d)\n",
				g.demoTick-1, g.worldTic, i, tx, ty, z, nx, ny, z+momz, stepX, stepY, xmove, ymove)
		}
		var tmfloor, tmceil int64
		moveOK := false
		if skullActive {
			probe := g.probeSkullFlyMove(i, th.Type, nx, ny)
			if probe.hitTarget {
				target := probe.target
				if debugSkull {
					targetType := int16(0)
					targetX, targetY := int64(0), int64(0)
					if target.kind == lineAttackTargetThing && g.m != nil && target.idx >= 0 && target.idx < len(g.m.Things) {
						targetType = g.m.Things[target.idx].Type
						targetX, targetY = g.thingPosFixed(target.idx, g.m.Things[target.idx])
					}
					fmt.Printf("skull-fly-debug tic=%d world=%d idx=%d event=hit-target kind=%d target=%d target_type=%d target_pos=(%d,%d) at=(%d,%d,%d)\n",
						g.demoTick-1, g.worldTic, i, target.kind, target.idx, targetType, targetX, targetY, nx, ny, z+momz)
				}
				damage := 3 * (1 + doomPRandomN(8))
				switch target.kind {
				case lineAttackTargetPlayer:
					g.damagePlayerFrom(damage, "Monster hit you", tx, ty, true, i)
				case lineAttackTargetThing:
					if target.idx >= 0 && target.idx < len(g.m.Things) && thingTypeIsShootable(g.m.Things[target.idx].Type) {
						g.damageShootableThingFrom(target.idx, damage, false, i, tx, ty, true)
					}
				}
				g.resetLostSoulCharge(i, th.Type)
				skullActive = false
				momx = 0
				momy = 0
				momz = 0
				continue
			}
			tmfloor, tmceil, moveOK = probe.tmfloor, probe.tmceil, probe.ok
		} else {
			probe := g.probeMonsterMove(i, th.Type, nx, ny)
			tmfloor, tmceil, moveOK = probe.tmfloor, probe.tmceil, probe.ok
		}
		if !moveOK {
			if debugSkull {
				fmt.Printf("skull-fly-debug tic=%d world=%d idx=%d event=blocked-zero-xy at=(%d,%d,%d) mom_before=(%d,%d,%d)\n",
					g.demoTick-1, g.worldTic, i, nx, ny, z+momz, g.thingMomX[i], g.thingMomY[i], g.thingMomZ[i])
			}
			g.thingMomX[i] = 0
			g.thingMomY[i] = 0
			momx = 0
			momy = 0
			continue
		}
		prevX, prevY := tx, ty
		g.setThingPosFixed(i, nx, ny)
		g.setThingSupportState(i, z, tmfloor, tmceil)
		g.checkWalkSpecialLinesForActor(prevX, prevY, nx, ny, i, false)
		if debugSkull {
			fmt.Printf("skull-fly-debug tic=%d world=%d idx=%d event=move-ok pos=(%d,%d,%d) floor=%d ceil=%d\n",
				g.demoTick-1, g.worldTic, i, nx, ny, z, tmfloor, tmceil)
		}
		tx, ty = nx, ny
	}

	_, floorZ, ceilZ := g.thingSupportState(i, th)
	nz := z + momz
	height := g.thingCurrentHeight(i, th)
	if nz <= floorZ {
		nz = floorZ
		momz = -momz
	} else if nz+height > ceilZ {
		nz = ceilZ - height
		if momz > 0 {
			momz = -momz
		}
	}
	g.setThingSupportState(i, nz, floorZ, ceilZ)
	g.thingMomX[i] = momx
	g.thingMomY[i] = momy
	g.thingMomZ[i] = momz
	if debugSkull {
		fmt.Printf("skull-fly-debug tic=%d world=%d idx=%d event=finish pos=(%d,%d,%d) mom=(%d,%d,%d) floor=%d ceil=%d\n",
			g.demoTick-1, g.worldTic, i, tx, ty, nz, momx, momy, momz, floorZ, ceilZ)
	}
}

func monsterPainChance(typ int16) int {
	switch typ {
	case 3004: // zombieman
		return 200
	case 9, 65: // shotgun guy / chaingunner
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

func monsterPainActionPhase(typ int16) int {
	frameTics := monsterPainFrameTics(typ)
	switch len(frameTics) {
	case 0:
		return -1
	case 1:
		return 0
	default:
		return 1
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
		if !g.advanceZeroTicMonsterAttackFrames(i, typ, tx, ty, g.p.x, g.p.y, dist) {
			return false
		}
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
	case 65: // chaingunner
		return 10
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
		return 30
	case 65: // chaingunner
		return 19
	case 3001: // imp
		return 22
	case 3002, 58: // demon/spectre
		return 24
	case 3005: // cacodemon
		return 15
	case 3006: // lost soul
		return 22
	case 3003, 69: // baron/knight
		return 24
	case 16: // cyberdemon
		return 66
	case 7: // spider mastermind (single volley cycle)
		return 29
	default:
		return 0
	}
}

func monsterUsesExplicitAttackFrames(typ int16) bool {
	switch typ {
	case 3004, 9, 65, 3001, 3002, 58, 3005, 3006, 3003, 69, 7, 16, 64, 66, 67, 68, 71, 84: // core roster plus explicit advanced attacks
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
	faceX, faceY := px, py
	if targetX, targetY, _, _, _, ok := g.monsterAttackTargetPos(i); ok {
		faceX, faceY = targetX, targetY
	}
	switch typ {
	case 3004: // zombieman
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 1:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 9: // shotgun guy
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 1:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 65: // chaingunner
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 3:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
			if !g.chaingunnerRefireKeepsAttack(i, typ, tx, ty) {
				if i >= 0 && i < len(g.thingAttackTics) {
					g.thingAttackTics[i] = 0
				}
				if i >= 0 && i < len(g.thingAttackFireTics) {
					g.thingAttackFireTics[i] = -1
				}
				g.resetMonsterPostAttackState(i, typ)
			}
		case 1, 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3001: // imp
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3002, 58: // demon/spectre
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3005: // cacodemon
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3006: // lost soul
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 1:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 3003, 69: // baron/hell knight
		switch phase {
		case 0, 1:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 64: // arch-vile
		switch phase {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 9:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 66: // revenant missile
		switch phase {
		case 0, 1, 3:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 2:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 67: // mancubus
		switch phase {
		case 0, 2, 3, 5, 6, 8, 9:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 1, 4, 7:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 71: // pain elemental
		switch phase {
		case 0, 1, 2:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 3:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 84: // wolf ss
		switch phase {
		case 0, 1, 3:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 2, 4:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 68: // arachnotron
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 1:
			_ = g.monsterAttack(i, typ, dist)
		case 3:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
			if !g.spiderRefireKeepsAttack(i, typ, tx, ty) {
				if i >= 0 && i < len(g.thingAttackTics) {
					g.thingAttackTics[i] = 0
				}
				if i >= 0 && i < len(g.thingAttackFireTics) {
					g.thingAttackFireTics[i] = -1
				}
				g.resetMonsterPostAttackState(i, typ)
			}
		}
	case 16: // cyberdemon
		switch phase {
		case 0, 2, 4:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 1, 3, 5:
			_ = g.monsterAttack(i, typ, dist)
		}
	case 7: // spider mastermind
		switch phase {
		case 0:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
		case 1, 2:
			_ = g.monsterAttack(i, typ, dist)
		case 3:
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
			if !g.spiderRefireKeepsAttack(i, typ, tx, ty) {
				if i >= 0 && i < len(g.thingAttackTics) {
					g.thingAttackTics[i] = 0
				}
				if i >= 0 && i < len(g.thingAttackFireTics) {
					g.thingAttackFireTics[i] = -1
				}
				g.resetMonsterPostAttackState(i, typ)
			}
		}
	}
}

func (g *game) tickMonsterAttackState(i int, typ int16, tx, ty, px, py, dist int64) bool {
	if i < 0 || i >= len(g.thingAttackTics) {
		return false
	}
	if monsterUsesExplicitAttackFrames(typ) {
		if i >= 0 && i < len(g.thingStateTics) {
			g.thingStateTics[i]--
			if g.thingStateTics[i] <= 0 {
				if !g.advanceMonsterAttackPhase(i, typ, tx, ty, px, py, dist) {
					return false
				}
				if !g.advanceZeroTicMonsterAttackFrames(i, typ, tx, ty, px, py, dist) {
					return false
				}
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
			faceX, faceY := px, py
			if targetX, targetY, _, _, _, ok := g.monsterTargetPos(i); ok {
				faceX, faceY = targetX, targetY
			}
			g.faceMonsterToward(i, tx, ty, faceX, faceY)
			_ = g.monsterAttack(i, typ, dist)
			g.thingAttackFireTics[i] = -1
		}
	}
	g.thingAttackTics[i]--
	if g.thingAttackTics[i] <= 0 {
		g.thingAttackTics[i] = 0
		if i >= 0 && i < len(g.thingAttackFireTics) {
			g.thingAttackFireTics[i] = -1
		}
		g.resetMonsterPostAttackState(i, typ)
		return false
	}
	if i >= 0 && i < len(g.thingStateTics) && g.thingState[i] == monsterStateAttack {
		g.thingStateTics[i] = g.thingAttackTics[i]
	}
	return true
}

func (g *game) advanceMonsterAttackPhase(i int, typ int16, tx, ty, px, py, dist int64) bool {
	phase := 0
	if i >= 0 && i < len(g.thingAttackPhase) {
		phase = g.thingAttackPhase[i]
	}
	nextPhase := phase + 1
	if nextPhase >= len(monsterAttackFrameTics(typ)) {
		if loopPhase, loop := g.nextMonsterAttackLoopPhase(i, typ, tx, ty); loop {
			nextPhase = loopPhase
		}
	}
	if nextPhase >= len(monsterAttackFrameTics(typ)) {
		if typ == 3006 {
			g.resetLostSoulCharge(i, typ)
			return false
		}
		g.thingAttackTics[i] = 0
		if i >= 0 && i < len(g.thingAttackFireTics) {
			g.thingAttackFireTics[i] = -1
		}
		g.resetMonsterPostAttackState(i, typ)
		return false
	}
	if i >= 0 && i < len(g.thingAttackPhase) {
		g.thingAttackPhase[i] = nextPhase
	}
	if i >= 0 && i < len(g.thingStateTics) {
		g.thingStateTics[i] = monsterAttackFrameDuration(typ, nextPhase)
	}
	g.runMonsterAttackPhaseEntry(i, typ, nextPhase, tx, ty, px, py, dist)
	if i >= 0 && i < len(g.thingState) && g.thingState[i] != monsterStateAttack {
		return false
	}
	return true
}

func (g *game) advanceZeroTicMonsterAttackFrames(i int, typ int16, tx, ty, px, py, dist int64) bool {
	for i >= 0 && i < len(g.thingState) && g.thingState[i] == monsterStateAttack &&
		i < len(g.thingStateTics) && g.thingStateTics[i] <= 0 {
		if !g.advanceMonsterAttackPhase(i, typ, tx, ty, px, py, dist) {
			return false
		}
	}
	return i >= 0 && i < len(g.thingState) && g.thingState[i] == monsterStateAttack
}

func (g *game) nextMonsterAttackLoopPhase(i int, typ int16, tx, ty int64) (int, bool) {
	switch typ {
	case 65:
		// Doom's S_CPOS_ATK4 nextstate is always S_CPOS_ATK2; A_CPosRefire
		// already decided on phase-entry whether to stay attacking.
		return 1, true
	case 3006:
		if i >= 0 && i < len(g.thingSkullFly) && g.thingSkullFly[i] {
			return 2, true
		}
		return 0, false
	case 7, 68:
		// Doom's S_SPID_ATK4 and S_BSPI_ATK4 both nextstate back to ATK2.
		return 1, true
	default:
		return 0, false
	}
}

func (g *game) resetMonsterPostAttackState(i int, typ int16) {
	if g == nil || i < 0 {
		return
	}
	g.clearMonsterPainState(i)
	if i >= 0 && i < len(g.thingStatePhase) {
		g.thingStatePhase[i] = monsterSeeStartPhase(typ)
	}
	g.setMonsterThinkState(i, typ, monsterStateSee, g.monsterSeeStateTicsForPhase(i, typ))
}

func (g *game) chaingunnerRefireKeepsAttack(i int, typ int16, tx, ty int64) bool {
	if doomrand.PRandom() < 40 {
		return true
	}
	if !g.monsterHasTarget(i) {
		return false
	}
	return g.monsterHasLOSTarget(i, typ, tx, ty)
}

func (g *game) spiderRefireKeepsAttack(i int, typ int16, tx, ty int64) bool {
	// Doom A_SpidRefire keeps attacking on low rolls and only leaves the
	// attack loop when the roll fails and the target is gone or out of sight.
	if doomrand.PRandom() < 10 {
		return true
	}
	if !g.monsterHasTarget(i) {
		return false
	}
	return g.monsterHasLOSTarget(i, typ, tx, ty)
}

func demoTraceMonsterAttackState(typ int16, phase int) (int, bool) {
	switch typ {
	case 3004:
		if phase >= 0 && phase <= 2 {
			return 184 + phase, true
		}
	case 9:
		if phase >= 0 && phase <= 2 {
			return 218 + phase, true
		}
	case 65:
		if phase >= 0 && phase <= 3 {
			return 416 + phase, true
		}
	case 3001:
		if phase >= 0 && phase <= 2 {
			return 452 + phase, true
		}
	case 3002, 58:
		if phase >= 0 && phase <= 2 {
			return 485 + phase, true
		}
	case 3006:
		if phase >= 0 && phase <= 3 {
			return 589 + phase, true
		}
	case 3005:
		if phase >= 0 && phase <= 2 {
			return 504 + phase, true
		}
	case 3003:
		if phase >= 0 && phase <= 2 {
			return 538 + phase, true
		}
	case 69:
		if phase >= 0 && phase <= 2 {
			return 567 + phase, true
		}
	case 64:
		if phase >= 0 && phase <= 10 {
			return 256 + phase, true
		}
	case 66:
		if phase >= 0 && phase <= 5 {
			return 336 + phase, true
		}
	case 67:
		if phase >= 0 && phase <= 9 {
			return 377 + phase, true
		}
	case 68:
		if phase >= 0 && phase <= 3 {
			return 648 + phase, true
		}
	case 71:
		if phase >= 0 && phase <= 3 {
			return 709 + phase, true
		}
	case 16:
		if phase >= 0 && phase <= 5 {
			return 685 + phase, true
		}
	case 7:
		if phase >= 0 && phase <= 3 {
			return 616 + phase, true
		}
	case 84:
		if phase >= 0 && phase <= 2 {
			return 737 + phase, true
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
	if typ == 68 && phase < 0 {
		return 20
	}
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
	r := doomrand.PRandom()
	if want := runtimeDebugEnv("GD_DEBUG_MONSTER_RNG_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("monster-rng-debug tic=%d world=%d idx=%d type=%d site=missilerange roll=%d dist=%d rnd=%d prnd=%d pos=(%d,%d)\n",
					g.demoTick-1, g.worldTic, i, typ, r, d, rnd, prnd, tx, ty)
			}
		}
	}
	return r >= d
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
		if g.debugMonsterChaseEnabled() {
			g.debugMonsterChase(i, fmt.Sprintf("diag candidate=%d turnaround=%d", diag, turnaround))
		}
		if diag != turnaround && g.monsterTryWalk(i, typ, diag) {
			if g.debugMonsterChaseEnabled() {
				g.debugMonsterChase(i, fmt.Sprintf("diag success dir=%d", diag))
			}
			return
		}
	}

	swapRoll := doomrand.PRandom()
	if want := runtimeDebugEnv("GD_DEBUG_MONSTER_RNG_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("monster-rng-debug tic=%d world=%d idx=%d type=%d site=chase-swap roll=%d absdy=%d absdx=%d rnd=%d prnd=%d pos=(%d,%d)\n",
					g.demoTick-1, g.worldTic, i, typ, swapRoll, abs(deltay), abs(deltax), rnd, prnd, tx, ty)
			}
		}
	}
	if swapRoll > 200 || abs(deltay) > abs(deltax) {
		d1, d2 = d2, d1
		if g.debugMonsterChaseEnabled() {
			g.debugMonsterChase(i, fmt.Sprintf("swap d1=%d d2=%d", d1, d2))
		}
	}

	if d1 == turnaround {
		d1 = monsterDirNoDir
	}
	if d2 == turnaround {
		d2 = monsterDirNoDir
	}

	if d1 != monsterDirNoDir && g.monsterTryWalk(i, typ, d1) {
		if g.debugMonsterChaseEnabled() {
			g.debugMonsterChase(i, fmt.Sprintf("d1 success dir=%d", d1))
		}
		return
	}
	if d2 != monsterDirNoDir && g.monsterTryWalk(i, typ, d2) {
		if g.debugMonsterChaseEnabled() {
			g.debugMonsterChase(i, fmt.Sprintf("d2 success dir=%d", d2))
		}
		return
	}

	if olddir != monsterDirNoDir && g.monsterTryWalk(i, typ, olddir) {
		if g.debugMonsterChaseEnabled() {
			g.debugMonsterChase(i, fmt.Sprintf("olddir success dir=%d", olddir))
		}
		return
	}

	scanRoll := doomrand.PRandom()
	if want := runtimeDebugEnv("GD_DEBUG_MONSTER_RNG_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("monster-rng-debug tic=%d world=%d idx=%d type=%d site=chase-scan roll=%d rnd=%d prnd=%d pos=(%d,%d)\n",
					g.demoTick-1, g.worldTic, i, typ, scanRoll, rnd, prnd, tx, ty)
			}
		}
	}
	if (scanRoll & 1) != 0 {
		for dir := int(monsterDirEast); dir <= int(monsterDirSouthEast); dir++ {
			d := monsterMoveDir(dir)
			if d != turnaround && g.monsterTryWalk(i, typ, d) {
				if g.debugMonsterChaseEnabled() {
					g.debugMonsterChase(i, fmt.Sprintf("scan success dir=%d", d))
				}
				return
			}
		}
	} else {
		for dir := int(monsterDirSouthEast); dir >= int(monsterDirEast); dir-- {
			d := monsterMoveDir(dir)
			if d != turnaround && g.monsterTryWalk(i, typ, d) {
				if g.debugMonsterChaseEnabled() {
					g.debugMonsterChase(i, fmt.Sprintf("reverse scan success dir=%d", d))
				}
				return
			}
		}
	}

	if turnaround != monsterDirNoDir && g.monsterTryWalk(i, typ, turnaround) {
		if g.debugMonsterChaseEnabled() {
			g.debugMonsterChase(i, fmt.Sprintf("turnaround success dir=%d", turnaround))
		}
		return
	}
	g.thingMoveDir[i] = monsterDirNoDir
}

func (g *game) monsterTryWalk(i int, typ int16, dir monsterMoveDir) bool {
	if i < 0 || i >= len(g.thingMoveDir) {
		return false
	}
	debugChase := g.debugMonsterChaseEnabled()
	g.thingMoveDir[i] = dir
	if debugChase {
		g.debugMonsterChase(i, fmt.Sprintf("trywalk dir=%d", dir))
	}
	if !g.monsterMoveInDir(i, typ, dir) {
		if debugChase {
			g.debugMonsterChase(i, fmt.Sprintf("trywalk blocked dir=%d", dir))
		}
		return false
	}
	if i >= 0 && i < len(g.thingMoveCount) {
		g.thingMoveCount[i] = doomrand.PRandom() & 15
		if want := runtimeDebugEnv("GD_DEBUG_MONSTER_RNG_TIC"); want != "" {
			var wantTic int
			if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
				if g.demoTick-1 == wantTic || g.worldTic == wantTic {
					rnd, prnd := doomrand.State()
					tx, ty := int64(0), int64(0)
					if g.m != nil && i >= 0 && i < len(g.m.Things) {
						tx, ty = g.thingPosFixed(i, g.m.Things[i])
					}
					fmt.Printf("monster-rng-debug tic=%d world=%d idx=%d type=%d site=trywalk-movecount dir=%d movecount=%d rnd=%d prnd=%d pos=(%d,%d)\n",
						g.demoTick-1, g.worldTic, i, typ, dir, g.thingMoveCount[i], rnd, prnd, tx, ty)
				}
			}
		}
		if debugChase {
			g.debugMonsterChase(i, fmt.Sprintf("trywalk moved dir=%d movecount=%d", dir, g.thingMoveCount[i]))
		}
	}
	return true
}

func (g *game) debugMonsterChase(i int, msg string) {
	if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_CHASE") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_CHASE"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantIdx >= 0 && i != wantIdx {
		return
	}
	if wantTic >= 0 && g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	tx, ty := int64(0), int64(0)
	if g.m != nil && i >= 0 && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	fmt.Printf("monster-chase-debug tic=%d world=%d idx=%d type=%d msg=%s pos=(%d,%d) movedir=%d movecount=%d angle=%d target=(%d,%d)\n",
		g.demoTick-1, g.worldTic, i, g.m.Things[i].Type, msg, tx, ty, g.thingMoveDir[i], g.thingMoveCount[i], g.thingWorldAngle(i, g.m.Things[i]), g.p.x, g.p.y)
}

func (g *game) debugMonsterChaseEnabled() bool {
	return g != nil && runtimeDebugEnv("GD_DEBUG_MONSTER_CHASE") != ""
}

func (g *game) debugMonsterMove(i int, msg string) {
	if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_MOVE") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_MOVE"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantIdx >= 0 && i != wantIdx {
		return
	}
	if wantTic >= 0 && g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	tx, ty := int64(0), int64(0)
	if g.m != nil && i >= 0 && i < len(g.m.Things) {
		tx, ty = g.thingPosFixed(i, g.m.Things[i])
	}
	fmt.Printf("monster-move-debug tic=%d world=%d idx=%d type=%d msg=%s pos=(%d,%d) movedir=%d movecount=%d angle=%d\n",
		g.demoTick-1, g.worldTic, i, g.m.Things[i].Type, msg, tx, ty, g.thingMoveDir[i], g.thingMoveCount[i], g.thingWorldAngle(i, g.m.Things[i]))
}

func (g *game) debugMonsterMoveEnabled() bool {
	return g != nil && runtimeDebugEnv("GD_DEBUG_MONSTER_MOVE") != ""
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
	debugMove := g.debugMonsterMoveEnabled()
	if debugMove {
		g.debugMonsterMove(i, fmt.Sprintf("move dir=%d from=(%d,%d) to=(%d,%d)", dir, x, y, nx, ny))
	}
	probe := g.probeMonsterMove(i, typ, nx, ny)
	tmfloor, tmceil, probeLines, ok := probe.tmfloor, probe.tmceil, probe.probeLines, probe.ok
	if !ok {
		if probe.floatOK && monsterCanFloat(typ) {
			z, floorZ, ceilZ := g.thingSupportState(i, g.m.Things[i])
			if z < probe.tmfloor {
				z += 4 * fracUnit
			} else {
				z -= 4 * fracUnit
			}
			g.setThingSupportState(i, z, floorZ, ceilZ)
			if i >= 0 && i < len(g.thingInFloat) {
				g.thingInFloat[i] = true
			}
			if debugMove {
				g.debugMonsterMove(i, fmt.Sprintf("move float-adjust dir=%d z=%d floor=%d", dir, z, probe.tmfloor))
			}
			return true
		}
		if debugMove {
			g.debugMonsterMove(i, fmt.Sprintf("move blocked dir=%d", dir))
		}
		lines := probeLines
		if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_MOVE_LINES") == "" {
			// no-op
		} else {
			var wantTic, wantIdx int
			if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_MOVE_LINES"), "%d:%d", &wantTic, &wantIdx); err == nil {
				if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && (wantIdx < 0 || wantIdx == i) {
					fmt.Printf("monster-move-lines-debug tic=%d world=%d idx=%d dir=%d lines=%v from=(%d,%d) to=(%d,%d)\n",
						g.demoTick-1, g.worldTic, i, dir, lines, x, y, nx, ny)
				}
			}
		}
		if len(lines) == 0 {
			return false
		}
		if i >= 0 && i < len(g.thingMoveDir) {
			// Doom P_Move clears movedir only when retrying touched special
			// lines after a blocked move, so successful door-use leaves
			// DI_NODIR but ordinary blocked movement preserves olddir.
			g.thingMoveDir[i] = monsterDirNoDir
		}
		for _, lineIdx := range lines {
			if g.useSpecialLineForActor(lineIdx, 0, false) {
				return true
			}
		}
		return false
	}
	prevX, prevY := x, y
	z, _, _ := g.thingSupportState(i, g.m.Things[i])
	g.setThingPosFixed(i, nx, ny)
	if monsterCanFloat(typ) {
		g.setThingSupportState(i, z, tmfloor, tmceil)
		if i >= 0 && i < len(g.thingInFloat) {
			g.thingInFloat[i] = false
		}
	} else {
		g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
	}
	g.checkWalkSpecialLinesForActor(prevX, prevY, nx, ny, i, false)
	if debugMove {
		g.debugMonsterMove(i, fmt.Sprintf("move success dir=%d", dir))
	}
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

func (g *game) startLostSoulCharge(i int) bool {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return false
	}
	tx, ty, tz, theight, _, ok := g.monsterAttackTargetPos(i)
	if !ok {
		return false
	}
	sx, sy := g.thingPosFixed(i, g.m.Things[i])
	sz, _, _ := g.thingSupportState(i, g.m.Things[i])
	g.faceMonsterToward(i, sx, sy, tx, ty)
	angle := g.thingWorldAngle(i, g.m.Things[i])
	const skullSpeed = 20 * fracUnit
	momx := fixedMul(skullSpeed, doomFineCosine(angle))
	momy := fixedMul(skullSpeed, doomFineSineAtAngle(angle))
	dist := doomApproxDistance(tx-sx, ty-sy) / skullSpeed
	if dist < 1 {
		dist = 1
	}
	momz := (tz + (theight >> 1) - sz) / dist
	if i < len(g.thingSkullFly) {
		g.thingSkullFly[i] = true
	}
	g.setThingMomentum(i, momx, momy, momz)
	g.emitSoundEventAt(soundEventMonsterAttackSkull, sx, sy)
	return true
}

func (g *game) monsterAttack(i int, typ int16, dist int64) bool {
	meleeOnly := isMeleeOnlyMonster(typ)
	var sx, sy int64
	if i >= 0 && g.m != nil && i < len(g.m.Things) {
		sx, sy = g.thingPosFixed(i, g.m.Things[i])
	}
	targetX, targetY, _, _, _, ok := g.monsterAttackTargetPos(i)
	if !ok {
		return false
	}
	if monsterAttackCallsFaceTarget(typ) {
		g.faceMonsterToward(i, sx, sy, targetX, targetY)
	}
	if g.monsterCanMeleeTarget(i, typ, dist, sx, sy, targetX, targetY) {
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
	if typ == 3006 {
		return g.startLostSoulCharge(i)
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
		g.faceMonsterToward(i, sx, sy, targetX, targetY)
		switch phase {
		case 1:
			g.setThingWorldAngle(i, g.thingWorldAngle(i, g.m.Things[i])+fatSpread)
			if !g.spawnMonsterProjectile(i, typ) {
				return false
			}
			if !g.spawnMonsterProjectileAngleOffset(i, typ, fatSpread) {
				return false
			}
		case 4:
			g.setThingWorldAngle(i, g.thingWorldAngle(i, g.m.Things[i])-fatSpread)
			if !g.spawnMonsterProjectile(i, typ) {
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
	tx, ty, _, _, _, ok := g.monsterAttackTargetPos(i)
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
		g.damagePlayerFrom(damage, msg, attackerX, attackerY, true, i)
		return
	}
	targetIdx, ok := g.monsterTargetThingIdx(i)
	if !ok {
		g.damagePlayerFrom(damage, msg, attackerX, attackerY, true, i)
		return
	}
	g.damageShootableThingFrom(targetIdx, damage, false, i, attackerX, attackerY, true)
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
	if idx < len(g.thingSkullFly) {
		g.thingSkullFly[idx] = false
	}
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
		if corpseIdx < len(g.thingXDeath) && g.thingXDeath[corpseIdx] {
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
		if corpseIdx < len(g.thingXDeath) {
			g.thingXDeath[corpseIdx] = false
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
		if want := runtimeDebugEnv("GD_DEBUG_LINE_ATTACK"); want != "" {
			var wantTic, wantIdx, wantPellet int
			if _, err := fmt.Sscanf(want, "%d:%d:%d", &wantTic, &wantIdx, &wantPellet); err == nil {
				if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && (wantIdx < 0 || i == wantIdx) && (wantPellet < 0 || pellet == wantPellet) {
					g.debugLineAttackIntercepts(actor, angle, monsterAttackRange, slope)
				}
			}
		}
		outcome := g.lineAttackTrace(actor, angle, monsterAttackRange, slope, true)
		if want := runtimeDebugEnv("GD_DEBUG_HITSCAN_ATTACK"); want != "" {
			var wantTic, wantIdx int
			if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantIdx); err == nil {
				if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && (wantIdx < 0 || i == wantIdx) {
					rnd, prnd := doomrand.State()
					fmt.Printf("hitscan-attack-debug tic=%d world=%d idx=%d type=%d actor=(%d,%d,%d) pellet=%d base_angle=%d angle=%d slope=%d damage=%d target_kind=%d target_idx=%d impact=(%d,%d,%d) puff=%t blood=%t\n",
						g.demoTick-1, g.worldTic, i, typ, actor.x, actor.y, actor.shootZ, pellet, baseAngle, angle, slope, damage,
						outcome.target.kind, outcome.target.idx, outcome.impactX, outcome.impactY, outcome.impactZ,
						outcome.spawnPuff, outcome.spawnBlood)
					fmt.Printf("hitscan-attack-rng-debug tic=%d world=%d idx=%d type=%d pellet=%d rnd=%d prnd=%d\n",
						g.demoTick-1, g.worldTic, i, typ, pellet, rnd, prnd)
				}
			}
		}
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
	case 3002, 58:
		return true
	default:
		return false
	}
}

func monsterHasMeleeAttack(typ int16) bool {
	switch typ {
	case 3001, 3002, 3003, 58, 66, 69:
		return true
	default:
		return false
	}
}

func monsterMeleeDamage(typ int16) int {
	switch typ {
	case 3002, 58: // demon/spectre
		return 4 * (1 + doomPRandomN(10))
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
	if g.m == nil || len(g.m.Nodes) == 0 || len(g.m.SubSectors) == 0 || len(g.m.Segs) == 0 {
		return g.actorHasLOSByInterceptScan(ax, ay, az, aheight, bx, by, bz, bheight)
	}
	sightZStart := az + aheight - (aheight >> 2)
	sight := losTrace{
		trace:       divline{x: ax, y: ay, dx: bx - ax, dy: by - ay},
		t2x:         bx,
		t2y:         by,
		sightZStart: sightZStart,
		topSlope:    (bz + bheight) - sightZStart,
		bottomSlope: bz - sightZStart,
	}
	g.validCount++
	return g.crossBSPLOS(uint16(len(g.m.Nodes)-1), &sight)
}

type losTrace struct {
	trace       divline
	t2x         int64
	t2y         int64
	sightZStart int64
	topSlope    int64
	bottomSlope int64
}

func doomSightDivlineSide(x, y int64, line divline) int {
	if line.dx == 0 {
		if x == line.x {
			return 2
		}
		if x <= line.x {
			return b2i(line.dy > 0)
		}
		return b2i(line.dy < 0)
	}
	if line.dy == 0 {
		if y == line.y {
			return 2
		}
		if y <= line.y {
			return b2i(line.dx < 0)
		}
		return b2i(line.dx > 0)
	}
	dx := x - line.x
	dy := y - line.y
	left := (line.dy >> fracBits) * (dx >> fracBits)
	right := (dy >> fracBits) * (line.dx >> fracBits)
	if right < left {
		return 0
	}
	if left == right {
		return 2
	}
	return 1
}

func (g *game) actorHasLOSByInterceptScan(ax, ay, az, aheight, bx, by, bz, bheight int64) bool {
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
	trace := divline{x: ax, y: ay, dx: bx - ax, dy: by - ay}
	for i, ld := range g.lines {
		s1 := doomSightDivlineSide(ld.x1, ld.y1, trace)
		s2 := doomSightDivlineSide(ld.x2, ld.y2, trace)
		if s1 == s2 {
			continue
		}
		lineDX := ld.dx
		lineDY := ld.dy
		if lineDX == 0 && lineDY == 0 && (ld.x1 != ld.x2 || ld.y1 != ld.y2) {
			lineDX = ld.x2 - ld.x1
			lineDY = ld.y2 - ld.y1
		}
		lineDL := divline{x: ld.x1, y: ld.y1, dx: lineDX, dy: lineDY}
		s1 = doomSightDivlineSide(trace.x, trace.y, lineDL)
		s2 = doomSightDivlineSide(trace.x+trace.dx, trace.y+trace.dy, lineDL)
		if s1 == s2 {
			continue
		}
		frac := interceptVector(trace, lineDL)
		if frac <= 0 || frac >= fracUnit {
			continue
		}
		intercepts = insertInterceptOrdered(intercepts, intercept{frac: float64(frac) / float64(fracUnit), line: i})
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

func (g *game) crossBSPLOS(child uint16, sight *losTrace) bool {
	if child&0x8000 != 0 {
		ss := int(child & 0x7fff)
		if ss < 0 || ss >= len(g.m.SubSectors) {
			return false
		}
		return g.crossSubsectorLOS(ss, sight)
	}
	ni := int(child)
	if ni < 0 || ni >= len(g.m.Nodes) {
		return false
	}
	n := g.m.Nodes[ni]
	partition := divline{
		x:  int64(n.X) << fracBits,
		y:  int64(n.Y) << fracBits,
		dx: int64(n.DX) << fracBits,
		dy: int64(n.DY) << fracBits,
	}
	side := doomSightDivlineSide(sight.trace.x, sight.trace.y, partition)
	if side == 2 {
		side = 0
	}
	if !g.crossBSPLOS(n.ChildID[side], sight) {
		return false
	}
	if side == doomSightDivlineSide(sight.t2x, sight.t2y, partition) {
		return true
	}
	return g.crossBSPLOS(n.ChildID[side^1], sight)
}

func (g *game) crossSubsectorLOS(ss int, sight *losTrace) bool {
	if g == nil || g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) {
		return false
	}
	if want := runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK"); want != "" {
		var wantTic, wantIdx int
		if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantIdx); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				fmt.Printf("monster-look-debug tic=%d world=%d site=los-subsector ss=%d ax=%d ay=%d bx=%d by=%d\n",
					g.demoTick-1, g.worldTic, ss, sight.trace.x, sight.trace.y, sight.t2x, sight.t2y)
			}
		}
	}
	sub := g.m.SubSectors[ss]
	for off := 0; off < int(sub.SegCount); off++ {
		segIdx := int(sub.FirstSeg) + off
		if segIdx < 0 || segIdx >= len(g.m.Segs) {
			continue
		}
		sg := g.m.Segs[segIdx]
		lineIdx := int(sg.Linedef)
		if lineIdx < 0 || lineIdx >= len(g.lines) || lineIdx >= len(g.m.Linedefs) {
			continue
		}
		if lineIdx >= len(g.lineValid) {
			g.lineValid = append(g.lineValid, make([]int, lineIdx-len(g.lineValid)+1)...)
		}
		if g.lineValid[lineIdx] == g.validCount {
			continue
		}
		g.lineValid[lineIdx] = g.validCount
		ld := g.lines[lineIdx]
		s1 := doomSightDivlineSide(ld.x1, ld.y1, sight.trace)
		s2 := doomSightDivlineSide(ld.x2, ld.y2, sight.trace)
		if s1 == s2 {
			continue
		}
		lineDX := ld.dx
		lineDY := ld.dy
		if lineDX == 0 && lineDY == 0 && (ld.x1 != ld.x2 || ld.y1 != ld.y2) {
			lineDX = ld.x2 - ld.x1
			lineDY = ld.y2 - ld.y1
		}
		lineDL := divline{x: ld.x1, y: ld.y1, dx: lineDX, dy: lineDY}
		s1 = doomSightDivlineSide(sight.trace.x, sight.trace.y, lineDL)
		s2 = doomSightDivlineSide(sight.t2x, sight.t2y, lineDL)
		if want := runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK"); want != "" {
			var wantTic, wantIdx int
			if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantIdx); err == nil {
				if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && (lineIdx == 185 || ss == 39) {
					fmt.Printf("monster-look-debug tic=%d world=%d site=los-line ss=%d line=%d s1=%d s2=%d v1=(%d,%d) v2=(%d,%d)\n",
						g.demoTick-1, g.worldTic, ss, lineIdx, s1, s2, ld.x1, ld.y1, ld.x2, ld.y2)
				}
			}
		}
		if s1 == s2 {
			continue
		}
		if (ld.flags & mlTwoSided) == 0 {
			g.debugMonsterLOSBlock("onesided", lineIdx, sight)
			return false
		}
		front, back := g.segSectorIndices(segIdx)
		if front < 0 || back < 0 || front >= len(g.sectorFloor) || back >= len(g.sectorFloor) || front >= len(g.sectorCeil) || back >= len(g.sectorCeil) {
			g.debugMonsterLOSBlock("badsectors", lineIdx, sight)
			return false
		}
		if g.sectorFloor[front] == g.sectorFloor[back] && g.sectorCeil[front] == g.sectorCeil[back] {
			continue
		}
		openTop := g.sectorCeil[front]
		if g.sectorCeil[back] < openTop {
			openTop = g.sectorCeil[back]
		}
		openBottom := g.sectorFloor[front]
		if g.sectorFloor[back] > openBottom {
			openBottom = g.sectorFloor[back]
		}
		if openBottom >= openTop {
			g.debugMonsterLOSBlock("closed", lineIdx, sight)
			return false
		}
		frac := interceptVector(sight.trace, lineDL)
		if frac <= 0 {
			continue
		}
		if g.sectorFloor[front] != g.sectorFloor[back] {
			if slope := fixedDiv(openBottom-sight.sightZStart, frac); slope > sight.bottomSlope {
				sight.bottomSlope = slope
			}
		}
		if g.sectorCeil[front] != g.sectorCeil[back] {
			if slope := fixedDiv(openTop-sight.sightZStart, frac); slope < sight.topSlope {
				sight.topSlope = slope
			}
		}
		if sight.topSlope <= sight.bottomSlope {
			g.debugMonsterLOSBlock("slope", lineIdx, sight)
			return false
		}
	}
	return true
}

func (g *game) debugMonsterLOSBlock(reason string, lineIdx int, sight *losTrace) {
	if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK"), "%d:%d", &wantTic, &wantIdx); err != nil {
		return
	}
	if wantTic >= 0 && g.demoTick-1 != wantTic && g.worldTic != wantTic {
		return
	}
	fmt.Printf("monster-look-debug tic=%d world=%d site=los-block line=%d reason=%s ax=%d ay=%d bx=%d by=%d\n",
		g.demoTick-1, g.worldTic, lineIdx, reason, sight.trace.x, sight.trace.y, sight.t2x, sight.t2y)
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
	fromSector := g.sectorAt(x, y)
	if i >= 0 && i < len(g.m.Things) {
		fromSector = g.thingSectorCached(i, g.m.Things[i])
	}
	if g.sightRejected(fromSector, g.playerSector()) {
		if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK") == "" {
			return false
		}
		var wantTic, wantIdx int
		if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK"), "%d:%d", &wantTic, &wantIdx); err == nil {
			if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && wantIdx == i {
				fmt.Printf("monster-look-debug tic=%d world=%d idx=%d site=los-reject from=%d to=%d actor=(%d,%d)\n",
					g.demoTick-1, g.worldTic, i, fromSector, g.playerSector(), x, y)
			}
		}
		return false
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
	ok := g.actorHasLOS(x, y, z, monsterHeight(typ), g.p.x, g.p.y, g.p.z, playerHeight)
	if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK") == "" {
		return ok
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK"), "%d:%d", &wantTic, &wantIdx); err == nil {
		if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && wantIdx == i {
			fmt.Printf("monster-look-debug tic=%d world=%d idx=%d site=los ok=%t actor=(%d,%d,%d) player=(%d,%d,%d)\n",
				g.demoTick-1, g.worldTic, i, ok, x, y, z, g.p.x, g.p.y, g.p.z)
		}
	}
	return ok
}

func (g *game) sightRejected(fromSector, toSector int) bool {
	if g == nil || g.m == nil || g.m.RejectMatrix == nil {
		return false
	}
	rejected, err := g.m.RejectMatrix.Rejects(fromSector, toSector)
	return err == nil && rejected
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
	if !g.monsterTargetAlive() {
		return false, false
	}
	sec := g.thingSectorCached(i, g.m.Things[i])
	if sec < 0 || sec >= len(g.sectorSoundTarget) || !g.sectorSoundTarget[sec] {
		return false, false
	}
	g.setMonsterTargetPlayer(i)
	if i < len(g.thingAmbush) && g.thingAmbush[i] {
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
				if want := runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK"); want != "" {
					var wantTic, wantIdx int
					if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantIdx); err == nil {
						if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && wantIdx == i {
							fmt.Printf("monster-look-debug tic=%d world=%d idx=%d site=look los=0 look=%d allaround=%t tx=%d ty=%d player=(%d,%d)\n",
								g.demoTick-1, g.worldTic, i, look, allAround, tx, ty, g.p.x, g.p.y)
						}
					}
				}
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
			if want := runtimeDebugEnv("GD_DEBUG_MONSTER_LOOK"); want != "" {
				var wantTic, wantIdx int
				if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantIdx); err == nil {
					if (g.demoTick-1 == wantTic || g.worldTic == wantTic) && wantIdx == i {
						fmt.Printf("monster-look-debug tic=%d world=%d idx=%d site=look acquired look=%d allaround=%t tx=%d ty=%d player=(%d,%d)\n",
							g.demoTick-1, g.worldTic, i, look, allAround, tx, ty, g.p.x, g.p.y)
					}
				}
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
	if tmfloor, tmceil, _, ok := g.tryMoveProbeMonster(i, typ, nx, ny); ok {
		g.setThingPosFixed(i, nx, ny)
		if monsterCanFloat(typ) {
			z, _, _ := g.thingSupportState(i, g.m.Things[i])
			g.setThingSupportState(i, z, tmfloor, tmceil)
		} else {
			g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
		}
		return
	}
	if tmfloor, tmceil, _, ok := g.tryMoveProbeMonster(i, typ, x+dx, y); ok {
		g.setThingPosFixed(i, x+dx, y)
		if monsterCanFloat(typ) {
			z, _, _ := g.thingSupportState(i, g.m.Things[i])
			g.setThingSupportState(i, z, tmfloor, tmceil)
		} else {
			g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
		}
		return
	}
	if tmfloor, tmceil, _, ok := g.tryMoveProbeMonster(i, typ, x, y+dy); ok {
		g.setThingPosFixed(i, x, y+dy)
		if monsterCanFloat(typ) {
			z, _, _ := g.thingSupportState(i, g.m.Things[i])
			g.setThingSupportState(i, z, tmfloor, tmceil)
		} else {
			g.setThingSupportState(i, tmfloor, tmfloor, tmceil)
		}
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
	if i >= 0 && i < len(g.thingAmbush) {
		g.thingAmbush[i] = false
	}
}

func (g *game) debugMonsterAngle(i int, src string, angle uint32) {
	if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_ANGLE") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_ANGLE"), "%d:%d", &wantTic, &wantIdx); err != nil {
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
	if g == nil || runtimeDebugEnv("GD_DEBUG_MONSTER_ATTACK") == "" {
		return
	}
	var wantTic, wantIdx int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_MONSTER_ATTACK"), "%d:%d", &wantTic, &wantIdx); err != nil {
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

type monsterMoveProbeResult struct {
	tmfloor    int64
	tmceil     int64
	tmdrop     int64
	probeLines []int
	checkPosOK bool
	floatOK    bool
	ok         bool
}

type skullFlyProbeResult struct {
	tmfloor   int64
	tmceil    int64
	target    lineAttackTarget
	hitTarget bool
	ok        bool
}

func (g *game) probeMonsterMove(i int, typ int16, x, y int64) monsterMoveProbeResult {
	if g.m == nil || len(g.m.Sectors) == 0 || i < 0 || i >= len(g.m.Things) {
		return monsterMoveProbeResult{}
	}
	tmfloor, tmceil, tmdrop, checkPosOK := g.checkPositionForActor(x, y, thingTypeRadius(typ), true, i, true)
	probeLines := append([]int(nil), g.probeSpecialLinesForMover(i)...)
	if g.debugMonsterMoveEnabled() {
		g.debugMonsterMove(i, fmt.Sprintf("probe to=(%d,%d) checkpos=%v floor=%d ceil=%d drop=%d", x, y, checkPosOK, tmfloor, tmceil, tmdrop))
	}
	if !checkPosOK {
		return monsterMoveProbeResult{probeLines: probeLines}
	}
	height := g.thingCurrentHeight(i, g.m.Things[i])
	z, _, _ := g.thingSupportState(i, g.m.Things[i])
	if tmceil-tmfloor < height {
		return monsterMoveProbeResult{tmfloor: tmfloor, tmceil: tmceil, tmdrop: tmdrop, probeLines: probeLines, checkPosOK: true}
	}
	floatOK := true
	if tmceil-z < height {
		return monsterMoveProbeResult{tmfloor: tmfloor, tmceil: tmceil, tmdrop: tmdrop, probeLines: probeLines, checkPosOK: true, floatOK: floatOK}
	}
	if tmfloor-z > stepHeight {
		return monsterMoveProbeResult{tmfloor: tmfloor, tmceil: tmceil, tmdrop: tmdrop, probeLines: probeLines, checkPosOK: true, floatOK: floatOK}
	}
	if !g.thingCanDropOff(i, typ) && !monsterCanFloat(typ) && tmfloor-tmdrop > stepHeight {
		return monsterMoveProbeResult{tmfloor: tmfloor, tmceil: tmceil, tmdrop: tmdrop, probeLines: probeLines, checkPosOK: true, floatOK: floatOK}
	}
	return monsterMoveProbeResult{tmfloor: tmfloor, tmceil: tmceil, tmdrop: tmdrop, probeLines: probeLines, checkPosOK: true, floatOK: floatOK, ok: true}
}

func (g *game) tryMoveProbeMonster(i int, typ int16, x, y int64) (int64, int64, []int, bool) {
	probe := g.probeMonsterMove(i, typ, x, y)
	return probe.tmfloor, probe.tmceil, probe.probeLines, probe.ok
}

func (g *game) probeSkullFlyMove(i int, typ int16, x, y int64) skullFlyProbeResult {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) || len(g.m.Sectors) == 0 {
		return skullFlyProbeResult{}
	}
	const maxThingBlockRadius = 32 * fracUnit
	radius := thingTypeRadius(typ)
	tmboxTop := y + radius
	tmboxBottom := y - radius
	tmboxRight := x + radius
	tmboxLeft := x - radius
	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return skullFlyProbeResult{}
	}
	tmfloor := g.sectorFloor[sec]
	tmceil := g.sectorCeil[sec]
	tmdrop := tmfloor

	if g.stats.Health > 0 && g.playerMobjHealth > 0 && abs(g.p.x-x) < radius+playerRadius && abs(g.p.y-y) < radius+playerRadius {
		return skullFlyProbeResult{target: lineAttackTarget{kind: lineAttackTargetPlayer}, hitTarget: true}
	}

	visitThing := func(other int) (lineAttackTarget, bool) {
		if other == i || other < 0 || other >= len(g.m.Things) {
			return lineAttackTarget{}, false
		}
		oth := g.m.Things[other]
		if other < len(g.thingCollected) && g.thingCollected[other] {
			return lineAttackTarget{}, false
		}
		if !thingTypeIsShootable(oth.Type) && !isPickupType(oth.Type) && !thingTypeBlocksActorMovement(oth.Type, true) {
			return lineAttackTarget{}, false
		}
		ox, oy := g.thingPosFixed(other, oth)
		oradius := g.thingCurrentRadius(other, oth)
		if abs(ox-x) >= radius+oradius || abs(oy-y) >= radius+oradius {
			return lineAttackTarget{}, false
		}
		return lineAttackTarget{kind: lineAttackTargetThing, idx: other}, true
	}
	if g.bmapWidth > 0 && g.bmapHeight > 0 {
		left := int((tmboxLeft - g.bmapOriginX - maxThingBlockRadius) >> (fracBits + 7))
		right := int((tmboxRight - g.bmapOriginX + maxThingBlockRadius) >> (fracBits + 7))
		bottom := int((tmboxBottom - g.bmapOriginY - maxThingBlockRadius) >> (fracBits + 7))
		top := int((tmboxTop - g.bmapOriginY + maxThingBlockRadius) >> (fracBits + 7))
		if left < 0 {
			left = 0
		}
		if bottom < 0 {
			bottom = 0
		}
		if right >= g.bmapWidth {
			right = g.bmapWidth - 1
		}
		if top >= g.bmapHeight {
			top = g.bmapHeight - 1
		}
		for by := bottom; by <= top; by++ {
			for bx := left; bx <= right; bx++ {
				var hit lineAttackTarget
				if !g.blockThingsIterator(bx, by, func(other int) bool {
					var ok bool
					hit, ok = visitThing(other)
					return !ok
				}) {
					return skullFlyProbeResult{target: hit, hitTarget: true}
				}
			}
		}
	} else {
		for other := range g.m.Things {
			if hit, ok := visitThing(other); ok {
				return skullFlyProbeResult{target: hit, hitTarget: true}
			}
		}
	}

	g.validCount++
	xl := int((tmboxLeft - g.bmapOriginX) >> (fracBits + 7))
	xh := int((tmboxRight - g.bmapOriginX) >> (fracBits + 7))
	yl := int((tmboxBottom - g.bmapOriginY) >> (fracBits + 7))
	yh := int((tmboxTop - g.bmapOriginY) >> (fracBits + 7))
	processPhysLine := func(physIdx int) bool {
		if physIdx < 0 || physIdx >= len(g.lines) {
			return true
		}
		if physIdx >= len(g.lineValid) {
			g.lineValid = append(g.lineValid, make([]int, physIdx-len(g.lineValid)+1)...)
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
		if (ld.flags & mlBlockMonsters) != 0 {
			return false
		}
		opentop, openbottom, lowfloor, _ := g.lineOpening(ld)
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
					return skullFlyProbeResult{}
				}
			}
		}
	} else {
		for lineIdx := range g.lines {
			if !processPhysLine(lineIdx) {
				return skullFlyProbeResult{}
			}
		}
	}
	height := g.thingCurrentHeight(i, g.m.Things[i])
	z, _, _ := g.thingSupportState(i, g.m.Things[i])
	if tmceil-tmfloor < height {
		return skullFlyProbeResult{tmfloor: tmfloor, tmceil: tmceil}
	}
	if tmceil-z < height {
		return skullFlyProbeResult{tmfloor: tmfloor, tmceil: tmceil}
	}
	if tmfloor-z > stepHeight {
		return skullFlyProbeResult{tmfloor: tmfloor, tmceil: tmceil}
	}
	if !g.thingCanDropOff(i, typ) && !monsterCanFloat(typ) && tmfloor-tmdrop > stepHeight {
		return skullFlyProbeResult{tmfloor: tmfloor, tmceil: tmceil}
	}
	return skullFlyProbeResult{tmfloor: tmfloor, tmceil: tmceil, ok: true}
}

func (g *game) touchedSpecialLinesForMonsterMove(i int, x, y int64) []int {
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
		if ld.idx >= 0 && ld.idx < len(g.lineSpecial) && g.lineSpecial[ld.idx] != 0 {
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
	for _, lineIdx := range g.touchedSpecialLinesForMonsterMove(i, x, y) {
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

func (g *game) thingCanDropOff(i int, typ int16) bool {
	if monsterCanDropOff(typ) {
		return true
	}
	return g != nil && i >= 0 && i < len(g.thingDead) && g.thingDead[i] && monsterLeavesCorpse(typ)
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
