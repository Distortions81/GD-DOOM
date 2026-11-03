package doomruntime

// Boom encodes the selected weapon itself, including SSG (slot 9). The old
// Doom demo path resolves the fist/saw and shotgun/SSG toggles in P_PlayerThink.
func commandWeapon(slot int) weaponID {
	switch slot {
	case 1:
		return weaponFist
	case 2:
		return weaponPistol
	case 3:
		return weaponShotgun
	case 4:
		return weaponChaingun
	case 5:
		return weaponRocketLauncher
	case 6:
		return weaponPlasma
	case 7:
		return weaponBFG
	case 8:
		return weaponChainsaw
	case 9:
		return weaponSuperShotgun
	}
	return 0
}
func (g *game) selectCommandWeapon(slot int) {
	if !g.boomMomentumDropoff {
		g.selectWeaponSlot(((slot - 1) & 7) + 1)
		return
	}
	if next := commandWeapon(slot); next != 0 {
		g.queueWeaponSwitch(next)
	}
}

// Default Boom/Doom preference order. Ammo thresholds differ for SSG and BFG
// in the old demo compatibility path (three shells and 41 cells).
func (g *game) fallbackWeapon(boom bool) weaponID {
	shells, cells := 3, 41
	if boom {
		shells, cells = 2, 40
	}
	for _, c := range []struct {
		weapon weaponID
		usable bool
	}{
		{weaponPlasma, g.stats.Cells >= 1}, {weaponSuperShotgun, g.stats.Shells >= shells},
		{weaponChaingun, g.stats.Bullets >= 1}, {weaponShotgun, g.stats.Shells >= 1},
		{weaponPistol, g.stats.Bullets >= 1}, {weaponChainsaw, true},
		{weaponRocketLauncher, g.stats.Rockets >= 1}, {weaponBFG, g.stats.Cells >= cells},
		{weaponFist, true},
	} {
		if c.weapon != g.inventory.ReadyWeapon && c.usable && g.weaponOwned(c.weapon) {
			return c.weapon
		}
	}
	return g.inventory.ReadyWeapon
}

// This is input policy, not simulation: demos and received network commands
// must never invent a switch when an empty weapon attempts to fire.
func (g *game) localWeaponCommand(slot int, fire bool) int {
	if !g.boomMomentumDropoff {
		return slot
	}
	if !g.weaponAttackDown {
		g.boomAutoSwitchDone = false
	}
	if !g.isDead && g.weaponAttackDown && !g.canFireSelectedWeapon() && (!g.boomAutoSwitchDone || fire && g.inventory.PendingWeapon == 0) {
		g.boomAutoSwitchDone = true
		return demoTraceWeaponID(g.fallbackWeapon(true)) + 1
	}
	next := commandWeapon(slot)
	if slot == 1 && g.weaponOwned(weaponChainsaw) && g.inventory.ReadyWeapon != weaponChainsaw {
		next = weaponChainsaw
	}
	if slot == 3 && g.weaponOwned(weaponSuperShotgun) && (!g.weaponOwned(weaponShotgun) || g.inventory.ReadyWeapon != weaponSuperShotgun) {
		next = weaponSuperShotgun
	}
	if next != 0 {
		return demoTraceWeaponID(next) + 1
	}
	return slot
}
