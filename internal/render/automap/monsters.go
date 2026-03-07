package automap

import (
	"math"

	"gddoom/internal/doomrand"
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
		tx := int64(th.X) << fracBits
		ty := int64(th.Y) << fracBits
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
			if dist <= monsterWakeRange && g.monsterHasLOS(tx, ty, px, py) {
				g.thingAggro[i] = true
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
	case 3002: // demon
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
	return g.monsterHasLOS(tx, ty, px, py)
}

func (g *game) monsterCheckMissileRange(i int, typ int16, dist, tx, ty, px, py int64) bool {
	if isMeleeOnlyMonster(typ) {
		return false
	}
	if !g.monsterHasLOS(tx, ty, px, py) {
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

	x := int64(g.m.Things[i].X) << fracBits
	y := int64(g.m.Things[i].Y) << fracBits
	nx := x + dx
	ny := y + dy
	if !g.tryMoveProbe(nx, ny) {
		return false
	}
	g.m.Things[i].X = int16(nx >> fracBits)
	g.m.Things[i].Y = int16(ny >> fracBits)
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
		sx = int64(g.m.Things[i].X) << fracBits
		sy = int64(g.m.Things[i].Y) << fracBits
	}
	if dist <= monsterMeleeRange && monsterHasMeleeAttack(typ) {
		damage := monsterMeleeDamage(typ)
		if damage > 0 {
			g.damagePlayerFrom(damage, "Monster hit you", sx, sy, true)
			return true
		}
	}
	if meleeOnly {
		return false
	}
	if typ == 3004 {
		// Zombieman: single bullet with Doom-style spread and chance to miss.
		g.emitSoundEvent(soundEventShootPistol)
		g.monsterHitscanAttack(sx, sy, 1)
		return true
	}
	if typ == 9 {
		// Sergeant: 3 pellets.
		g.emitSoundEvent(soundEventShootShotgun)
		g.monsterHitscanAttack(sx, sy, 3)
		return true
	}
	if usesMonsterProjectile(typ) {
		if g.spawnMonsterProjectile(i, typ) {
			g.emitSoundEvent(projectileLaunchSoundEvent(typ))
			return true
		}
		return false
	}
	damage := monsterRangedDamage(typ)
	if damage <= 0 {
		return false
	}
	g.emitSoundEvent(soundEventShootPistol)
	g.damagePlayerFrom(damage, "Monster shot you", sx, sy, true)
	return true
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
	if !g.monsterHasLOS(sx, sy, g.p.x, g.p.y) {
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

func (g *game) monsterHasLOS(ax, ay, bx, by int64) bool {
	for _, ld := range g.lines {
		if _, ok := segmentIntersectFrac(ax, ay, bx, by, ld.x1, ld.y1, ld.x2, ld.y2); !ok {
			continue
		}
		if (ld.flags&mlTwoSided) == 0 || ld.sideNum1 < 0 {
			return false
		}
		_, _, _, openrange := g.lineOpening(ld)
		if openrange <= 0 {
			return false
		}
	}
	return true
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
	if g.tryMoveProbe(nx, ny) {
		g.m.Things[i].X = int16(nx >> fracBits)
		g.m.Things[i].Y = int16(ny >> fracBits)
		return
	}
	if g.tryMoveProbe(x+dx, y) {
		g.m.Things[i].X = int16((x + dx) >> fracBits)
		return
	}
	if g.tryMoveProbe(x, y+dy) {
		g.m.Things[i].Y = int16((y + dy) >> fracBits)
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
	saved := g.p
	ok := g.tryMove(x, y)
	g.p = saved
	return ok
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
