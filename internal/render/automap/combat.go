package automap

import (
	"math"

	"gddoom/internal/doomrand"
)

const (
	pistolRange  = 2048 * fracUnit
	shotgunRange = 2048 * fracUnit
)

type weaponID int

const (
	weaponFist weaponID = iota + 1
	weaponPistol
	weaponShotgun
	weaponChaingun
	weaponRocketLauncher
	weaponPlasma
	weaponBFG
	weaponChainsaw
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
	g.ensureWeaponDefaults()
	g.ensureWeaponHasAmmo()
	if !g.canFireSelectedWeapon() {
		g.setHUDMessage("No ammo", 20)
		g.useFlash = max(g.useFlash, 20)
		return
	}
	hit := g.fireSelectedWeapon()
	if !hit {
		g.setHUDMessage("Miss", 10)
	}
}

func (g *game) fireSelectedWeapon() bool {
	switch g.inventory.ReadyWeapon {
	case weaponFist:
		return g.fireMelee(64*fracUnit, 2*(1+doomrand.PRandom()%10))
	case weaponChainsaw:
		// Doom chainsaw damage per puff.
		return g.fireMelee(64*fracUnit, 2*(1+doomrand.PRandom()%10))
	case weaponPistol:
		g.stats.Bullets--
		return g.fireBullet(g.p.angle, pistolRange)
	case weaponChaingun:
		g.stats.Bullets--
		return g.fireBullet(g.p.angle, pistolRange)
	case weaponShotgun:
		g.stats.Shells--
		hit := false
		for i := 0; i < 7; i++ {
			if g.fireBullet(g.p.angle, shotgunRange) {
				hit = true
			}
		}
		return hit
	case weaponRocketLauncher:
		g.stats.Rockets--
		g.setHUDMessage("Rocket weapon not wired yet", 12)
		return false
	case weaponPlasma:
		g.stats.Cells--
		g.setHUDMessage("Plasma weapon not wired yet", 12)
		return false
	case weaponBFG:
		g.stats.Cells -= 40
		g.setHUDMessage("BFG not wired yet", 12)
		return false
	default:
		return false
	}
}

func (g *game) fireMelee(rng int64, damage int) bool {
	idx, ok := g.pickHitscanMonsterTargetAtAngle(g.p.angle, rng, 20*fracUnit)
	if !ok {
		return false
	}
	g.damageMonster(idx, damage)
	return true
}

func (g *game) fireBullet(baseAngle uint32, rng int64) bool {
	ang := addDoomBulletSpread(baseAngle)
	idx, ok := g.pickHitscanMonsterTargetAtAngle(ang, rng, 20*fracUnit)
	if !ok {
		return false
	}
	damage := 5 * (1 + (doomrand.PRandom() % 3))
	g.damageMonster(idx, damage)
	return true
}

func addDoomBulletSpread(base uint32) uint32 {
	// Doom's hitscan horizontal spread: (P_Random - P_Random) << 18.
	delta := (doomrand.PRandom() - doomrand.PRandom()) << 18
	return base + uint32(int32(delta))
}

func (g *game) pickHitscanMonsterTarget() (int, bool) {
	return g.pickHitscanMonsterTargetAtAngle(g.p.angle, pistolRange, 20*fracUnit)
}

func (g *game) pickHitscanMonsterTargetAtAngle(angle uint32, rng int64, radius int64) (int, bool) {
	if g.m == nil {
		return -1, false
	}
	ang := angleToRadians(angle)
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
		if t <= 0 || t > float64(rng) {
			continue
		}
		perp := math.Abs(rx*dirY - ry*dirX)
		if perp > float64(radius) {
			continue
		}
		if !g.monsterHasLOS(g.p.x, g.p.y, int64(th.X)<<fracBits, int64(th.Y)<<fracBits) {
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

func (g *game) ensureWeaponDefaults() {
	if g.inventory.Weapons == nil {
		g.inventory.Weapons = map[int16]bool{}
	}
	if g.inventory.ReadyWeapon == 0 {
		g.inventory.ReadyWeapon = weaponPistol
	}
}

func (g *game) ensureWeaponHasAmmo() {
	if g.canFireSelectedWeapon() {
		return
	}
	if g.stats.Shells > 0 && g.inventory.Weapons[2001] {
		g.inventory.ReadyWeapon = weaponShotgun
		return
	}
	if g.stats.Bullets > 0 && g.inventory.Weapons[2002] {
		g.inventory.ReadyWeapon = weaponChaingun
		return
	}
	if g.stats.Bullets > 0 {
		g.inventory.ReadyWeapon = weaponPistol
		return
	}
	if g.stats.Cells >= 40 && g.inventory.Weapons[2006] {
		g.inventory.ReadyWeapon = weaponBFG
		return
	}
	if g.stats.Cells > 0 && g.inventory.Weapons[2004] {
		g.inventory.ReadyWeapon = weaponPlasma
		return
	}
	if g.stats.Rockets > 0 && g.inventory.Weapons[2003] {
		g.inventory.ReadyWeapon = weaponRocketLauncher
		return
	}
	if g.inventory.Weapons[2005] {
		g.inventory.ReadyWeapon = weaponChainsaw
		return
	}
	g.inventory.ReadyWeapon = weaponFist
}

func (g *game) canFireSelectedWeapon() bool {
	switch g.inventory.ReadyWeapon {
	case weaponFist, weaponChainsaw:
		return true
	case weaponPistol, weaponChaingun:
		return g.stats.Bullets > 0
	case weaponShotgun:
		return g.stats.Shells > 0
	case weaponRocketLauncher:
		return g.stats.Rockets > 0
	case weaponPlasma:
		return g.stats.Cells > 0
	case weaponBFG:
		return g.stats.Cells >= 40
	default:
		return false
	}
}

func (g *game) selectWeaponSlot(slot int) {
	g.ensureWeaponDefaults()
	switch slot {
	case 1:
		if g.inventory.Weapons[2005] {
			g.inventory.ReadyWeapon = weaponChainsaw
		} else {
			g.inventory.ReadyWeapon = weaponFist
		}
	case 2:
		g.inventory.ReadyWeapon = weaponPistol
	case 3:
		if g.inventory.Weapons[2001] {
			g.inventory.ReadyWeapon = weaponShotgun
		}
	case 4:
		if g.inventory.Weapons[2002] {
			g.inventory.ReadyWeapon = weaponChaingun
		}
	case 5:
		if g.inventory.Weapons[2003] {
			g.inventory.ReadyWeapon = weaponRocketLauncher
		}
	case 6:
		if g.inventory.Weapons[2004] {
			g.inventory.ReadyWeapon = weaponPlasma
		}
	case 7:
		if g.inventory.Weapons[2006] {
			g.inventory.ReadyWeapon = weaponBFG
		}
	}
}

func weaponName(id weaponID) string {
	switch id {
	case weaponFist:
		return "fist"
	case weaponPistol:
		return "pistol"
	case weaponShotgun:
		return "shotgun"
	case weaponChaingun:
		return "chaingun"
	case weaponRocketLauncher:
		return "rocket"
	case weaponPlasma:
		return "plasma"
	case weaponBFG:
		return "bfg"
	case weaponChainsaw:
		return "chainsaw"
	default:
		return "unknown"
	}
}
