package automap

import (
	"math"

	"gddoom/internal/doomrand"
)

const (
	monsterWakeRange   = 1024 * fracUnit
	monsterMeleeRange  = 64 * fracUnit
	monsterAttackRange = 1024 * fracUnit
	monsterAttackTics  = 35
)

func (g *game) tickMonsters() {
	if g.m == nil || g.isDead {
		return
	}
	px := g.p.x
	py := g.p.y
	for i, th := range g.m.Things {
		if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
			continue
		}
		if !isMonster(th.Type) || g.thingHP[i] <= 0 {
			continue
		}
		if g.thingCooldown[i] > 0 {
			g.thingCooldown[i]--
		}
		tx := int64(th.X) << fracBits
		ty := int64(th.Y) << fracBits
		dx := px - tx
		dy := py - ty
		dist := hypotFixed(dx, dy)

		if !g.thingAggro[i] {
			if dist <= monsterWakeRange && g.monsterHasLOS(tx, ty, px, py) {
				g.thingAggro[i] = true
			} else {
				continue
			}
		}

		if g.thingCooldown[i] == 0 && dist <= monsterAttackRange && g.monsterHasLOS(tx, ty, px, py) {
			g.faceMonsterToward(i, tx, ty, px, py)
			didAttack := g.monsterAttack(i, th.Type, dist)
			if didAttack {
				g.thingCooldown[i] = monsterAttackCooldown(th.Type)
				// Attacking consumes this tic's action for more Doom-like cadence.
				continue
			}
		}

		if dist > monsterMeleeRange {
			g.moveMonsterToward(i, th.Type, tx, ty, px, py, monsterMoveStep(th.Type))
		}
	}
}

func (g *game) monsterAttack(i int, typ int16, dist int64) bool {
	meleeOnly := isMeleeOnlyMonster(typ)
	var sx, sy int64
	if i >= 0 && g.m != nil && i < len(g.m.Things) {
		sx = int64(g.m.Things[i].X) << fracBits
		sy = int64(g.m.Things[i].Y) << fracBits
	}
	if dist <= monsterMeleeRange {
		damage := monsterMeleeDamage(typ)
		if damage > 0 {
			g.damagePlayerFrom(damage, "Monster hit you", sx, sy, true)
			return true
		}
	}
	if meleeOnly {
		return false
	}
	if !shouldAttemptRangedAttack(typ, dist) {
		return false
	}
	if typ == 3004 {
		// Zombieman: single bullet with Doom-style spread and chance to miss.
		g.monsterHitscanAttack(sx, sy, 1)
		return true
	}
	if typ == 9 {
		// Sergeant: 3 pellets.
		g.monsterHitscanAttack(sx, sy, 3)
		return true
	}
	if usesMonsterProjectile(typ) {
		if g.spawnMonsterProjectile(i, typ) {
			return true
		}
		return false
	}
	damage := monsterRangedDamage(typ)
	if damage <= 0 {
		return false
	}
	g.damagePlayerFrom(damage, "Monster shot you", sx, sy, true)
	return true
}

func (g *game) monsterHitscanAttack(sx, sy int64, pellets int) {
	if pellets <= 0 {
		return
	}
	base := math.Atan2(float64(g.p.y-sy), float64(g.p.x-sx))
	total := 0
	for i := 0; i < pellets; i++ {
		off := float64((doomPRandomN(256)-doomPRandomN(256))<<20) * (2 * math.Pi / 4294967296.0)
		ang := base + off
		if !g.monsterBulletCanHitPlayer(sx, sy, ang, monsterAttackRange) {
			continue
		}
		total += 3 * (1 + doomPRandomN(5))
	}
	if total > 0 {
		g.damagePlayerFrom(total, "Monster shot you", sx, sy, true)
	}
}

func (g *game) monsterBulletCanHitPlayer(sx, sy int64, ang float64, rng int64) bool {
	if !g.monsterHasLOS(sx, sy, g.p.x, g.p.y) {
		return false
	}
	dx := float64(g.p.x - sx)
	dy := float64(g.p.y - sy)
	fwd := dx*math.Cos(ang) + dy*math.Sin(ang)
	if fwd <= 0 || fwd > float64(rng) {
		return false
	}
	perp := math.Abs(dx*math.Sin(ang) - dy*math.Cos(ang))
	return perp <= float64(playerRadius)
}

func shouldAttemptRangedAttack(typ int16, dist int64) bool {
	// Approximate Doom-style missile chance: closer enemies fire more often.
	base := 80
	switch typ {
	case 3004: // zombieman
		base = 100
	case 9: // sergeant
		base = 110
	case 3001: // imp
		base = 90
	case 3005, 3003, 16: // caco/baron/cyber
		base = 75
	}
	atten := int(dist / (256 * fracUnit))
	chance := base - atten*8
	if chance < 8 {
		chance = 8
	}
	return doomPRandomN(256) < chance
}

func monsterMoveStep(typ int16) int64 {
	switch typ {
	case 3004, 9:
		return 8 * fracUnit
	case 3001:
		return 8 * fracUnit
	case 3002, 3006:
		return 10 * fracUnit
	case 3005, 3003:
		return 8 * fracUnit
	case 16, 7:
		return 12 * fracUnit
	default:
		return 8 * fracUnit
	}
}

func monsterAttackCooldown(typ int16) int {
	switch typ {
	case 9:
		return 22 + doomPRandomN(10)
	case 3004:
		return 28 + doomPRandomN(12)
	case 3002, 3006:
		return 18 + doomPRandomN(8)
	default:
		return monsterAttackTics + doomPRandomN(10)
	}
}

func isMeleeOnlyMonster(typ int16) bool {
	switch typ {
	case 3002, 3006:
		return true
	default:
		return false
	}
}

func monsterMeleeDamage(typ int16) int {
	switch typ {
	case 3002: // demon
		return 4 * (1 + doomPRandomN(10))
	case 3006: // lost soul
		return 3 * (1 + doomPRandomN(8))
	default:
		return 3 * (1 + doomPRandomN(8))
	}
}

func monsterRangedDamage(typ int16) int {
	switch typ {
	case 3004: // zombieman hitscan
		return 3 * (1 + doomPRandomN(5))
	case 9: // sergeant pellets
		pellets := 3
		dmg := 0
		for p := 0; p < pellets; p++ {
			dmg += 3 * (1 + doomPRandomN(5))
		}
		return dmg
	case 3001: // imp fireball approx
		return 3 * (1 + doomPRandomN(8))
	case 3005: // caco ball approx
		return 5 * (1 + doomPRandomN(8))
	case 3003: // baron ball approx
		return 8 * (1 + doomPRandomN(8))
	case 16: // rocket-like
		return 20 + doomPRandomN(60)
	default:
		return 3 * (1 + doomPRandomN(8))
	}
}

func (g *game) monsterHasLOS(ax, ay, bx, by int64) bool {
	for _, ld := range g.lines {
		if _, ok := segmentIntersectFrac(ax, ay, bx, by, ld.x1, ld.y1, ld.x2, ld.y2); !ok {
			continue
		}
		if (ld.flags&mlTwoSided) == 0 || ld.sideNum1 < 0 {
			return false
		}
		_, _, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			return false
		}
	}
	return true
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
	if g.tryMoveProbe(nx, ny) {
		g.m.Things[i].X = int16(nx >> fracBits)
		g.m.Things[i].Y = int16(ny >> fracBits)
		return
	}
	if g.tryMoveProbe(x+dx, y) {
		g.m.Things[i].X = int16((x + dx) >> fracBits)
		return
	}
	if g.tryMoveProbe(x, y+dy) {
		g.m.Things[i].Y = int16((y + dy) >> fracBits)
	}
}

func (g *game) faceMonsterToward(i int, fromX, fromY, toX, toY int64) {
	if g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	dx := float64(toX - fromX)
	dy := float64(toY - fromY)
	if math.Abs(dx) < 1e-6 && math.Abs(dy) < 1e-6 {
		return
	}
	deg := math.Atan2(dy, dx) * (180.0 / math.Pi)
	if deg < 0 {
		deg += 360
	}
	g.m.Things[i].Angle = int16(math.Round(deg)) % 360
}

func (g *game) tryMoveProbe(x, y int64) bool {
	if g.m == nil || len(g.m.Sectors) == 0 {
		return false
	}
	saved := g.p
	ok := g.tryMove(x, y)
	g.p = saved
	return ok
}

func hypotFixed(dx, dy int64) int64 {
	return int64(math.Hypot(float64(dx), float64(dy)))
}

func doomPRandomN(n int) int {
	if n <= 0 {
		return 0
	}
	return doomrand.PRandom() % n
}
