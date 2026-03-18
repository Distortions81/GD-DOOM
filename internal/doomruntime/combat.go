package doomruntime

import (
	"math"
	"sort"
	"strings"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

const (
	pistolRange        = 2048 * fracUnit
	shotgunRange       = 2048 * fracUnit
	bulletTargetRadius = 20 * fracUnit
	doomGunSpreadShift = 18
	doomAimFallbackAng = uint32(1 << 26)
	doomAimTopSlope    = (100 * fracUnit) / 160
	doomAimBottomSlope = -((100 * fracUnit) / 160)
)

type lineAttackTargetMask uint8

const (
	lineAttackMaskPlayer lineAttackTargetMask = 1 << iota
	lineAttackMaskShootables
)

type lineAttackTargetKind uint8

const (
	lineAttackTargetNone lineAttackTargetKind = iota
	lineAttackTargetPlayer
	lineAttackTargetThing
)

type lineAttackActor struct {
	isPlayer   bool
	thingIdx   int
	x          int64
	y          int64
	shootZ     int64
	targetMask lineAttackTargetMask
}

type lineAttackTarget struct {
	kind lineAttackTargetKind
	idx  int
}

type lineAttackIntercept struct {
	frac   int64
	order  int
	isLine bool
	line   int
	target lineAttackTarget
}

type lineAttackOutcome struct {
	target     lineAttackTarget
	lineIdx    int
	dist       int64
	impactX    int64
	impactY    int64
	impactZ    int64
	spawnPuff  bool
	spawnBlood bool
}

type weaponID int

const (
	weaponFist weaponID = iota + 1
	weaponPistol
	weaponShotgun
	weaponSuperShotgun
	weaponChaingun
	weaponRocketLauncher
	weaponPlasma
	weaponBFG
	weaponChainsaw
)

func (g *game) initThingCombatState() {
	for i, th := range g.m.Things {
		if slot := playerSlotFromThingType(th.Type); slot != 0 {
			// Doom spawns the active player inline during map thing iteration,
			// and P_SpawnMobj consumes one P_Random() for its lastlook field.
			if slot == g.localSlot {
				_ = doomrand.PRandom() & 3
			}
			continue
		}
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if info, ok := demoTraceThingInfoForType(th.Type); ok {
			if i >= 0 && i < len(g.thingReactionTics) {
				g.thingReactionTics[i] = info.reaction
			}
			if i >= 0 && i < len(g.thingLastLook) {
				g.thingLastLook[i] = doomrand.PRandom() & 3
			}
			if info.spawnTics > 0 {
				spawnTics := 1 + (doomrand.PRandom() % info.spawnTics)
				if i >= 0 && i < len(g.thingThinkWait) {
					g.thingThinkWait[i] = max(spawnTics-1, 0)
				}
				if i >= 0 && i < len(g.thingState) && i < len(g.thingStateTics) {
					g.thingState[i] = monsterStateSpawn
					g.thingStateTics[i] = spawnTics
				}
				if i >= 0 && i < len(g.thingStatePhase) {
					g.thingStatePhase[i] = 0
				}
			}
		} else {
			if i >= 0 && i < len(g.thingLastLook) {
				g.thingLastLook[i] = doomrand.PRandom() & 3
			}
			if i >= 0 && i < len(g.thingReactionTics) {
				g.thingReactionTics[i] = demoTraceSpawnReactionTime(th.Type)
			}
		}
		if !thingTypeIsShootable(th.Type) {
			continue
		}
		g.thingHP[i] = shootableThingSpawnHealth(th.Type)
		if i >= 0 && i < len(g.thingState) && i < len(g.thingStateTics) {
			g.thingState[i] = monsterStateSpawn
			if g.thingStateTics[i] == 0 {
				if isMonster(th.Type) {
					g.thingStateTics[i] = monsterSpawnStateTics(th.Type)
				} else if isBarrelThingType(th.Type) {
					g.thingStateTics[i] = barrelSpawnStateTics[0]
				}
			}
		}
		if i >= 0 && i < len(g.thingStatePhase) {
			g.thingStatePhase[i] = 0
		}
	}
}

func demoTraceSpawnReactionTime(typ int16) int {
	switch {
	case isMonster(typ):
		return monsterReactionTimeTics(typ)
	case playerSlotFromThingType(typ) != 0:
		return 0
	default:
		return 8
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
	case 58: // spectre
		return 150
	case 3006: // lost soul
		return 100
	case 3005: // cacodemon
		return 400
	case 3003: // baron
		return 1000
	case 69: // hell knight
		return 500
	case 16: // cyberdemon
		return 4000
	case 7: // spider mastermind
		return 3000
	case 64: // arch-vile
		return 700
	case 65: // chaingunner
		return 70
	case 66: // revenant
		return 300
	case 67: // mancubus
		return 600
	case 68: // arachnotron
		return 500
	case 71: // pain elemental
		return 400
	case 84: // wolfenstein ss
		return 50
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
	_ = hit
}

func (g *game) setAttackHeld(held bool) {
	g.statusAttackDown = held
	if !held {
		g.weaponRefire = false
		g.weaponAttackDown = false
	}
}

func (g *game) tickWeaponFire() {
	g.tickWeaponOverlay()
}

func (g *game) weaponActionReady(state weaponPspriteState) {
	if g == nil || g.isDead {
		if g != nil && g.isDead {
			g.setWeaponPSpriteState(weaponInfo(g.inventory.ReadyWeapon).downstate, false)
		}
		return
	}
	if g.inventory.ReadyWeapon == weaponChainsaw && state == weaponStateSawReady {
		g.emitSoundEvent(soundEventSawIdle)
	}
	if g.inventory.PendingWeapon != 0 {
		g.setWeaponPSpriteState(weaponInfo(g.inventory.ReadyWeapon).downstate, false)
		return
	}
	if g.statusAttackDown {
		info := weaponInfo(g.inventory.ReadyWeapon)
		if !g.weaponAttackDown || !info.nonAutoRefire {
			g.weaponAttackDown = true
			g.fireWeaponStateSequence()
			return
		}
	} else {
		g.weaponAttackDown = false
		g.weaponRefire = false
	}
}

func (g *game) weaponActionRefire(_ weaponPspriteState) {
	if g == nil {
		return
	}
	if g.statusAttackDown && g.inventory.PendingWeapon == 0 && !g.isDead {
		g.weaponRefire = true
		g.weaponAttackDown = true
		g.fireWeaponStateSequence()
		return
	}
	g.weaponRefire = false
	g.weaponAttackDown = false
	g.ensureWeaponHasAmmo()
	if g.inventory.PendingWeapon != 0 {
		g.setWeaponPSpriteState(weaponInfo(g.inventory.ReadyWeapon).downstate, false)
	}
}

func (g *game) weaponActionGunFlash(_ weaponPspriteState) {
	if g == nil {
		return
	}
	g.startWeaponFlashState(flashStartState(g.inventory.ReadyWeapon))
}

func (g *game) weaponActionBFGSound(_ weaponPspriteState) {
	if g == nil {
		return
	}
	g.emitSoundEvent(soundEventShootBFG)
}

func (g *game) weaponActionLower(_ weaponPspriteState) {
	if g == nil {
		return
	}
	g.weaponPSpriteY += weaponLowerSpeed
	if g.weaponPSpriteY < weaponBottomY {
		return
	}
	if g.isDead {
		g.weaponPSpriteY = weaponBottomY
		g.setWeaponPSpriteState(weaponStateNone, false)
		return
	}
	if g.inventory.PendingWeapon == 0 {
		g.inventory.PendingWeapon = g.inventory.ReadyWeapon
	}
	g.bringUpWeapon()
}

func (g *game) weaponActionRaise(_ weaponPspriteState) {
	if g == nil {
		return
	}
	g.weaponPSpriteY -= weaponRaiseSpeed
	if g.weaponPSpriteY > weaponTopY {
		return
	}
	g.weaponPSpriteY = weaponTopY
	g.setWeaponPSpriteState(weaponInfo(g.inventory.ReadyWeapon).readystate, false)
}

func (g *game) weaponActionPunch(_ weaponPspriteState) {
	g.fireFist()
}

func (g *game) weaponActionFirePistol(_ weaponPspriteState) {
	g.firePistol(!g.weaponRefire)
}

func (g *game) weaponActionFireShotgun(_ weaponPspriteState) {
	g.fireShotgun()
}

func (g *game) weaponActionFireSuperShotgun(_ weaponPspriteState) {
	g.fireSuperShotgun()
}

func (g *game) weaponActionFireChaingun(state weaponPspriteState) {
	g.fireChaingun(state)
}

func (g *game) weaponActionFireMissile(_ weaponPspriteState) {
	g.fireRocketLauncher()
}

func (g *game) weaponActionSaw(_ weaponPspriteState) {
	g.fireChainsaw()
}

func (g *game) weaponActionFirePlasma(_ weaponPspriteState) {
	g.firePlasma()
}

func (g *game) weaponActionFireBFG(_ weaponPspriteState) {
	g.fireBFG()
}

func (g *game) weaponActionCheckReload(_ weaponPspriteState) {
	if g == nil {
		return
	}
	g.ensureWeaponHasAmmo()
}

func (g *game) weaponActionOpenShotgun2(_ weaponPspriteState) {
	g.emitSoundEvent(soundEventShotgunOpen)
}

func (g *game) weaponActionLoadShotgun2(_ weaponPspriteState) {
	g.emitSoundEvent(soundEventShotgunLoad)
}

func (g *game) weaponActionCloseShotgun2(state weaponPspriteState) {
	g.emitSoundEvent(soundEventShotgunClose)
	g.weaponActionRefire(state)
}

func (g *game) weaponActionLight0(_ weaponPspriteState) {}

func (g *game) weaponActionLight1(_ weaponPspriteState) {}

func (g *game) weaponActionLight2(_ weaponPspriteState) {}

func (g *game) fireSelectedWeapon() bool {
	switch g.inventory.ReadyWeapon {
	case weaponFist:
		return g.fireFist()
	case weaponPistol:
		return g.firePistol(!g.weaponRefire)
	case weaponShotgun:
		return g.fireShotgun()
	case weaponSuperShotgun:
		return g.fireSuperShotgun()
	case weaponChaingun:
		return g.fireChaingun(weaponStateChaingunAtk1)
	case weaponRocketLauncher:
		return g.fireRocketLauncher()
	case weaponPlasma:
		return g.firePlasma()
	case weaponBFG:
		return g.fireBFG()
	case weaponChainsaw:
		return g.fireChainsaw()
	default:
		return false
	}
}

func (g *game) fireWeaponStateSequence() {
	g.ensureWeaponDefaults()
	if !g.canFireSelectedWeapon() {
		g.ensureWeaponHasAmmo()
		if !g.canFireSelectedWeapon() {
			g.setHUDMessage("No ammo", 20)
			g.useFlash = max(g.useFlash, 20)
			return
		}
	}
	g.propagateNoiseAlertFrom(g.p.x, g.p.y)
	g.setWeaponPSpriteState(weaponStateForAttack(g.inventory.ReadyWeapon), false)
}

func (g *game) fireFist() bool {
	damage := 2 * (1 + (doomrand.PRandom() % 10))
	if g.inventory.Strength {
		damage *= 10
	}
	angle := addDoomAngleSpread(g.p.angle, doomGunSpreadShift)
	hit, targetX, targetY := g.fireMeleeAtAngle(angle, 64*fracUnit, damage)
	if hit {
		g.emitSoundEvent(soundEventPunch)
		g.p.angle = angleToThing(g.p.x, g.p.y, targetX, targetY)
	}
	return hit
}

func (g *game) fireChainsaw() bool {
	damage := 2 * (1 + (doomrand.PRandom() % 10))
	angle := addDoomAngleSpread(g.p.angle, doomGunSpreadShift)
	hit, targetX, targetY := g.fireMeleeAtAngle(angle, 64*fracUnit+fracUnit, damage)
	if !hit {
		g.emitSoundEvent(soundEventSawFull)
		return false
	}
	g.emitSoundEvent(soundEventSawHit)
	g.p.angle = turnTowardChainsawTarget(g.p.angle, angleToThing(g.p.x, g.p.y, targetX, targetY))
	return true
}

func (g *game) firePistol(accurate bool) bool {
	g.stats.Bullets--
	g.emitSoundEvent(soundEventShootPistol)
	g.startWeaponFlashState(weaponStatePistolFlash)
	slope := g.bulletSlopeForAim(g.p.angle, pistolRange)
	return g.fireGunShot(g.p.angle, pistolRange, slope, accurate)
}

func (g *game) fireShotgun() bool {
	g.stats.Shells--
	g.emitSoundEvent(soundEventShootShotgun)
	g.startWeaponFlashState(weaponStateShotgunFlash1)
	slope := g.bulletSlopeForAim(g.p.angle, shotgunRange)
	hit := false
	for i := 0; i < 7; i++ {
		if g.fireGunShot(g.p.angle, shotgunRange, slope, false) {
			hit = true
		}
	}
	return hit
}

func (g *game) fireSuperShotgun() bool {
	g.stats.Shells -= 2
	g.emitSoundEvent(soundEventShootSuperShotgun)
	g.startWeaponFlashState(weaponStateSuperShotgunFlash1)
	slope := g.bulletSlopeForAim(g.p.angle, shotgunRange)
	hit := false
	for i := 0; i < 20; i++ {
		damage := doomGunShotDamage()
		angle := addDoomAngleSpread(g.p.angle, doomGunSpreadShift+1)
		pelletSlope := slope + int64((doomrand.PRandom()-doomrand.PRandom())<<5)
		outcome := g.lineAttackTrace(g.playerLineAttackActor(), angle, shotgunRange, pelletSlope, true)
		if g.applyLineAttackOutcome(g.playerLineAttackActor(), outcome, damage) {
			hit = true
		}
	}
	return hit
}

func (g *game) fireChaingun(state weaponPspriteState) bool {
	g.stats.Bullets--
	g.emitSoundEvent(soundEventShootPistol)
	flash := weaponStateChaingunFlash1
	if state == weaponStateChaingunAtk2 {
		flash = weaponStateChaingunFlash2
	}
	g.startWeaponFlashState(flash)
	slope := g.bulletSlopeForAim(g.p.angle, pistolRange)
	return g.fireGunShot(g.p.angle, pistolRange, slope, !g.weaponRefire)
}

func (g *game) fireRocketLauncher() bool {
	g.stats.Rockets--
	g.emitSoundEvent(soundEventShootRocket)
	return g.spawnPlayerRocket()
}

func (g *game) firePlasma() bool {
	g.stats.Cells--
	flash := weaponStatePlasmaFlash1
	if doomrand.PRandom()&1 != 0 {
		flash = weaponStatePlasmaFlash2
	}
	g.startWeaponFlashState(flash)
	g.emitSoundEvent(soundEventShootPlasma)
	return g.spawnPlayerPlasma()
}

func (g *game) fireBFG() bool {
	g.stats.Cells -= 40
	g.startWeaponFlashState(weaponStateBFGFlash1)
	return g.spawnPlayerBFG()
}

func (g *game) fireMeleeAtAngle(angle uint32, rng int64, damage int) (bool, int64, int64) {
	slope := g.bulletSlopeForAim(angle, rng)
	if damage <= 0 {
		return false, 0, 0
	}
	outcome := g.lineAttackTrace(g.playerLineAttackActor(), angle, rng, slope, false)
	if !g.applyLineAttackOutcome(g.playerLineAttackActor(), outcome, damage) {
		return false, 0, 0
	}
	if outcome.target.kind != lineAttackTargetThing || g.m == nil || outcome.target.idx < 0 || outcome.target.idx >= len(g.m.Things) {
		return true, g.p.x, g.p.y
	}
	tx, ty := g.thingPosFixed(outcome.target.idx, g.m.Things[outcome.target.idx])
	return true, tx, ty
}

func (g *game) fireGunShot(baseAngle uint32, rng int64, slope int64, accurate bool) bool {
	damage := doomGunShotDamage()
	angle := baseAngle
	if !accurate {
		angle = addDoomAngleSpread(baseAngle, doomGunSpreadShift)
	}
	actor := g.playerLineAttackActor()
	outcome := g.lineAttackTrace(actor, angle, rng, slope, true)
	return g.applyLineAttackOutcome(actor, outcome, damage)
}

func (g *game) playerShootZ() int64 {
	return g.p.z + (playerHeight >> 1) + 8*fracUnit
}

func (g *game) monsterShootZ(i int, typ int16) int64 {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return 8 * fracUnit
	}
	z, _, _ := g.thingSupportState(i, g.m.Things[i])
	return z + (monsterHeight(typ) >> 1) + 8*fracUnit
}

func (g *game) playerLineAttackActor() lineAttackActor {
	return lineAttackActor{
		isPlayer:   true,
		thingIdx:   -1,
		x:          g.p.x,
		y:          g.p.y,
		shootZ:     g.playerShootZ(),
		targetMask: lineAttackMaskShootables,
	}
}

func (g *game) monsterLineAttackActor(i int, typ int16) lineAttackActor {
	sx, sy := g.thingPosFixed(i, g.m.Things[i])
	return lineAttackActor{
		isPlayer:   false,
		thingIdx:   i,
		x:          sx,
		y:          sy,
		shootZ:     g.monsterShootZ(i, typ),
		targetMask: lineAttackMaskPlayer | lineAttackMaskShootables,
	}
}

func (g *game) lineAttackEnd(actor lineAttackActor, angle uint32, distance int64) (int64, int64) {
	return actor.x + fixedMul(distance, doomFineCosine(angle)), actor.y + fixedMul(distance, doomFineSineAtAngle(angle))
}

func lineAttackThingFrac(trace divline, x, y, radius int64) (int64, bool) {
	tracePositive := (trace.dx ^ trace.dy) > 0
	x1, y1 := x-radius, y-radius
	x2, y2 := x+radius, y+radius
	if tracePositive {
		y1 = y + radius
		y2 = y - radius
	}
	if doomPointOnDivlineSide(x1, y1, trace) == doomPointOnDivlineSide(x2, y2, trace) {
		return 0, false
	}
	dl := divline{x: x1, y: y1, dx: x2 - x1, dy: y2 - y1}
	frac := interceptVector(trace, dl)
	if frac < 0 {
		return 0, false
	}
	return frac, true
}

func (g *game) lineAttackThingTargetable(i int, mask lineAttackTargetMask, excludeThing int) (lineAttackTarget, bool) {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) || i == excludeThing {
		return lineAttackTarget{}, false
	}
	if i < len(g.thingCollected) && g.thingCollected[i] {
		return lineAttackTarget{}, false
	}
	th := g.m.Things[i]
	if mask&lineAttackMaskShootables != 0 && thingTypeIsShootable(th.Type) && i < len(g.thingHP) && g.thingHP[i] > 0 {
		if i < len(g.thingDead) && g.thingDead[i] {
			return lineAttackTarget{}, false
		}
		return lineAttackTarget{kind: lineAttackTargetThing, idx: i}, true
	}
	return lineAttackTarget{}, false
}

func (g *game) lineAttackTargetState(target lineAttackTarget) (x, y, z, height, radius int64, noBlood bool, ok bool) {
	switch target.kind {
	case lineAttackTargetPlayer:
		return g.p.x, g.p.y, g.p.z, playerHeight, playerRadius, false, !g.isDead
	case lineAttackTargetThing:
		if g == nil || g.m == nil || target.idx < 0 || target.idx >= len(g.m.Things) {
			return 0, 0, 0, 0, 0, false, false
		}
		th := g.m.Things[target.idx]
		x, y = g.thingPosFixed(target.idx, th)
		z, _, _ = g.thingSupportState(target.idx, th)
		height = g.thingCurrentHeight(target.idx, th)
		radius = thingTypeRadius(th.Type)
		noBlood = thingTypeNoBlood(th.Type)
		return x, y, z, height, radius, noBlood, true
	default:
		return 0, 0, 0, 0, 0, false, false
	}
}

func (g *game) collectLineAttackIntercepts(actor lineAttackActor, angle uint32, distance int64) []lineAttackIntercept {
	if g == nil {
		return nil
	}
	if len(g.lineValid) < len(g.lines) {
		g.lineValid = append(g.lineValid, make([]int, len(g.lines)-len(g.lineValid))...)
	}
	x1 := actor.x
	y1 := actor.y
	x2, y2 := g.lineAttackEnd(actor, angle, distance)
	trace := divline{x: x1, y: y1, dx: x2 - x1, dy: y2 - y1}
	intercepts := make([]lineAttackIntercept, 0, 32)
	order := 0

	appendLine := func(physIdx int) {
		if physIdx < 0 || physIdx >= len(g.lines) {
			return
		}
		if g.lineValid[physIdx] == g.validCount {
			return
		}
		g.lineValid[physIdx] = g.validCount
		ld := g.lines[physIdx]
		var s1, s2 int
		if trace.dx > 16*fracUnit || trace.dy > 16*fracUnit || trace.dx < -16*fracUnit || trace.dy < -16*fracUnit {
			s1 = doomPointOnDivlineSide(ld.x1, ld.y1, trace)
			s2 = doomPointOnDivlineSide(ld.x2, ld.y2, trace)
		} else {
			s1 = g.pointOnLineSide(trace.x, trace.y, ld)
			s2 = g.pointOnLineSide(trace.x+trace.dx, trace.y+trace.dy, ld)
		}
		if s1 == s2 {
			return
		}
		frac := interceptVector(trace, divline{x: ld.x1, y: ld.y1, dx: ld.dx, dy: ld.dy})
		if frac < 0 {
			return
		}
		intercepts = append(intercepts, lineAttackIntercept{
			frac:   frac,
			order:  order,
			isLine: true,
			line:   physIdx,
		})
		order++
	}

	var thingSeen []bool
	if g.m != nil {
		thingSeen = make([]bool, len(g.m.Things))
	}
	appendThing := func(i int) {
		if thingSeen == nil || i < 0 || i >= len(thingSeen) || thingSeen[i] {
			return
		}
		thingSeen[i] = true
		target, ok := g.lineAttackThingTargetable(i, actor.targetMask, actor.thingIdx)
		if !ok {
			return
		}
		tx, ty, _, _, radius, _, ok := g.lineAttackTargetState(target)
		if !ok {
			return
		}
		frac, hit := lineAttackThingFrac(trace, tx, ty, radius)
		if !hit {
			return
		}
		intercepts = append(intercepts, lineAttackIntercept{
			frac:   frac,
			order:  order,
			isLine: false,
			target: target,
		})
		order++
	}

	if g.m != nil && g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		const (
			mapBlockShift = fracBits + 7
			mapBToFrac    = 7
		)
		sx := x1
		sy := y1
		ex := x2
		ey := y2
		if ((sx - g.bmapOriginX) & ((1 << mapBlockShift) - 1)) == 0 {
			sx += fracUnit
		}
		if ((sy - g.bmapOriginY) & ((1 << mapBlockShift) - 1)) == 0 {
			sy += fracUnit
		}

		rx1 := sx - g.bmapOriginX
		ry1 := sy - g.bmapOriginY
		rx2 := ex - g.bmapOriginX
		ry2 := ey - g.bmapOriginY

		xt1 := int(rx1 >> mapBlockShift)
		yt1 := int(ry1 >> mapBlockShift)
		xt2 := int(rx2 >> mapBlockShift)
		yt2 := int(ry2 >> mapBlockShift)

		mapxstep, mapystep := 0, 0
		xstep, ystep := int64(256*fracUnit), int64(256*fracUnit)
		partial := int64(fracUnit)

		if xt2 > xt1 {
			mapxstep = 1
			partial = fracUnit - ((rx1 >> mapBToFrac) & (fracUnit - 1))
			ystep = fixedDiv(ry2-ry1, abs(rx2-rx1))
		} else if xt2 < xt1 {
			mapxstep = -1
			partial = (rx1 >> mapBToFrac) & (fracUnit - 1)
			ystep = fixedDiv(ry2-ry1, abs(rx2-rx1))
		}
		yintercept := (ry1 >> mapBToFrac) + fixedMul(partial, ystep)

		if yt2 > yt1 {
			mapystep = 1
			partial = fracUnit - ((ry1 >> mapBToFrac) & (fracUnit - 1))
			xstep = fixedDiv(rx2-rx1, abs(ry2-ry1))
		} else if yt2 < yt1 {
			mapystep = -1
			partial = (ry1 >> mapBToFrac) & (fracUnit - 1)
			xstep = fixedDiv(rx2-rx1, abs(ry2-ry1))
		}
		xintercept := (rx1 >> mapBToFrac) + fixedMul(partial, xstep)

		mapx, mapy := xt1, yt1
		g.validCount++
		for count := 0; count < 64; count++ {
			_ = g.blockLinesIterator(mapx, mapy, func(lineIdx int) bool {
				if lineIdx < 0 || lineIdx >= len(g.physForLine) {
					return true
				}
				appendLine(g.physForLine[lineIdx])
				return true
			})
			if len(g.thingBlockLinks) != g.bmapWidth*g.bmapHeight {
				g.rebuildThingBlockmap()
			}
			if mapx >= 0 && mapy >= 0 && mapx < g.bmapWidth && mapy < g.bmapHeight {
				cell := mapy*g.bmapWidth + mapx
				for i := g.thingBlockLinks[cell]; i >= 0; i = g.thingBlockNext[i] {
					appendThing(i)
				}
			}
			if mapx == xt2 && mapy == yt2 {
				break
			}
			if (yintercept >> fracBits) == int64(mapy) {
				yintercept += ystep
				mapx += mapxstep
			} else if (xintercept >> fracBits) == int64(mapx) {
				xintercept += xstep
				mapy += mapystep
			}
		}
	} else {
		g.validCount++
		for physIdx := range g.lines {
			appendLine(physIdx)
		}
		if g.m != nil {
			for i := range g.m.Things {
				appendThing(i)
			}
		}
	}

	if actor.targetMask&lineAttackMaskPlayer != 0 && !actor.isPlayer && !g.isDead {
		if frac, ok := lineAttackThingFrac(trace, g.p.x, g.p.y, playerRadius); ok {
			intercepts = append(intercepts, lineAttackIntercept{
				frac:   frac,
				order:  order,
				isLine: false,
				target: lineAttackTarget{kind: lineAttackTargetPlayer, idx: -1},
			})
		}
	}

	sort.SliceStable(intercepts, func(i, j int) bool {
		if intercepts[i].frac == intercepts[j].frac {
			return intercepts[i].order < intercepts[j].order
		}
		return intercepts[i].frac < intercepts[j].frac
	})
	return intercepts
}

func (g *game) aimLineAttack(actor lineAttackActor, angle uint32, distance int64) (int64, bool) {
	intercepts := g.collectLineAttackIntercepts(actor, angle, distance)
	topSlope := int64(doomAimTopSlope)
	bottomSlope := int64(doomAimBottomSlope)
	for _, in := range intercepts {
		if in.frac > fracUnit {
			break
		}
		if in.isLine {
			ld := g.lines[in.line]
			if (ld.flags & mlTwoSided) == 0 {
				return 0, false
			}
			opentop, openbottom, _, _ := g.lineOpening(ld)
			if openbottom >= opentop {
				return 0, false
			}
			dist := fixedMul(distance, in.frac)
			if dist <= 0 {
				continue
			}
			front, back := g.physLineSectors(ld)
			if front >= 0 && back >= 0 && g.sectorFloor[front] != g.sectorFloor[back] {
				slope := fixedDiv(openbottom-actor.shootZ, dist)
				if slope > bottomSlope {
					bottomSlope = slope
				}
			}
			if front >= 0 && back >= 0 && g.sectorCeil[front] != g.sectorCeil[back] {
				slope := fixedDiv(opentop-actor.shootZ, dist)
				if slope < topSlope {
					topSlope = slope
				}
			}
			if topSlope <= bottomSlope {
				return 0, false
			}
			continue
		}

		_, _, z, height, _, _, ok := g.lineAttackTargetState(in.target)
		if !ok {
			continue
		}
		dist := fixedMul(distance, in.frac)
		if dist <= 0 {
			continue
		}
		thingTopSlope := fixedDiv(z+height-actor.shootZ, dist)
		if thingTopSlope < bottomSlope {
			continue
		}
		bottomSlopeThing := fixedDiv(z-actor.shootZ, dist)
		if bottomSlopeThing > topSlope {
			continue
		}
		thingTop := thingTopSlope
		if thingTop > topSlope {
			thingTop = topSlope
		}
		thingBottom := bottomSlopeThing
		if thingBottom < bottomSlope {
			thingBottom = bottomSlope
		}
		return (thingTop + thingBottom) / 2, true
	}
	return 0, false
}

func (g *game) shootSpecialLine(lineIdx int, shooterIsPlayer bool) {
	if g == nil || lineIdx < 0 || lineIdx >= len(g.lineSpecial) {
		return
	}
	info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
	if !shooterIsPlayer && info.Special != 46 {
		return
	}
	if g.activateShootLineSpecial(lineIdx, info) && !info.Repeat {
		g.lineSpecial[lineIdx] = 0
	}
}

func (g *game) lineAttackTrace(actor lineAttackActor, angle uint32, distance, slope int64, activateSpecials bool) lineAttackOutcome {
	intercepts := g.collectLineAttackIntercepts(actor, angle, distance)
	trace := divline{x: actor.x, y: actor.y}
	trace.dx = fixedMul(distance, doomFineCosine(angle))
	trace.dy = fixedMul(distance, doomFineSineAtAngle(angle))

	for _, in := range intercepts {
		if in.frac > fracUnit {
			break
		}
		if in.isLine {
			ld := g.lines[in.line]
			if activateSpecials && ld.idx >= 0 && ld.idx < len(g.lineSpecial) && g.lineSpecial[ld.idx] != 0 {
				g.shootSpecialLine(ld.idx, actor.isPlayer)
			}
			hitLine := (ld.flags & mlTwoSided) == 0
			if !hitLine {
				opentop, openbottom, _, openrange := g.lineOpening(ld)
				if openrange <= 0 {
					hitLine = true
				} else {
					dist := fixedMul(distance, in.frac)
					if dist <= 0 {
						continue
					}
					front, back := g.physLineSectors(ld)
					if front >= 0 && back >= 0 && g.sectorFloor[front] != g.sectorFloor[back] {
						openSlope := fixedDiv(openbottom-actor.shootZ, dist)
						if openSlope > slope {
							hitLine = true
						}
					}
					if !hitLine && front >= 0 && back >= 0 && g.sectorCeil[front] != g.sectorCeil[back] {
						openSlope := fixedDiv(opentop-actor.shootZ, dist)
						if openSlope < slope {
							hitLine = true
						}
					}
				}
			}
			if !hitLine {
				continue
			}
			frac := in.frac - fixedDiv(4*fracUnit, distance)
			x := trace.x + fixedMul(trace.dx, frac)
			y := trace.y + fixedMul(trace.dy, frac)
			z := actor.shootZ + fixedMul(slope, fixedMul(frac, distance))
			front, back := g.physLineSectors(ld)
			if g.m != nil && front >= 0 && front < len(g.m.Sectors) && isSkyFlatName(g.m.Sectors[front].CeilingPic) {
				if z > g.sectorCeil[front] {
					return lineAttackOutcome{}
				}
				if back >= 0 && back < len(g.m.Sectors) && isSkyFlatName(g.m.Sectors[back].CeilingPic) {
					return lineAttackOutcome{}
				}
			}
			return lineAttackOutcome{
				lineIdx:   ld.idx,
				dist:      fixedMul(distance, in.frac),
				impactX:   x,
				impactY:   y,
				impactZ:   z,
				spawnPuff: true,
			}
		}

		_, _, z, height, _, noBlood, ok := g.lineAttackTargetState(in.target)
		if !ok {
			continue
		}
		dist := fixedMul(distance, in.frac)
		if dist <= 0 {
			continue
		}
		topSlope := fixedDiv(z+height-actor.shootZ, dist)
		if topSlope < slope {
			continue
		}
		bottomSlope := fixedDiv(z-actor.shootZ, dist)
		if bottomSlope > slope {
			continue
		}
		frac := in.frac - fixedDiv(10*fracUnit, distance)
		x := trace.x + fixedMul(trace.dx, frac)
		y := trace.y + fixedMul(trace.dy, frac)
		iz := actor.shootZ + fixedMul(slope, fixedMul(frac, distance))
		return lineAttackOutcome{
			target:     in.target,
			dist:       dist,
			impactX:    x,
			impactY:    y,
			impactZ:    iz,
			spawnPuff:  noBlood,
			spawnBlood: !noBlood,
		}
	}
	return lineAttackOutcome{}
}

func (g *game) applyLineAttackOutcome(actor lineAttackActor, outcome lineAttackOutcome, damage int) bool {
	if outcome.spawnPuff {
		g.spawnHitscanPuff(outcome.impactX, outcome.impactY, outcome.impactZ)
	}
	if outcome.spawnBlood {
		g.spawnHitscanBlood(outcome.impactX, outcome.impactY, outcome.impactZ, damage)
	}
	switch outcome.target.kind {
	case lineAttackTargetThing:
		if damage > 0 {
			g.damageShootableThingFrom(outcome.target.idx, damage, actor.isPlayer, actor.thingIdx)
		}
		return true
	case lineAttackTargetPlayer:
		if damage > 0 {
			g.damagePlayerFrom(damage, "Monster shot you", actor.x, actor.y, true)
		}
		return true
	default:
		return false
	}
}

func (g *game) bulletSlopeForAim(baseAngle uint32, rng int64) int64 {
	actor := g.playerLineAttackActor()
	if slope, ok := g.aimLineAttack(actor, baseAngle, rng); ok {
		return slope
	}
	if slope, ok := g.aimLineAttack(actor, baseAngle+doomAimFallbackAng, rng); ok {
		return slope
	}
	if slope, ok := g.aimLineAttack(actor, baseAngle-doomAimFallbackAng, rng); ok {
		return slope
	}
	return 0
}

func (g *game) aimSlopeAtAngle(angle uint32, rng int64) (float64, bool) {
	slope, ok := g.aimLineAttack(g.playerLineAttackActor(), angle, rng)
	if !ok {
		return 0, false
	}
	return float64(slope) / float64(fracUnit), true
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

func (g *game) pickHitscanMonsterTargetAtAngleWithSlope(angle uint32, rng int64, radius int64, slope int64, useSlope bool) (int, bool) {
	idx, _, ok := g.pickHitscanMonsterTargetAtAngleWithSlopeDist(angle, rng, radius, slope, useSlope)
	return idx, ok
}

func (g *game) pickHitscanMonsterTargetAtAngleWithSlopeDist(angle uint32, rng int64, radius int64, slope int64, useSlope bool) (int, float64, bool) {
	_ = radius
	traceSlope := slope
	if !useSlope {
		var ok bool
		traceSlope, ok = g.aimLineAttack(g.playerLineAttackActor(), angle, rng)
		if !ok {
			return -1, 0, false
		}
	}
	outcome := g.lineAttackTrace(g.playerLineAttackActor(), angle, rng, traceSlope, false)
	if outcome.target.kind != lineAttackTargetThing {
		return -1, 0, false
	}
	return outcome.target.idx, float64(outcome.dist), true
}

func (g *game) hitscanWallImpactDistance(angle uint32, rng int64, slope int64) (float64, int, bool) {
	return g.hitscanWallImpactDistanceFrom(g.p.x, g.p.y, g.playerShootZ(), angle, rng, slope)
}

func (g *game) hitscanWallImpactDistanceFrom(sx, sy, shootZ int64, angle uint32, rng int64, slope int64) (float64, int, bool) {
	actor := lineAttackActor{
		isPlayer:   false,
		thingIdx:   -1,
		x:          sx,
		y:          sy,
		shootZ:     shootZ,
		targetMask: 0,
	}
	outcome := g.lineAttackTrace(actor, angle, rng, slope, false)
	if outcome.lineIdx < 0 {
		return 0, -1, false
	}
	return float64(outcome.dist), outcome.lineIdx, true
}

func (g *game) spawnHitscanPuffAtDistance(angle uint32, slope, dist int64) {
	g.spawnHitscanPuffFromSource(g.p.x, g.p.y, g.playerShootZ(), angle, slope, dist)
}

func (g *game) spawnHitscanPuffFromSource(sx, sy, shootZ int64, angle uint32, slope, dist int64) {
	if dist <= 0 {
		return
	}
	x := sx + fixedMul(dist, doomFineCosine(angle))
	y := sy + fixedMul(dist, doomFineSineAtAngle(angle))
	z := shootZ + fixedMul(slope, dist)
	// Doom line hits use 4-unit backoff before spawning a puff.
	x -= fixedMul(4*fracUnit, doomFineCosine(angle))
	y -= fixedMul(4*fracUnit, doomFineSineAtAngle(angle))
	z += int64((doomrand.PRandom() - doomrand.PRandom()) << 10)
	g.spawnHitscanPuff(x, y, z)
}

func (g *game) spawnHitscanBloodAtDistance(angle uint32, slope, dist int64, damage int) {
	g.spawnHitscanBloodFromSource(g.p.x, g.p.y, g.playerShootZ(), angle, slope, dist, damage)
}

func (g *game) spawnHitscanBloodFromSource(sx, sy, shootZ int64, angle uint32, slope, dist int64, damage int) {
	if dist <= 0 {
		return
	}
	x := sx + fixedMul(dist, doomFineCosine(angle))
	y := sy + fixedMul(dist, doomFineSineAtAngle(angle))
	z := shootZ + fixedMul(slope, dist)
	// Doom thing hits use 10-unit backoff before spawning blood.
	x -= fixedMul(10*fracUnit, doomFineCosine(angle))
	y -= fixedMul(10*fracUnit, doomFineSineAtAngle(angle))
	z += int64((doomrand.PRandom() - doomrand.PRandom()) << 10)
	g.spawnHitscanBlood(x, y, z, damage)
}

func monsterHitHeight(typ int16) int64 {
	return monsterHeight(typ)
}

func (g *game) damageMonster(thingIdx int, damage int) {
	g.damageMonsterFrom(thingIdx, damage, true, -1)
}

func (g *game) damageMonsterFrom(thingIdx int, damage int, sourcePlayer bool, sourceThing int) {
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
	if thingIdx >= 0 && thingIdx < len(g.thingReactionTics) {
		g.thingReactionTics[thingIdx] = 0
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
		if thingIdx >= 0 && thingIdx < len(g.thingState) && thingIdx < len(g.thingStateTics) {
			g.thingState[thingIdx] = monsterStateDeath
			g.thingStateTics[thingIdx] = g.thingDeathTics[thingIdx]
		}
		if thingIdx >= 0 && thingIdx < len(g.thingStatePhase) {
			g.thingStatePhase[thingIdx] = 0
		}
		deathEv := monsterDeathSoundEvent(thingType)
		deathDelay := monsterDeathSoundDelayTics(thingType)
		tx, ty := g.thingPosFixed(thingIdx, g.m.Things[thingIdx])
		if deathDelay > 0 {
			g.emitSoundEventDelayedAt(deathEv, deathDelay, tx, ty, true)
		} else {
			g.emitSoundEventAt(deathEv, tx, ty)
		}
		g.setHUDMessage("Monster killed", 15)
		g.bonusFlashTic = max(g.bonusFlashTic, 4)
		g.spawnMonsterDrop(thingIdx, thingType)
		if thingType == 71 {
			baseAngle := uint32(0)
			if thingIdx >= 0 && thingIdx < len(g.thingAngleState) {
				baseAngle = g.thingAngleState[thingIdx]
			}
			_ = g.spawnPainLostSoul(thingIdx, baseAngle+degToAngle(90))
			_ = g.spawnPainLostSoul(thingIdx, baseAngle+degToAngle(180))
			_ = g.spawnPainLostSoul(thingIdx, baseAngle+degToAngle(270))
		}
		g.handleBossDeath(thingIdx, thingType)
	} else {
		if thingIdx >= 0 && thingIdx < len(g.thingPainTics) {
			chance := monsterPainChance(thingType)
			if chance > 0 && (chance >= 256 || doomrand.PRandom() < chance) {
				wasInPain := g.thingPainTics[thingIdx] > 0
				if thingIdx >= 0 && thingIdx < len(g.thingJustHit) {
					// Doom only marks JUSTHIT when the pain state triggers.
					g.thingJustHit[thingIdx] = true
				}
				g.thingPainTics[thingIdx] = max(g.thingPainTics[thingIdx], monsterPainDurationTics(thingType))
				if !wasInPain {
					tx, ty := g.thingPosFixed(thingIdx, g.m.Things[thingIdx])
					g.emitSoundEventAt(monsterPainSoundEvent(thingType), tx, ty)
				}
				if thingIdx >= 0 && thingIdx < len(g.thingState) && thingIdx < len(g.thingStateTics) {
					g.thingState[thingIdx] = monsterStatePain
					g.thingStateTics[thingIdx] = g.thingPainTics[thingIdx]
				}
				if thingIdx >= 0 && thingIdx < len(g.thingStatePhase) {
					g.thingStatePhase[thingIdx] = 0
				}
			}
		}
		g.maybeRetargetMonsterAfterDamage(thingIdx, thingType, sourcePlayer, sourceThing)
		g.setHUDMessage("Hit", 8)
	}
}

func (g *game) maybeRetargetMonsterAfterDamage(thingIdx int, thingType int16, sourcePlayer bool, sourceThing int) {
	if g == nil || g.m == nil || thingIdx < 0 || thingIdx >= len(g.m.Things) {
		return
	}
	if thingIdx >= len(g.thingThreshold) {
		return
	}
	if thingType != 64 && g.thingThreshold[thingIdx] > 0 {
		return
	}
	if sourcePlayer {
		g.setMonsterTargetPlayer(thingIdx)
		g.thingThreshold[thingIdx] = monsterBaseThreshold
		return
	}
	if sourceThing < 0 || sourceThing == thingIdx || sourceThing >= len(g.m.Things) {
		return
	}
	if g.m.Things[sourceThing].Type == 64 {
		return
	}
	if sourceThing >= len(g.thingHP) || g.thingHP[sourceThing] <= 0 {
		return
	}
	g.setMonsterTargetThing(thingIdx, sourceThing)
	g.thingThreshold[thingIdx] = monsterBaseThreshold
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
	g.thingAngleState = append(g.thingAngleState, thingDegToWorldAngle(th.Angle))
	g.thingZState = append(g.thingZState, 0)
	g.thingFloorState = append(g.thingFloorState, 0)
	g.thingCeilState = append(g.thingCeilState, 0)
	g.thingSupportValid = append(g.thingSupportValid, false)
	g.thingBlockCell = append(g.thingBlockCell, -1)
	g.thingBlockNext = append(g.thingBlockNext, -1)
	g.thingHP = append(g.thingHP, 0)
	g.thingAggro = append(g.thingAggro, false)
	g.thingTargetPlayer = append(g.thingTargetPlayer, false)
	g.thingTargetIdx = append(g.thingTargetIdx, -1)
	g.thingThreshold = append(g.thingThreshold, 0)
	g.thingCooldown = append(g.thingCooldown, 0)
	g.thingMoveDir = append(g.thingMoveDir, 0)
	g.thingMoveCount = append(g.thingMoveCount, 0)
	g.thingJustAtk = append(g.thingJustAtk, false)
	g.thingJustHit = append(g.thingJustHit, false)
	g.thingReactionTics = append(g.thingReactionTics, 0)
	g.thingWakeTics = append(g.thingWakeTics, 0)
	g.thingLastLook = append(g.thingLastLook, 0)
	g.thingDead = append(g.thingDead, false)
	g.thingDeathTics = append(g.thingDeathTics, 0)
	g.thingAttackTics = append(g.thingAttackTics, 0)
	g.thingAttackPhase = append(g.thingAttackPhase, 0)
	g.thingAttackFireTics = append(g.thingAttackFireTics, -1)
	g.thingPainTics = append(g.thingPainTics, 0)
	g.thingThinkWait = append(g.thingThinkWait, 0)
	g.thingState = append(g.thingState, monsterStateSpawn)
	g.thingStateTics = append(g.thingStateTics, 0)
	g.thingStatePhase = append(g.thingStatePhase, 0)
	g.thingWorldAnimRef = append(g.thingWorldAnimRef, g.buildThingWorldAnimRef(th))
	sec := -1
	sec = g.sectorAt(x, y)
	g.thingSectorCache = append(g.thingSectorCache, sec)
	if sec >= 0 && sec < len(g.sectorFloor) {
		g.thingFloorState[len(g.m.Things)-1] = g.sectorFloor[sec]
		g.thingZState[len(g.m.Things)-1] = g.thingFloorState[len(g.m.Things)-1]
	}
	if sec >= 0 && sec < len(g.sectorCeil) {
		g.thingCeilState[len(g.m.Things)-1] = g.sectorCeil[sec]
	}
	if sec >= 0 {
		g.thingSupportValid[len(g.m.Things)-1] = true
	}
	g.updateThingBlockmapIndex(len(g.m.Things) - 1)
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
	case 88:
		return soundEventBossBrainPain
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
	case 88:
		return soundEventBossBrainDeath
	case 3004:
		return soundEventDeathZombie
	case 9:
		return soundEventDeathShotgunGuy
	case 65:
		return soundEventDeathChaingunner
	case 3001:
		return soundEventDeathImp
	case 3002, 58:
		return soundEventDeathDemon
	case 3005:
		return soundEventDeathCaco
	case 3003:
		return soundEventDeathBaron
	case 69:
		return soundEventDeathKnight
	case 16:
		return soundEventDeathCyber
	case 7:
		return soundEventDeathSpider
	case 68:
		return soundEventDeathArachnotron
	case 3006:
		return soundEventDeathLostSoul
	case 67:
		return soundEventDeathMancubus
	case 66:
		return soundEventDeathRevenant
	case 71:
		return soundEventDeathPainElemental
	case 84:
		return soundEventDeathWolfSS
	case 64:
		return soundEventDeathArchvile
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

func (g *game) queueWeaponSwitch(id weaponID) bool {
	g.ensureWeaponDefaults()
	if id == 0 || id == g.inventory.ReadyWeapon {
		g.inventory.PendingWeapon = 0
		return false
	}
	if !g.weaponOwned(id) {
		return false
	}
	g.inventory.PendingWeapon = id
	if g.weaponState == weaponStateNone {
		g.bringUpWeapon()
		return true
	}
	return true
}

func (g *game) applyPendingWeapon() bool {
	next := g.inventory.PendingWeapon
	if next == 0 {
		return false
	}
	if next == g.inventory.ReadyWeapon {
		g.inventory.PendingWeapon = 0
		return false
	}
	g.weaponRefire = false
	g.weaponAttackDown = false
	g.clearWeaponOverlay()
	g.inventory.ReadyWeapon = next
	g.inventory.PendingWeapon = 0
	g.bringUpWeapon()
	return true
}

func (g *game) ensureWeaponHasAmmo() {
	if g.canFireSelectedWeapon() {
		return
	}
	switchTo := func(id weaponID) bool {
		queued := g.queueWeaponSwitch(id)
		if queued && g.weaponState != weaponStateNone {
			g.setWeaponPSpriteState(weaponInfo(g.inventory.ReadyWeapon).downstate, false)
		}
		return queued
	}
	if g.weaponOwned(weaponPlasma) && weaponAmmoCount(g.stats, ammoKindCells) >= 1 {
		switchTo(weaponPlasma)
		return
	}
	if g.weaponOwned(weaponSuperShotgun) && weaponAmmoCount(g.stats, ammoKindShells) > 2 {
		switchTo(weaponSuperShotgun)
		return
	}
	if g.weaponOwned(weaponChaingun) && weaponAmmoCount(g.stats, ammoKindBullets) >= 1 {
		switchTo(weaponChaingun)
		return
	}
	if g.weaponOwned(weaponShotgun) && weaponAmmoCount(g.stats, ammoKindShells) >= 1 {
		switchTo(weaponShotgun)
		return
	}
	if weaponAmmoCount(g.stats, ammoKindBullets) >= 1 {
		switchTo(weaponPistol)
		return
	}
	if g.weaponOwned(weaponChainsaw) {
		switchTo(weaponChainsaw)
		return
	}
	if g.weaponOwned(weaponRocketLauncher) && weaponAmmoCount(g.stats, ammoKindRockets) >= 1 {
		switchTo(weaponRocketLauncher)
		return
	}
	if g.weaponOwned(weaponBFG) && weaponAmmoCount(g.stats, ammoKindCells) > 40 {
		switchTo(weaponBFG)
		return
	}
	switchTo(weaponFist)
}

func (g *game) canFireSelectedWeapon() bool {
	info := weaponInfo(g.inventory.ReadyWeapon)
	if info.ammo == ammoKindNone {
		return true
	}
	return weaponAmmoCount(g.stats, info.ammo) >= info.minAmmo
}

func (g *game) selectWeaponSlot(slot int) {
	g.ensureWeaponDefaults()
	next := g.inventory.ReadyWeapon
	switch slot {
	case 1:
		if g.inventory.Weapons[2005] && !(g.inventory.ReadyWeapon == weaponChainsaw && g.inventory.Strength) {
			next = weaponChainsaw
		} else {
			next = weaponFist
		}
	case 2:
		next = weaponPistol
	case 3:
		if g.weaponOwned(weaponSuperShotgun) && g.inventory.ReadyWeapon == weaponShotgun {
			next = weaponSuperShotgun
		} else if g.weaponOwned(weaponShotgun) {
			next = weaponShotgun
		} else if g.weaponOwned(weaponSuperShotgun) {
			next = weaponSuperShotgun
		}
	case 4:
		if g.weaponOwned(weaponChaingun) {
			next = weaponChaingun
		}
	case 5:
		if g.weaponOwned(weaponRocketLauncher) {
			next = weaponRocketLauncher
		}
	case 6:
		if g.weaponOwned(weaponPlasma) {
			next = weaponPlasma
		}
	case 7:
		if g.weaponOwned(weaponBFG) {
			next = weaponBFG
		}
	}
	g.queueWeaponSwitch(next)
}

func (g *game) weaponOwned(id weaponID) bool {
	switch id {
	case weaponFist:
		return true
	case weaponPistol:
		return true
	case weaponShotgun:
		return g.inventory.Weapons[2001]
	case weaponSuperShotgun:
		return g.inventory.Weapons[82] && g.isCommercialWeaponSet()
	case weaponChaingun:
		return g.inventory.Weapons[2002]
	case weaponRocketLauncher:
		return g.inventory.Weapons[2003]
	case weaponPlasma:
		return g.inventory.Weapons[2004] && g.isCommercialWeaponSet()
	case weaponBFG:
		return g.inventory.Weapons[2006] && g.isCommercialWeaponSet()
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
		weaponSuperShotgun,
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
		g.queueWeaponSwitch(next)
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
	case weaponSuperShotgun:
		return "supershotgun"
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

func (g *game) isCommercialWeaponSet() bool {
	if g == nil || g.m == nil {
		return true
	}
	name := strings.ToUpper(strings.TrimSpace(string(g.m.Name)))
	return strings.HasPrefix(name, "MAP")
}

func (g *game) handleBossDeath(thingIdx int, thingType int16) {
	if g == nil || g.m == nil || !g.monsterTargetAlive() {
		return
	}
	name := strings.ToUpper(strings.TrimSpace(string(g.m.Name)))
	if name == "" {
		return
	}
	if thingType == 88 {
		g.requestLevelExit(false, "Boss brain destroyed")
		return
	}
	for i, th := range g.m.Things {
		if i == thingIdx || th.Type != thingType {
			continue
		}
		if i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if i < len(g.thingHP) && g.thingHP[i] > 0 {
			return
		}
	}
	if strings.HasPrefix(name, "MAP") {
		if name != "MAP07" {
			return
		}
		switch thingType {
		case 67:
			_ = g.activateTaggedFloor(666, mapdata.FloorLowerToLowest)
		case 68:
			_ = g.activateTaggedFloor(667, mapdata.FloorRaiseToTexture)
		}
		return
	}
	episode, slot, ok := episodeMapSlot(mapdata.MapName(name))
	if !ok {
		return
	}
	switch {
	case episode == 1 && slot == 8 && thingType == 3003:
		_ = g.activateTaggedFloor(666, mapdata.FloorLowerToLowest)
	case episode == 2 && slot == 8 && thingType == 16:
		g.requestLevelExit(false, "Boss death exit")
	case episode == 3 && slot == 8 && thingType == 7:
		g.requestLevelExit(false, "Boss death exit")
	case episode == 4 && slot == 6 && thingType == 16:
		_ = g.activateTaggedDoor(666, mapdata.DoorBlazeOpen)
	case episode == 4 && slot == 8 && thingType == 7:
		_ = g.activateTaggedFloor(666, mapdata.FloorLowerToLowest)
	}
}

func angleToThing(sx, sy, tx, ty int64) uint32 {
	ang := math.Atan2(float64(ty-sy), float64(tx-sx))
	if ang < 0 {
		ang += 2 * math.Pi
	}
	return uint32(ang * (4294967296.0 / (2 * math.Pi)))
}

func turnTowardAngle(cur, want, step uint32) uint32 {
	if cur == want || step == 0 {
		return want
	}
	diff := want - cur
	if diff == 0 {
		return want
	}
	if diff < 0x80000000 {
		if diff < step {
			return want
		}
		return cur + step
	}
	if ^diff+1 < step {
		return want
	}
	return cur - step
}

func turnTowardChainsawTarget(cur, want uint32) uint32 {
	const (
		chainsawTurnStep = doomAng90 / 20
		chainsawTurnSnap = doomAng90 / 21
	)
	diff := want - cur
	if diff > doomAng180 {
		if diff < ^uint32(chainsawTurnStep-1) {
			return want + chainsawTurnSnap
		}
		return cur - chainsawTurnStep
	}
	if diff > chainsawTurnStep {
		return want - chainsawTurnSnap
	}
	return cur + chainsawTurnStep
}
