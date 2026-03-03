package automap

import "gddoom/internal/doomrand"

func (g *game) tickWorldLogic() {
	g.worldTic++
	if g.inventory.RadSuitTics > 0 {
		g.inventory.RadSuitTics--
	}
	g.applySectorHazardDamage()
	g.tickMonsters()
}

func (g *game) applySectorHazardDamage() {
	if g.m == nil || len(g.m.Sectors) == 0 || g.stats.Health <= 0 {
		return
	}
	// Doom applies periodic special-sector effects every 32 tics.
	if (g.worldTic & 31) != 0 {
		return
	}
	// Sector damage applies while standing on the floor.
	if g.p.z != g.p.floorz {
		return
	}
	sec := g.sectorAt(g.p.x, g.p.y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return
	}
	hasSuit := g.inventory.RadSuitTics > 0
	damage := hazardDamage(g.m.Sectors[sec].Special, hasSuit)
	if damage <= 0 {
		return
	}
	g.damagePlayer(damage, "Ouch! damaging floor")
}

func hazardDamage(special int16, hasSuit bool) int {
	switch special {
	case 7:
		if !hasSuit {
			return 5
		}
	case 5:
		if !hasSuit {
			return 10
		}
	case 4, 16:
		// Doom behavior: with suit these sectors still occasionally hurt.
		if !hasSuit || doomrand.PRandom() < 5 {
			return 20
		}
	}
	return 0
}

func (g *game) damagePlayer(amount int, msg string) {
	if amount <= 0 || g.stats.Health <= 0 {
		return
	}
	g.stats.Health -= amount
	g.damageFlashTic = max(g.damageFlashTic, 8)
	if g.stats.Health < 0 {
		g.stats.Health = 0
	}
	if g.stats.Health == 0 {
		g.isDead = true
		msg = "You Died"
	}
	g.setHUDMessage(msg, 20)
	g.emitSoundEvent(soundEventOof)
}
