package automap

import (
	"math"

	"gddoom/internal/doomrand"
)

const (
	pistolRange = 2048 * fracUnit
)

func (g *game) initThingCombatState() {
	for i, th := range g.m.Things {
		if !isMonster(th.Type) {
			continue
		}
		g.thingHP[i] = monsterSpawnHealth(th.Type)
	}
}

func monsterSpawnHealth(typ int16) int {
	switch typ {
	case 3004: // zombie man
		return 20
	case 9: // sergeant
		return 30
	case 3001: // imp
		return 60
	case 3002: // demon
		return 150
	case 3006: // lost soul
		return 100
	case 3005: // cacodemon
		return 400
	case 3003: // baron
		return 1000
	case 16: // cyberdemon
		return 4000
	case 7: // spider mastermind
		return 3000
	default:
		return 100
	}
}

func (g *game) handleFire() {
	if g.isDead {
		return
	}
	if g.stats.Bullets <= 0 {
		g.setHUDMessage("No ammo", 20)
		g.useFlash = max(g.useFlash, 20)
		return
	}
	g.stats.Bullets--
	targetIdx, ok := g.pickHitscanMonsterTarget()
	if !ok {
		g.setHUDMessage("Miss", 10)
		return
	}
	damage := 5 * (1 + (doomrand.PRandom() % 3))
	g.damageMonster(targetIdx, damage)
}

func (g *game) pickHitscanMonsterTarget() (int, bool) {
	ang := angleToRadians(g.p.angle)
	dirX := math.Cos(ang)
	dirY := math.Sin(ang)
	px := float64(g.p.x)
	py := float64(g.p.y)
	bestDist := math.Inf(1)
	bestIdx := -1

	for i, th := range g.m.Things {
		if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
			continue
		}
		if !isMonster(th.Type) || g.thingHP[i] <= 0 {
			continue
		}
		tx := float64(int64(th.X) << fracBits)
		ty := float64(int64(th.Y) << fracBits)
		rx := tx - px
		ry := ty - py
		t := rx*dirX + ry*dirY
		if t <= 0 || t > float64(pistolRange) {
			continue
		}
		perp := math.Abs(rx*dirY - ry*dirX)
		radius := float64(20 * fracUnit)
		if perp > radius {
			continue
		}
		if t < bestDist {
			bestDist = t
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return -1, false
	}
	return bestIdx, true
}

func (g *game) damageMonster(thingIdx int, damage int) {
	if thingIdx < 0 || thingIdx >= len(g.thingHP) || damage <= 0 {
		return
	}
	if g.thingHP[thingIdx] <= 0 {
		return
	}
	g.thingHP[thingIdx] -= damage
	if g.thingHP[thingIdx] <= 0 {
		g.thingHP[thingIdx] = 0
		g.thingCollected[thingIdx] = true
		g.setHUDMessage("Monster killed", 15)
		g.bonusFlashTic = max(g.bonusFlashTic, 4)
	} else {
		g.setHUDMessage("Hit", 8)
	}
}
