package doomruntime

import (
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

type bossSpawnCube struct {
	order     int64
	floorz    int64
	ceilz     int64
	angle     uint32
	lastLook  int
	x         int64
	y         int64
	z         int64
	vx        int64
	vy        int64
	vz        int64
	targetIdx int
	stateTics int
	stateStep int
	reaction  int
}

type bossSpawnFire struct {
	order    int64
	floorz   int64
	ceilz    int64
	lastLook int
	x        int64
	y        int64
	z        int64
	tics     int
}

func (g *game) tickBossBrainSpecials() {
	g.tickBossBrainSpawners()
	g.tickBossSpawnCubes()
	g.tickBossSpawnFires()
}

// S_BRAINEYE waits for A_Look to find a target before A_BrainAwake.
func (g *game) tickBossBrainSpawner(i int) {
	if g == nil || g.m == nil || i < 0 || i >= len(g.thingStateTics) {
		return
	}
	g.thingStateTics[i]--
	if g.thingStateTics[i] > 0 {
		return
	}
	if g.thingStatePhase[i] == 0 {
		x, y := g.thingPosFixed(i, g.m.Things[i])
		g.thingThreshold[i] = 0
		if _, wake := g.monsterAcquireSectorSoundTarget(i, x, y); !wake && !g.monsterLookForPlayer(i, false, x, y) {
			g.thingStateTics[i] = 10
			return
		}
		g.thingStatePhase[i] = 1
		g.thingStateTics[i] = 181
		g.bossBrainTargetOrder = 0
		g.emitSoundEventAt(soundEventBossBrainAwake, x, y)
		return
	}
	g.thingStatePhase[i] = 2
	g.thingStateTics[i] = 150
	_ = g.bossBrainSpit(i)
}

func (g *game) tickBossBrainSpawners() {
	if g == nil || g.m == nil {
		return
	}
	for i, th := range g.m.Things {
		if th.Type == 89 {
			g.tickBossBrainSpawner(i)
		}
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
	angle := doomPointToAngle2(sx, sy, tx, ty)
	lastLook := doomrand.PRandom() & 3
	speed := int64(10 * fracUnit)
	vx := fixedMul(speed, doomFineCosine(angle))
	vy := fixedMul(speed, doomFineSineAtAngle(angle))
	dist := doomApproxDistance(tx-sx, ty-sy) / speed
	if dist < 1 {
		dist = 1
	}
	vz := (tz - sz) / dist
	cube := bossSpawnCube{
		x: sx + (vx >> 1), y: sy + (vy >> 1), z: sz + 32*fracUnit + (vz >> 1),
		vx: vx, vy: vy, vz: vz, angle: angle, lastLook: lastLook,
		order: g.allocThinkerOrder(), targetIdx: targetIdx,
		stateTics: randomizedMissileSpawnTics(3),
	}
	// A_BrainSpit uses the state definition's three tics, not its randomized
	// instance countdown, and does not call the spawn-state action itself.
	if vy != 0 {
		cube.reaction = int(((ty - sy) / vy) / 3)
	}
	cube.floorz, cube.ceilz = g.projectileSpawnSupportStateAt(cube.x, cube.y)
	g.bossSpawnCubes = append(g.bossSpawnCubes, cube)
	return true
}

func (g *game) advanceBossSpawnCube(cube *bossSpawnCube) bool {
	cube.x += cube.vx
	cube.y += cube.vy
	cube.floorz, cube.ceilz = g.projectileSpawnSupportStateAt(cube.x, cube.y)
	cube.z += cube.vz
	if cube.z <= cube.floorz {
		cube.z = cube.floorz
		if cube.vz < 0 {
			cube.vz = 0
		}
	}
	if cube.ceilz > cube.floorz && cube.z+32*fracUnit > cube.ceilz {
		cube.z = cube.ceilz - 32*fracUnit
		if cube.vz > 0 {
			cube.vz = 0
		}
	}
	cube.stateTics--
	if cube.stateTics <= 0 {
		cube.stateStep = (cube.stateStep + 1) & 3
		cube.stateTics = 3
		if cube.stateStep == 0 {
			g.emitSoundEventAt(soundEventBossBrainCube, cube.x, cube.y)
		}
		if !g.bossCubeRunSpawnAction(cube) {
			return false
		}
	}
	return true
}

func (g *game) tickBossSpawnCubeByOrder(order int64) {
	for i := range g.bossSpawnCubes {
		if g.bossSpawnCubes[i].order != order {
			continue
		}
		cube := g.bossSpawnCubes[i]
		if g.advanceBossSpawnCube(&cube) {
			g.bossSpawnCubes[i] = cube
		} else {
			g.bossSpawnCubes = append(g.bossSpawnCubes[:i], g.bossSpawnCubes[i+1:]...)
		}
		return
	}
}

func (g *game) tickBossSpawnCubes() {
	for i := 0; i < len(g.bossSpawnCubes); {
		order := g.bossSpawnCubes[i].order
		before := len(g.bossSpawnCubes)
		g.tickBossSpawnCubeByOrder(order)
		if len(g.bossSpawnCubes) == before {
			i++
		}
	}
}

func (g *game) bossCubeRunSpawnAction(cube *bossSpawnCube) bool {
	if g == nil || cube == nil {
		return false
	}
	cube.reaction--
	if cube.reaction != 0 {
		return true
	}
	g.resolveBossCube(*cube)
	return false
}

func (g *game) tickBossSpawnFireByOrder(order int64) {
	for i := range g.bossSpawnFires {
		if g.bossSpawnFires[i].order != order {
			continue
		}
		fx := &g.bossSpawnFires[i]
		fx.tics--
		if fx.z < fx.floorz {
			fx.z = fx.floorz
		}
		if fx.z+16*fracUnit > fx.ceilz {
			fx.z = fx.ceilz - 16*fracUnit
		}
		if fx.tics <= 0 {
			g.bossSpawnFires = append(g.bossSpawnFires[:i], g.bossSpawnFires[i+1:]...)
		}
		return
	}
}

func (g *game) tickBossSpawnFires() {
	for i := 0; i < len(g.bossSpawnFires); {
		before := len(g.bossSpawnFires)
		g.tickBossSpawnFireByOrder(g.bossSpawnFires[i].order)
		if len(g.bossSpawnFires) == before {
			i++
		}
	}
}

func bossBrainSpawnType(r int) int16 {
	switch {
	case r < 50:
		return 3001
	case r < 90:
		return 3002
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
	floorZ, ceilZ = g.projectileSpawnSupportStateAt(tx, ty)
	g.bossSpawnFires = append(g.bossSpawnFires, bossSpawnFire{x: tx, y: ty, z: tz, tics: 32, order: g.allocThinkerOrder(), floorz: floorZ, ceilz: ceilZ, lastLook: doomrand.PRandom() & 3})
	g.emitSoundEventAt(soundEventTeleport, tx, ty)
	typ := bossBrainSpawnType(doomrand.PRandom())
	idx := g.appendRuntimeThing(mapdata.Thing{
		X:    int16(tx >> fracBits),
		Y:    int16(ty >> fracBits),
		Type: typ,
		// Keep synthetic map flags separate from the zero spawnpoint that
		// identifies this direct P_SpawnMobj creation for session queries.
		Flags: skillMask,
	}, false)
	if idx < 0 {
		return
	}
	g.setThingPosFixed(idx, tx, ty)
	g.setThingSupportState(idx, tz, floorZ, ceilZ)
	g.thingHP[idx] = monsterSpawnHealth(typ)
	g.ensureMonsterAIState()
	g.thingState[idx] = monsterStateSpawn
	g.thingStatePhase[idx] = 0
	g.thingStateTics[idx] = monsterSpawnStateTics(typ)
	g.thingMoveDir[idx] = monsterDirEast
	if monsterUsesExactDoomStateMachine(typ) {
		g.thingDoomState[idx] = monsterInitialDoomState(typ)
	}
	if g.monsterLookForPlayer(idx, true, tx, ty) {
		g.thingAggro[idx] = true
		if monsterUsesExactDoomStateMachine(typ) {
			g.setExactDoomMonsterState(idx, typ, monsterDoomSeeState(typ))
		} else {
			g.thingStatePhase[idx] = monsterSeeStartPhase(typ)
			g.setMonsterThinkState(idx, typ, monsterStateSee, g.monsterSeeStateTicsForPhase(idx, typ))
			g.thingResumeChaseNow[idx] = true
			// P_SetMobjState runs A_Chase before the telefrag and new thinker tick.
			g.tickGenericMonsterState(idx, g.m.Things[idx])
		}
	}
	tx, ty = g.thingPosFixed(idx, g.m.Things[idx])
	if floor, ceil, ok := g.teleportDestinationHeights(tx, ty); ok && g.teleportStompDestinationThings(tx, ty, monsterRadius(typ), idx, false, tx, ty) {
		g.setThingPosFixed(idx, tx, ty)
		z, _, _ := g.thingSupportState(idx, g.m.Things[idx])
		g.setThingSupportState(idx, z, floor, ceil)
	}
}

func (g *game) tickBossBrain(i int, th mapdata.Thing) {
	g.tickMonsterMomentum(i, th)
	phase := g.thingStatePhase[i]
	if phase == 0 || phase == 5 {
		return
	}
	g.thingStateTics[i]--
	if g.thingStateTics[i] > 0 {
		return
	}
	if phase == 1 {
		g.thingStatePhase[i] = 0
		g.thingStateTics[i] = -1
		return
	}
	g.thingStatePhase[i]++
	if phase == 4 {
		g.thingStateTics[i] = -1
		g.requestLevelExit(false, "Boss brain destroyed")
		return
	}
	g.thingStateTics[i] = 10
}

func (g *game) damageBossBrain(i, damage int, sourcePlayer bool, sourceThing int, ix, iy int64, hasInflictor bool, iz int64, hasInflictorZ bool) {
	if i >= len(g.thingHP) || g.thingHP[i] <= 0 {
		return
	}
	g.ensureMonsterAIState()
	g.applyMonsterDamageThrust(i, damage, sourcePlayer, sourceThing, ix, iy, hasInflictor, iz, hasInflictorZ, g.thingHP[i])
	g.thingHP[i] -= damage
	if g.thingHP[i] <= 0 {
		g.thingDead[i] = true
		g.thingState[i] = monsterStateDeath
		g.thingStatePhase[i] = 2
		x, y := g.thingPosFixed(i, g.m.Things[i])
		for bx := x - 196*fracUnit; bx < x+320*fracUnit; bx += 8 * fracUnit {
			g.spawnBossBrainExplosion(bx, y-320*fracUnit, false)
		}
		g.emitSoundEventAt(soundEventBossBrainDeath, x, y)
		g.thingStateTics[i] = max(1, 100-(doomrand.PRandom()&3))
		return
	}
	if doomrand.PRandom() < 255 {
		g.thingJustHit[i] = true
		g.thingState[i] = monsterStatePain
		g.thingStatePhase[i] = 1
		g.thingStateTics[i] = 36
		x, y := g.thingPosFixed(i, g.m.Things[i])
		g.emitSoundEventAt(soundEventBossBrainPain, x, y)
	}
	g.thingReactionTics[i] = 0
	g.maybeRetargetMonsterAfterDamage(i, 88, sourcePlayer, sourceThing)
}

// Brain explosions are live MT_ROCKET thinkers in a separate state chain.
// They retain MF_MISSILE, so hitting a floor or ceiling enters S_EXPLODE1.
func (g *game) spawnBossBrainExplosion(x, y int64, scattered bool) {
	if scattered {
		x += int64(doomrand.PRandom()-doomrand.PRandom()) * 2048
	}
	z := int64(128) + int64(doomrand.PRandom())*2*fracUnit
	lastLook := doomrand.PRandom() & 3
	vz := int64(doomrand.PRandom()) * 512
	tics := max(1, 10-(doomrand.PRandom()&7))
	floor, ceil := g.projectileSpawnSupportStateAt(x, y)
	g.projectiles = append(g.projectiles, projectile{
		x: x, y: y, z: z, prevX: x, prevY: y, prevZ: z,
		vz: vz, floorz: floor, ceilz: ceil,
		radius: 11 * fracUnit, height: 8 * fracUnit,
		kind: projectileBrainExplosion, sourceThing: -1, lastLook: lastLook,
		frameTics: tics, ttl: 1 << 30, order: g.allocThinkerOrder(),
	})
}
