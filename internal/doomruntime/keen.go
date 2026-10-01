package doomruntime

import "gddoom/internal/mapdata"

var (
	keenDeathFrames = []byte{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L'}
	keenDeathTics   = []int{6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, -1}
	keenPainFrames  = []byte{'M', 'M'}
	keenPainTics    = []int{4, 8}
)

func (g *game) tickKeen(i int, th mapdata.Thing) {
	g.tickMonsterMomentum(i, th)
	if g.thingStateTics[i] == -1 || (i < len(g.thingGibbed) && g.thingGibbed[i]) {
		g.tickNightmareRespawn(i, th)
		return
	}
	g.thingStateTics[i]--
	if g.thingStateTics[i] > 0 {
		if g.thingState[i] == monsterStatePain {
			g.syncMonsterPainTics(i, th.Type)
		}
		return
	}
	g.thingStatePhase[i]++
	phase := g.thingStatePhase[i]
	x, y := g.thingPosFixed(i, th)
	switch g.thingState[i] {
	case monsterStatePain:
		if phase == 1 {
			g.thingStateTics[i] = keenPainTics[phase]
			g.emitSoundEventAt(monsterPainSoundEvent(th.Type), x, y)
			g.syncMonsterPainTics(i, th.Type)
		} else {
			g.thingState[i], g.thingStatePhase[i], g.thingStateTics[i] = monsterStateSpawn, 0, -1
			g.thingPainTics[i] = 0
		}
	case monsterStateDeath:
		g.thingStateTics[i] = keenDeathTics[phase]
		if phase == 2 {
			g.emitSoundEventAt(monsterDeathSoundEvent(th.Type), x, y)
		}
		if phase == 10 {
			// A_KeenDie clears solidity, then opens tag 666 only once every
			// other Keen is dead; it runs on S_COMMKEEN11, before the last frame.
			for j, other := range g.m.Things {
				if j != i && other.Type == 72 && !g.thingCollected[j] && g.thingHP[j] > 0 {
					return
				}
			}
			g.activateTaggedDoor(666, mapdata.DoorOpen)
		}
	}
}
