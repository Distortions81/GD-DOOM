package automap

func normalizeCheatLevel(v int) int {
	if v < 0 {
		return 0
	}
	if v > 3 {
		return 3
	}
	return v
}

func (g *game) applyCheatLevel(level int, announce bool) {
	g.cheatLevel = normalizeCheatLevel(level)
	switch g.cheatLevel {
	case 0:
		if announce {
			g.setHUDMessage("Cheats OFF", 70)
		}
	case 1:
		g.parity.reveal = revealAllMap
		g.parity.iddt = 2
		if announce {
			g.setHUDMessage("IDDT + allmap", 70)
		}
	case 2:
		g.parity.reveal = revealAllMap
		g.parity.iddt = 2
		g.grantIDFA()
		if announce {
			g.setHUDMessage("IDFA", 70)
		}
	case 3:
		g.parity.reveal = revealAllMap
		g.parity.iddt = 2
		g.grantIDKFA()
		g.invulnerable = true
		if announce {
			g.setHUDMessage("IDKFA + IDDQD", 70)
		}
	}
}

func (g *game) grantIDFA() {
	if g.inventory.Weapons == nil {
		g.inventory.Weapons = map[int16]bool{}
	}
	g.inventory.Weapons[2001] = true
	g.inventory.Weapons[2002] = true
	g.inventory.Weapons[2003] = true
	g.inventory.Weapons[2004] = true
	g.inventory.Weapons[2005] = true
	g.inventory.Weapons[2006] = true
	maxBullets, maxShells, maxRockets, maxCells := ammoCaps(g.inventory.Backpack)
	g.stats.Bullets = maxBullets
	g.stats.Shells = maxShells
	g.stats.Rockets = maxRockets
	g.stats.Cells = maxCells
	if g.stats.Health < 100 {
		g.stats.Health = 100
	}
	if g.stats.Armor < 200 {
		g.stats.Armor = 200
	}
	g.ensureWeaponHasAmmo()
}

func (g *game) grantIDKFA() {
	g.grantIDFA()
	g.inventory.BlueKey = true
	g.inventory.RedKey = true
	g.inventory.YellowKey = true
}
