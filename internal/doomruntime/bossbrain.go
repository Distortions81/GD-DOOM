package doomruntime

import (
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

type bossSpawnCube struct {
	x         int64
	y         int64
	z         int64
	vx        int64
	vy        int64
	vz        int64
	targetIdx int
	ticsLeft  int
}

type bossSpawnFire struct {
	x    int64
	y    int64
	z    int64
	tics int
}

func (g *game) tickBossBrainSpecials() {
	g.tickBossBrainSpawners()
	g.tickBossSpawnCubes()
	g.tickBossSpawnFires()
}

func (g *game) tickBossBrainSpawners() {
	if g == nil || g.m == nil {
		return
	}
	for i, th := range g.m.Things {
		if th.Type != 89 {
			continue
		}
		if i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if i >= len(g.thingCooldown) {
			continue
		}
		if g.thingCooldown[i] > 0 {
			g.thingCooldown[i]--
			if g.thingCooldown[i] == 0 {
				_ = g.bossBrainSpit(i)
				g.thingCooldown[i] = 150
			}
			continue
		}
		g.thingCooldown[i] = 181
		x, y := g.thingPosFixed(i, th)
		g.emitSoundEventAt(soundEventBossBrainAwake, x, y)
	}
}

func (g *game) bossBrainTargetIndices() []int {
	if g == nil || g.m == nil {
		return nil
	}
	out := make([]int, 0, 8)
	for i, th := range g.m.Things {
		if th.Type != 87 {
			continue
		}
		if i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		out = append(out, i)
	}
	return out
}

func (g *game) bossBrainSpit(spawnerIdx int) bool {
	targets := g.bossBrainTargetIndices()
	if len(targets) == 0 || g == nil || g.m == nil || spawnerIdx < 0 || spawnerIdx >= len(g.m.Things) {
		return false
	}
	g.bossBrainEasyToggle = !g.bossBrainEasyToggle
	if normalizeSkillLevel(g.opts.SkillLevel) <= 2 && !g.bossBrainEasyToggle {
		return false
	}
	targetIdx := targets[g.bossBrainTargetOrder%len(targets)]
	g.bossBrainTargetOrder = (g.bossBrainTargetOrder + 1) % len(targets)
	sx, sy := g.thingPosFixed(spawnerIdx, g.m.Things[spawnerIdx])
	g.emitSoundEventAt(soundEventBossBrainSpit, sx, sy)
	return g.spawnBossCube(spawnerIdx, targetIdx)
}

func (g *game) spawnBossCube(spawnerIdx, targetIdx int) bool {
	if g == nil || g.m == nil || spawnerIdx < 0 || spawnerIdx >= len(g.m.Things) || targetIdx < 0 || targetIdx >= len(g.m.Things) {
		return false
	}
	spawner := g.m.Things[spawnerIdx]
	target := g.m.Things[targetIdx]
	sx, sy := g.thingPosFixed(spawnerIdx, spawner)
	sz, _, _ := g.thingSupportState(spawnerIdx, spawner)
	tx, ty := g.thingPosFixed(targetIdx, target)
	tz, _, _ := g.thingSupportState(targetIdx, target)
	dx := tx - sx
	dy := ty - sy
	dz := tz - sz
	dist := doomApproxDistance(dx, dy)
	if dist <= 0 {
		dist = fracUnit
	}
	speed := int64(10 * fracUnit)
	vx := fixedMul(speed, fixedDiv(dx, dist))
	vy := fixedMul(speed, fixedDiv(dy, dist))
	vz := fixedMul(speed, fixedDiv(dz, dist))
	tics := int(dist / speed)
	if tics < 1 {
		tics = 1
	}
	g.emitSoundEventAt(soundEventBossBrainCube, sx, sy)
	g.bossSpawnCubes = append(g.bossSpawnCubes, bossSpawnCube{
		x:         sx,
		y:         sy,
		z:         sz,
		vx:        vx,
		vy:        vy,
		vz:        vz,
		targetIdx: targetIdx,
		ticsLeft:  tics,
	})
	return true
}

func (g *game) tickBossSpawnCubes() {
	if g == nil || len(g.bossSpawnCubes) == 0 {
		return
	}
	dst := g.bossSpawnCubes[:0]
	for _, cube := range g.bossSpawnCubes {
		cube.x += cube.vx
		cube.y += cube.vy
		cube.z += cube.vz
		cube.ticsLeft--
		if cube.ticsLeft <= 0 {
			g.resolveBossCube(cube)
			continue
		}
		dst = append(dst, cube)
	}
	g.bossSpawnCubes = dst
}

func (g *game) tickBossSpawnFires() {
	if g == nil || len(g.bossSpawnFires) == 0 {
		return
	}
	dst := g.bossSpawnFires[:0]
	for _, fx := range g.bossSpawnFires {
		fx.tics--
		if fx.tics > 0 {
			dst = append(dst, fx)
		}
	}
	g.bossSpawnFires = dst
}

func bossBrainSpawnType(r int) int16 {
	switch {
	case r < 50:
		return 3001
	case r < 90:
		return 9
	case r < 120:
		return 58
	case r < 130:
		return 71
	case r < 160:
		return 3005
	case r < 162:
		return 64
	case r < 172:
		return 66
	case r < 192:
		return 68
	case r < 222:
		return 67
	case r < 246:
		return 69
	default:
		return 3003
	}
}

func (g *game) resolveBossCube(cube bossSpawnCube) {
	if g == nil || g.m == nil || cube.targetIdx < 0 || cube.targetIdx >= len(g.m.Things) {
		return
	}
	target := g.m.Things[cube.targetIdx]
	tx, ty := g.thingPosFixed(cube.targetIdx, target)
	tz, floorZ, ceilZ := g.thingSupportState(cube.targetIdx, target)
	g.bossSpawnFires = append(g.bossSpawnFires, bossSpawnFire{x: tx, y: ty, z: tz, tics: 32})
	g.emitSoundEventAt(soundEventTeleport, tx, ty)
	typ := bossBrainSpawnType(doomrand.PRandom())
	idx := g.appendRuntimeThing(mapdata.Thing{
		X:    int16(tx >> fracBits),
		Y:    int16(ty >> fracBits),
		Type: typ,
	}, false)
	if idx < 0 {
		return
	}
	g.setThingPosFixed(idx, tx, ty)
	g.setThingSupportState(idx, tz, floorZ, ceilZ)
	g.thingHP[idx] = monsterSpawnHealth(typ)
	g.thingAggro[idx] = true
	g.thingReactionTics[idx] = 0
	g.thingState[idx] = monsterStateSee
	g.thingStatePhase[idx] = monsterSeeStartPhase(typ)
	g.thingStateTics[idx] = monsterSeeStateTicsAtPhase(typ, g.thingStatePhase[idx], g.fastMonstersActive())
	if idx < len(g.thingMoveDir) {
		g.thingMoveDir[idx] = monsterDirNoDir
	}
	if idx < len(g.thingMoveCount) {
		g.thingMoveCount[idx] = 0
	}
}
