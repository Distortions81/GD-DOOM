package doomruntime

import (
	"fmt"
)

import "gddoom/internal/doomrand"

const playerDeathViewFallSpeed = fracUnit

type sectorLightEffectKind uint8

const (
	sectorLightEffectNone sectorLightEffectKind = iota
	sectorLightEffectFireFlicker
	sectorLightEffectLightFlash
	sectorLightEffectStrobe
	sectorLightEffectGlow
)

type sectorLightEffect struct {
	kind       sectorLightEffectKind
	minLight   int16
	maxLight   int16
	count      int
	minTime    int
	maxTime    int
	darkTime   int
	brightTime int
	direction  int
}

const (
	sectorLightGlowSpeed    = 8
	sectorLightStrobeBright = 5
	sectorLightFastDark     = 15
	sectorLightSlowDark     = 35
)

func (g *game) beginWorldTic() {
	g.prunePendingDoors()
	g.worldTic++
	doomrand.SetDebugTic(g.worldTic)
	g.ticDisplayText = formatTicDisplay(g.worldTic, g.ticRateDisplay)
}

func (g *game) finishWorldTic() {
	g.refreshSectorPlaneCacheLighting()
}

func (g *game) initSectorLightEffects() {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		return
	}
	g.sectorLightFx = make([]sectorLightEffect, len(g.m.Sectors))
	for sec := range g.m.Sectors {
		special := g.m.Sectors[sec].Special
		switch special {
		case 1:
			g.spawnSectorLightFlash(sec)
			g.m.Sectors[sec].Special = 0
		case 2:
			g.spawnSectorStrobeFlash(sec, sectorLightFastDark, false)
			g.m.Sectors[sec].Special = 0
		case 3:
			g.spawnSectorStrobeFlash(sec, sectorLightSlowDark, false)
			g.m.Sectors[sec].Special = 0
		case 4:
			g.spawnSectorStrobeFlash(sec, sectorLightFastDark, false)
			// Doom keeps special 4 as damaging strobe slime.
			g.m.Sectors[sec].Special = 4
		case 8:
			g.spawnSectorGlow(sec)
			g.m.Sectors[sec].Special = 0
		case 12:
			g.spawnSectorStrobeFlash(sec, sectorLightSlowDark, true)
			g.m.Sectors[sec].Special = 0
		case 13:
			g.spawnSectorStrobeFlash(sec, sectorLightFastDark, true)
			g.m.Sectors[sec].Special = 0
		case 17:
			g.spawnSectorFireFlicker(sec)
			g.m.Sectors[sec].Special = 0
		}
	}
}

func (g *game) spawnSectorFireFlicker(sec int) {
	if sec < 0 || sec >= len(g.m.Sectors) || sec >= len(g.sectorLightFx) {
		return
	}
	maxLight := g.m.Sectors[sec].Light
	minLight := g.findMinSurroundingSectorLight(sec, maxLight) + 16
	if minLight > maxLight {
		minLight = maxLight
	}
	g.sectorLightFx[sec] = sectorLightEffect{
		kind:     sectorLightEffectFireFlicker,
		minLight: minLight,
		maxLight: maxLight,
		count:    4,
	}
}

func (g *game) spawnSectorLightFlash(sec int) {
	if sec < 0 || sec >= len(g.m.Sectors) || sec >= len(g.sectorLightFx) {
		return
	}
	maxLight := g.m.Sectors[sec].Light
	g.sectorLightFx[sec] = sectorLightEffect{
		kind:     sectorLightEffectLightFlash,
		minLight: g.findMinSurroundingSectorLight(sec, maxLight),
		maxLight: maxLight,
		minTime:  7,
		maxTime:  64,
		count:    (doomrand.PRandom() & 64) + 1,
	}
}

func (g *game) spawnSectorStrobeFlash(sec int, darkTime int, inSync bool) {
	if sec < 0 || sec >= len(g.m.Sectors) || sec >= len(g.sectorLightFx) {
		return
	}
	maxLight := g.m.Sectors[sec].Light
	minLight := g.findMinSurroundingSectorLight(sec, maxLight)
	if minLight == maxLight {
		minLight = 0
	}
	count := 1
	if !inSync {
		count = (doomrand.PRandom() & 7) + 1
	}
	g.sectorLightFx[sec] = sectorLightEffect{
		kind:       sectorLightEffectStrobe,
		minLight:   minLight,
		maxLight:   maxLight,
		darkTime:   darkTime,
		brightTime: sectorLightStrobeBright,
		count:      count,
	}
}

func (g *game) spawnSectorGlow(sec int) {
	if sec < 0 || sec >= len(g.m.Sectors) || sec >= len(g.sectorLightFx) {
		return
	}
	maxLight := g.m.Sectors[sec].Light
	g.sectorLightFx[sec] = sectorLightEffect{
		kind:      sectorLightEffectGlow,
		minLight:  g.findMinSurroundingSectorLight(sec, maxLight),
		maxLight:  maxLight,
		direction: -1,
	}
}

func (g *game) tickSectorLightEffects() {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 || len(g.sectorLightFx) != len(g.m.Sectors) {
		return
	}
	for sec := range g.sectorLightFx {
		fx := &g.sectorLightFx[sec]
		switch fx.kind {
		case sectorLightEffectFireFlicker:
			fx.count--
			if fx.count != 0 {
				continue
			}
			roll := doomrand.PRandom()
			if want := runtimeDebugEnv("GD_DEBUG_WORLD_RNG_TIC"); want != "" {
				var wantTic int
				if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
					if g.demoTick-1 == wantTic || g.worldTic == wantTic {
						rnd, prnd := doomrand.State()
						fmt.Printf("world-rng-debug tic=%d world=%d kind=fireflicker sec=%d roll=%d rnd=%d prnd=%d\n",
							g.demoTick-1, g.worldTic, sec, roll, rnd, prnd)
					}
				}
			}
			amount := (roll & 3) * 16
			if int(g.m.Sectors[sec].Light)-amount < int(fx.minLight) {
				g.m.Sectors[sec].Light = fx.minLight
			} else {
				g.m.Sectors[sec].Light = fx.maxLight - int16(amount)
			}
			fx.count = 4
		case sectorLightEffectLightFlash:
			fx.count--
			if fx.count != 0 {
				continue
			}
			if g.m.Sectors[sec].Light == fx.maxLight {
				g.m.Sectors[sec].Light = fx.minLight
				roll := doomrand.PRandom()
				if want := runtimeDebugEnv("GD_DEBUG_WORLD_RNG_TIC"); want != "" {
					var wantTic int
					if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
						if g.demoTick-1 == wantTic || g.worldTic == wantTic {
							rnd, prnd := doomrand.State()
							fmt.Printf("world-rng-debug tic=%d world=%d kind=lightflash-dark sec=%d roll=%d rnd=%d prnd=%d\n",
								g.demoTick-1, g.worldTic, sec, roll, rnd, prnd)
						}
					}
				}
				fx.count = (roll & fx.minTime) + 1
			} else {
				g.m.Sectors[sec].Light = fx.maxLight
				roll := doomrand.PRandom()
				if want := runtimeDebugEnv("GD_DEBUG_WORLD_RNG_TIC"); want != "" {
					var wantTic int
					if _, err := fmt.Sscanf(want, "%d", &wantTic); err == nil {
						if g.demoTick-1 == wantTic || g.worldTic == wantTic {
							rnd, prnd := doomrand.State()
							fmt.Printf("world-rng-debug tic=%d world=%d kind=lightflash-bright sec=%d roll=%d rnd=%d prnd=%d\n",
								g.demoTick-1, g.worldTic, sec, roll, rnd, prnd)
						}
					}
				}
				fx.count = (roll & fx.maxTime) + 1
			}
		case sectorLightEffectStrobe:
			fx.count--
			if fx.count != 0 {
				continue
			}
			if g.m.Sectors[sec].Light == fx.minLight {
				g.m.Sectors[sec].Light = fx.maxLight
				fx.count = fx.brightTime
			} else {
				g.m.Sectors[sec].Light = fx.minLight
				fx.count = fx.darkTime
			}
		case sectorLightEffectGlow:
			switch fx.direction {
			case -1:
				g.m.Sectors[sec].Light -= sectorLightGlowSpeed
				if g.m.Sectors[sec].Light <= fx.minLight {
					g.m.Sectors[sec].Light += sectorLightGlowSpeed
					fx.direction = 1
				}
			case 1:
				g.m.Sectors[sec].Light += sectorLightGlowSpeed
				if g.m.Sectors[sec].Light >= fx.maxLight {
					g.m.Sectors[sec].Light -= sectorLightGlowSpeed
					fx.direction = -1
				}
			}
		}
	}
}

func (g *game) findMinSurroundingSectorLight(sec int, maxLight int16) int16 {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return maxLight
	}
	minLight := maxLight
	found := false
	for _, ld := range g.m.Linedefs {
		s0, s1 := int(ld.SideNum[0]), int(ld.SideNum[1])
		if s0 < 0 || s0 >= len(g.m.Sidedefs) {
			continue
		}
		sec0 := int(g.m.Sidedefs[s0].Sector)
		sec1 := -1
		if s1 >= 0 && s1 < len(g.m.Sidedefs) {
			sec1 = int(g.m.Sidedefs[s1].Sector)
		}
		switch {
		case sec0 == sec && sec1 >= 0 && sec1 < len(g.m.Sectors):
			other := g.m.Sectors[sec1].Light
			if !found || other < minLight {
				minLight = other
			}
			found = true
		case sec1 == sec && sec0 >= 0 && sec0 < len(g.m.Sectors):
			other := g.m.Sectors[sec0].Light
			if !found || other < minLight {
				minLight = other
			}
			found = true
		}
	}
	if !found {
		return maxLight
	}
	return minLight
}

func (g *game) tickPlayerViewHeight() {
	if g.p.viewHeight == 0 {
		g.p.viewHeight = playerViewHeight
	}
	aliveEye := g.p.z + g.p.viewHeight
	if g.playerViewZ == 0 && !g.isDead {
		g.playerViewZ = aliveEye
	}
	if !g.isDead {
		if g.p.z <= g.p.floorz {
			g.p.viewHeight += g.p.deltaViewHeight
			if g.p.viewHeight > playerViewHeight {
				g.p.viewHeight = playerViewHeight
				g.p.deltaViewHeight = 0
			}
			if g.p.viewHeight < playerViewHeightMin {
				g.p.viewHeight = playerViewHeightMin
				if g.p.deltaViewHeight <= 0 {
					g.p.deltaViewHeight = 1
				}
			}
			if g.p.deltaViewHeight != 0 {
				g.p.deltaViewHeight += fracUnit / 4
				if g.p.deltaViewHeight == 0 {
					g.p.deltaViewHeight = 1
				}
			}
		}
		aliveEye = g.p.z + g.p.viewHeight
		if aliveEye > g.p.ceilz-4*fracUnit {
			aliveEye = g.p.ceilz - 4*fracUnit
		}
		g.playerViewZ = aliveEye
		return
	}
	target := g.p.floorz
	if g.playerViewZ > target {
		g.playerViewZ -= playerDeathViewFallSpeed
		if g.playerViewZ < target {
			g.playerViewZ = target
		}
		return
	}
	g.playerViewZ = target
}

func (g *game) trackSecrets() {
	if g.m == nil || len(g.m.Sectors) == 0 || len(g.secretFound) != len(g.m.Sectors) {
		return
	}
	sec := g.playerSector()
	if sec < 0 || sec >= len(g.m.Sectors) {
		return
	}
	sectorFloor := g.playerSectorFloor(sec)
	if g.p.z != sectorFloor {
		return
	}
	if g.m.Sectors[sec].Special != 9 || g.secretFound[sec] {
		return
	}
	g.secretFound[sec] = true
	g.secretsFound++
	g.m.Sectors[sec].Special = 0
}

func (g *game) applySectorHazardDamage() {
	if g.m == nil || len(g.m.Sectors) == 0 || g.stats.Health <= 0 {
		return
	}
	// Doom applies periodic special-sector effects every 32 tics.
	if (g.worldTic & 31) != 0 {
		return
	}
	sec := g.playerSector()
	if sec < 0 || sec >= len(g.m.Sectors) {
		return
	}
	// Doom gates special-sector effects on the current subsector sector floor,
	// not on the local support floor cached in mo->floorz.
	if g.p.z != g.playerSectorFloor(sec) {
		return
	}
	hasSuit := g.inventory.RadSuitTics > 0
	special := g.m.Sectors[sec].Special
	damage := hazardDamage(special, hasSuit)
	if damage <= 0 {
		return
	}
	g.damagePlayer(damage, "Ouch! damaging floor")
	if special == 11 && g.stats.Health <= 10 {
		g.requestLevelExit(false, "Level Complete")
	}
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
	case 11:
		return 20
	}
	return 0
}

func (g *game) playerSectorFloor(sec int) int64 {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return 0
	}
	if sec < len(g.sectorFloor) {
		return g.sectorFloor[sec]
	}
	return int64(g.m.Sectors[sec].FloorHeight) << fracBits
}

func (g *game) damagePlayer(amount int, msg string) {
	g.damagePlayerFrom(amount, msg, 0, 0, false, -1)
}

func (g *game) damagePlayerFrom(amount int, msg string, attackerX, attackerY int64, hasAttacker bool, attackerThing int) {
	const playerPainChance = 255

	if amount <= 0 || g.stats.Health <= 0 || g.isDead {
		return
	}
	if want := runtimeDebugEnv("GD_DEBUG_PLAYER_DAMAGE_TIC"); want != "" {
		var tic int
		if _, err := fmt.Sscanf(want, "%d", &tic); err == nil {
			if g.demoTick-1 == tic || g.worldTic == tic {
				fmt.Printf("player-damage-debug tic=%d world=%d amount=%d msg=%q attacker=(%d,%d) has_attacker=%t pre_health=%d pre_armor=%d\n",
					g.demoTick-1, g.worldTic, amount, msg, attackerX, attackerY, hasAttacker, g.stats.Health, g.stats.Armor)
			}
		}
	}
	if hasAttacker {
		g.applyPlayerDamageThrust(amount, attackerX, attackerY)
	}
	if g.playerInvulnerable() {
		return
	}
	if g.stats.ArmorType != 0 && g.stats.Armor > 0 {
		saved := 0
		if g.stats.ArmorType == 1 {
			saved = amount / 3
		} else {
			saved = amount / 2
		}
		if g.stats.Armor <= saved {
			saved = g.stats.Armor
			g.stats.ArmorType = 0
		}
		g.stats.Armor -= saved
		amount -= saved
	}
	g.stats.Health -= amount
	g.playerMobjHealth -= amount
	g.damageFlashTic = max(g.damageFlashTic, 8)
	g.statusDamageCount += amount
	if g.statusDamageCount > 100 {
		g.statusDamageCount = 100
	}
	g.statusHasAttacker = hasAttacker
	if hasAttacker {
		g.statusAttackerX = attackerX
		g.statusAttackerY = attackerY
		g.statusAttackerThing = attackerThing
	} else {
		g.statusAttackerThing = -1
	}
	if g.stats.Health < 0 {
		g.stats.Health = 0
	}
	if g.stats.Health == 0 {
		g.isDead = true
		// Doom's P_KillMobj always shortens the death state's tics with
		// `P_Random() & 3`, including the player path. We don't model the
		// full player death mobj state here, but we must consume the RNG.
		_ = doomrand.PRandom() & 3
		msg = "You Died"
		g.emitSoundEvent(soundEventPlayerDeath)
	} else {
		if doomrand.PRandom() < playerPainChance {
			g.setPlayerMobjState(doomStatePlayerPain1, 4)
		}
	}
	g.setHUDMessage(msg, 20)
}

func (g *game) applyPlayerDamageThrust(amount int, attackerX, attackerY int64) {
	if g == nil || amount <= 0 {
		return
	}
	thrust := int64(amount) * (fracUnit >> 3)
	worldAng := doomPointToAngle2(attackerX, attackerY, g.p.x, g.p.y)
	g.p.momx += fixedMul(thrust, doomFineCosine(worldAng))
	g.p.momy += fixedMul(thrust, doomFineSineAtAngle(worldAng))
}
