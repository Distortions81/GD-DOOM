package automap

import (
	"math"
	"sort"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

const (
	monsterWakeRange   = 1024 * fracUnit
	monsterMeleeRange  = 64 * fracUnit
	monsterAttackRange = 1024 * fracUnit
	monsterAttackTics  = 35

	monsterDiagFrac = 47000
)

type monsterMoveDir uint8

const (
	monsterDirEast monsterMoveDir = iota
	monsterDirNorthEast
	monsterDirNorth
	monsterDirNorthWest
	monsterDirWest
	monsterDirSouthWest
	monsterDirSouth
	monsterDirSouthEast
	monsterDirNoDir
)

var (
	monsterOpposite = [9]monsterMoveDir{
		monsterDirWest,
		monsterDirSouthWest,
		monsterDirSouth,
		monsterDirSouthEast,
		monsterDirEast,
		monsterDirNorthEast,
		monsterDirNorth,
		monsterDirNorthWest,
		monsterDirNoDir,
	}
	monsterDiags = [4]monsterMoveDir{
		monsterDirNorthWest,
		monsterDirNorthEast,
		monsterDirSouthWest,
		monsterDirSouthEast,
	}
	monsterXSpeed = [8]int64{
		fracUnit,
		monsterDiagFrac,
		0,
		-monsterDiagFrac,
		-fracUnit,
		-monsterDiagFrac,
		0,
		monsterDiagFrac,
	}
	monsterYSpeed = [8]int64{
		0,
		monsterDiagFrac,
		fracUnit,
		monsterDiagFrac,
		0,
		-monsterDiagFrac,
		-fracUnit,
		-monsterDiagFrac,
	}
)

func (g *game) tickMonsters() {
	if g.m == nil {
		return
	}
	g.ensureMonsterAIState()
	px := g.p.x
	py := g.p.y
	for i, th := range g.m.Things {
		if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
			continue
		}
		if i >= 0 && i < len(g.thingDead) && g.thingDead[i] && i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0 {
			g.thingDeathTics[i]--
		}
		if !isMonster(th.Type) || g.thingHP[i] <= 0 {
			continue
		}
		tx, ty := g.thingPosFixed(i, th)
		dx := px - tx
		dy := py - ty
		dist := hypotFixed(dx, dy)

		if i >= 0 && i < len(g.thingAttackTics) && g.thingAttackTics[i] > 0 {
			g.thingAttackTics[i]--
		}
		if i >= 0 && i < len(g.thingAttackFireTics) && g.thingAttackFireTics[i] >= 0 {
			if g.thingAttackFireTics[i] > 0 {
				g.thingAttackFireTics[i]--
			}
			if g.thingAttackFireTics[i] == 0 {
				g.faceMonsterToward(i, tx, ty, px, py)
				_ = g.monsterAttack(i, th.Type, dist)
				g.thingAttackFireTics[i] = -1
			}
		}
		if i >= 0 && i < len(g.thingPainTics) && g.thingPainTics[i] > 0 {
			g.thingPainTics[i]--
		}
		if i >= 0 && i < len(g.thingPainTics) && g.thingPainTics[i] > 0 {
			if i >= 0 && i < len(g.thingAttackFireTics) {
				g.thingAttackFireTics[i] = -1
			}
			continue
		}
		if i >= 0 && i < len(g.thingAttackTics) && g.thingAttackTics[i] > 0 {
			continue
		}

		if !g.thingAggro[i] {
			heardPlayer := g.monsterHeardPlayer(i, tx, ty)
			if heardPlayer {
				g.thingAggro[i] = true
				g.emitMonsterSeeSound(i, th.Type, tx, ty)
			} else if dist <= monsterWakeRange && g.monsterHasLOSPlayer(th.Type, tx, ty) {
				g.thingAggro[i] = true
				g.emitMonsterSeeSound(i, th.Type, tx, ty)
			} else {
				continue
			}
		}
		if !g.monsterChaseReady(i, th.Type) {
			continue
		}
		if i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
			g.thingReactionTics[i]--
		}

		// Doom A_Chase: prevent consecutive missile attacks.
		if g.thingJustAtk[i] {
			g.thingJustAtk[i] = false
			g.monsterPickNewChaseDir(i, th.Type, px, py)
			continue
		}

		if g.monsterCanMelee(th.Type, dist, tx, ty, px, py) {
			g.faceMonsterToward(i, tx, ty, px, py)
			if g.startMonsterAttackState(i, th.Type, false) {
				continue
			}
		}

		if g.monsterCanTryMissileNow(i) && g.monsterCheckMissileRange(i, th.Type, dist, tx, ty, px, py) {
			g.faceMonsterToward(i, tx, ty, px, py)
			if g.startMonsterAttackState(i, th.Type, true) {
				continue
			}
		}

		g.thingMoveCount[i]--
		if g.thingMoveCount[i] < 0 || !g.monsterMoveInDir(i, th.Type, g.thingMoveDir[i]) {
			g.monsterPickNewChaseDir(i, th.Type, px, py)
		}
		g.emitMonsterActiveSound(i, th.Type, tx, ty)
	}
}

func (g *game) emitMonsterSeeSound(i int, typ int16, x, y int64) {
	ev, fullVolume := monsterSeeSoundEvent(typ)
	if ev < 0 {
		return
	}
	if fullVolume {
		g.emitSoundEvent(ev)
		return
	}
	g.emitSoundEventAt(ev, x, y)
}

func (g *game) emitMonsterActiveSound(i int, typ int16, x, y int64) {
	ev := monsterActiveSoundEvent(typ)
	if ev < 0 {
		return
	}
	if !shouldEmitMonsterActiveSound(doomrand.PRandom()) {
		return
	}
	g.emitSoundEventAt(ev, x, y)
}

func shouldEmitMonsterActiveSound(r int) bool {
	return r >= 0 && r < 3
}

func monsterSeeSoundEvent(typ int16) (soundEvent, bool) {
	switch typ {
	case 3004, 9, 65:
		return soundEventMonsterSeePosit, false
	case 3001:
		return soundEventMonsterSeeImp, false
	case 3002, 58:
		return soundEventMonsterSeeDemon, false
	case 3005:
		return soundEventMonsterSeeCaco, false
	case 3003:
		return soundEventMonsterSeeBaron, false
	case 69:
		return soundEventMonsterSeeKnight, false
	case 7:
		return soundEventMonsterSeeSpider, true
	case 68:
		return soundEventMonsterSeeArachnotron, false
	case 16:
		return soundEventMonsterSeeCyber, true
	case 71:
		return soundEventMonsterSeePainElemental, false
	case 84:
		return soundEventMonsterSeeWolfSS, false
	case 64:
		return soundEventMonsterSeeArchvile, false
	case 66:
		return soundEventMonsterSeeRevenant, false
	default:
		return -1, false
	}
}

func monsterActiveSoundEvent(typ int16) soundEvent {
	switch typ {
	case 3004, 9, 65, 84:
		return soundEventMonsterActivePosit
	case 3001:
		return soundEventMonsterActiveImp
	case 3002, 58, 3005, 3003, 69, 3006, 7, 16, 71, 67:
		return soundEventMonsterActiveDemon
	case 68:
		return soundEventMonsterActiveArachnotron
	case 64:
		return soundEventMonsterActiveArchvile
	case 66:
		return soundEventMonsterActiveRevenant
	default:
		return -1
	}
}

func (g *game) ensureMonsterAIState() {
	if g.m == nil {
		return
	}
	n := len(g.m.Things)
	if len(g.thingMoveDir) != n {
		old := g.thingMoveDir
		g.thingMoveDir = make([]monsterMoveDir, n)
		for i := range g.thingMoveDir {
			g.thingMoveDir[i] = monsterDirNoDir
		}
		copy(g.thingMoveDir, old)
	}
	if len(g.thingMoveCount) != n {
		old := g.thingMoveCount
		g.thingMoveCount = make([]int, n)
		copy(g.thingMoveCount, old)
	}
	if len(g.thingJustAtk) != n {
		old := g.thingJustAtk
		g.thingJustAtk = make([]bool, n)
		copy(g.thingJustAtk, old)
	}
	if len(g.thingJustHit) != n {
		old := g.thingJustHit
		g.thingJustHit = make([]bool, n)
		copy(g.thingJustHit, old)
	}
	if len(g.thingReactionTics) != n {
		old := g.thingReactionTics
		g.thingReactionTics = make([]int, n)
		copy(g.thingReactionTics, old)
	}
	if len(g.thingDead) != n {
		old := g.thingDead
		g.thingDead = make([]bool, n)
		copy(g.thingDead, old)
	}
	if len(g.thingDropped) != n {
		old := g.thingDropped
		g.thingDropped = make([]bool, n)
		copy(g.thingDropped, old)
	}
	if len(g.thingX) != n {
		old := g.thingX
		g.thingX = make([]int64, n)
		copy(g.thingX, old)
		for i := len(old); i < n; i++ {
			g.thingX[i] = int64(g.m.Things[i].X) << fracBits
		}
	}
	if len(g.thingY) != n {
		old := g.thingY
		g.thingY = make([]int64, n)
		copy(g.thingY, old)
		for i := len(old); i < n; i++ {
			g.thingY[i] = int64(g.m.Things[i].Y) << fracBits
		}
	}
	if len(g.thingDeathTics) != n {
		old := g.thingDeathTics
		g.thingDeathTics = make([]int, n)
		copy(g.thingDeathTics, old)
	}
	if len(g.thingAttackTics) != n {
		old := g.thingAttackTics
		g.thingAttackTics = make([]int, n)
		copy(g.thingAttackTics, old)
	}
	if len(g.thingAttackFireTics) != n {
		old := g.thingAttackFireTics
		g.thingAttackFireTics = make([]int, n)
		for i := range g.thingAttackFireTics {
			g.thingAttackFireTics[i] = -1
		}
		copy(g.thingAttackFireTics, old)
	}
	if len(g.thingPainTics) != n {
		old := g.thingPainTics
		g.thingPainTics = make([]int, n)
		copy(g.thingPainTics, old)
	}
	if len(g.thingThinkWait) != n {
		old := g.thingThinkWait
		g.thingThinkWait = make([]int, n)
		copy(g.thingThinkWait, old)
	}
}

func monsterPainChance(typ int16) int {
	switch typ {
	case 3004: // zombieman
		return 200
	case 9: // shotgun guy
		return 170
	case 3001: // imp
		return 200
	case 3002, 58: // demon/spectre
		return 180
	case 3006: // lost soul
		return 256
	case 3005: // cacodemon
		return 128
	case 3003: // baron
		return 50
	case 16: // cyberdemon
		return 20
	case 7: // spider mastermind
		return 40
	default:
		return 100
	}
}

func monsterPainDurationTics(typ int16) int {
	switch typ {
	case 16:
		return 10
	case 7:
		return 8
	default:
		return 6
	}
}

func (g *game) startMonsterAttackAnim(i int, typ int16) {
	if i < 0 || i >= len(g.thingAttackTics) {
		return
	}
	total := monsterAttackStateTotalTics(typ)
	if total <= 0 {
		total = monsterAttackAnimTotalTics(typ)
	}
	if total <= 0 {
		g.thingAttackTics[i] = 0
		return
	}
	g.thingAttackTics[i] = total
}

func (g *game) startMonsterAttackState(i int, typ int16, missile bool) bool {
	if i < 0 || g.m == nil || i >= len(g.m.Things) {
		return false
	}
	g.startMonsterAttackAnim(i, typ)
	if i < 0 || i >= len(g.thingAttackFireTics) {
		// Fallback for malformed state in tests.
		tx := int64(g.m.Things[i].X) << fracBits
		ty := int64(g.m.Things[i].Y) << fracBits
		dist := hypotFixed(g.p.x-tx, g.p.y-ty)
		return g.monsterAttack(i, typ, dist)
	}
	delay := monsterAttackFireDelayTics(typ)
	g.thingAttackFireTics[i] = delay
	if delay <= 0 {
		tx := int64(g.m.Things[i].X) << fracBits
		ty := int64(g.m.Things[i].Y) << fracBits
		dist := hypotFixed(g.p.x-tx, g.p.y-ty)
		if !g.monsterAttack(i, typ, dist) {
			g.thingAttackTics[i] = 0
			g.thingAttackFireTics[i] = -1
			return false
		}
		g.thingAttackFireTics[i] = -1
	}
	if missile && i >= 0 && i < len(g.thingJustAtk) {
		g.thingJustAtk[i] = true
	}
	return true
}

func (g *game) monsterCanTryMissileNow(i int) bool {
	// Doom A_Chase gate: in non-fast modes, missile attacks only when movecount is 0.
	if g.fastMonstersActive() {
		return true
	}
	if i < 0 || i >= len(g.thingMoveCount) {
		return true
	}
	// Match Doom's `if (actor->movecount) goto nomissile;`
	// Any non-zero value (including negative) blocks missile attacks.
	return g.thingMoveCount[i] == 0
}

func monsterAttackFireDelayTics(typ int16) int {
	switch typ {
	case 3004: // zombieman
		return 10
	case 9: // sergeant
		return 16
	case 3001: // imp
		return 16
	case 3002, 58: // demon/spectre
		return 16
	case 3005: // cacodemon
		return 10
	case 3003, 69: // baron/knight
		return 16
	case 16: // cyberdemon
		return 6
	case 7: // spider mastermind
		return 20
	case 3006: // lost soul
		return 0
	default:
		return 0
	}
}

func monsterAttackStateTotalTics(typ int16) int {
	switch typ {
	case 3004: // zombieman
		return 26
	case 9: // sergeant
		return 24
	case 3001: // imp
		return 22
	case 3002, 58: // demon/spectre
		return 24
	case 3005: // cacodemon
		return 15
	case 3003, 69: // baron/knight
		return 24
	case 16: // cyberdemon
		return 66
	case 7: // spider mastermind (single volley cycle)
		return 29
	case 3006: // lost soul
		return 10
	default:
		return 0
	}
}

func (g *game) monsterChaseReady(i int, typ int16) bool {
	if i < 0 || i >= len(g.thingThinkWait) {
		return true
	}
	if g.thingThinkWait[i] > 0 {
		g.thingThinkWait[i]--
		return false
	}
	wait := monsterThinkInterval(typ, g.fastMonstersActive())
	if wait < 1 {
		wait = 1
	}
	g.thingThinkWait[i] = wait - 1
	return true
}

func monsterThinkInterval(typ int16, fast bool) int {
	// Matches Doom run-state tics for common monsters (A_Chase cadence).
	switch typ {
	case 3004, 9, 84, 67:
		if fast {
			return 2
		}
		return 4
	case 3002, 58, 64, 66:
		return 2
	case 3006:
		return 6
	case 65, 3001, 3003, 3005, 7, 16, 68, 69, 71:
		return 3
	default:
		return 3
	}
}

func monsterReactionTimeTics(typ int16) int {
	switch typ {
	case 3004, 9, 3001, 3002, 3006, 3005, 3003, 16, 7, 58, 64, 65, 66, 67, 68, 69, 71, 84:
		return 8
	default:
		return 0
	}
}

func (g *game) monsterCanMelee(typ int16, dist, tx, ty, px, py int64) bool {
	if !monsterHasMeleeAttack(typ) {
		return false
	}
	if dist >= monsterMeleeRange-20*fracUnit+playerRadius {
		return false
	}
	return g.monsterHasLOSPlayer(typ, tx, ty)
}

func (g *game) monsterCheckMissileRange(i int, typ int16, dist, tx, ty, px, py int64) bool {
	if isMeleeOnlyMonster(typ) {
		return false
	}
	if !g.monsterHasLOSPlayer(typ, tx, ty) {
		return false
	}
	if i >= 0 && i < len(g.thingJustHit) && g.thingJustHit[i] {
		g.thingJustHit[i] = false
		return true
	}
	if i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
		return false
	}

	d := int((dist - 64*fracUnit) >> fracBits)
	if !monsterHasMeleeAttack(typ) {
		d -= 128
	}

	switch typ {
	case 64: // archvile
		if d > 14*64 {
			return false
		}
	case 66: // revenant
		if d < 196 {
			return false
		}
		d >>= 1
	}

	if typ == 16 || typ == 7 || typ == 3006 {
		d >>= 1
	}
	if d < 0 {
		d = 0
	}
	if d > 200 {
		d = 200
	}
	if typ == 16 && d > 160 {
		d = 160
	}
	return doomrand.PRandom() >= d
}

func (g *game) monsterPickNewChaseDir(i int, typ int16, targetX, targetY int64) {
	if g.m == nil || i < 0 || i >= len(g.m.Things) || i >= len(g.thingMoveDir) {
		return
	}
	tx := int64(g.m.Things[i].X) << fracBits
	ty := int64(g.m.Things[i].Y) << fracBits
	olddir := g.thingMoveDir[i]
	if olddir > monsterDirNoDir {
		olddir = monsterDirNoDir
	}
	turnaround := monsterOpposite[olddir]

	deltax := targetX - tx
	deltay := targetY - ty

	d1 := monsterDirNoDir
	d2 := monsterDirNoDir
	if deltax > 10*fracUnit {
		d1 = monsterDirEast
	} else if deltax < -10*fracUnit {
		d1 = monsterDirWest
	}
	if deltay < -10*fracUnit {
		d2 = monsterDirSouth
	} else if deltay > 10*fracUnit {
		d2 = monsterDirNorth
	}

	if d1 != monsterDirNoDir && d2 != monsterDirNoDir {
		diag := monsterDiags[(b2i(deltay < 0)<<1)+b2i(deltax > 0)]
		if diag != turnaround && g.monsterTryWalk(i, typ, diag) {
			return
		}
	}

	if doomrand.PRandom() > 200 || abs(deltay) > abs(deltax) {
		d1, d2 = d2, d1
	}

	if d1 == turnaround {
		d1 = monsterDirNoDir
	}
	if d2 == turnaround {
		d2 = monsterDirNoDir
	}

	if d1 != monsterDirNoDir && g.monsterTryWalk(i, typ, d1) {
		return
	}
	if d2 != monsterDirNoDir && g.monsterTryWalk(i, typ, d2) {
		return
	}

	if olddir != monsterDirNoDir && g.monsterTryWalk(i, typ, olddir) {
		return
	}

	if (doomrand.PRandom() & 1) != 0 {
		for dir := int(monsterDirEast); dir <= int(monsterDirSouthEast); dir++ {
			d := monsterMoveDir(dir)
			if d != turnaround && g.monsterTryWalk(i, typ, d) {
				return
			}
		}
	} else {
		for dir := int(monsterDirSouthEast); dir >= int(monsterDirEast); dir-- {
			d := monsterMoveDir(dir)
			if d != turnaround && g.monsterTryWalk(i, typ, d) {
				return
			}
		}
	}

	if turnaround != monsterDirNoDir && g.monsterTryWalk(i, typ, turnaround) {
		return
	}
	g.thingMoveDir[i] = monsterDirNoDir
}

func (g *game) monsterTryWalk(i int, typ int16, dir monsterMoveDir) bool {
	if i < 0 || i >= len(g.thingMoveDir) {
		return false
	}
	g.thingMoveDir[i] = dir
	if !g.monsterMoveInDir(i, typ, dir) {
		return false
	}
	if i >= 0 && i < len(g.thingMoveCount) {
		g.thingMoveCount[i] = doomrand.PRandom() & 15
	}
	return true
}

func (g *game) monsterMoveInDir(i int, typ int16, dir monsterMoveDir) bool {
	if g.m == nil || i < 0 || i >= len(g.m.Things) {
		return false
	}
	if dir >= monsterDirNoDir {
		return false
	}
	step := monsterMoveStep(typ, g.fastMonstersActive())
	dx := fixedMul(step, monsterXSpeed[dir])
	dy := fixedMul(step, monsterYSpeed[dir])
	if dx == 0 && dy == 0 {
		return false
	}

	x, y := g.thingPosFixed(i, g.m.Things[i])
	nx := x + dx
	ny := y + dy
	if !g.tryMoveProbeMonster(i, typ, nx, ny) {
		return g.monsterUseBlockingSpecialLines(i, nx, ny)
	}
	prevX, prevY := x, y
	g.setThingPosFixed(i, nx, ny)
	g.checkWalkSpecialLinesForActor(prevX, prevY, nx, ny, false)
	g.faceMonsterMoveDir(i, dir)
	return true
}

func (g *game) faceMonsterMoveDir(i int, dir monsterMoveDir) {
	if g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	g.m.Things[i].Angle = monsterDirAngle(dir)
}

func monsterDirAngle(dir monsterMoveDir) int16 {
	switch dir {
	case monsterDirEast:
		return 0
	case monsterDirNorthEast:
		return 45
	case monsterDirNorth:
		return 90
	case monsterDirNorthWest:
		return 135
	case monsterDirWest:
		return 180
	case monsterDirSouthWest:
		return 225
	case monsterDirSouth:
		return 270
	case monsterDirSouthEast:
		return 315
	default:
		return 0
	}
}

func (g *game) monsterAttack(i int, typ int16, dist int64) bool {
	meleeOnly := isMeleeOnlyMonster(typ)
	var sx, sy int64
	if i >= 0 && g.m != nil && i < len(g.m.Things) {
		sx, sy = g.thingPosFixed(i, g.m.Things[i])
	}
	if dist <= monsterMeleeRange && monsterHasMeleeAttack(typ) {
		damage := monsterMeleeDamage(typ)
		if damage > 0 {
			if ev := monsterMeleeAttackSoundEvent(typ); ev >= 0 {
				g.emitSoundEventAt(ev, sx, sy)
			}
			g.damagePlayerFrom(damage, "Monster hit you", sx, sy, true)
			return true
		}
	}
	if meleeOnly {
		return false
	}
	if typ == 3004 {
		// Zombieman: single bullet with Doom-style spread and chance to miss.
		g.emitSoundEventAt(soundEventShootPistol, sx, sy)
		g.monsterHitscanAttack(sx, sy, 1)
		return true
	}
	if typ == 9 {
		// Sergeant: 3 pellets.
		g.emitSoundEventAt(soundEventShootShotgun, sx, sy)
		g.monsterHitscanAttack(sx, sy, 3)
		return true
	}
	if usesMonsterProjectile(typ) {
		if g.spawnMonsterProjectile(i, typ) {
			g.emitSoundEventAt(projectileLaunchSoundEvent(typ), sx, sy)
			return true
		}
		return false
	}
	damage := monsterRangedDamage(typ)
	if damage <= 0 {
		return false
	}
	g.emitSoundEventAt(soundEventShootPistol, sx, sy)
	g.damagePlayerFrom(damage, "Monster shot you", sx, sy, true)
	return true
}

func monsterMeleeAttackSoundEvent(typ int16) soundEvent {
	switch typ {
	case 3001, 3003, 69:
		return soundEventMonsterAttackClaw
	case 3002, 58:
		return soundEventMonsterAttackSgt
	case 3006:
		return soundEventMonsterAttackSkull
	default:
		return -1
	}
}

func (g *game) monsterHitscanAttack(sx, sy int64, pellets int) {
	if pellets <= 0 {
		return
	}
	base := math.Atan2(float64(g.p.y-sy), float64(g.p.x-sx))
	total := 0
	for i := 0; i < pellets; i++ {
		off := float64((doomPRandomN(256)-doomPRandomN(256))<<20) * (2 * math.Pi / 4294967296.0)
		ang := base + off
		if !g.monsterBulletCanHitPlayer(sx, sy, ang, monsterAttackRange) {
			continue
		}
		total += 3 * (1 + doomPRandomN(5))
	}
	if total > 0 {
		g.damagePlayerFrom(total, "Monster shot you", sx, sy, true)
	}
}

func (g *game) monsterBulletCanHitPlayer(sx, sy int64, ang float64, rng int64) bool {
	if !g.monsterHasLOSPlayer(0, sx, sy) {
		return false
	}
	dx := float64(g.p.x - sx)
	dy := float64(g.p.y - sy)
	fwd := dx*math.Cos(ang) + dy*math.Sin(ang)
	if fwd <= 0 || fwd > float64(rng) {
		return false
	}
	perp := math.Abs(dx*math.Sin(ang) - dy*math.Cos(ang))
	return perp <= float64(playerRadius)
}

func monsterMoveStep(typ int16, fast bool) int64 {
	scale := int64(1)
	if fast {
		scale = 2
	}
	switch typ {
	case 3004, 9, 3001, 84, 65:
		return 8 * fracUnit * scale
	case 3002, 58:
		return 10 * fracUnit * scale
	case 3005, 3003, 69, 66:
		return 8 * fracUnit * scale
	case 16:
		return 16 * fracUnit * scale
	case 7, 68, 67, 64, 71:
		return 12 * fracUnit * scale
	case 3006:
		return 8 * fracUnit * scale
	default:
		return 8 * fracUnit * scale
	}
}

func monsterAttackCooldown(typ int16, fast bool) int {
	scale := 1
	if fast {
		scale = 2
	}
	switch typ {
	case 9:
		base := 22 + doomPRandomN(10)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	case 3004, 65, 84:
		base := 28 + doomPRandomN(12)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	case 3002, 3006, 58:
		base := 18 + doomPRandomN(8)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	default:
		base := monsterAttackTics + doomPRandomN(10)
		if scale == 1 {
			return base
		}
		return max(base/scale, 1)
	}
}

func (g *game) fastMonstersActive() bool {
	return g.opts.FastMonsters || g.opts.SkillLevel == 5
}

func isMeleeOnlyMonster(typ int16) bool {
	switch typ {
	case 3002, 3006, 58:
		return true
	default:
		return false
	}
}

func monsterHasMeleeAttack(typ int16) bool {
	switch typ {
	case 3001, 3002, 3003, 3006, 58, 66, 69:
		return true
	default:
		return false
	}
}

func monsterMeleeDamage(typ int16) int {
	switch typ {
	case 3002, 58: // demon/spectre
		return 4 * (1 + doomPRandomN(10))
	case 3006: // lost soul
		return 3 * (1 + doomPRandomN(8))
	case 3001: // imp
		return 3 * (1 + doomPRandomN(8))
	case 3003, 69: // baron/hell knight
		return 10 * (1 + doomPRandomN(8))
	case 66: // revenant
		return 6 * (1 + doomPRandomN(10))
	default:
		return 3 * (1 + doomPRandomN(8))
	}
}

func monsterRangedDamage(typ int16) int {
	switch typ {
	case 3004, 84: // zombieman/wolfenstein-ss hitscan-like
		return 3 * (1 + doomPRandomN(5))
	case 65: // chaingunner (single burst approximation)
		return 3 * (1 + doomPRandomN(5))
	case 9: // sergeant pellets
		pellets := 3
		dmg := 0
		for p := 0; p < pellets; p++ {
			dmg += 3 * (1 + doomPRandomN(5))
		}
		return dmg
	case 3001: // imp fireball
		return 3 * (1 + doomPRandomN(8))
	case 3005: // caco ball
		return 5 * (1 + doomPRandomN(8))
	case 3003, 69: // baron/hell knight ball
		return 8 * (1 + doomPRandomN(8))
	case 16: // rocket-like
		return 20 + doomPRandomN(60)
	case 66: // revenant tracer-like
		return 10 * (1 + doomPRandomN(8))
	case 67, 68: // mancubus/arachnotron approximation
		return 5 * (1 + doomPRandomN(8))
	case 64: // archvile flame approximation
		return 20 + doomPRandomN(20)
	case 7: // spider mastermind chaingun-like burst approximation
		dmg := 0
		for i := 0; i < 3; i++ {
			dmg += 3 * (1 + doomPRandomN(5))
		}
		return dmg
	default:
		return 3 * (1 + doomPRandomN(8))
	}
}

func (g *game) actorHasLOS(ax, ay, az, aheight, bx, by, bz, bheight int64) bool {
	if g == nil {
		return false
	}
	dx := bx - ax
	dy := by - ay
	totalDist := math.Hypot(float64(dx), float64(dy))
	if totalDist <= 0 {
		return true
	}

	sightZStart := az + aheight - (aheight >> 2)
	topSlope := float64((bz+bheight)-sightZStart) / totalDist
	bottomSlope := float64(bz-sightZStart) / totalDist

	intercepts := make([]intercept, 0, 8)
	for i, ld := range g.lines {
		frac, ok := segmentIntersectFrac(ax, ay, bx, by, ld.x1, ld.y1, ld.x2, ld.y2)
		if !ok || frac <= 0 || frac >= 1 {
			continue
		}
		intercepts = append(intercepts, intercept{frac: frac, line: i})
	}
	if len(intercepts) == 0 {
		return true
	}
	sort.Slice(intercepts, func(i, j int) bool { return intercepts[i].frac < intercepts[j].frac })

	for _, it := range intercepts {
		ld := g.lines[it.line]
		if (ld.flags&mlTwoSided) == 0 || ld.sideNum1 < 0 {
			return false
		}
		front, back := g.physLineSectors(ld)
		if front < 0 || back < 0 || front >= len(g.sectorFloor) || back >= len(g.sectorFloor) || front >= len(g.sectorCeil) || back >= len(g.sectorCeil) {
			return false
		}
		if g.sectorFloor[front] == g.sectorFloor[back] && g.sectorCeil[front] == g.sectorCeil[back] {
			continue
		}
		opentop, openbottom, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			return false
		}
		dist := totalDist * it.frac
		if dist <= 0 {
			continue
		}
		if g.sectorFloor[front] != g.sectorFloor[back] {
			slope := float64(openbottom-sightZStart) / dist
			if slope > bottomSlope {
				bottomSlope = slope
			}
		}
		if g.sectorCeil[front] != g.sectorCeil[back] {
			slope := float64(opentop-sightZStart) / dist
			if slope < topSlope {
				topSlope = slope
			}
		}
		if topSlope <= bottomSlope {
			return false
		}
	}
	return true
}

func (g *game) playerHasLOSMonster(i int, th mapdata.Thing) bool {
	if g == nil || g.m == nil || len(g.sectorFloor) == 0 {
		return true
	}
	tx, ty := g.thingPosFixed(i, th)
	tz := g.thingFloorZ(tx, ty)
	return g.actorHasLOS(g.p.x, g.p.y, g.p.z, playerHeight, tx, ty, tz, monsterHeight(th.Type))
}

func (g *game) monsterHasLOSPlayer(typ int16, x, y int64) bool {
	if g == nil || g.m == nil || len(g.sectorFloor) == 0 {
		return true
	}
	if typ == 0 {
		typ = 3004
	}
	z := g.thingFloorZ(x, y)
	return g.actorHasLOS(x, y, z, monsterHeight(typ), g.p.x, g.p.y, g.p.z, playerHeight)
}

func (g *game) monsterHeardPlayer(i int, tx, ty int64) bool {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return false
	}
	sec := g.sectorAt(tx, ty)
	if sec < 0 || sec >= len(g.sectorSoundTarget) || !g.sectorSoundTarget[sec] {
		return false
	}
	if int(g.m.Things[i].Flags)&thingFlagAmbush != 0 {
		return g.monsterHasLOSPlayer(g.m.Things[i].Type, tx, ty)
	}
	return true
}

func (g *game) propagateNoiseAlertFrom(x, y int64) {
	if g == nil || g.m == nil || len(g.m.Sectors) == 0 {
		return
	}
	sec := g.sectorAt(x, y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return
	}
	best := make([]int, len(g.m.Sectors))
	for i := range best {
		best[i] = -1
	}
	g.propagateSectorNoise(sec, 0, best)
}

func (g *game) propagateSectorNoise(sec int, soundBlocks int, best []int) {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return
	}
	traversed := soundBlocks + 1
	if sec < len(best) && best[sec] != -1 && best[sec] <= traversed {
		return
	}
	if sec < len(best) {
		best[sec] = traversed
	}
	if sec < len(g.sectorSoundTarget) {
		g.sectorSoundTarget[sec] = true
	}
	for _, ld := range g.lines {
		front, back := g.physLineSectors(ld)
		if front != sec && back != sec {
			continue
		}
		if back < 0 {
			continue
		}
		_, _, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			continue
		}
		other := front
		if other == sec {
			other = back
		}
		if other < 0 || other >= len(g.m.Sectors) {
			continue
		}
		if (ld.flags & lineSoundBlock) != 0 {
			if soundBlocks == 0 {
				g.propagateSectorNoise(other, 1, best)
			}
			continue
		}
		g.propagateSectorNoise(other, soundBlocks, best)
	}
}

func (g *game) physLineSectors(ld physLine) (int, int) {
	if g == nil || g.m == nil {
		return -1, -1
	}
	front := -1
	back := -1
	if ld.sideNum0 >= 0 && int(ld.sideNum0) < len(g.m.Sidedefs) {
		front = int(g.m.Sidedefs[int(ld.sideNum0)].Sector)
	}
	if ld.sideNum1 >= 0 && int(ld.sideNum1) < len(g.m.Sidedefs) {
		back = int(g.m.Sidedefs[int(ld.sideNum1)].Sector)
	}
	return front, back
}

func (g *game) moveMonsterToward(i int, typ int16, x, y, tx, ty, step int64) {
	ang := math.Atan2(float64(ty-y), float64(tx-x))
	g.faceMonsterToward(i, x, y, tx, ty)
	if typ == 3001 {
		// Imps in Doom don't steer perfectly every tic; add small random drift.
		switch doomPRandomN(5) {
		case 0:
			ang += math.Pi / 8
		case 1:
			ang -= math.Pi / 8
		}
	}
	dx := int64(math.Cos(ang) * float64(step))
	dy := int64(math.Sin(ang) * float64(step))
	nx := x + dx
	ny := y + dy
	if g.tryMoveProbeMonster(i, typ, nx, ny) {
		g.setThingPosFixed(i, nx, ny)
		return
	}
	if g.tryMoveProbeMonster(i, typ, x+dx, y) {
		g.setThingPosFixed(i, x+dx, y)
		return
	}
	if g.tryMoveProbeMonster(i, typ, x, y+dy) {
		g.setThingPosFixed(i, x, y+dy)
	}
}

func (g *game) faceMonsterToward(i int, fromX, fromY, toX, toY int64) {
	if g.m == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	dx := float64(toX - fromX)
	dy := float64(toY - fromY)
	if math.Abs(dx) < 1e-6 && math.Abs(dy) < 1e-6 {
		return
	}
	deg := math.Atan2(dy, dx) * (180.0 / math.Pi)
	if deg < 0 {
		deg += 360
	}
	g.m.Things[i].Angle = int16(math.Round(deg)) % 360
}

func (g *game) tryMoveProbe(x, y int64) bool {
	if g.m == nil || len(g.m.Sectors) == 0 {
		return false
	}
	_, _, _, ok := g.checkPositionForActor(x, y, 20*fracUnit, true, -1, true)
	return ok
}

func (g *game) tryMoveProbeMonster(i int, typ int16, x, y int64) bool {
	if g.m == nil || len(g.m.Sectors) == 0 || i < 0 || i >= len(g.m.Things) {
		return false
	}
	tmfloor, tmceil, tmdrop, ok := g.checkPositionForActor(x, y, monsterRadius(typ), true, i, true)
	if !ok {
		return false
	}
	height := monsterHeight(typ)
	cx, cy := g.thingPosFixed(i, g.m.Things[i])
	z := g.thingFloorZ(cx, cy)
	if tmceil-tmfloor < height {
		return false
	}
	if tmceil-z < height {
		return false
	}
	if tmfloor-z > stepHeight {
		return false
	}
	if !monsterCanDropOff(typ) && !monsterCanFloat(typ) && tmfloor-tmdrop > stepHeight {
		return false
	}
	return true
}

func (g *game) blockedSpecialLinesForMonsterMove(i int, x, y int64) []int {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) {
		return nil
	}
	radius := monsterRadius(g.m.Things[i].Type)
	tmboxTop := y + radius
	tmboxBottom := y - radius
	tmboxRight := x + radius
	tmboxLeft := x - radius
	lines := make([]int, 0, 4)

	g.validCount++
	processPhysLine := func(physIdx int) {
		if physIdx < 0 || physIdx >= len(g.lines) {
			return
		}
		if g.lineValid[physIdx] == g.validCount {
			return
		}
		g.lineValid[physIdx] = g.validCount
		ld := g.lines[physIdx]
		if tmboxRight <= ld.bbox[3] || tmboxLeft >= ld.bbox[2] || tmboxTop <= ld.bbox[1] || tmboxBottom >= ld.bbox[0] {
			return
		}
		box := [4]int64{tmboxTop, tmboxBottom, tmboxRight, tmboxLeft}
		if g.boxOnLineSide(box, ld) != -1 {
			return
		}
		blocked := false
		switch {
		case ld.sideNum1 < 0:
			blocked = true
		case (ld.flags & mlBlocking) != 0:
			blocked = true
		case (ld.flags & mlBlockMonsters) != 0:
			blocked = true
		default:
			_, _, _, openrange := g.lineOpening(ld)
			blocked = openrange <= 0
		}
		if blocked && ld.idx >= 0 && ld.idx < len(g.lineSpecial) && g.lineSpecial[ld.idx] != 0 {
			lines = append(lines, ld.idx)
		}
	}

	iter := func(lineIdx int) bool {
		if lineIdx < 0 || lineIdx >= len(g.physForLine) {
			return true
		}
		processPhysLine(g.physForLine[lineIdx])
		return true
	}

	xl := int((tmboxLeft - g.bmapOriginX) >> (fracBits + 7))
	xh := int((tmboxRight - g.bmapOriginX) >> (fracBits + 7))
	yl := int((tmboxBottom - g.bmapOriginY) >> (fracBits + 7))
	yh := int((tmboxTop - g.bmapOriginY) >> (fracBits + 7))
	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		for bx := xl; bx <= xh; bx++ {
			for by := yl; by <= yh; by++ {
				_ = g.blockLinesIterator(bx, by, iter)
			}
		}
	} else {
		for physIdx := range g.lines {
			processPhysLine(physIdx)
		}
	}
	return lines
}

func (g *game) monsterUseBlockingSpecialLines(i int, x, y int64) bool {
	for _, lineIdx := range g.blockedSpecialLinesForMonsterMove(i, x, y) {
		if g.useSpecialLineForActor(lineIdx, 0, false) {
			return true
		}
	}
	return false
}

func monsterCanFloat(typ int16) bool {
	switch typ {
	case 3005, 3006, 71:
		return true
	default:
		return false
	}
}

func monsterCanDropOff(typ int16) bool {
	switch typ {
	case 3005, 3006, 71:
		return true
	default:
		return false
	}
}

func monsterRadius(typ int16) int64 {
	switch typ {
	case 3004, 9, 65, 84, 3001, 64, 66:
		return 20 * fracUnit
	case 3002, 58:
		return 30 * fracUnit
	case 3005, 71:
		return 31 * fracUnit
	case 3003, 69:
		return 24 * fracUnit
	case 3006:
		return 16 * fracUnit
	case 7:
		return 128 * fracUnit
	case 68:
		return 64 * fracUnit
	case 16:
		return 40 * fracUnit
	case 67:
		return 48 * fracUnit
	default:
		return 20 * fracUnit
	}
}

func monsterHeight(typ int16) int64 {
	switch typ {
	case 3003, 69, 67, 68:
		return 64 * fracUnit
	case 16:
		return 110 * fracUnit
	case 7:
		return 100 * fracUnit
	default:
		return 56 * fracUnit
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
