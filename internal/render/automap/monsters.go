package automap

import (
	"math"

	"gddoom/internal/doomrand"
)

const (
	monsterWakeRange   = 1024 * fracUnit
	monsterMeleeRange  = 64 * fracUnit
	monsterAttackRange = 1024 * fracUnit
	monsterStep        = 8 * fracUnit
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
			if dist <= monsterMeleeRange {
				damage := 3 * (1 + doomPRandomN(8))
				g.damagePlayer(damage, "Monster hit you")
			} else {
				damage := 3 * (1 + doomPRandomN(5))
				g.damagePlayer(damage, "Monster shot you")
			}
			g.thingCooldown[i] = monsterAttackTics
		}

		if dist > monsterMeleeRange {
			g.moveMonsterToward(i, tx, ty, px, py)
		}
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

func (g *game) moveMonsterToward(i int, x, y, tx, ty int64) {
	ang := math.Atan2(float64(ty-y), float64(tx-x))
	dx := int64(math.Cos(ang) * float64(monsterStep))
	dy := int64(math.Sin(ang) * float64(monsterStep))
	nx := x + dx
	ny := y + dy
	if g.tryMove(nx, ny) {
		g.m.Things[i].X = int16(nx >> fracBits)
		g.m.Things[i].Y = int16(ny >> fracBits)
		return
	}
	if g.tryMove(x+dx, y) {
		g.m.Things[i].X = int16((x + dx) >> fracBits)
		return
	}
	if g.tryMove(x, y+dy) {
		g.m.Things[i].Y = int16((y + dy) >> fracBits)
	}
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
