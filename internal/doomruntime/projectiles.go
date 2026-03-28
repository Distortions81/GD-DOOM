package doomruntime

import (
	"fmt"
	"math"

	"gddoom/internal/doomrand"
)

type projectileKind int

const (
	projectileFireball projectileKind = iota
	projectilePlasmaBall
	projectileBaronBall
	projectileTracer
	projectileFatShot
	projectileRocket
	projectilePlayerPlasma
	projectileBFGBall
)

type projectile struct {
	x            int64
	y            int64
	z            int64
	prevX        int64
	prevY        int64
	prevZ        int64
	vx           int64
	vy           int64
	vz           int64
	floorz       int64
	ceilz        int64
	radius       int64
	height       int64
	ttl          int
	sourceX      int64
	sourceY      int64
	sourceThing  int
	sourceType   int16
	sourcePlayer bool
	tracerPlayer bool
	lastLook     int
	frame        int
	frameTics    int
	angle        uint32
	kind         projectileKind
	order        int64
	deferredTick bool
}

type projectileImpact struct {
	x            int64
	y            int64
	z            int64
	floorz       int64
	ceilz        int64
	kind         projectileKind
	order        int64
	sourceThing  int
	sourceType   int16
	sourcePlayer bool
	lastLook     int
	tics         int
	totalTics    int
	phase        int
	phaseTics    int
	angle        uint32
	sprayDone    bool
}

func usesMonsterProjectile(typ int16) bool {
	switch typ {
	case 3001, 3005, 3003, 69, 16, 66, 67, 68:
		return true
	default:
		return false
	}
}

func monsterProjectileKind(typ int16) projectileKind {
	switch typ {
	case 3005, 68:
		// Cacodemon shot uses BAL2 in vanilla Doom.
		return projectilePlasmaBall
	case 3003, 69:
		return projectileBaronBall
	case 66:
		return projectileTracer
	case 67:
		return projectileFatShot
	case 16:
		return projectileRocket
	default:
		return projectileFireball
	}
}

func monsterProjectileSpeed(typ int16, fast bool) int64 {
	scale := int64(1)
	if fast {
		scale = 2
	}
	switch typ {
	case 3003, 69:
		return 15 * fracUnit * scale
	case 66, 67, 16:
		return 20 * fracUnit * scale
	case 68:
		return 25 * fracUnit * scale
	default:
		return 10 * fracUnit * scale
	}
}

func monsterProjectileRadius(typ int16) int64 {
	switch typ {
	case 16:
		return 11 * fracUnit
	case 66:
		return 11 * fracUnit
	case 67:
		return 6 * fracUnit
	case 68:
		return 13 * fracUnit
	default:
		return 6 * fracUnit
	}
}

func monsterProjectileHeight(typ int16) int64 {
	switch typ {
	case 16:
		return 8 * fracUnit
	case 68:
		return 8 * fracUnit
	default:
		return 8 * fracUnit
	}
}

func monsterProjectileTTL(typ int16) int {
	switch typ {
	case 16:
		return 10 * doomTicsPerSecond
	default:
		return 8 * doomTicsPerSecond
	}
}

func (g *game) spawnMonsterProjectile(thingIdx int, typ int16) bool {
	if g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) {
		return false
	}
	if !usesMonsterProjectile(typ) {
		return false
	}
	th := g.m.Things[thingIdx]
	sx, sy := g.thingPosFixed(thingIdx, th)
	sourceZ, _, _ := g.thingSupportState(thingIdx, th)
	sz := sourceZ + 32*fracUnit
	tx, ty, tz, height, _, ok := g.monsterTargetPos(thingIdx)
	if !ok {
		return false
	}
	_ = height
	kind := monsterProjectileKind(typ)
	lastLook := doomrand.PRandom() & 3
	aimAngle := g.monsterAimAngleToTarget(thingIdx, sx, sy)

	dx := fixedMul(monsterProjectileSpeed(typ, g.fastMonstersActive()), doomFineCosine(aimAngle))
	dy := fixedMul(monsterProjectileSpeed(typ, g.fastMonstersActive()), doomFineSineAtAngle(aimAngle))
	speed := monsterProjectileSpeed(typ, g.fastMonstersActive())
	vx := dx
	vy := dy
	dist := doomApproxDistance(tx-sx, ty-sy) / speed
	if dist < 1 {
		dist = 1
	}
	vz := (tz - sourceZ) / dist
	if vx == 0 && vy == 0 {
		return false
	}

	p := projectile{
		x:            sx,
		y:            sy,
		z:            sz,
		prevX:        sx,
		prevY:        sy,
		prevZ:        sz,
		vx:           vx,
		vy:           vy,
		vz:           vz,
		radius:       monsterProjectileRadius(typ),
		height:       monsterProjectileHeight(typ),
		ttl:          monsterProjectileTTL(typ),
		sourceX:      sx,
		sourceY:      sy,
		sourceThing:  thingIdx,
		sourceType:   typ,
		tracerPlayer: typ == 66,
		lastLook:     lastLook,
		frameTics:    randomizedMissileSpawnTics(projectileSpawnStateTics(kind)),
		kind:         kind,
		angle:        aimAngle,
		order:        g.allocThinkerOrder(),
		deferredTick: true,
	}
	p.floorz, p.ceilz = g.projectileSupportStateAt(p.x, p.y, p.radius)
	if !g.finishProjectileSpawn(&p, true) {
		return false
	}
	g.projectiles = append(g.projectiles, p)
	g.emitSoundEventAt(projectileLaunchSoundEvent(typ), sx, sy)
	return true
}

func (g *game) spawnMonsterProjectileAngleOffset(thingIdx int, typ int16, angleOffset uint32) bool {
	if !g.spawnMonsterProjectile(thingIdx, typ) {
		return false
	}
	p := &g.projectiles[len(g.projectiles)-1]
	if angleOffset == 0 {
		return true
	}
	p.angle += angleOffset
	speed := monsterProjectileSpeed(typ, g.fastMonstersActive())
	p.vx = fixedMul(speed, doomFineCosine(p.angle))
	p.vy = fixedMul(speed, doomFineSineAtAngle(p.angle))
	return true
}

func (g *game) tickProjectiles() {
	if len(g.projectiles) == 0 {
		return
	}
	kept := g.projectiles[:0]
	for _, p := range g.projectiles {
		if p.deferredTick {
			kept = append(kept, p)
			continue
		}
		if next, keep := g.advanceProjectile(p); keep {
			kept = append(kept, next)
		}
	}
	g.projectiles = kept
}

func (g *game) tickProjectileByOrder(order int64) {
	if g == nil || len(g.projectiles) == 0 {
		return
	}
	for i := range g.projectiles {
		if g.projectiles[i].order != order || g.projectiles[i].deferredTick {
			continue
		}
		p := g.projectiles[i]
		if next, keep := g.advanceProjectile(p); keep {
			g.projectiles[i] = next
		} else {
			g.projectiles = append(g.projectiles[:i], g.projectiles[i+1:]...)
		}
		return
	}
}

func (g *game) tickDeferredProjectiles() {
	if len(g.projectiles) == 0 {
		return
	}
	kept := g.projectiles[:0]
	for _, p := range g.projectiles {
		if !p.deferredTick {
			kept = append(kept, p)
			continue
		}
		p.deferredTick = false
		next, keep := g.advanceProjectile(p)
		if keep {
			kept = append(kept, next)
		}
	}
	g.projectiles = kept
}

func (g *game) advanceProjectile(p projectile) (projectile, bool) {
	if p.ttl <= 0 {
		return projectile{}, false
	}
	ox, oy, oz := p.x, p.y, p.z
	nx := ox + p.vx
	ny := oy + p.vy
	nz := oz + p.vz
	thingHit, hitThing := g.projectileHitsShootableThingAlongPath(p, ox, oy, oz, nx, ny, nz)
	blocked, blockFrac, tmfloorz, tmceilingz, _ := g.projectileBlockedAt(p, ox, oy, oz, nx, ny, nz)
	if want := runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"); want != "" {
		var tic int
		if _, err := fmt.Sscanf(want, "%d", &tic); err == nil && (g.demoTick-1 == tic || g.worldTic == tic) && p.kind == projectileFireball {
			fmt.Printf("projectile-debug tic=%d world=%d phase=advance from=(%d,%d,%d) to=(%d,%d,%d) hitThing=%t hitFrac=%f hitIdx=%d blocked=%t blockFrac=%f floor=%d ceil=%d\n",
				g.demoTick-1, g.worldTic, ox, oy, oz, nx, ny, nz, hitThing, thingHit.frac, thingHit.idx, blocked, blockFrac, p.floorz, p.ceilz)
		}
	}
	if hitThing && (!blocked || thingHit.frac <= blockFrac) {
		if thingHit.isPlayer {
			if dmg := projectileDamage(p); dmg > 0 {
				g.damagePlayerFrom(dmg, projectileHitMessage(p.kind), ox, oy, true, p.sourceThing)
			}
		} else if g.projectileCanDamageThing(p, thingHit.idx) {
			if dmg := projectileDamage(p); dmg > 0 {
				g.damageShootableThingFrom(thingHit.idx, dmg, p.sourcePlayer, p.sourceThing, ox, oy, true)
			}
		}
		g.explodeProjectileAt(p, ox, oy, oz)
		return projectile{}, false
	}
	if blocked {
		g.explodeProjectileAt(p, ox, oy, oz)
		return projectile{}, false
	}
	p.x = nx
	p.y = ny
	p.floorz, p.ceilz = tmfloorz, tmceilingz
	p.z = nz
	p.prevX = ox
	p.prevY = oy
	p.prevZ = oz
	if p.z <= p.floorz {
		p.z = p.floorz
		g.explodeProjectileAt(p, p.x, p.y, p.z)
		return projectile{}, false
	}
	if p.z+p.height > p.ceilz {
		p.z = p.ceilz - p.height
		g.explodeProjectileAt(p, p.x, p.y, p.z)
		return projectile{}, false
	}
	g.tickProjectileSpecial(&p)
	g.tickProjectileAnim(&p)
	p.ttl--
	if p.ttl <= 0 {
		g.explodeProjectileAt(p, p.x, p.y, p.z)
		return projectile{}, false
	}
	return p, true
}

func sameMonsterSpecies(a, b int16) bool {
	if a == b {
		return true
	}
	return (a == 3003 && b == 69) || (a == 69 && b == 3003)
}

func (g *game) projectileCanDamageThing(p projectile, thingIdx int) bool {
	if g == nil || g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) {
		return false
	}
	if p.sourcePlayer || p.sourceThing < 0 || p.sourceThing >= len(g.m.Things) {
		return true
	}
	hitType := g.m.Things[thingIdx].Type
	if hitType == 1 {
		return true
	}
	return !sameMonsterSpecies(p.sourceType, hitType)
}

func projectileDamage(p projectile) int {
	base := projectileBaseDamage(p.kind)
	if base <= 0 {
		return 0
	}
	return base * (1 + doomPRandomN(8))
}

func projectileSplashDamage(kind projectileKind) int {
	switch kind {
	case projectileRocket:
		return 128
	default:
		return 0
	}
}

func (g *game) projectileSplashDamage(p projectile, x, y, z int64) {
	damage := projectileSplashDamage(p.kind)
	if damage <= 0 {
		return
	}
	g.radiusAttackAt(x, y, z, p.height, -1, damage, projectileHitMessage(p.kind), p.sourcePlayer, p.sourceThing)
}

func (g *game) explodeProjectileAt(p projectile, x, y, z int64) {
	if g == nil {
		return
	}
	if p.kind == projectileRocket {
		idx := g.spawnProjectileImpactFromDeferredRandom(p, x, y, z)
		g.projectileSplashDamage(p, x, y, z)
		g.finalizeDeferredProjectileImpact(idx)
		g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), x, y)
		return
	}
	g.spawnProjectileImpactFrom(p, x, y, z)
	g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), x, y)
	g.projectileSplashDamage(p, x, y, z)
}

func (g *game) spawnPlayerRocket() bool {
	if g == nil {
		return false
	}
	const (
		rocketSpeed  = 20 * fracUnit
		rocketRadius = 11 * fracUnit
		rocketHeight = 8 * fracUnit
		rocketTTL    = 10 * doomTicsPerSecond
	)
	angle, slope := g.playerMissileAim(g.p.angle, 1024*fracUnit)
	vx := fixedMul(rocketSpeed, doomFineCosine(angle))
	vy := fixedMul(rocketSpeed, doomFineSineAtAngle(angle))
	if vx == 0 && vy == 0 {
		return false
	}
	vz := fixedMul(rocketSpeed, slope)
	sx := g.p.x
	sy := g.p.y
	sz := g.p.z + 32*fracUnit
	lastLook := doomrand.PRandom() & 3
	p := projectile{
		x:            sx,
		y:            sy,
		z:            sz,
		prevX:        sx,
		prevY:        sy,
		prevZ:        sz,
		vx:           vx,
		vy:           vy,
		vz:           vz,
		radius:       rocketRadius,
		height:       rocketHeight,
		ttl:          rocketTTL,
		sourceX:      g.p.x,
		sourceY:      g.p.y,
		sourceThing:  -1,
		sourceType:   16,
		sourcePlayer: true,
		lastLook:     lastLook,
		frameTics:    randomizedMissileSpawnTics(projectileSpawnStateTics(projectileRocket)),
		kind:         projectileRocket,
		angle:        angle,
		order:        g.allocThinkerOrder(),
	}
	p.floorz, p.ceilz = g.projectileSupportStateAt(p.x, p.y, p.radius)
	if !g.finishProjectileSpawn(&p, true) {
		return false
	}
	g.projectiles = append(g.projectiles, p)
	return true
}

func projectileBaseDamage(kind projectileKind) int {
	switch kind {
	case projectileFireball:
		return 3
	case projectilePlasmaBall, projectilePlayerPlasma:
		return 5
	case projectileBaronBall:
		return 8
	case projectileTracer:
		return 10
	case projectileFatShot:
		return 8
	case projectileRocket:
		return 20
	case projectileBFGBall:
		return 100
	default:
		return 0
	}
}

func (g *game) spawnPlayerPlasma() bool {
	return g.spawnPlayerMissile(projectilePlayerPlasma, 25*fracUnit, 13*fracUnit, 8*fracUnit, 10*doomTicsPerSecond)
}

func (g *game) spawnPlayerBFG() bool {
	return g.spawnPlayerMissile(projectileBFGBall, 25*fracUnit, 13*fracUnit, 8*fracUnit, 10*doomTicsPerSecond)
}

func (g *game) spawnPlayerMissile(kind projectileKind, speed, radius, height int64, ttl int) bool {
	if g == nil {
		return false
	}
	angle, slope := g.playerMissileAim(g.p.angle, 1024*fracUnit)
	vx := fixedMul(speed, doomFineCosine(angle))
	vy := fixedMul(speed, doomFineSineAtAngle(angle))
	if vx == 0 && vy == 0 {
		return false
	}
	vz := fixedMul(speed, slope)
	sx := g.p.x
	sy := g.p.y
	sz := g.p.z + 32*fracUnit
	lastLook := doomrand.PRandom() & 3
	p := projectile{
		x:            sx,
		y:            sy,
		z:            sz,
		prevX:        sx,
		prevY:        sy,
		prevZ:        sz,
		vx:           vx,
		vy:           vy,
		vz:           vz,
		radius:       radius,
		height:       height,
		ttl:          ttl,
		sourceX:      g.p.x,
		sourceY:      g.p.y,
		sourceThing:  -1,
		sourceType:   0,
		sourcePlayer: true,
		lastLook:     lastLook,
		frameTics:    randomizedMissileSpawnTics(projectileSpawnStateTics(kind)),
		kind:         kind,
		angle:        angle,
		order:        g.allocThinkerOrder(),
	}
	p.floorz, p.ceilz = g.projectileSupportStateAt(p.x, p.y, p.radius)
	if !g.finishProjectileSpawn(&p, true) {
		return false
	}
	g.projectiles = append(g.projectiles, p)
	return true
}

func (g *game) finishProjectileSpawn(p *projectile, advance bool) bool {
	if g == nil || p == nil {
		return false
	}
	ox, oy, oz := p.x, p.y, p.z
	nx := ox + (p.vx >> 1)
	ny := oy + (p.vy >> 1)
	nz := oz + (p.vz >> 1)
	if len(g.sectorFloor) == 0 || len(g.sectorCeil) == 0 {
		if advance {
			p.x = nx
			p.y = ny
			p.z = nz
		}
		return true
	}
	thingHit, hitThing := g.projectileHitsShootableThingAlongPath(*p, ox, oy, oz, nx, ny, nz)
	blocked, blockFrac, tmfloorz, tmceilingz, _ := g.projectileBlockedAt(*p, ox, oy, oz, nx, ny, nz)
	if want := runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"); want != "" {
		var tic int
		if _, err := fmt.Sscanf(want, "%d", &tic); err == nil && (g.demoTick-1 == tic || g.worldTic == tic) {
			fmt.Printf("projectile-spawn-debug tic=%d world=%d sourceThing=%d sourceType=%d kind=%d from=(%d,%d,%d) to=(%d,%d,%d) hitThing=%t hitIdx=%d hitFrac=%f blocked=%t blockFrac=%f floor=%d ceil=%d\n",
				g.demoTick-1, g.worldTic, p.sourceThing, p.sourceType, p.kind, ox, oy, oz, nx, ny, nz,
				hitThing, thingHit.idx, thingHit.frac, blocked, blockFrac, tmfloorz, tmceilingz)
		}
	}
	if hitThing && (!blocked || thingHit.frac <= blockFrac) {
		if thingHit.isPlayer {
			if dmg := projectileDamage(*p); dmg > 0 {
				g.damagePlayerFrom(dmg, projectileHitMessage(p.kind), ox, oy, true, p.sourceThing)
			}
		} else if g.projectileCanDamageThing(*p, thingHit.idx) {
			if dmg := projectileDamage(*p); dmg > 0 {
				g.damageShootableThingFrom(thingHit.idx, dmg, p.sourcePlayer, p.sourceThing, ox, oy, true)
			}
		}
		g.explodeProjectileAt(*p, ox, oy, oz)
		return false
	}
	if blocked {
		g.explodeProjectileAt(*p, ox, oy, oz)
		return false
	}
	p.floorz, p.ceilz = tmfloorz, tmceilingz
	if advance {
		p.x = nx
		p.y = ny
		p.z = nz
	}
	if p.z <= p.floorz {
		p.z = p.floorz
		g.explodeProjectileAt(*p, p.x, p.y, p.z)
		return false
	}
	if p.z+p.height > p.ceilz {
		p.z = p.ceilz - p.height
		g.explodeProjectileAt(*p, p.x, p.y, p.z)
		return false
	}
	return true
}

func (g *game) applyBFGSpray(center uint32) {
	if g == nil {
		return
	}
	for i := 0; i < 40; i++ {
		ang := center - degToAngle(45) + uint32((float64(degToAngle(90))/40.0)*float64(i))
		slope, ok := g.aimLineAttack(g.playerLineAttackActor(), ang, 1024*fracUnit)
		if !ok {
			continue
		}
		outcome := g.lineAttackTrace(g.playerLineAttackActor(), ang, 1024*fracUnit, slope, false)
		if outcome.target.kind != lineAttackTargetThing {
			continue
		}
		damage := 0
		for j := 0; j < 15; j++ {
			damage += (doomrand.PRandom() & 7) + 1
		}
		g.spawnHitscanPuff(outcome.impactX, outcome.impactY, outcome.impactZ)
		g.damageShootableThingFrom(outcome.target.idx, damage, true, -1, g.p.x, g.p.y, true)
	}
}

func (g *game) tickProjectileImpacts() {
	if len(g.projectileImpacts) == 0 {
		return
	}
	keep := g.projectileImpacts[:0]
	for _, fx := range g.projectileImpacts {
		if !g.advanceProjectileImpactTic(&fx) {
			continue
		}
		keep = append(keep, fx)
	}
	g.projectileImpacts = keep
}

func (g *game) tickProjectileImpactByOrder(order int64) {
	if g == nil || len(g.projectileImpacts) == 0 {
		return
	}
	for i := range g.projectileImpacts {
		if g.projectileImpacts[i].order != order {
			continue
		}
		fx := g.projectileImpacts[i]
		if g.advanceProjectileImpactTic(&fx) {
			g.projectileImpacts[i] = fx
		} else {
			g.projectileImpacts = append(g.projectileImpacts[:i], g.projectileImpacts[i+1:]...)
		}
		return
	}
}

func (g *game) spawnProjectileImpact(kind projectileKind, x, y, z int64, angle uint32) {
	const maxImpacts = 64
	if len(g.projectileImpacts) >= maxImpacts {
		copy(g.projectileImpacts, g.projectileImpacts[1:])
		g.projectileImpacts = g.projectileImpacts[:maxImpacts-1]
	}
	if want := runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("projectile-impact-debug tic=%d world=%d phase=pre kind=%d pos=(%d,%d,%d) rnd=%d prnd=%d\n",
					g.demoTick-1, g.worldTic, kind, x, y, z, rnd, prnd)
			}
		}
	}
	first := randomizedStateTics(projectileImpactPhaseTics(kind, 0))
	if want := runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("projectile-impact-debug tic=%d world=%d phase=post kind=%d first=%d pos=(%d,%d,%d) rnd=%d prnd=%d\n",
					g.demoTick-1, g.worldTic, kind, first, x, y, z, rnd, prnd)
			}
		}
	}
	tics := first
	for phase := 1; ; phase++ {
		next := projectileImpactPhaseTics(kind, phase)
		if next <= 0 {
			break
		}
		tics += next
	}
	floorz, ceilz := g.projectileSupportStateAt(x, y, demoTraceProjectileImpactRadius(kind))
	g.projectileImpacts = append(g.projectileImpacts, projectileImpact{
		x:         x,
		y:         y,
		z:         z,
		floorz:    floorz,
		ceilz:     ceilz,
		kind:      kind,
		order:     g.allocThinkerOrder(),
		tics:      tics,
		totalTics: tics,
		phaseTics: first,
		angle:     angle,
	})
}

func (g *game) spawnProjectileImpactDeferredRandom(kind projectileKind, x, y, z int64, angle uint32) int {
	const maxImpacts = 64
	if len(g.projectileImpacts) >= maxImpacts {
		copy(g.projectileImpacts, g.projectileImpacts[1:])
		g.projectileImpacts = g.projectileImpacts[:maxImpacts-1]
	}
	if want := runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("projectile-impact-deferred-debug tic=%d world=%d phase=spawn kind=%d pos=(%d,%d,%d) rnd=%d prnd=%d\n",
					g.demoTick-1, g.worldTic, kind, x, y, z, rnd, prnd)
			}
		}
	}
	first := projectileImpactPhaseTics(kind, 0)
	tics := first
	for phase := 1; ; phase++ {
		next := projectileImpactPhaseTics(kind, phase)
		if next <= 0 {
			break
		}
		tics += next
	}
	floorz, ceilz := g.projectileSupportStateAt(x, y, demoTraceProjectileImpactRadius(kind))
	g.projectileImpacts = append(g.projectileImpacts, projectileImpact{
		x:         x,
		y:         y,
		z:         z,
		floorz:    floorz,
		ceilz:     ceilz,
		kind:      kind,
		order:     g.allocThinkerOrder(),
		tics:      tics,
		totalTics: tics,
		phaseTics: first,
		angle:     angle,
	})
	return len(g.projectileImpacts) - 1
}

func (g *game) spawnProjectileImpactFrom(p projectile, x, y, z int64) {
	g.spawnProjectileImpact(p.kind, x, y, z, p.angle)
	if len(g.projectileImpacts) == 0 {
		return
	}
	fx := &g.projectileImpacts[len(g.projectileImpacts)-1]
	if p.order > 0 {
		// Doom keeps the same missile thinker when it enters its death states,
		// so the exploded projectile must retain its original thinker order.
		fx.order = p.order
	}
	fx.floorz = p.floorz
	fx.ceilz = p.ceilz
	fx.sourceThing = p.sourceThing
	fx.sourceType = p.sourceType
	fx.sourcePlayer = p.sourcePlayer
	fx.lastLook = p.lastLook
}

func (g *game) spawnProjectileImpactFromDeferredRandom(p projectile, x, y, z int64) int {
	idx := g.spawnProjectileImpactDeferredRandom(p.kind, x, y, z, p.angle)
	if idx < 0 || idx >= len(g.projectileImpacts) {
		return -1
	}
	fx := &g.projectileImpacts[idx]
	if p.order > 0 {
		fx.order = p.order
	}
	fx.floorz = p.floorz
	fx.ceilz = p.ceilz
	fx.sourceThing = p.sourceThing
	fx.sourceType = p.sourceType
	fx.sourcePlayer = p.sourcePlayer
	fx.lastLook = p.lastLook
	return idx
}

func (g *game) finalizeDeferredProjectileImpact(idx int) {
	if g == nil || idx < 0 || idx >= len(g.projectileImpacts) {
		return
	}
	fx := &g.projectileImpacts[idx]
	base := projectileImpactPhaseTics(fx.kind, fx.phase)
	if base <= 0 {
		return
	}
	if want := runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("projectile-impact-deferred-debug tic=%d world=%d phase=finalize-pre kind=%d base=%d rnd=%d prnd=%d\n",
					g.demoTick-1, g.worldTic, fx.kind, base, rnd, prnd)
			}
		}
	}
	first := randomizedStateTics(base)
	if want := runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"); want != "" {
		var wantTic int
		if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
			if g.demoTick-1 == wantTic || g.worldTic == wantTic {
				rnd, prnd := doomrand.State()
				fmt.Printf("projectile-impact-deferred-debug tic=%d world=%d phase=finalize-post kind=%d first=%d rnd=%d prnd=%d\n",
					g.demoTick-1, g.worldTic, fx.kind, first, rnd, prnd)
			}
		}
	}
	fx.tics -= base - first
	fx.totalTics -= base - first
	fx.phaseTics = first
}

func (g *game) advanceProjectileImpactTic(fx *projectileImpact) bool {
	if fx == nil {
		return false
	}
	fx.tics--
	fx.phaseTics--
	if fx.phaseTics <= 0 {
		fx.phase++
		next := projectileImpactPhaseTics(fx.kind, fx.phase)
		if next <= 0 {
			return false
		}
		fx.phaseTics = next
		if fx.kind == projectileBFGBall && !fx.sprayDone && fx.phase == 2 {
			fx.sprayDone = true
			g.applyBFGSpray(fx.angle)
		}
	}
	return fx.tics > 0
}

func projectileSpawnStateTics(kind projectileKind) int {
	switch kind {
	case projectileTracer:
		return 2
	case projectilePlayerPlasma:
		return 6
	case projectileRocket:
		return 1
	case projectileBFGBall:
		return 4
	default:
		return 4
	}
}

func projectileImpactPhaseTics(kind projectileKind, phase int) int {
	switch kind {
	case projectileBFGBall:
		if phase >= 0 && phase <= 5 {
			return 8
		}
	case projectileRocket, projectileFatShot, projectileTracer:
		switch phase {
		case 0:
			return 8
		case 1:
			return 6
		case 2:
			return 4
		}
	case projectilePlayerPlasma:
		if phase >= 0 && phase <= 4 {
			return 4
		}
	default:
		if phase >= 0 && phase <= 2 {
			return 6
		}
	}
	return 0
}

func randomizedStateTics(base int) int {
	if base <= 1 {
		return 1
	}
	tics := base - (doomrand.PRandom() & 3)
	if tics < 1 {
		return 1
	}
	return tics
}

func randomizedMissileSpawnTics(base int) int {
	if base < 1 {
		base = 1
	}
	tics := base - (doomrand.PRandom() & 3)
	if tics < 1 {
		return 1
	}
	return tics
}

func (g *game) tickProjectileAnim(p *projectile) {
	if p == nil {
		return
	}
	p.frameTics--
	if p.frameTics > 0 {
		return
	}
	switch p.kind {
	case projectileRocket:
		p.frame = 0
	default:
		p.frame ^= 1
	}
	p.frameTics = projectileSpawnStateTics(p.kind)
}

func (g *game) projectileSupportStateAt(x, y, radius int64) (int64, int64) {
	if g == nil || g.m == nil || len(g.sectorFloor) == 0 || len(g.sectorCeil) == 0 {
		return 0, 0
	}
	if tmfloor, tmceil, _, ok := g.checkPositionForActor(x, y, radius, false, -1, false); ok {
		return tmfloor, tmceil
	}
	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
		return 0, 0
	}
	return g.sectorFloor[sec], g.sectorCeil[sec]
}

func (g *game) refreshProjectileSupportInSector(sec int) {
	if g == nil || sec < 0 {
		return
	}
	for i := range g.projectiles {
		p := &g.projectiles[i]
		if !g.actorTouchesSector(sec, p.x, p.y, p.radius) {
			continue
		}
		p.floorz, p.ceilz = g.projectileSupportStateAt(p.x, p.y, p.radius)
	}
	for i := range g.projectileImpacts {
		fx := &g.projectileImpacts[i]
		radius := demoTraceProjectileImpactRadius(fx.kind)
		if !g.actorTouchesSector(sec, fx.x, fx.y, radius) {
			continue
		}
		fx.floorz, fx.ceilz = g.projectileSupportStateAt(fx.x, fx.y, radius)
	}
}

func (g *game) projectileBlockedAt(p projectile, ox, oy, oz, nx, ny, nz int64) (bool, float64, int64, int64, int64) {
	if g.m == nil {
		return false, 1, 0, 0, nz
	}
	sec := g.sectorAt(nx, ny)
	if sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
		g.debugProjectileBlock(p, ox, oy, oz, nx, ny, nz, "bad-sector", 1, nx, ny, nz)
		return true, 1, 0, 0, nz
	}
	tmfloorz := g.sectorFloor[sec]
	tmceilingz := g.sectorCeil[sec]
	tmdropoffz := tmfloorz
	tmbox := [4]int64{
		ny + p.radius,
		ny - p.radius,
		nx + p.radius,
		nx - p.radius,
	}
	bestFrac := 2.0
	bestX, bestY, bestZ := nx, ny, nz
	processLine := func(physIdx int) bool {
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
		if tmbox[2] <= ld.bbox[3] || tmbox[3] >= ld.bbox[2] || tmbox[0] <= ld.bbox[1] || tmbox[1] >= ld.bbox[0] {
			return true
		}
		if g.boxOnLineSide(tmbox, ld) != -1 {
			return true
		}
		frac := 1.0
		if f, ok := segmentIntersectFrac(ox, oy, nx, ny, ld.x1, ld.y1, ld.x2, ld.y2); ok {
			frac = f
		}
		hx := ox + int64(float64(nx-ox)*frac)
		hy := oy + int64(float64(ny-oy)*frac)
		hz := oz
		if ld.sideNum1 < 0 {
			g.debugProjectileBlock(p, ox, oy, oz, nx, ny, nz, "onesided", frac, hx, hy, hz)
			bestFrac, bestX, bestY, bestZ = frac, hx, hy, hz
			return false
		}
		opentop, openbottom, _, openrange := g.lineOpening(ld)
		if g == nil || runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC") == "" || p.kind != projectileFireball {
			// no-op
		} else {
			var tic int
			if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"), "%d", &tic); err == nil && (g.demoTick-1 == tic || g.worldTic == tic) {
				fmt.Printf("projectile-line-debug tic=%d world=%d line=%d opentop=%d openbottom=%d openrange=%d oz=%d height=%d\n",
					g.demoTick-1, g.worldTic, ld.idx, opentop, openbottom, openrange, oz, p.height)
			}
		}
		if opentop < tmceilingz {
			tmceilingz = opentop
		}
		if openbottom > tmfloorz {
			tmfloorz = openbottom
		}
		if lowfloor, ok := g.lineLowFloor(ld); ok && lowfloor < tmdropoffz {
			tmdropoffz = lowfloor
		}
		return true
	}
	g.validCount++
	iter := func(lineIdx int) bool {
		if lineIdx < 0 || lineIdx >= len(g.physForLine) {
			return true
		}
		return processLine(g.physForLine[lineIdx])
	}
	if g.m != nil && g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		xl := int((tmbox[3] - g.bmapOriginX) >> (fracBits + 7))
		xh := int((tmbox[2] - g.bmapOriginX) >> (fracBits + 7))
		yl := int((tmbox[1] - g.bmapOriginY) >> (fracBits + 7))
		yh := int((tmbox[0] - g.bmapOriginY) >> (fracBits + 7))
		for bx := xl; bx <= xh; bx++ {
			for by := yl; by <= yh; by++ {
				if !g.blockLinesIterator(bx, by, iter) {
					return true, bestFrac, bestX, bestY, bestZ
				}
			}
		}
	} else {
		for i := range g.lines {
			if !processLine(i) {
				return true, bestFrac, bestX, bestY, bestZ
			}
		}
	}
	if tmceilingz-tmfloorz < p.height {
		g.debugProjectileBlock(p, ox, oy, oz, nx, ny, nz, "fit", 1, nx, ny, oz)
		return true, 1, tmfloorz, tmceilingz, oz
	}
	if tmceilingz-oz < p.height {
		g.debugProjectileBlock(p, ox, oy, oz, nx, ny, nz, "ceil", 1, nx, ny, oz)
		return true, 1, tmfloorz, tmceilingz, oz
	}
	if tmfloorz-oz > 24*fracUnit {
		g.debugProjectileBlock(p, ox, oy, oz, nx, ny, nz, "step", 1, nx, ny, oz)
		return true, 1, tmfloorz, tmceilingz, oz
	}
	_ = tmdropoffz
	return false, 1, tmfloorz, tmceilingz, nz
}

func (g *game) lineLowFloor(ld physLine) (int64, bool) {
	if g == nil {
		return 0, false
	}
	if ld.sideNum0 < 0 || ld.sideNum1 < 0 || int(ld.sideNum0) >= len(g.m.Sidedefs) || int(ld.sideNum1) >= len(g.m.Sidedefs) {
		return 0, false
	}
	s0 := int(g.m.Sidedefs[ld.sideNum0].Sector)
	s1 := int(g.m.Sidedefs[ld.sideNum1].Sector)
	if s0 < 0 || s1 < 0 || s0 >= len(g.sectorFloor) || s1 >= len(g.sectorFloor) {
		return 0, false
	}
	if g.sectorFloor[s0] < g.sectorFloor[s1] {
		return g.sectorFloor[s0], true
	}
	return g.sectorFloor[s1], true
}

func (g *game) debugProjectileBlock(p projectile, ox, oy, oz, nx, ny, nz int64, reason string, frac float64, hx, hy, hz int64) {
	if g == nil || runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC") == "" || p.kind != projectileFireball {
		return
	}
	var tic int
	if _, err := fmt.Sscanf(runtimeDebugEnv("GD_DEBUG_PROJECTILE_TIC"), "%d", &tic); err != nil {
		return
	}
	if g.demoTick-1 != tic && g.worldTic != tic {
		return
	}
	fmt.Printf("projectile-block-debug tic=%d world=%d reason=%s from=(%d,%d,%d) to=(%d,%d,%d) frac=%f hit=(%d,%d,%d)\n",
		g.demoTick-1, g.worldTic, reason, ox, oy, oz, nx, ny, nz, frac, hx, hy, hz)
}

type projectileThingHit struct {
	idx      int
	isPlayer bool
	frac     float64
	x        int64
	y        int64
	z        int64
}

func (g *game) projectileHitsShootableThingAlongPath(p projectile, ox, oy, oz, nx, ny, nz int64) (projectileThingHit, bool) {
	if g == nil {
		return projectileThingHit{}, false
	}
	overlapsCircle := func(ax, ay, aradius, bx, by, bradius int64) bool {
		blockdist := aradius + bradius
		dx := ax - bx
		dy := ay - by
		return dx*dx+dy*dy < blockdist*blockdist
	}
	overlapsSquare := func(ax, ay, aradius, bx, by, bradius int64) bool {
		blockdist := aradius + bradius
		dx := ax - bx
		if dx < 0 {
			dx = -dx
		}
		dy := ay - by
		if dy < 0 {
			dy = -dy
		}
		return dx < blockdist && dy < blockdist
	}
	trace := divline{x: ox, y: oy, dx: nx - ox, dy: ny - oy}
	bestFrac := int64(fracUnit + 1)
	best := projectileThingHit{}
	if g.m != nil {
		for i, th := range g.m.Things {
			if i == p.sourceThing {
				continue
			}
			if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
				continue
			}
			if !thingTypeIsShootable(th.Type) || i >= len(g.thingHP) || g.thingHP[i] <= 0 {
				continue
			}
			tx, ty := g.thingPosFixed(i, th)
			tz, _, _ := g.thingSupportState(i, th)
			height := g.thingCurrentHeight(i, th)
			var (
				frac int64
				hx   int64
				hy   int64
				hz   int64
			)
			if !p.sourcePlayer && isMonster(th.Type) {
				if actorsOverlapXY(nx, ny, p.radius, tx, ty, thingTypeRadius(th.Type)) && oz <= tz+height && oz+p.height >= tz {
					frac = fracUnit
					hx, hy, hz = ox, oy, oz
				} else {
					radius := thingTypeRadius(th.Type) + p.radius
					var ok bool
					frac, ok = lineAttackThingFrac(trace, tx, ty, radius)
					if !ok || frac <= 0 || frac > fracUnit {
						if !overlapsCircle(nx, ny, p.radius, tx, ty, thingTypeRadius(th.Type)) {
							continue
						}
						frac = fracUnit
					}
					hx = ox + fixedMul(nx-ox, frac)
					hy = oy + fixedMul(ny-oy, frac)
					hz = oz + fixedMul(nz-oz, frac)
				}
			} else {
				radius := thingTypeRadius(th.Type) + p.radius
				var ok bool
				frac, ok = lineAttackThingFrac(trace, tx, ty, radius)
				if !ok || frac <= 0 || frac > fracUnit {
					overlapsAtDest := overlapsCircle(nx, ny, p.radius, tx, ty, thingTypeRadius(th.Type))
					if p.sourcePlayer && p.kind == projectileRocket {
						overlapsAtDest = overlapsAtDest || overlapsSquare(nx, ny, p.radius, tx, ty, thingTypeRadius(th.Type))
					}
					if !overlapsAtDest {
						continue
					}
					frac = fracUnit
				}
				hx = ox + fixedMul(nx-ox, frac)
				hy = oy + fixedMul(ny-oy, frac)
				hz = oz + fixedMul(nz-oz, frac)
			}
			if frac >= bestFrac {
				continue
			}
			if hz > tz+height || hz+p.height < tz {
				continue
			}
			bestFrac = frac
			best = projectileThingHit{
				idx:      i,
				isPlayer: false,
				frac:     float64(frac) / float64(fracUnit),
				x:        hx,
				y:        hy,
				z:        hz,
			}
		}
	}
	if !p.sourcePlayer && !g.isDead && g.stats.Health > 0 && bestFrac > fracUnit {
		playerFrac, ok := lineAttackThingFrac(trace, g.p.x, g.p.y, playerRadius+p.radius)
		if !ok || playerFrac <= 0 || playerFrac > fracUnit {
			if !actorsOverlapXY(nx, ny, p.radius, g.p.x, g.p.y, playerRadius) {
				return best, bestFrac <= fracUnit
			}
			playerFrac = fracUnit
		}
		playerZ := oz + fixedMul(nz-oz, playerFrac)
		if playerZ <= g.p.z+playerHeight && playerZ+p.height >= g.p.z {
			best = projectileThingHit{
				idx:      -1,
				isPlayer: true,
				frac:     float64(playerFrac) / float64(fracUnit),
				x:        ox + fixedMul(nx-ox, playerFrac),
				y:        oy + fixedMul(ny-oy, playerFrac),
				z:        playerZ,
			}
			bestFrac = playerFrac
		}
	}
	return best, bestFrac <= fracUnit
}

func projectileHitMessage(kind projectileKind) string {
	switch kind {
	case projectileBFGBall:
		return "BFG blast"
	case projectileRocket:
		return "Rocket blast"
	case projectileBaronBall:
		return "Baron ball hit"
	case projectileTracer:
		return "Revenant missile hit"
	case projectileFatShot:
		return "Mancubus fireball hit"
	case projectilePlasmaBall, projectilePlayerPlasma:
		return "Cacodemon ball hit"
	default:
		return "Fireball hit"
	}
}

func projectileColor(kind projectileKind) [2]uint8 {
	switch kind {
	case projectileBFGBall:
		return [2]uint8{191, 160}
	case projectileRocket:
		return [2]uint8{248, 188}
	case projectileBaronBall:
		return [2]uint8{130, 84}
	case projectileTracer:
		return [2]uint8{208, 144}
	case projectileFatShot:
		return [2]uint8{220, 126}
	case projectilePlasmaBall:
		return [2]uint8{210, 120}
	case projectilePlayerPlasma:
		return [2]uint8{224, 192}
	default:
		return [2]uint8{236, 86}
	}
}

func projectileViewRadius(p projectile) float64 {
	return math.Max(3, float64(p.radius)/fracUnit)
}

func projectileLaunchSoundEvent(typ int16) soundEvent {
	switch typ {
	case 16:
		return soundEventShootRocket
	case 3001, 3003, 69, 3005, 66, 67, 68:
		return soundEventShootFireball
	default:
		return soundEventShootPistol
	}
}

func projectileImpactSoundEvent(kind projectileKind) soundEvent {
	if kind == projectileRocket {
		return soundEventBarrelExplode
	}
	if kind == projectileBFGBall {
		return soundEventImpactRocket
	}
	return soundEventImpactFire
}

func (g *game) tickProjectileSpecial(p *projectile) {
	if g == nil || p == nil {
		return
	}
	if p.kind != projectileTracer || !p.tracerPlayer {
		return
	}
	if g.worldTic&3 != 0 {
		return
	}
	g.spawnTracerSmokeTrail(p.x, p.y, p.z, p.vx, p.vy)
	dx := g.p.x - p.x
	dy := g.p.y - p.y
	dxy := hypotFixed(dx, dy)
	if dxy <= 0 {
		return
	}
	speed := monsterProjectileSpeed(66, g.fastMonstersActive())
	targetVX := int64((float64(dx) / float64(dxy)) * float64(speed))
	targetVY := int64((float64(dy) / float64(dxy)) * float64(speed))
	targetVZ := int64((float64((g.p.z+(playerHeight/2))-p.z) / float64(dxy)) * float64(speed))
	p.vx += (targetVX - p.vx) / 2
	p.vy += (targetVY - p.vy) / 2
	p.vz += (targetVZ - p.vz) / 2
	if p.vx != 0 || p.vy != 0 {
		p.angle = angleToThing(p.x, p.y, p.x+p.vx, p.y+p.vy)
	}
}
