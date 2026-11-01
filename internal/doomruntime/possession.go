package doomruntime

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/hud"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Possession leaves the marine in the world and drives the actual monster mobj.
// No player inventory, health or save-state fields are borrowed for the monster.
type monsterPossession struct {
	thing     int
	cmd       moveCmd
	fire      bool
	armed     bool
	useDown   bool
	cooldown  int
	turnHeld  int
	prevAngle uint32
}

func (g *game) possessionSupported() bool {
	return g != nil && g.opts.AuthorityClient == nil && len(g.authorityPlayers) == 0 &&
		g.opts.DemoScript == nil && strings.TrimSpace(g.opts.RecordDemoPath) == "" &&
		g.opts.LiveTicSource == nil && g.opts.LiveTicSink == nil &&
		normalizeGameMode(g.opts.GameMode) == gameModeSingle
}

func (g *game) possessionModeEnabled() bool {
	return g.possessionSupported() && g.opts.PlayAsMonster
}

func (g *game) togglePossessionMode() {
	if !g.possessionSupported() {
		g.setHUDMessage("Possession needs local play without demo recording", 105)
		return
	}
	g.releasePossession()
	g.opts.PlayAsMonster = !g.opts.PlayAsMonster
	g.possessionFireDown = true // Require a fresh click after toggling.
	if g.opts.PlayAsMonster {
		g.setHUDMessage("Monster mode: aim + Fire to possess; Use returns", 140)
	} else {
		g.setHUDMessage("Monster mode OFF", 70)
	}
}

// Intercept local commands before the marine's weapon thinker can consume fire.
func (g *game) possessionCommand(cmd moveCmd, use, fire bool) (moveCmd, bool, bool) {
	if !g.possessionModeEnabled() {
		g.releasePossession()
		return cmd, use, fire
	}
	pressed := fire && !g.possessionFireDown
	g.possessionFireDown = fire
	if g.possession != nil {
		p := g.possession
		if g.isDead || !g.possessionBodyAlive() || (use && !p.useDown) {
			g.releasePossession()
			return moveCmd{}, false, false
		}
		p.useDown = use
		if !fire {
			p.armed = true
		}
		p.cmd, p.fire = cmd, fire && p.armed
		return moveCmd{}, false, false
	}
	if pressed && !g.isDead {
		if i, ok := g.pickHitscanMonsterTarget(); ok && isMonster(g.m.Things[i].Type) &&
			g.thingActiveInSession(i) && !g.thingDead[i] && g.thingHP[i] > 0 {
			g.ensureMonsterAIState()
			angle := g.p.angle
			g.p.momx, g.p.momy = 0, 0
			g.setThingWorldAngle(i, angle)
			g.setThingMomentum(i, 0, 0, 0)
			g.possession = &monsterPossession{thing: i, prevAngle: angle, useDown: use}
			g.setHUDMessage("Possessed! Fire attacks; Use returns to marine", 105)
		} else {
			g.setHUDMessage("Aim at a living monster", 35)
		}
	}
	return cmd, use, false
}

func (g *game) possessionBodyAlive() bool {
	if g.possession == nil || g.m == nil {
		return false
	}
	i := g.possession.thing
	return i >= 0 && i < len(g.m.Things) && i < len(g.thingHP) && g.thingHP[i] > 0 &&
		i < len(g.thingDead) && !g.thingDead[i] && g.thingActiveInSession(i)
}

func (g *game) releasePossession() {
	if g.possession == nil {
		return
	}
	// The body retains its current position, health and normal AI state. Its
	// ordinary thinker resumes on the next world tic, including death animation.
	g.possession = nil
	g.turnHeld = 0
	g.setHUDMessage("Returned to marine", 70)
}

func (g *game) finishPossessionTic() {
	if g.possession == nil {
		return
	}
	if !g.possessionModeEnabled() || g.isDead || !g.possessionBodyAlive() {
		g.releasePossession()
	}
}

func (g *game) tickPossessedMonster(i int, th mapdata.Thing) {
	p := g.possession
	angle := g.thingWorldAngle(i, th)
	p.prevAngle = angle
	// Reuse player turn acceleration/sensitivity without moving the marine.
	marine, marineTurnHeld := g.p, g.turnHeld
	g.turnHeld = p.turnHeld
	g.p = player{angle: angle}
	turn := p.cmd
	turn.forward, turn.side = 0, 0
	g.updatePlayer(turn)
	angle = g.p.angle
	p.turnHeld = g.turnHeld
	g.p, g.turnHeld = marine, marineTurnHeld
	g.setThingWorldAngle(i, angle)
	g.tickMonsterMomentum(i, th)
	if !g.possessionBodyAlive() {
		return
	}
	// The species' own radius/height and monster line restrictions govern movement.
	if !g.thingSkullFly[i] {
		f, s := float64(p.cmd.forward)/50, float64(p.cmd.side)/50
		if n := math.Hypot(f, s); n > 1 {
			f /= n
			s /= n
		}
		speed := monsterMoveStep(th.Type, g.fastMonstersActive())
		dx := int64(f*float64(fixedMul(speed, doomFineCosine(angle))) + s*float64(fixedMul(speed, doomFineCosine(angle-doomAng90))))
		dy := int64(f*float64(fixedMul(speed, doomFineSineAtAngle(angle))) + s*float64(fixedMul(speed, doomFineSineAtAngle(angle-doomAng90))))
		if dx != 0 || dy != 0 {
			x, y := g.thingPosFixed(i, th)
			if !g.movePossessedMonster(i, th, x+dx, y+dy) {
				if dx != 0 {
					g.movePossessedMonster(i, th, x+dx, y)
				}
				x, y = g.thingPosFixed(i, th)
				if dy != 0 {
					g.movePossessedMonster(i, th, x, y+dy)
				}
			}
		}
	}
	if p.cooldown > 0 {
		p.cooldown--
	}
	if p.fire && p.cooldown == 0 {
		g.firePossessedMonster(i, th)
		p.cooldown = max(8, monsterAttackAnimTotalTics(th.Type))
	}
}

func (g *game) movePossessedMonster(i int, th mapdata.Thing, x, y int64) bool {
	probe := g.probeMonsterMove(i, th.Type, x, y)
	if !probe.ok {
		for _, line := range probe.probeLines {
			g.useSpecialLineForActorIndex(line, 0, false, i)
		}
		if probe.floatOK && monsterCanFloat(th.Type) {
			z, floor, ceil := g.thingSupportState(i, th)
			if z < probe.tmfloor {
				z += 4 * fracUnit
			} else {
				z -= 4 * fracUnit
			}
			g.setThingSupportState(i, z, floor, ceil)
		}
		return false
	}
	oldX, oldY := g.thingPosFixed(i, th)
	z, _, _ := g.thingSupportState(i, th)
	if !monsterCanFloat(th.Type) {
		z = probe.tmfloor
	}
	g.setThingPosFixed(i, x, y)
	g.setThingSupportState(i, z, probe.tmfloor, probe.tmceil)
	g.checkWalkSpecialLinesForActorWithCandidates(oldX, oldY, x, y, i, false, probe.probeLines)
	return true
}

func (g *game) firePossessedMonster(i int, th mapdata.Thing) {
	angle := g.thingWorldAngle(i, th)
	actor := g.monsterLineAttackActor(i, th.Type)
	slope, target, aimed := g.aimLineAttackTarget(actor, angle, monsterAttackRange)
	if !aimed {
		slope = 0
	}
	// Melee uses the same wall-aware trace as bullets, so bites cannot cross walls.
	if monsterHasMeleeAttack(th.Type) || th.Type == 3005 {
		hit := g.lineAttackTrace(actor, angle, monsterMeleeRange, slope, false)
		if hit.target.kind != lineAttackTargetNone {
			damage := monsterMeleeDamage(th.Type)
			if th.Type == 3005 {
				damage = 10 * (1 + doomPRandomN(6))
			}
			if damage > 0 {
				if ev := monsterMeleeAttackSoundEvent(th.Type); ev >= 0 {
					g.emitSoundEventAt(ev, actor.x, actor.y)
				}
				g.applyLineAttackOutcome(actor, hit, damage)
				return
			}
		}
	}
	if isMeleeOnlyMonster(th.Type) {
		return
	}
	switch th.Type {
	case 3004, 9, 65, 84, 7:
		pellets, sound := 1, soundEventShootPistol
		if th.Type != 3004 {
			sound = soundEventShootShotgun
		}
		if th.Type == 9 || th.Type == 7 {
			pellets = 3
		}
		g.emitSoundEventAt(sound, actor.x, actor.y)
		g.monsterHitscanAttack(i, th.Type, actor.x, actor.y, pellets)
	case 3006:
		g.thingSkullFly[i] = true
		speed := int64(20 * fracUnit)
		g.setThingMomentum(i, fixedMul(speed, doomFineCosine(angle)), fixedMul(speed, doomFineSineAtAngle(angle)), fixedMul(speed, slope))
		g.emitSoundEventAt(soundEventMonsterAttackSkull, actor.x, actor.y)
	case 71:
		g.spawnPainLostSoul(i, angle)
	case 64:
		if !aimed {
			return
		}
		if target.kind == lineAttackTargetThing {
			g.setMonsterTargetThing(i, target.idx)
		} else {
			g.setMonsterTargetPlayer(i)
		}
		g.emitSoundEventAt(soundEventMonsterAttackArchvile, actor.x, actor.y)
		g.spawnArchVileFire(i)
		g.archVileBlast(i, actor.x, actor.y)
	default:
		if !usesMonsterProjectile(th.Type) {
			return
		}
		g.spawnPossessedProjectile(i, th.Type, angle, slope, target)
		if th.Type == 67 {
			g.spawnPossessedProjectile(i, th.Type, angle+0x08000000, slope, target)
		}
	}
}

func (g *game) spawnPossessedProjectile(i int, typ int16, angle uint32, slope int64, target lineAttackTarget) {
	x, y := g.thingPosFixed(i, g.m.Things[i])
	z, _, _ := g.thingSupportState(i, g.m.Things[i])
	z += 32 * fracUnit
	if typ == 66 {
		z += 16 * fracUnit
	}
	speed, kind := monsterProjectileSpeed(typ, g.fastMonstersActive()), monsterProjectileKind(typ)
	p := projectile{x: x, y: y, z: z, prevX: x, prevY: y, prevZ: z,
		vx: fixedMul(speed, doomFineCosine(angle)), vy: fixedMul(speed, doomFineSineAtAngle(angle)), vz: fixedMul(speed, slope),
		radius: monsterProjectileRadius(typ), height: monsterProjectileHeight(typ),
		sourceX: x, sourceY: y, sourceThing: i, sourceType: typ,
		lastLook: doomPRandomN(4), kind: kind, angle: angle,
		frameTics: randomizedMissileSpawnTics(projectileSpawnStateTics(kind)), order: g.allocThinkerOrder(), deferredTick: true}
	if typ == 66 {
		p.tracerPlayer = target.kind == lineAttackTargetPlayer
		if target.kind == lineAttackTargetThing {
			p.tracerThingTarget = target.idx + 1
		}
	}
	p.floorz, p.ceilz = g.projectileSpawnSupportStateAt(x, y)
	if g.finishProjectileSpawn(&p, true) {
		g.projectiles = append(g.projectiles, p)
	}
	g.emitSoundEventAt(projectileLaunchSoundEvent(typ), x, y)
}

func (g *game) preparePossessionCamera(alpha float64) {
	if !g.possessionBodyAlive() {
		return
	}
	p := g.possession
	th := g.m.Things[p.thing]
	x, y, _ := g.thingRenderPosFixed(p.thing, th, alpha)
	g.renderPX, g.renderPY = float64(x)/fracUnit, float64(y)/fracUnit
	g.renderAngle = lerpAngle(p.prevAngle, g.thingWorldAngle(p.thing, th), alpha)
}

func (g *game) possessionEyeZ() (float64, bool) {
	if !g.possessionBodyAlive() {
		return 0, false
	}
	i := g.possession.thing
	th := g.m.Things[i]
	_, _, z := g.thingRenderPosFixed(i, th, g.renderAlpha)
	_, _, ceil := g.thingSupportState(i, th)
	eye := min(z+monsterHeight(th.Type)*3/4, ceil-4*fracUnit)
	return float64(eye) / fracUnit, true
}

func (g *game) drawPossessionOverlay(screen *ebiten.Image) {
	if !g.possessionModeEnabled() {
		return
	}
	text := "MONSTER MODE: Aim + Fire to possess"
	hint := "IDMONSTER: mode off"
	if g.possessionBodyAlive() {
		text = fmt.Sprintf("MONSTER HP %d", g.thingHP[g.possession.thing])
		hint = "Fire: attack | Use: return to marine"
	}

	// Keep instructions on screen even at classic resolution with a large HUD setting.
	scale := min(g.hudScaleValue(), float64(g.viewW)/320, float64(g.viewH)/200)
	hud.DrawHUDMessage(screen, hud.MessageInputs{
		ViewW: g.viewW, ViewH: g.viewH, SourcePort: g.hudUsesLogicalLayout(),
		HUDScale: scale, Message: text + "\n" + hint, X: 2, Y: 16,
	}, g.drawHUTextAt)
	rect := g.walkRenderViewportRect()
	x, y := float64(rect.Min.X+rect.Dx()/2), float64(rect.Min.Y+rect.Dy()/2)
	white := color.RGBA{255, 255, 255, 255}
	ebitenutil.DrawRect(screen, x-4, y, 9, 1, white)
	ebitenutil.DrawRect(screen, x, y-4, 1, 9, white)
}

// Listener and automap follow the controlled body; the marine remains the
// simulation's player and can still be hurt while left unattended.
func (g *game) possessionViewpoint() (int64, int64, uint32) {
	if g.possessionBodyAlive() {
		i := g.possession.thing
		x, y := g.thingPosFixed(i, g.m.Things[i])
		return x, y, g.thingWorldAngle(i, g.m.Things[i])
	}
	return g.p.x, g.p.y, g.p.angle
}
