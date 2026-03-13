package doomruntime

import (
	"math"
	"sort"
)

type projectileKind int

const (
	projectileFireball projectileKind = iota
	projectilePlasmaBall
	projectileBaronBall
	projectileRocket
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
	kind         projectileKind
}

type projectileImpact struct {
	x         int64
	y         int64
	z         int64
	kind      projectileKind
	tics      int
	totalTics int
}

func usesMonsterProjectile(typ int16) bool {
	switch typ {
	case 3001, 3005, 3003, 16:
		return true
	default:
		return false
	}
}

func monsterProjectileKind(typ int16) projectileKind {
	switch typ {
	case 3005:
		// Cacodemon shot uses BAL2 in vanilla Doom.
		return projectilePlasmaBall
	case 3003:
		return projectileBaronBall
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
	case 16:
		return 20 * fracUnit * scale
	default:
		return 10 * fracUnit * scale
	}
}

func monsterProjectileRadius(typ int16) int64 {
	switch typ {
	case 16:
		return 11 * fracUnit
	default:
		return 6 * fracUnit
	}
}

func monsterProjectileHeight(typ int16) int64 {
	switch typ {
	case 16:
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

	dx := tx - sx
	dy := ty - sy
	dz := tz - sz
	dxy := hypotFixed(dx, dy)
	if dxy <= 0 {
		return false
	}

	speed := monsterProjectileSpeed(typ, g.fastMonstersActive())
	vx := int64((float64(dx) / float64(dxy)) * float64(speed))
	vy := int64((float64(dy) / float64(dxy)) * float64(speed))
	vz := int64((float64(dz) / float64(dxy)) * float64(speed))
	if vx == 0 && vy == 0 {
		return false
	}

	g.projectiles = append(g.projectiles, projectile{
		x:           sx,
		y:           sy,
		z:           sz,
		vx:          vx,
		vy:          vy,
		vz:          vz,
		radius:      monsterProjectileRadius(typ),
		height:      monsterProjectileHeight(typ),
		ttl:         monsterProjectileTTL(typ),
		sourceX:     sx,
		sourceY:     sy,
		sourceThing: thingIdx,
		sourceType:  typ,
		kind:        monsterProjectileKind(typ),
	})
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
			g.spawnProjectileImpact(p.kind, thingHit.x, thingHit.y, thingHit.z)
			g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), thingHit.x, thingHit.y)
			if dmg := projectileDamage(p); dmg > 0 {
				g.damageShootableThing(thingHit.idx, dmg)
			}
			g.projectileSplashDamage(p, thingHit.x, thingHit.y, thingHit.z)
			continue
		}
		if blocked {
			g.spawnProjectileImpact(p.kind, hx, hy, hz)
			g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), hx, hy)
			g.projectileSplashDamage(p, hx, hy, hz)
			continue
		}
		p.x = nx
		p.y = ny
		p.z = nz
		p.ttl--
		if p.ttl <= 0 {
			g.spawnProjectileImpact(p.kind, p.x, p.y, p.z)
			g.emitSoundEventAt(projectileImpactSoundEvent(p.kind), p.x, p.y)
			g.projectileSplashDamage(p, p.x, p.y, p.z)
			continue
		}
		if !p.sourcePlayer && g.projectileHitsPlayer(p) {
			g.spawnProjectileImpact(p.kind, p.x, p.y, p.z)
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
	if p.sourcePlayer {
		switch p.kind {
		case projectileRocket:
			return 20 * (1 + doomPRandomN(8))
		default:
			return 0
		}
	}
	return monsterRangedDamage(p.sourceType)
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
	})
	return true
}

func (g *game) tickProjectileImpacts() {
	if len(g.projectileImpacts) == 0 {
		return
	}
	keep := g.projectileImpacts[:0]
	for _, fx := range g.projectileImpacts {
		fx.tics--
		if fx.tics <= 0 {
			continue
		}
		keep = append(keep, fx)
	}
	g.projectileImpacts = keep
}

func (g *game) spawnProjectileImpact(kind projectileKind, x, y, z int64) {
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
	sort.Slice(intercepts, func(i, j int) bool { return intercepts[i].frac < intercepts[j].frac })

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
	case projectileRocket:
		return "Rocket blast"
	case projectileBaronBall:
		return "Baron ball hit"
	case projectilePlasmaBall:
		return "Cacodemon ball hit"
	default:
		return "Fireball hit"
	}
}

func projectileColor(kind projectileKind) [2]uint8 {
	switch kind {
	case projectileRocket:
		return [2]uint8{248, 188}
	case projectileBaronBall:
		return [2]uint8{130, 84}
	case projectilePlasmaBall:
		return [2]uint8{210, 120}
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
	case 3001, 3003, 3005:
		return soundEventShootFireball
	default:
		return soundEventShootPistol
	}
}

func projectileImpactSoundEvent(kind projectileKind) soundEvent {
	if kind == projectileRocket {
		return soundEventImpactRocket
	}
	return soundEventImpactFire
}
