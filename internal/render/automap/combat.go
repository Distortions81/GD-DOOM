package automap

import (
	"math"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

const (
	pistolRange        = 2048 * fracUnit
	shotgunRange       = 2048 * fracUnit
	bulletTargetRadius = 20 * fracUnit
	doomGunSpreadShift = 18
	doomAimTopSlope    = 100.0 / 160.0
	doomAimBottomSlope = -100.0 / 160.0
	doomAimFallbackAng = uint32(1 << 26)
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
		if i >= 0 && i < len(g.thingMoveDir) {
			g.thingMoveDir[i] = monsterDirNoDir
		}
		if i >= 0 && i < len(g.thingReactionTics) {
			g.thingReactionTics[i] = monsterReactionTimeTics(th.Type)
		}
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
	g.propagateNoiseAlertFrom(g.p.x, g.p.y)
	hit := g.fireSelectedWeapon()
	g.startWeaponOverlayFire(g.inventory.ReadyWeapon)
	g.weaponRefire = true
	_ = hit
}

func (g *game) setAttackHeld(held bool) {
	g.statusAttackDown = held
	if !held {
		g.weaponRefire = false
	}
}

func (g *game) tickWeaponFire() {
	g.tickWeaponOverlay()
	if g.weaponFireCooldown > 0 {
		g.weaponFireCooldown--
	}
	if !g.statusAttackDown || g.isDead || g.weaponFireCooldown > 0 {
		return
	}
	g.handleFire()
	g.weaponFireCooldown = weaponRefireDelay(g.inventory.ReadyWeapon)
}

func weaponRefireDelay(id weaponID) int {
	// Approximate Doom p_pspr fire cadence while preserving immediate first-shot.
	// Delay N means next allowed shot is after N+1 tics in tickWeaponFire.
	switch id {
	case weaponPistol:
		return 14
	case weaponChaingun:
		return 4
	case weaponShotgun:
		return 37
	case weaponFist:
		return 17
	case weaponChainsaw:
		return 4
	default:
		return 0
	}
}

func (g *game) fireSelectedWeapon() bool {
	switch g.inventory.ReadyWeapon {
	case weaponFist:
		return g.fireFist()
	case weaponChainsaw:
		return g.fireChainsaw()
	case weaponPistol:
		g.stats.Bullets--
		slope := g.bulletSlopeForAim(g.p.angle, pistolRange)
		g.emitSoundEvent(soundEventShootPistol)
		return g.fireGunShot(g.p.angle, pistolRange, slope, !g.weaponRefire)
	case weaponChaingun:
		g.stats.Bullets--
		slope := g.bulletSlopeForAim(g.p.angle, pistolRange)
		g.emitSoundEvent(soundEventShootPistol)
		return g.fireGunShot(g.p.angle, pistolRange, slope, !g.weaponRefire)
	case weaponShotgun:
		g.stats.Shells--
		slope := g.bulletSlopeForAim(g.p.angle, shotgunRange)
		g.emitSoundEvent(soundEventShootShotgun)
		hit := false
		for i := 0; i < 7; i++ {
			if g.fireGunShot(g.p.angle, shotgunRange, slope, false) {
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

func (g *game) fireFist() bool {
	damage := 2 * (1 + (doomrand.PRandom() % 10))
	angle := addDoomAngleSpread(g.p.angle, doomGunSpreadShift)
	return g.fireMeleeAtAngle(angle, 64*fracUnit, damage)
}

func (g *game) fireChainsaw() bool {
	damage := 2 * (1 + (doomrand.PRandom() % 10))
	angle := addDoomAngleSpread(g.p.angle, doomGunSpreadShift)
	// Doom uses MELEERANGE+1 to avoid clipping through nearby targets.
	return g.fireMeleeAtAngle(angle, 64*fracUnit+fracUnit, damage)
}

func (g *game) fireMeleeAtAngle(angle uint32, rng int64, damage int) bool {
	slope := g.bulletSlopeForAim(angle, rng)
	idx, ok := g.pickHitscanMonsterTargetAtAngleWithSlope(angle, rng, bulletTargetRadius, slope, true)
	if !ok || damage <= 0 {
		return false
	}
	g.damageMonster(idx, damage)
	return true
}

func (g *game) fireGunShot(baseAngle uint32, rng int64, slope float64, accurate bool) bool {
	damage := doomGunShotDamage()
	angle := baseAngle
	if !accurate {
		angle = addDoomAngleSpread(baseAngle, doomGunSpreadShift)
	}
	idx, dist, ok := g.pickHitscanMonsterTargetAtAngleWithSlopeDist(angle, rng, bulletTargetRadius, slope, true)
	if !ok {
		if wallDist, lineIdx, wallHit := g.hitscanWallImpactDistance(angle, rng, slope); wallHit {
			if lineIdx >= 0 && lineIdx < len(g.lineSpecial) {
				info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
				if g.activateShootLineSpecial(lineIdx, info) && !info.Repeat {
					g.lineSpecial[lineIdx] = 0
				}
			}
			g.spawnHitscanPuffAtDistance(angle, slope, wallDist)
		}
		return false
	}
	g.spawnHitscanBloodAtDistance(angle, slope, dist)
	g.damageMonster(idx, damage)
	return true
}

func (g *game) playerShootZ() float64 {
	return float64(g.p.z + (playerHeight >> 1) + 8*fracUnit)
}

func (g *game) bulletSlopeForAim(baseAngle uint32, rng int64) float64 {
	if slope, ok := g.aimSlopeAtAngle(baseAngle, rng); ok {
		return slope
	}
	if slope, ok := g.aimSlopeAtAngle(baseAngle+doomAimFallbackAng, rng); ok {
		return slope
	}
	if slope, ok := g.aimSlopeAtAngle(baseAngle-doomAimFallbackAng, rng); ok {
		return slope
	}
	return 0
}

func (g *game) aimSlopeAtAngle(angle uint32, rng int64) (float64, bool) {
	if g.m == nil {
		return 0, false
	}
	ang := angleToRadians(angle)
	dirX := math.Cos(ang)
	dirY := math.Sin(ang)
	px := float64(g.p.x)
	py := float64(g.p.y)
	shootZ := g.playerShootZ()
	bestDist := math.Inf(1)
	bestSlope := 0.0
	found := false

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
		dist := rx*dirX + ry*dirY
		if dist <= 0 || dist > float64(rng) {
			continue
		}
		perp := math.Abs(rx*dirY - ry*dirX)
		if perp > float64(bulletTargetRadius) {
			continue
		}
		txFixed, tyFixed := g.thingPosFixed(i, th)
		if !g.monsterHasLOS(g.p.x, g.p.y, txFixed, tyFixed) {
			continue
		}

		floorZ := float64(g.thingFloorZ(txFixed, tyFixed))
		topZ := floorZ + float64(monsterHitHeight(th.Type))
		topSlope := (topZ - shootZ) / dist
		bottomSlope := (floorZ - shootZ) / dist
		if topSlope < doomAimBottomSlope || bottomSlope > doomAimTopSlope {
			continue
		}
		if topSlope > doomAimTopSlope {
			topSlope = doomAimTopSlope
		}
		if bottomSlope < doomAimBottomSlope {
			bottomSlope = doomAimBottomSlope
		}
		if dist < bestDist {
			bestDist = dist
			bestSlope = (topSlope + bottomSlope) * 0.5
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return bestSlope, true
}

func doomGunShotDamage() int {
	return 5 * (1 + (doomrand.PRandom() % 3))
}

func addDoomAngleSpread(base uint32, shift uint) uint32 {
	// Doom-style spread: (P_Random - P_Random) << shift.
	delta := (doomrand.PRandom() - doomrand.PRandom()) << shift
	return base + uint32(int32(delta))
}

func (g *game) pickHitscanMonsterTarget() (int, bool) {
	return g.pickHitscanMonsterTargetAtAngle(g.p.angle, pistolRange, bulletTargetRadius)
}

func (g *game) pickHitscanMonsterTargetAtAngle(angle uint32, rng int64, radius int64) (int, bool) {
	return g.pickHitscanMonsterTargetAtAngleWithSlope(angle, rng, radius, 0, false)
}

func (g *game) pickHitscanMonsterTargetAtAngleWithSlope(angle uint32, rng int64, radius int64, slope float64, useSlope bool) (int, bool) {
	idx, _, ok := g.pickHitscanMonsterTargetAtAngleWithSlopeDist(angle, rng, radius, slope, useSlope)
	return idx, ok
}

func (g *game) pickHitscanMonsterTargetAtAngleWithSlopeDist(angle uint32, rng int64, radius int64, slope float64, useSlope bool) (int, float64, bool) {
	if g.m == nil {
		return -1, 0, false
	}
	ang := angleToRadians(angle)
	dirX := math.Cos(ang)
	dirY := math.Sin(ang)
	px := float64(g.p.x)
	py := float64(g.p.y)
	shootZ := g.playerShootZ()
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
		txFixed, tyFixed := g.thingPosFixed(i, th)
		if !g.monsterHasLOS(g.p.x, g.p.y, txFixed, tyFixed) {
			continue
		}
		if useSlope {
			floorZ := float64(g.thingFloorZ(txFixed, tyFixed))
			topZ := floorZ + float64(monsterHitHeight(th.Type))
			topSlope := (topZ - shootZ) / t
			bottomSlope := (floorZ - shootZ) / t
			if slope < bottomSlope || slope > topSlope {
				continue
			}
		}
		if t < bestDist {
			bestDist = t
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return -1, 0, false
	}
	return bestIdx, bestDist, true
}

func (g *game) hitscanWallImpactDistance(angle uint32, rng int64, slope float64) (float64, int, bool) {
	if len(g.lines) == 0 {
		return 0, -1, false
	}
	px := g.p.x
	py := g.p.y
	ang := angleToRadians(angle)
	x2 := px + int64(math.Cos(ang)*float64(rng))
	y2 := py + int64(math.Sin(ang)*float64(rng))
	shootZ := g.playerShootZ()
	bestDist := math.Inf(1)
	bestLine := -1
	found := false
	for _, ld := range g.lines {
		frac, ok := segmentIntersectFrac(px, py, x2, y2, ld.x1, ld.y1, ld.x2, ld.y2)
		if !ok || frac <= 0 {
			continue
		}
		dist := frac * float64(rng)
		if dist <= 0 || dist >= bestDist {
			continue
		}
		hitsWall := false
		if (ld.flags&mlTwoSided) == 0 || ld.sideNum1 < 0 {
			hitsWall = true
		} else if g.m != nil && len(g.m.Sidedefs) > 0 && len(g.m.Sectors) > 0 {
			opentop, openbottom, _, openrange := g.lineOpening(ld)
			if openrange <= 0 {
				hitsWall = true
			} else {
				zAtDist := shootZ + slope*dist
				if zAtDist > float64(opentop) || zAtDist < float64(openbottom) {
					hitsWall = true
				}
			}
		}
		if !hitsWall {
			continue
		}
		bestDist = dist
		bestLine = ld.idx
		found = true
	}
	if !found {
		return 0, -1, false
	}
	return bestDist, bestLine, true
}

func (g *game) spawnHitscanPuffAtDistance(angle uint32, slope, dist float64) {
	if dist <= 0 {
		return
	}
	px := float64(g.p.x)
	py := float64(g.p.y)
	ang := angleToRadians(angle)
	x := px + math.Cos(ang)*dist
	y := py + math.Sin(ang)*dist
	z := g.playerShootZ() + slope*dist
	// Doom line hits use 4-unit backoff before spawning a puff.
	const backoff = float64(4 * fracUnit)
	x -= math.Cos(ang) * backoff
	y -= math.Sin(ang) * backoff
	z += float64((doomrand.PRandom() - doomrand.PRandom()) << 10)
	g.spawnHitscanPuff(int64(x), int64(y), int64(z))
}

func (g *game) spawnHitscanBloodAtDistance(angle uint32, slope, dist float64) {
	if dist <= 0 {
		return
	}
	px := float64(g.p.x)
	py := float64(g.p.y)
	ang := angleToRadians(angle)
	x := px + math.Cos(ang)*dist
	y := py + math.Sin(ang)*dist
	z := g.playerShootZ() + slope*dist
	// Doom thing hits use 10-unit backoff before spawning blood.
	const backoff = float64(10 * fracUnit)
	x -= math.Cos(ang) * backoff
	y -= math.Sin(ang) * backoff
	z += float64((doomrand.PRandom() - doomrand.PRandom()) << 10)
	g.spawnHitscanBlood(int64(x), int64(y), int64(z))
}

func monsterHitHeight(typ int16) int64 {
	h := int64(monsterRenderHeight(typ) * fracUnit)
	if h <= 0 {
		return 56 * fracUnit
	}
	return h
}

func (g *game) damageMonster(thingIdx int, damage int) {
	if thingIdx < 0 || thingIdx >= len(g.thingHP) || damage <= 0 {
		return
	}
	if g.m == nil || thingIdx >= len(g.m.Things) {
		return
	}
	if g.thingHP[thingIdx] <= 0 {
		return
	}
	thingType := g.m.Things[thingIdx].Type
	g.thingHP[thingIdx] -= damage
	if thingIdx >= 0 && thingIdx < len(g.thingAggro) {
		g.thingAggro[thingIdx] = true
	}
	if thingIdx >= 0 && thingIdx < len(g.thingJustHit) {
		// Doom P_CheckMissileRange: recently-hit monsters retaliate immediately.
		g.thingJustHit[thingIdx] = true
	}
	if g.thingHP[thingIdx] <= 0 {
		g.thingHP[thingIdx] = 0
		if thingIdx >= 0 && thingIdx < len(g.thingDead) {
			g.thingDead[thingIdx] = true
		}
		if thingIdx >= 0 && thingIdx < len(g.thingDeathTics) {
			g.thingDeathTics[thingIdx] = monsterDeathAnimTotalTics(thingType)
		}
		if thingIdx >= 0 && thingIdx < len(g.thingPainTics) {
			g.thingPainTics[thingIdx] = 0
		}
		if thingIdx >= 0 && thingIdx < len(g.thingAttackTics) {
			g.thingAttackTics[thingIdx] = 0
		}
		if thingIdx >= 0 && thingIdx < len(g.thingAttackFireTics) {
			g.thingAttackFireTics[thingIdx] = -1
		}
		deathEv := monsterDeathSoundEvent(thingType)
		deathDelay := monsterDeathSoundDelayTics(thingType)
		if deathDelay > 0 {
			g.emitSoundEventDelayed(deathEv, deathDelay)
		} else {
			g.emitSoundEvent(deathEv)
		}
		g.setHUDMessage("Monster killed", 15)
		g.bonusFlashTic = max(g.bonusFlashTic, 4)
		g.spawnMonsterDrop(thingIdx, thingType)
	} else {
		if thingIdx >= 0 && thingIdx < len(g.thingPainTics) {
			chance := monsterPainChance(thingType)
			if chance > 0 && (chance >= 256 || doomrand.PRandom() < chance) {
				wasInPain := g.thingPainTics[thingIdx] > 0
				g.thingPainTics[thingIdx] = max(g.thingPainTics[thingIdx], monsterPainDurationTics(thingType))
				if !wasInPain {
					g.emitSoundEvent(monsterPainSoundEvent(thingType))
				}
			}
		}
		g.setHUDMessage("Hit", 8)
	}
}

func monsterDropPickupType(typ int16) (int16, bool) {
	switch typ {
	case 84, 3004: // wolfenstein-ss, zombieman
		return 2007, true // clip
	case 9: // shotgun guy
		return 2001, true // shotgun
	case 65: // chaingunner
		return 2002, true // chaingun
	default:
		return 0, false
	}
}

func (g *game) appendRuntimeThing(th mapdata.Thing, dropped bool) int {
	if g == nil || g.m == nil {
		return -1
	}
	x := int64(th.X) << fracBits
	y := int64(th.Y) << fracBits
	g.m.Things = append(g.m.Things, th)
	g.thingCollected = append(g.thingCollected, false)
	g.thingDropped = append(g.thingDropped, dropped)
	g.thingX = append(g.thingX, x)
	g.thingY = append(g.thingY, y)
	g.thingHP = append(g.thingHP, 0)
	g.thingAggro = append(g.thingAggro, false)
	g.thingCooldown = append(g.thingCooldown, 0)
	g.thingMoveDir = append(g.thingMoveDir, monsterDirNoDir)
	g.thingMoveCount = append(g.thingMoveCount, 0)
	g.thingJustAtk = append(g.thingJustAtk, false)
	g.thingJustHit = append(g.thingJustHit, false)
	g.thingReactionTics = append(g.thingReactionTics, 0)
	g.thingDead = append(g.thingDead, false)
	g.thingDeathTics = append(g.thingDeathTics, 0)
	g.thingAttackTics = append(g.thingAttackTics, 0)
	g.thingAttackFireTics = append(g.thingAttackFireTics, -1)
	g.thingPainTics = append(g.thingPainTics, 0)
	g.thingThinkWait = append(g.thingThinkWait, 0)
	sec := -1
	sec = g.sectorAt(x, y)
	g.thingSectorCache = append(g.thingSectorCache, sec)
	return len(g.m.Things) - 1
}

func (g *game) spawnMonsterDrop(thingIdx int, thingType int16) {
	if g == nil || g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) {
		return
	}
	dropType, ok := monsterDropPickupType(thingType)
	if !ok {
		return
	}
	src := g.m.Things[thingIdx]
	srcX, srcY := g.thingPosFixed(thingIdx, src)
	g.appendRuntimeThing(mapdata.Thing{
		X:    int16(srcX >> fracBits),
		Y:    int16(srcY >> fracBits),
		Type: dropType,
	}, true)
	g.setThingPosFixed(len(g.m.Things)-1, srcX, srcY)
}

func monsterPainSoundEvent(typ int16) soundEvent {
	switch typ {
	case 3002, 3005, 3003, 16, 7, 3006: // demon-family pain sound in Doom
		return soundEventMonsterPainDemon
	case 3004, 9, 3001: // former-human family + imp pain sound in Doom
		return soundEventMonsterPainHumanoid
	default:
		return soundEventMonsterPainHumanoid
	}
}

func monsterDeathSoundEvent(typ int16) soundEvent {
	switch typ {
	case 3004:
		return soundEventDeathZombie
	case 9:
		return soundEventDeathShotgunGuy
	case 3001:
		return soundEventDeathImp
	case 3002:
		return soundEventDeathDemon
	case 3005:
		return soundEventDeathCaco
	case 3003:
		return soundEventDeathBaron
	case 16:
		return soundEventDeathCyber
	case 7:
		return soundEventDeathSpider
	case 3006:
		return soundEventDeathLostSoul
	default:
		return soundEventMonsterDeath
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
	switchTo := func(id weaponID) {
		if g.inventory.ReadyWeapon != id {
			g.weaponRefire = false
			g.weaponFireCooldown = 0
			g.clearWeaponOverlay()
		}
		g.inventory.ReadyWeapon = id
	}
	if g.stats.Shells > 0 && g.inventory.Weapons[2001] {
		switchTo(weaponShotgun)
		return
	}
	if g.stats.Bullets > 0 && g.inventory.Weapons[2002] {
		switchTo(weaponChaingun)
		return
	}
	if g.stats.Bullets > 0 {
		switchTo(weaponPistol)
		return
	}
	if g.stats.Cells >= 40 && g.inventory.Weapons[2006] {
		switchTo(weaponBFG)
		return
	}
	if g.stats.Cells > 0 && g.inventory.Weapons[2004] {
		switchTo(weaponPlasma)
		return
	}
	if g.stats.Rockets > 0 && g.inventory.Weapons[2003] {
		switchTo(weaponRocketLauncher)
		return
	}
	if g.inventory.Weapons[2005] {
		switchTo(weaponChainsaw)
		return
	}
	switchTo(weaponFist)
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
	prev := g.inventory.ReadyWeapon
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
	if g.inventory.ReadyWeapon != prev {
		g.weaponRefire = false
		g.weaponFireCooldown = 0
		g.clearWeaponOverlay()
	}
}

func (g *game) weaponOwned(id weaponID) bool {
	switch id {
	case weaponFist:
		return true
	case weaponPistol:
		return true
	case weaponShotgun:
		return g.inventory.Weapons[2001]
	case weaponChaingun:
		return g.inventory.Weapons[2002]
	case weaponRocketLauncher:
		return g.inventory.Weapons[2003]
	case weaponPlasma:
		return g.inventory.Weapons[2004]
	case weaponBFG:
		return g.inventory.Weapons[2006]
	case weaponChainsaw:
		return g.inventory.Weapons[2005]
	default:
		return false
	}
}

func weaponCycleOrder() []weaponID {
	return []weaponID{
		weaponFist,
		weaponChainsaw,
		weaponPistol,
		weaponShotgun,
		weaponChaingun,
		weaponRocketLauncher,
		weaponPlasma,
		weaponBFG,
	}
}

func (g *game) cycleWeapon(step int) {
	if step == 0 {
		return
	}
	g.ensureWeaponDefaults()
	order := weaponCycleOrder()
	cur := g.inventory.ReadyWeapon
	start := -1
	for i, w := range order {
		if w == cur {
			start = i
			break
		}
	}
	if start < 0 {
		start = 0
	}
	n := len(order)
	for i := 1; i <= n; i++ {
		idx := (start + i*step) % n
		if idx < 0 {
			idx += n
		}
		next := order[idx]
		if !g.weaponOwned(next) {
			continue
		}
		if next == cur {
			continue
		}
		g.inventory.ReadyWeapon = next
		g.weaponRefire = false
		g.weaponFireCooldown = 0
		g.clearWeaponOverlay()
		return
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
