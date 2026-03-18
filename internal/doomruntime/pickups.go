package doomruntime

import (
	"fmt"

	"gddoom/internal/mapdata"
)

type playerInventory struct {
	BlueKey       bool
	RedKey        bool
	YellowKey     bool
	Backpack      bool
	Strength      bool
	InvulnTics    int
	InvisTics     int
	RadSuitTics   int
	AllMap        bool
	LightAmpTics  int
	ReadyWeapon   weaponID
	PendingWeapon weaponID
	Weapons       map[int16]bool
}

type playerStats struct {
	Health    int
	Armor     int
	ArmorType int
	Bullets   int
	Shells    int
	Rockets   int
	Cells     int
}

type playerLevelCarryover struct {
	Inventory playerInventory
	Stats     playerStats
}

func (g *game) initPlayerState() {
	g.inventory = playerInventory{
		ReadyWeapon: weaponPistol,
		Weapons: map[int16]bool{
			2002: false, // chaingun
			2001: false, // shotgun
			82:   false, // super shotgun
			2005: false, // chainsaw
			2003: false, // rocket launcher
			2004: false, // plasma gun
			2006: false, // BFG9000
		},
	}
	g.stats = playerStats{
		Health:    100,
		Armor:     0,
		ArmorType: 0,
		Bullets:   50,
		Shells:    0,
		Rockets:   0,
		Cells:     0,
	}
}

func (g *game) captureLevelCarryover() playerLevelCarryover {
	if g == nil {
		return playerLevelCarryover{}
	}
	inv := g.inventory
	if len(inv.Weapons) > 0 {
		inv.Weapons = cloneWeaponInventory(inv.Weapons)
	}
	return playerLevelCarryover{
		Inventory: inv,
		Stats:     g.stats,
	}
}

func (g *game) applyLevelCarryover(carry playerLevelCarryover) {
	if g == nil {
		return
	}
	g.inventory = carry.Inventory
	if len(g.inventory.Weapons) > 0 {
		g.inventory.Weapons = cloneWeaponInventory(g.inventory.Weapons)
	}
	g.stats = carry.Stats
	g.finishLevelForCarryover()
	g.ensureWeaponDefaults()
	g.inventory.PendingWeapon = 0
	g.clearWeaponOverlay()
}

// Match Doom's G_PlayerFinishLevel behavior for map transitions:
// keep weapons/ammo/health/armor, but clear cards and temporary powers.
func (g *game) finishLevelForCarryover() {
	if g == nil {
		return
	}
	g.inventory.BlueKey = false
	g.inventory.RedKey = false
	g.inventory.YellowKey = false
	g.inventory.Strength = false
	g.inventory.InvulnTics = 0
	g.inventory.InvisTics = 0
	g.inventory.RadSuitTics = 0
	g.inventory.AllMap = false
	g.inventory.LightAmpTics = 0
	g.statusAttackDown = false
	g.useFlash = 0
	g.damageFlashTic = 0
	g.bonusFlashTic = 0
	g.statusBonusCount = 0
	g.useText = ""
}

func cloneWeaponInventory(src map[int16]bool) map[int16]bool {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int16]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (pi playerInventory) keys() mapdata.KeyRing {
	return mapdata.KeyRing{
		Blue:   pi.BlueKey,
		Red:    pi.RedKey,
		Yellow: pi.YellowKey,
	}
}

func (pi playerInventory) keySummary() string {
	chars := []byte{'-', '-', '-'}
	if pi.BlueKey {
		chars[0] = 'B'
	}
	if pi.RedKey {
		chars[1] = 'R'
	}
	if pi.YellowKey {
		chars[2] = 'Y'
	}
	return string(chars)
}

func (g *game) processThingPickups() {
	if g.m == nil {
		return
	}
	if len(g.m.Things) == 0 || len(g.thingCollected) != len(g.m.Things) {
		return
	}
	for i, th := range g.m.Things {
		if !g.thingActiveInSession(i) {
			continue
		}
		if !isPickupType(th.Type) {
			continue
		}
		tx, ty := g.thingPosFixed(i, th)
		tz := g.thingFloorZ(tx, ty)
		radius, height := pickupTouchBounds(th.Type)
		if !canTouchPickup(g.p.x, g.p.y, g.p.z, playerRadius, playerHeight, tx, ty, tz, radius, height) {
			continue
		}
		dropped := i >= 0 && i < len(g.thingDropped) && g.thingDropped[i]
		msg, ev, picked := g.applyPickup(th.Type, dropped)
		if !picked {
			continue
		}
		g.thingCollected[i] = true
		g.setHUDMessage(msg, 45)
		g.emitSoundEvent(ev)
		g.bonusFlashTic = max(g.bonusFlashTic, 6)
		g.statusBonusCount += 6
		if g.statusBonusCount > 100 {
			g.statusBonusCount = 100
		}
	}
}

func (g *game) thingFloorZ(x, y int64) int64 {
	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.sectorFloor) {
		return 0
	}
	return g.sectorFloor[sec]
}

func (g *game) thingFloorZCached(i int, th mapdata.Thing) int64 {
	sec := g.thingSectorCached(i, th)
	if sec < 0 || sec >= len(g.sectorFloor) {
		x, y := g.thingPosFixed(i, th)
		return g.thingFloorZ(x, y)
	}
	return g.sectorFloor[sec]
}

func pickupTouchBounds(typ int16) (radius int64, height int64) {
	// Doom treats most specials as radius=20, height=16 for touch.
	switch typ {
	default:
		return 20 * fracUnit, 16 * fracUnit
	}
}

func canTouchPickup(px, py, pz, pradius, pheight, tx, ty, tz, tradius, theight int64) bool {
	blockdist := pradius + tradius
	if abs(px-tx) > blockdist || abs(py-ty) > blockdist {
		return false
	}
	delta := tz - pz
	if delta > pheight {
		return false
	}
	if delta+theight < -8*fracUnit {
		return false
	}
	return true
}

func isPickupType(typ int16) bool {
	switch typ {
	case 5, 6, 13, 38, 39, 40: // keys
		return true
	case 2011, 2012, 2013, 2014, 2023, 83: // health + berserk + soul/mega
		return true
	case 2015, 2018, 2019: // armor
		return true
	case 2022, 2024, 2025, 2026, 2045: // timed powerups
		return true
	case 2007, 2048, 2008, 2049, 2010, 2046, 2047, 17, 8: // ammo + backpack
		return true
	case 2001, 2002, 2003, 2004, 2005, 2006, 82: // weapons
		return true
	default:
		return false
	}
}

func (g *game) applyPickup(typ int16, dropped bool) (string, soundEvent, bool) {
	switch typ {
	case 5, 40:
		g.inventory.BlueKey = true
		return "Picked up a blue key", soundEventPowerUp, true
	case 13, 38:
		g.inventory.RedKey = true
		return "Picked up a red key", soundEventPowerUp, true
	case 6, 39:
		g.inventory.YellowKey = true
		return "Picked up a yellow key", soundEventPowerUp, true
	case 2011:
		return g.gainHealth(10, 100, "Picked up a stimpack")
	case 2012:
		return g.gainHealth(25, 100, "Picked up a medikit")
	case 2013:
		return g.gainHealth(100, 200, "Picked up a soulsphere")
	case 2014:
		return g.gainHealth(1, 200, "Picked up a health bonus")
	case 2023:
		if g.stats.Health < 100 {
			g.stats.Health = 100
		}
		g.inventory.Strength = true
		return "Berserk!", soundEventPowerUp, true
	case 2015:
		return g.gainArmor(1, 200, "Picked up an armor bonus")
	case 2018:
		if g.stats.Armor >= 100 {
			return "", 0, false
		}
		g.stats.Armor = 100
		g.stats.ArmorType = 1
		return "Picked up green armor", soundEventPowerUp, true
	case 2019:
		if g.stats.Armor >= 200 {
			return "", 0, false
		}
		g.stats.Armor = 200
		g.stats.ArmorType = 2
		return "Picked up blue armor", soundEventPowerUp, true
	case 2022:
		g.inventory.InvulnTics = 30 * doomTicsPerSecond
		return "Invulnerability!", soundEventPowerUp, true
	case 2024:
		g.inventory.InvisTics = 60 * doomTicsPerSecond
		return "Partial invisibility", soundEventPowerUp, true
	case 2025:
		tics := 60 * doomTicsPerSecond
		if g.inventory.RadSuitTics < tics {
			g.inventory.RadSuitTics = tics
		}
		return "Picked up a radiation suit", soundEventPowerUp, true
	case 2026:
		if g.inventory.AllMap {
			return "", 0, false
		}
		g.inventory.AllMap = true
		return "Computer area map", soundEventPowerUp, true
	case 2045:
		g.inventory.LightAmpTics = 120 * doomTicsPerSecond
		return "Light amplification visor", soundEventPowerUp, true
	case 83:
		changed := false
		if g.stats.Health != 200 {
			g.stats.Health = 200
			changed = true
		}
		if g.stats.Armor != 200 || g.stats.ArmorType != 2 {
			g.stats.Armor = 200
			g.stats.ArmorType = 2
			changed = true
		}
		if !changed {
			return "", 0, false
		}
		return "Megasphere!", soundEventPowerUp, true
	case 2007:
		amount := 10
		if dropped {
			amount = 5
		}
		return g.gainAmmo("bullets", amount, "Picked up a clip")
	case 2048:
		return g.gainAmmo("bullets", 50, "Picked up a box of bullets")
	case 2008:
		return g.gainAmmo("shells", 4, "Picked up 4 shotgun shells")
	case 2049:
		return g.gainAmmo("shells", 20, "Picked up a box of shells")
	case 2010:
		return g.gainAmmo("rockets", 1, "Picked up a rocket")
	case 2046:
		return g.gainAmmo("rockets", 5, "Picked up a box of rockets")
	case 2047:
		return g.gainAmmo("cells", 20, "Picked up an energy cell")
	case 17:
		return g.gainAmmo("cells", 100, "Picked up an energy cell pack")
	case 8:
		if g.inventory.Backpack {
			return g.gainAmmo("bullets", 10, "Picked up ammo from backpack")
		}
		g.inventory.Backpack = true
		g.gainAmmoNoMsg("bullets", 10)
		g.gainAmmoNoMsg("shells", 4)
		g.gainAmmoNoMsg("rockets", 1)
		g.gainAmmoNoMsg("cells", 20)
		return "Picked up a backpack", soundEventItemUp, true
	case 2001, 2002, 2003, 2004, 2005, 2006, 82:
		if g.inventory.Weapons[typ] {
			// Treat duplicate weapons as ammo pickups where sensible.
			switch typ {
			case 2001:
				amount := 8
				if dropped {
					amount = 4
				}
				return g.gainAmmo("shells", amount, "Picked up shells")
			case 2002:
				amount := 20
				if dropped {
					amount = 10
				}
				return g.gainAmmo("bullets", amount, "Picked up bullets")
			case 2003:
				return g.gainAmmo("rockets", 2, "Picked up rockets")
			case 2004:
				return g.gainAmmo("cells", 20, "Picked up cells")
			case 2006:
				return g.gainAmmo("cells", 40, "Picked up cells")
			case 82:
				return g.gainAmmo("shells", 8, "Picked up shells")
			default:
				return "", 0, false
			}
		}
		g.inventory.Weapons[typ] = true
		setReadyWeapon := func(id weaponID) {
			if !g.autoWeaponSwitch {
				return
			}
			g.queueWeaponSwitch(id)
		}
		switch typ {
		case 2001:
			if dropped {
				g.gainAmmoNoMsg("shells", 4)
			} else {
				g.gainAmmoNoMsg("shells", 8)
			}
			setReadyWeapon(weaponShotgun)
			return "Picked up a shotgun", soundEventWeaponUp, true
		case 2002:
			if dropped {
				g.gainAmmoNoMsg("bullets", 10)
			} else {
				g.gainAmmoNoMsg("bullets", 20)
			}
			setReadyWeapon(weaponChaingun)
			return "Picked up a chaingun", soundEventWeaponUp, true
		case 2003:
			g.gainAmmoNoMsg("rockets", 2)
			setReadyWeapon(weaponRocketLauncher)
			return "Picked up a rocket launcher", soundEventWeaponUp, true
		case 2004:
			g.gainAmmoNoMsg("cells", 40)
			setReadyWeapon(weaponPlasma)
			return "Picked up a plasma rifle", soundEventWeaponUp, true
		case 2005:
			setReadyWeapon(weaponChainsaw)
			return "Picked up a chainsaw", soundEventWeaponUp, true
		case 2006:
			g.gainAmmoNoMsg("cells", 40)
			setReadyWeapon(weaponBFG)
			return "Picked up a BFG9000", soundEventWeaponUp, true
		case 82:
			g.gainAmmoNoMsg("shells", 8)
			setReadyWeapon(weaponSuperShotgun)
			return "You got the super shotgun!", soundEventWeaponUp, true
		}
	}
	return "", 0, false
}

func (g *game) gainHealth(amount, cap int, msg string) (string, soundEvent, bool) {
	prev := g.stats.Health
	g.stats.Health += amount
	if g.stats.Health > cap {
		g.stats.Health = cap
	}
	if g.stats.Health == prev {
		return "", 0, false
	}
	return msg, soundEventItemUp, true
}

func (g *game) gainArmor(amount, cap int, msg string) (string, soundEvent, bool) {
	prev := g.stats.Armor
	g.stats.Armor += amount
	if g.stats.Armor > cap {
		g.stats.Armor = cap
	}
	if g.stats.Armor > 0 && g.stats.ArmorType == 0 {
		g.stats.ArmorType = 1
	}
	if g.stats.Armor == prev {
		return "", 0, false
	}
	return msg, soundEventItemUp, true
}

func (g *game) gainAmmo(kind string, amount int, msg string) (string, soundEvent, bool) {
	if !g.gainAmmoNoMsg(kind, amount) {
		return "", 0, false
	}
	return msg, soundEventItemUp, true
}

func (g *game) gainAmmoNoMsg(kind string, amount int) bool {
	maxBullets, maxShells, maxRockets, maxCells := ammoCaps(g.inventory.Backpack)
	switch kind {
	case "bullets":
		prev := g.stats.Bullets
		g.stats.Bullets += amount
		if g.stats.Bullets > maxBullets {
			g.stats.Bullets = maxBullets
		}
		return g.stats.Bullets != prev
	case "shells":
		prev := g.stats.Shells
		g.stats.Shells += amount
		if g.stats.Shells > maxShells {
			g.stats.Shells = maxShells
		}
		return g.stats.Shells != prev
	case "rockets":
		prev := g.stats.Rockets
		g.stats.Rockets += amount
		if g.stats.Rockets > maxRockets {
			g.stats.Rockets = maxRockets
		}
		return g.stats.Rockets != prev
	case "cells":
		prev := g.stats.Cells
		g.stats.Cells += amount
		if g.stats.Cells > maxCells {
			g.stats.Cells = maxCells
		}
		return g.stats.Cells != prev
	default:
		panic(fmt.Sprintf("unknown ammo kind %q", kind))
	}
}

func ammoCaps(backpack bool) (bullets int, shells int, rockets int, cells int) {
	bullets = 200
	shells = 50
	rockets = 50
	cells = 300
	if backpack {
		bullets *= 2
		shells *= 2
		rockets *= 2
		cells *= 2
	}
	return bullets, shells, rockets, cells
}
