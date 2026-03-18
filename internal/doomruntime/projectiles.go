package doomruntime

import (
	"math"
	"slices"

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
	vx           int64
	vy           int64
	vz           int64
	radius       int64
	height       int64
	ttl          int
	sourceX      int64
	sourceY      int64
	sourceThing  int
	sourceType   int16
	sourcePlayer bool
	tracerPlayer bool
	angle        uint32
	kind         projectileKind
}

type projectileImpact struct {
	x         int64
	y         int64
	z         int64
	kind      projectileKind
	tics      int
	totalTics int
	angle     uint32
	sprayDone bool
}

func usesMonsterProjectile(typ int16) bool {
	switch typ {
	case 3001, 3005, 3003, 16, 66, 67, 68:
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
	case 3003:
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
	case 3003:
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

func monsterMuzzleOffsetZ(typ int16) int64 {
	switch typ {
	case 16:
		return 72 * fracUnit
	case 3003:
		return 56 * fracUnit
	case 66:
		return 72 * fracUnit
	case 67:
		return 64 * fracUnit
	case 68:
		return 48 * fracUnit
	default:
		return 40 * fracUnit
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
	sx := int64(th.X) << fracBits
	sy := int64(th.Y) << fracBits
	sz := g.thingFloorZ(sx, sy) + monsterMuzzleOffsetZ(typ)
	tx := g.p.x
	ty := g.p.y
	tz := g.p.z + (playerHeight / 2)
	aimAngle := g.monsterAimAngleToPlayer(sx, sy)

	dx := fixedMul(monsterProjectileSpeed(typ, g.fastMonstersActive()), doomFineCosine(aimAngle))
	dy := fixedMul(monsterProjectileSpeed(typ, g.fastMonstersActive()), doomFineSineAtAngle(aimAngle))
	dz := tz - sz
	dxy := hypotFixed(tx-sx, ty-sy)
	if dxy <= 0 {
		return false
	}

	speed := monsterProjectileSpeed(typ, g.fastMonstersActive())
	vx := dx
	vy := dy
	vz := int64((float64(dz) / float64(dxy)) * float64(speed))
	if vx == 0 && vy == 0 {
		return false
	}

	g.projectiles = append(g.projectiles, projectile{
		x:            sx,
		y:            sy,
		z:            sz,
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
		kind:         monsterProjectileKind(typ),
		angle:        aimAngle,
	})
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
	ang := angleToRadians(p.angle)
	p.vx = int64(math.Cos(ang) * float64(speed))
	p.vy = int64(math.Sin(ang) * float64(speed))
	return true
}

func (g *game) tickProjectiles() {
	if len(g.projectiles) == 0 {
		return
	}
	kept := g.projectiles[:0]
	for _, p := range g.projectiles {
		if p.ttl <= 0 {
			continue
		}
		ox, oy, oz := p.x, p.y, p.z
		nx := ox + p.vx
		ny := oy + p.vy
		nz := oz + p.vz
		thingHit, hitThing := g.projectileHitsShootableThingAlongPath(p, ox, oy, oz, nx, ny, nz)
		blocked, blockFrac, hx, hy, hz := g.projectileBlockedAt(p, ox, oy, oz, nx, ny, nz)
		if hitThing && (!blocked || thingHit.frac <= blockFrac) {
			g.spawnProjectileImpact(p.kind, thingHit.x, thingHit.y, thingHit.z, p.angle)
			g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), thingHit.x, thingHit.y)
			if dmg := projectileDamage(p); dmg > 0 {
				g.damageShootableThing(thingHit.idx, dmg)
			}
			g.projectileSplashDamage(p, thingHit.x, thingHit.y, thingHit.z)
			continue
		}
		if blocked {
			g.spawnProjectileImpact(p.kind, hx, hy, hz, p.angle)
			g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), hx, hy)
			g.projectileSplashDamage(p, hx, hy, hz)
			continue
		}
		p.x = nx
		p.y = ny
		p.z = nz
		g.tickProjectileSpecial(&p)
		p.ttl--
		if p.ttl <= 0 {
			g.spawnProjectileImpact(p.kind, p.x, p.y, p.z, p.angle)
			g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), p.x, p.y)
			g.projectileSplashDamage(p, p.x, p.y, p.z)
			continue
		}
		if !p.sourcePlayer && g.projectileHitsPlayer(p) {
			g.spawnProjectileImpact(p.kind, p.x, p.y, p.z, p.angle)
			g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), p.x, p.y)
			dmg := projectileDamage(p)
			if dmg > 0 {
				g.damagePlayerFrom(dmg, projectileHitMessage(p.kind), p.sourceX, p.sourceY, true)
			}
			g.projectileSplashDamage(p, p.x, p.y, p.z)
			continue
		}
		kept = append(kept, p)
	}
	g.projectiles = kept
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
	g.radiusAttackAt(x, y, z, p.height, -1, damage, projectileHitMessage(p.kind))
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
	ang := angleToRadians(g.p.angle)
	vx := int64(math.Cos(ang) * float64(rocketSpeed))
	vy := int64(math.Sin(ang) * float64(rocketSpeed))
	if vx == 0 && vy == 0 {
		return false
	}
	slope := g.bulletSlopeForAim(g.p.angle, 1024*fracUnit)
	vz := fixedMul(rocketSpeed, slope)
	launchOffset := playerRadius + rocketRadius + 4*fracUnit
	sx := g.p.x + int64(math.Cos(ang)*float64(launchOffset))
	sy := g.p.y + int64(math.Sin(ang)*float64(launchOffset))
	sz := g.playerShootZ() - (rocketHeight >> 1)
	g.projectiles = append(g.projectiles, projectile{
		x:            sx,
		y:            sy,
		z:            sz,
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
		kind:         projectileRocket,
		angle:        g.p.angle,
	})
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
	ang := angleToRadians(g.p.angle)
	vx := int64(math.Cos(ang) * float64(speed))
	vy := int64(math.Sin(ang) * float64(speed))
	if vx == 0 && vy == 0 {
		return false
	}
	slope := g.bulletSlopeForAim(g.p.angle, 1024*fracUnit)
	vz := fixedMul(speed, slope)
	launchOffset := playerRadius + radius + 4*fracUnit
	sx := g.p.x + int64(math.Cos(ang)*float64(launchOffset))
	sy := g.p.y + int64(math.Sin(ang)*float64(launchOffset))
	sz := g.playerShootZ() - (height >> 1)
	g.projectiles = append(g.projectiles, projectile{
		x:            sx,
		y:            sy,
		z:            sz,
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
		kind:         kind,
		angle:        g.p.angle,
	})
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
		g.damageShootableThing(outcome.target.idx, damage)
	}
}

func (g *game) tickProjectileImpacts() {
	if len(g.projectileImpacts) == 0 {
		return
	}
	keep := g.projectileImpacts[:0]
	for _, fx := range g.projectileImpacts {
		elapsed := fx.totalTics - fx.tics
		if fx.kind == projectileBFGBall && !fx.sprayDone && elapsed >= 16 {
			fx.sprayDone = true
			g.applyBFGSpray(fx.angle)
		}
		fx.tics--
		if fx.tics <= 0 {
			continue
		}
		keep = append(keep, fx)
	}
	g.projectileImpacts = keep
}

func (g *game) spawnProjectileImpact(kind projectileKind, x, y, z int64, angle uint32) {
	const maxImpacts = 64
	if len(g.projectileImpacts) >= maxImpacts {
		copy(g.projectileImpacts, g.projectileImpacts[1:])
		g.projectileImpacts = g.projectileImpacts[:maxImpacts-1]
	}
	// Doom timings:
	// - Fireball families (BAL1/BAL2/BAL7): C/D/E at 6 tics each.
	// - Rocket (MISL): B/C/D at 8/6/4 tics.
	tics := 18
	g.projectileImpacts = append(g.projectileImpacts, projectileImpact{
		x:         x,
		y:         y,
		z:         z,
		kind:      kind,
		tics:      tics,
		totalTics: tics,
		angle:     angle,
	})
}

func (g *game) projectileBlockedAt(p projectile, ox, oy, oz, nx, ny, nz int64) (bool, float64, int64, int64, int64) {
	if g.m == nil {
		return false, 1, nx, ny, nz
	}
	sec := g.sectorAt(nx, ny)
	if sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
		return true, 1, nx, ny, nz
	}
	if nz < g.sectorFloor[sec] || nz+p.height > g.sectorCeil[sec] {
		return true, 1, nx, ny, nz
	}
	intercepts := make([]intercept, 0, 8)
	for i, ld := range g.lines {
		frac, ok := segmentIntersectFrac(ox, oy, nx, ny, ld.x1, ld.y1, ld.x2, ld.y2)
		if !ok {
			continue
		}
		intercepts = append(intercepts, intercept{frac: frac, line: i})
	}
	slices.SortFunc(intercepts, func(a, b intercept) int {
		if a.frac < b.frac {
			return -1
		}
		if a.frac > b.frac {
			return 1
		}
		return 0
	})

	for _, it := range intercepts {
		ld := g.lines[it.line]
		hx := ox + int64(float64(nx-ox)*it.frac)
		hy := oy + int64(float64(ny-oy)*it.frac)
		hz := oz + int64(float64(nz-oz)*it.frac)
		if ld.sideNum1 < 0 || (ld.flags&mlBlocking) != 0 {
			return true, it.frac, hx, hy, hz
		}
		opentop, openbottom, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			return true, it.frac, hx, hy, hz
		}
		if hz < openbottom || hz+p.height > opentop {
			return true, it.frac, hx, hy, hz
		}
	}
	return false, 1, nx, ny, nz
}

type projectileThingHit struct {
	idx  int
	frac float64
	x    int64
	y    int64
	z    int64
}

func (g *game) projectileHitsShootableThingAlongPath(p projectile, ox, oy, oz, nx, ny, nz int64) (projectileThingHit, bool) {
	if g == nil || g.m == nil {
		return projectileThingHit{}, false
	}
	trace := divline{x: ox, y: oy, dx: nx - ox, dy: ny - oy}
	bestFrac := int64(fracUnit + 1)
	best := projectileThingHit{}
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
		radius := thingTypeRadius(th.Type) + p.radius
		frac, ok := lineAttackThingFrac(trace, tx, ty, radius)
		if !ok || frac <= 0 || frac > fracUnit {
			if !actorsOverlapXY(nx, ny, p.radius, tx, ty, thingTypeRadius(th.Type)) {
				continue
			}
			frac = fracUnit
		}
		if frac >= bestFrac {
			continue
		}
		hx := ox + fixedMul(nx-ox, frac)
		hy := oy + fixedMul(ny-oy, frac)
		hz := oz + fixedMul(nz-oz, frac)
		tz, _, _ := g.thingSupportState(i, th)
		height := g.thingCurrentHeight(i, th)
		if hz > tz+height || hz+p.height < tz {
			continue
		}
		bestFrac = frac
		best = projectileThingHit{
			idx:  i,
			frac: float64(frac) / float64(fracUnit),
			x:    hx,
			y:    hy,
			z:    hz,
		}
	}
	return best, bestFrac <= fracUnit
}

func (g *game) projectileHitsPlayer(p projectile) bool {
	blockdist := playerRadius + p.radius
	if abs(g.p.x-p.x) > blockdist || abs(g.p.y-p.y) > blockdist {
		return false
	}
	delta := p.z - g.p.z
	if delta > playerHeight {
		return false
	}
	if delta+p.height < 0 {
		return false
	}
	return true
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
	case 3001, 3003, 3005, 66, 67, 68:
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
