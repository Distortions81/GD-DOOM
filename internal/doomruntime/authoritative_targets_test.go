package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func authoritativeTargetsTestGame() (*game, *authoritativePlayerState, *authoritativePlayerState) {
	g := &game{
		m: &mapdata.Map{
			Things:     []mapdata.Thing{{Type: 3002, X: 32, Angle: 180}},
			Sectors:    []mapdata.Sector{{CeilingHeight: 128}},
			SubSectors: []mapdata.SubSector{{}},
		},
		opts:                  Options{Headless: true, SkillLevel: 3},
		localSlot:             1,
		localPlayerThingIndex: -1,
		playerBlockOrder:      2,
		subSectorSec:          []int{0},
		thingCollected:        []bool{false},
		thingHP:               []int{150},
		thingAggro:            []bool{false},
		thingCooldown:         []int{0},
		thingZState:           []int64{0},
		thingFloorState:       []int64{0},
		thingCeilState:        []int64{128 * fracUnit},
		thingSupportValid:     []bool{true},
		stats:                 playerStats{Health: 100},
		playerMobjHealth:      100,
		p: player{x: 256 * fracUnit, ceilz: 128 * fracUnit, viewHeight: playerViewHeight,
			sector: -1, subsector: -1},
	}
	g.initPhysics()
	g.ensureMonsterAIState()
	first := g.captureAuthoritativePlayer()
	second := first
	second.localSlot = 2
	second.p.x = 0
	second.playerBlockOrder = 3
	g.authorityPlayers = []*authoritativePlayerState{&first, &second}
	return g, &first, &second
}

func TestAuthoritativeMonsterAcquiresLiveSecondSlotWithDeadAnchor(t *testing.T) {
	g, first, second := authoritativeTargetsTestGame()
	g.isDead, g.stats.Health, g.playerMobjHealth = true, 0, 0
	*first = g.captureAuthoritativePlayer()
	g.thingLastLook[0] = 0
	if !g.monsterLookForPlayer(0, true, 32*fracUnit, 0) {
		t.Fatal("dead first player prevented acquisition of live second player")
	}
	if got := g.monsterTargetPlayerSlot(0); got != 2 {
		t.Fatalf("target slot = %d, want 2", got)
	}
	x, y, _, _, _, ok := g.monsterTargetPos(0)
	if !ok || x != second.p.x || y != second.p.y {
		t.Fatalf("target position = (%d,%d), valid %t", x, y, ok)
	}
	if !g.monsterHasTarget(0) || !g.monsterHasLOSTarget(0, 3002, 32*fracUnit, 0) {
		t.Fatal("live second player did not remain a valid visible target")
	}
	if !g.isDead || g.localSlot != 1 {
		t.Fatal("target inspection replaced the active player context")
	}
}

func TestAuthoritativeMonsterAcquiresOnlyPresentSecondSlot(t *testing.T) {
	g, _, second := authoritativeTargetsTestGame()
	g.authorityPlayers = []*authoritativePlayerState{second}
	g.applyAuthoritativePlayer(*second)
	g.thingLastLook[0] = 0
	if !g.monsterLookForPlayer(0, true, 32*fracUnit, 0) || g.monsterTargetPlayerSlot(0) != 2 {
		t.Fatal("monster could not acquire player when slot 1 was absent")
	}
}

func TestAuthoritativeMonsterMeleeDamagesSelectedSlot(t *testing.T) {
	doomrand.Clear()
	g, first, second := authoritativeTargetsTestGame()
	g.withAuthoritativePlayer(second, func() { g.setMonsterTargetPlayer(0) })
	if !g.monsterAttack(0, 3002, 32*fracUnit) {
		t.Fatal("demon melee attack did not execute")
	}
	if second.stats.Health >= 100 || first.stats.Health != 100 || g.stats.Health != 100 {
		t.Fatalf("melee damage went to wrong body: first=%d second=%d active=%d", first.stats.Health, second.stats.Health, g.stats.Health)
	}
	if g.localSlot != 1 {
		t.Fatal("attack did not restore simulation player context")
	}
}

func TestAuthoritativeMonsterThinkerAttacksBeyondDeadAnchor(t *testing.T) {
	doomrand.Clear()
	g, first, second := authoritativeTargetsTestGame()
	g.isDead, g.stats.Health, g.playerMobjHealth = true, 0, 0
	*first = g.captureAuthoritativePlayer()
	for range 35 {
		g.tickMonsters()
		g.worldTic++
		if second.stats.Health < 100 {
			if first.stats.Health != 0 || g.stats.Health != 0 || g.localSlot != 1 {
				t.Fatal("thinker activation changed dead anchor")
			}
			return
		}
	}
	t.Fatal("monster thinker never damaged live player in slot 2")
}

func TestAuthoritativeMonsterSightUsesCandidateNotAnchor(t *testing.T) {
	g, _, second := authoritativeTargetsTestGame()
	// Facing west: slot 1 is behind and distant, slot 2 is in front.
	g.thingLastLook[0] = 0
	if !g.monsterLookForPlayer(0, false, 32*fracUnit, 0) || g.monsterTargetPlayerSlot(0) != 2 {
		t.Fatal("candidate sight/angle test used anchor coordinates")
	}
	second.inventory.InvisTics = 100
	if !g.monsterTargetHasShadow(0) {
		t.Fatal("target invisibility read anchor inventory")
	}
}

func TestAuthoritativeMonsterNoiseRetainsSourceSlot(t *testing.T) {
	g, _, second := authoritativeTargetsTestGame()
	g.sectorSoundTarget = make([]bool, 1)
	g.withAuthoritativePlayer(second, func() { g.propagateNoiseAlertFrom(g.p.x, g.p.y) })
	if len(g.sectorSoundPlayerSlot) != 1 || g.sectorSoundPlayerSlot[0] != 2 {
		t.Fatalf("noise identity = %v, want [2]", g.sectorSoundPlayerSlot)
	}
	if heard, wake := g.monsterAcquireSectorSoundTarget(0, 32*fracUnit, 0); !heard || !wake || g.monsterTargetPlayerSlot(0) != 2 {
		t.Fatalf("noise acquired heard=%t wake=%t slot=%d", heard, wake, g.monsterTargetPlayerSlot(0))
	}
}

func TestAuthoritativeMonsterInfightingKeepsThingIdentity(t *testing.T) {
	doomrand.Clear()
	g, first, second := authoritativeTargetsTestGame()
	other := g.appendRuntimeThing(mapdata.Thing{Type: 3001, X: 0}, false)
	if other < 0 {
		t.Fatal("could not spawn infighting target")
	}
	g.thingHP[other] = 60
	g.setMonsterTargetThing(0, other)
	g.damageMonsterTarget(0, 10, "hit", 32*fracUnit, 0)
	if g.thingHP[other] != 50 || first.stats.Health != 100 || second.stats.Health != 100 {
		t.Fatalf("infighting damaged wrong target: monster=%d p1=%d p2=%d", g.thingHP[other], first.stats.Health, second.stats.Health)
	}
	if g.monsterTargetPlayerSlot(0) != 0 {
		t.Fatal("infighting retained a player target identity")
	}
}

func TestAuthoritativeMonsterDepartedTargetDoesNotBecomeAnchor(t *testing.T) {
	g, first, second := authoritativeTargetsTestGame()
	g.withAuthoritativePlayer(second, func() { g.setMonsterTargetPlayer(0) })
	g.authorityPlayers = []*authoritativePlayerState{first}
	if g.monsterHasTarget(0) || g.monsterHasSimulationTarget(0) {
		t.Fatal("departed target became the remaining player")
	}
	g.damageMonsterTarget(0, 10, "hit", 32*fracUnit, 0)
	if first.stats.Health != 100 || g.stats.Health != 100 {
		t.Fatal("departed player's attack damaged anchor")
	}
}

func TestAuthoritativeMonsterTargetsClearedBeforeSlotReuse(t *testing.T) {
	g, _, second := authoritativeTargetsTestGame()
	g.withAuthoritativePlayer(second, func() { g.setMonsterTargetPlayer(0) })
	g.sectorSoundTarget = []bool{true}
	g.sectorSoundPlayerSlot = []int{2}
	g.projectileImpacts = []projectileImpact{{fireTargetPlayer: true, fireTargetPlayerSlot: 2}}
	g.clearAuthoritativePlayerTargets(2)
	if g.monsterHasTarget(0) || g.thingTargetPlayerSlot[0] != 0 || g.sectorSoundTarget[0] || g.sectorSoundPlayerSlot[0] != 0 || g.projectileImpacts[0].fireTargetPlayerSlot != 0 {
		t.Fatal("departed body retained target/noise/fire references")
	}
}

func TestAuthoritativeSkullCollisionCarriesHitPlayerSlot(t *testing.T) {
	doomrand.Clear()
	g, first, second := authoritativeTargetsTestGame()
	g.m.Things[0].Type = 3006
	target, hit := g.lostSoulChargeTargetAt(0, g.m.Things[0], 0, 0, 0)
	if !hit || target.playerSlot != 2 {
		t.Fatalf("skull contact target=%+v hit=%t", target, hit)
	}
	g.hitSkullFlyTarget(0, 3006, target)
	if second.stats.Health >= 100 || first.stats.Health != 100 || g.stats.Health != 100 {
		t.Fatal("skull contact damaged wrong body")
	}
}

func TestAuthoritativeArchVileFireAndBlastKeepVictim(t *testing.T) {
	doomrand.Clear()
	g, first, second := authoritativeTargetsTestGame()
	g.m.Things[0].Type = 64
	g.withAuthoritativePlayer(second, func() { g.setMonsterTargetPlayer(0) })
	g.spawnArchVileFire(0)
	fx := g.archVileFireForSource(0)
	if fx == nil || fx.fireTargetPlayerSlot != 2 {
		t.Fatal("fire did not capture victim identity")
	}
	second.p.x = 64 * fracUnit
	g.setMonsterTargetPlayer(0) // source now targets slot 1; existing fire follows 2.
	g.followArchVileFire(fx)
	if abs(fx.x-(second.p.x+24*fracUnit)) > fracUnit {
		t.Fatalf("fire followed wrong victim: x=%d want=%d", fx.x, second.p.x+24*fracUnit)
	}
	g.withAuthoritativePlayer(second, func() { g.setMonsterTargetPlayer(0) })
	// Remove fire to isolate direct blast and launch from radius damage.
	g.projectileImpacts = nil
	if !g.archVileBlast(0, 32*fracUnit, 0) {
		t.Fatal("arch-vile blast did not execute")
	}
	if second.stats.Health != 80 || second.p.momz != 10*fracUnit || first.stats.Health != 100 || g.p.momz != 0 {
		t.Fatalf("blast affected wrong body: p1hp=%d p2hp=%d p2momz=%d activeMomz=%d", first.stats.Health, second.stats.Health, second.p.momz, g.p.momz)
	}
}
