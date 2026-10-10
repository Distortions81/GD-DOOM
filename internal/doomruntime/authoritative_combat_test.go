package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func authorityCombatTestWorld() (*game, *authoritativePlayerState, *authoritativePlayerState) {
	g := newAuthoritativePlayersTestGame()
	a := g.captureAuthoritativePlayer()
	b := secondAuthoritativeTestPlayer(g)
	g.authorityPlayers = []*authoritativePlayerState{&a, &b}
	return g, &a, &b
}

func TestAuthoritativeHitscanHitsOtherPlayerAndKeepsShooterState(t *testing.T) {
	g, a, b := authorityCombatTestWorld()
	actor := g.playerLineAttackActor()
	slope, target, ok := g.aimLineAttackTarget(actor, 0, 1024*fracUnit)
	if !ok || target.kind != lineAttackTargetPlayer || target.playerSlot != 2 {
		t.Fatalf("aim target=%+v ok=%v", target, ok)
	}
	outcome := g.lineAttackTrace(actor, 0, 1024*fracUnit, slope, false)
	if !g.applyLineAttackOutcome(actor, outcome, 25) {
		t.Fatal("shot missed")
	}
	if g.stats.Health != 100 || a.stats.Health != 100 || b.stats.Health != 75 {
		t.Fatalf("shooter=%d/%d target=%d", g.stats.Health, a.stats.Health, b.stats.Health)
	}
	if g.localSlot != 1 || g.authorityDamageSource != 0 {
		t.Fatal("damage leaked target/source context")
	}
}

func TestAuthoritativeProjectileOwnerDoesNotHideOtherPlayers(t *testing.T) {
	g, _, _ := authorityCombatTestWorld()
	p := projectile{sourcePlayer: true, sourcePlayerSlot: 1, radius: 6 * fracUnit, height: 8 * fracUnit, sourceThing: -1}
	if hit, ok := g.projectileThingHitAtPosition(p, 64*fracUnit, 0, 32*fracUnit); !ok || !hit.isPlayer || hit.playerSlot != 2 {
		t.Fatalf("other player hit=%+v ok=%v", hit, ok)
	}
	if hit, ok := g.projectileThingHitAtPosition(p, 0, 0, 32*fracUnit); ok {
		t.Fatalf("missile hit own current body: %+v", hit)
	}
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation = 2
	p.sourcePlayerGeneration = 1
	if hit, ok := g.projectileThingHitAtPosition(p, 0, 0, 32*fracUnit); !ok || hit.playerSlot != 1 {
		t.Fatalf("old missile ignored respawned body: %+v %v", hit, ok)
	}
}

func TestAuthoritativeSplashDamagesEveryPlayerOnce(t *testing.T) {
	g, a, b := authorityCombatTestWorld()
	g.radiusAttackAt(32*fracUnit, 0, 32*fracUnit, 8*fracUnit, -1, 50, "Explosion", false, -1)
	if a.stats.Health != 66 || b.stats.Health != 66 || g.stats.Health != 66 {
		t.Fatalf("splash health=%d/%d active=%d", a.stats.Health, b.stats.Health, g.stats.Health)
	}
}

func TestAuthoritativeFriendlyFireAndFragAttribution(t *testing.T) {
	g, _, b := authorityCombatTestWorld()
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation = 1
	g.authorityRules.Scores[2].Generation = 1
	g.opts.GameMode = gameModeCoop
	apply := func() { g.damagePlayerFrom(200, "shot", 0, 0, true, -1) }
	g.applyPlayerDamageTarget(2, 1, apply)
	if b.stats.Health != 100 {
		t.Fatal("disabled friendly fire damaged teammate")
	}
	g.opts.GameMode = gameModeDeathmatch
	g.applyPlayerDamageTarget(2, 1, apply)
	if b.stats.Health != 0 || g.authorityRules.Scores[1].Frags != 1 || g.authorityRules.Scores[2].Deaths != 1 {
		t.Fatalf("kill attribution %+v", g.authorityRules.Scores)
	}
	g.applyPlayerDamageTarget(2, 1, apply)
	if g.authorityRules.Scores[1].Frags != 1 {
		t.Fatal("duplicate damage counted another frag")
	}
}

func TestAuthoritativeOldGenerationBFGDoesNotFireFromReplacement(t *testing.T) {
	g, _, target := authorityCombatTestWorld()
	g.opts.GameMode = gameModeDeathmatch
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation = 2
	g.authorityRules.Scores[2].Generation = 1
	impact := projectileImpact{
		kind: projectileBFGBall, sourcePlayer: true,
		sourcePlayerSlot: 1, sourcePlayerGeneration: 1,
		phase: 1, phaseTics: 1, tics: 20, ceilz: 128 * fracUnit,
	}
	beforeRNG, beforePlay := doomrand.State()
	g.advanceProjectileImpactTic(&impact)
	if !impact.sprayDone || target.stats.Health != 100 || len(g.hitscanPuffs) != 0 {
		t.Fatal("old BFG impact fired rays from a replacement incarnation")
	}
	if rnd, play := doomrand.State(); rnd != beforeRNG || play != beforePlay {
		t.Fatal("discarded BFG spray consumed damage RNG")
	}
	// A still-existing original body can legitimately emit its own spray.
	impact.phase, impact.phaseTics, impact.sprayDone, impact.sourcePlayerGeneration = 1, 1, false, 2
	g.advanceProjectileImpactTic(&impact)
	if target.stats.Health == 100 {
		t.Fatal("valid BFG owner no longer damages its target")
	}
}

func TestAuthoritativeOldProjectileDoesNotCreditReplacement(t *testing.T) {
	g, _, victim := authorityCombatTestWorld()
	g.opts.GameMode = gameModeDeathmatch
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation = 2
	g.authorityRules.Scores[2].Generation = 1
	g.applyPlayerDamageTargetFrom(2, authorityPlayerIdentity{Slot: 1, Generation: 1}, func() {
		g.damagePlayer(200, "old rocket")
	})
	if !victim.isDead || g.authorityRules.Scores[1].Frags != 0 || g.authorityRules.Scores[2].Deaths != 1 || g.authorityDamageSource != 0 {
		t.Fatalf("stale projectile credited replacement: %+v", g.authorityRules.Scores)
	}
}

func TestAuthoritativeSplashFacesActualShooterAndPreservesFriendlyFire(t *testing.T) {
	g, shooter, victim := authorityCombatTestWorld()
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation, g.authorityRules.Scores[2].Generation = 1, 1
	g.opts.GameMode = gameModeDeathmatch
	p := projectile{kind: projectileRocket, sourcePlayer: true, sourcePlayerSlot: 1, sourcePlayerGeneration: 1, sourceThing: -1, height: 8 * fracUnit}
	g.projectileSplashDamage(p, 32*fracUnit, 0, 32*fracUnit)
	if !victim.statusHasAttacker || victim.statusAttackerX != shooter.p.x || victim.statusAttackerX == victim.p.x {
		t.Fatalf("splash victim faced its own body: attackerX=%d shooterX=%d victimX=%d", victim.statusAttackerX, shooter.p.x, victim.p.x)
	}
	// Old co-op ordnance retains team policy even after its original body
	// has gone away, but cannot attribute damage to the replacement body.
	g, _, victim = authorityCombatTestWorld()
	g.opts.GameMode = gameModeCoop
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation, g.authorityRules.Scores[2].Generation = 2, 1
	g.projectileSplashDamage(p, 32*fracUnit, 0, 32*fracUnit)
	if victim.stats.Health != 100 || victim.statusHasAttacker {
		t.Fatal("old projectile bypassed friendly-fire or changed teammate's attacker")
	}
}

func TestAuthoritativeDelayedBarrelChainRetainsShooterIncarnation(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "original shooter", true: "replacement shooter"}[replace], func(t *testing.T) {
			g, victim, shooter := authoritativeTargetsTestGame()
			g.opts.GameMode = gameModeDeathmatch
			g.authorityRules = &authorityRulesState{}
			g.authorityRules.Scores[1].Generation, g.authorityRules.Scores[2].Generation = 1, 1
			g.m.Things[0].Type, g.thingHP[0] = barrelThingType, 20
			g.setThingPosFixed(0, 96*fracUnit, 0)
			secondBarrel := g.appendRuntimeThing(mapdata.Thing{Type: barrelThingType, X: 160}, false)
			g.thingHP[secondBarrel] = 20
			shooter.p.x = -512 * fracUnit
			g.stats.Health, g.playerMobjHealth = 40, 40
			*victim = g.captureAuthoritativePlayer()
			g.withAuthoritativePlayer(shooter, func() { g.damageBarrel(0, 30) })
			owner := g.authorityBarrelSources[0]
			if owner.PlayerSlot != 2 || owner.PlayerGeneration != 1 {
				t.Fatalf("initial barrel owner=%+v", owner)
			}
			if replace {
				g.authorityRules.Scores[2].Generation = 2
			}
			// Advance actual delayed explosion states, not a direct blast, so
			// ownership must survive well beyond the original damage callback.
			for range 45 {
				g.tickBarrelDeathState(0, g.m.Things[0])
				if g.thingDead[secondBarrel] {
					g.tickBarrelDeathState(secondBarrel, g.m.Things[secondBarrel])
				}
			}
			if got := g.authorityBarrelSources[secondBarrel]; got != owner {
				t.Fatalf("chain lost original owner: first=%+v second=%+v", owner, got)
			}
			if !victim.isDead || g.authorityRules.Scores[1].Deaths != 1 {
				t.Fatal("delayed chain did not damage its final player victim")
			}
			wantFrags := 1
			if replace {
				wantFrags = 0
			}
			if g.authorityRules.Scores[2].Frags != wantFrags {
				t.Fatalf("barrel credited wrong incarnation: %+v", g.authorityRules.Scores)
			}
		})
	}
}
