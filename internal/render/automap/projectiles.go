package automap

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
	x          int64
	y          int64
	z          int64
	vx         int64
	vy         int64
	vz         int64
	radius     int64
	height     int64
	ttl        int
	sourceX    int64
	sourceY    int64
	sourceType int16
	kind       projectileKind
}

type projectileImpact struct {
	x    int64
	y    int64
	z    int64
	kind projectileKind
	tics int
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
		x:          sx,
		y:          sy,
		z:          sz,
		vx:         vx,
		vy:         vy,
		vz:         vz,
		radius:     monsterProjectileRadius(typ),
		height:     monsterProjectileHeight(typ),
		ttl:        monsterProjectileTTL(typ),
		sourceX:    sx,
		sourceY:    sy,
		sourceType: typ,
		kind:       monsterProjectileKind(typ),
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
		if blocked, hx, hy, hz := g.projectileBlockedAt(p, ox, oy, oz, nx, ny, nz); blocked {
			g.spawnProjectileImpact(p.kind, hx, hy, hz)
			g.emitSoundEvent(projectileImpactSoundEvent(p.kind))
			continue
		}
		p.x = nx
		p.y = ny
		p.z = nz
		p.ttl--
		if p.ttl <= 0 {
			g.spawnProjectileImpact(p.kind, p.x, p.y, p.z)
			g.emitSoundEvent(projectileImpactSoundEvent(p.kind))
			continue
		}
		if g.projectileHitsPlayer(p) {
			g.spawnProjectileImpact(p.kind, p.x, p.y, p.z)
			g.emitSoundEvent(projectileImpactSoundEvent(p.kind))
			dmg := monsterRangedDamage(p.sourceType)
			if dmg > 0 {
				g.damagePlayerFrom(dmg, projectileHitMessage(p.kind), p.sourceX, p.sourceY, true)
			}
			continue
		}
		kept = append(kept, p)
	}
	g.projectiles = kept
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
	tics := 6
	if kind == projectileRocket {
		tics = 8
	}
	g.projectileImpacts = append(g.projectileImpacts, projectileImpact{
		x:    x,
		y:    y,
		z:    z,
		kind: kind,
		tics: tics,
	})
}

func (g *game) projectileBlockedAt(p projectile, ox, oy, oz, nx, ny, nz int64) (bool, int64, int64, int64) {
	if g.m == nil {
		return false, nx, ny, nz
	}
	sec := g.sectorAt(nx, ny)
	if sec < 0 || sec >= len(g.sectorFloor) || sec >= len(g.sectorCeil) {
		return true, nx, ny, nz
	}
	if nz < g.sectorFloor[sec] || nz+p.height > g.sectorCeil[sec] {
		return true, nx, ny, nz
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
			return true, hx, hy, hz
		}
		opentop, openbottom, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			return true, hx, hy, hz
		}
		if hz < openbottom || hz+p.height > opentop {
			return true, hx, hy, hz
		}
	}
	return false, nx, ny, nz
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
		return "Plasma ball hit"
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
