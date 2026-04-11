package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestHazardDamagePeriodicSpecial5(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 5}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 100},
		soundQueue:  make([]soundEvent, 0, 4),
	}
	g.runGameplayTic(moveCmd{}, false, false)
	if g.stats.Health != 90 {
		t.Fatalf("health after tic0 damage=%d want=90", g.stats.Health)
	}
	for i := 0; i < 32; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
	}
	if g.stats.Health != 80 {
		t.Fatalf("health after second period=%d want=80", g.stats.Health)
	}
}

func TestHazardDamageBlockedByRadSuit(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 5}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 100},
		inventory:   playerInventory{RadSuitTics: 40},
	}
	for i := 0; i < 32; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
	}
	if g.stats.Health != 100 {
		t.Fatalf("health with suit=%d want=100", g.stats.Health)
	}
	if g.inventory.RadSuitTics != 8 {
		t.Fatalf("radsuit tics=%d want=8", g.inventory.RadSuitTics)
	}
}

func TestHazardDamage_LastRadSuitTicStillProtects(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 5}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 100},
		inventory:   playerInventory{RadSuitTics: 1},
	}
	g.runGameplayTic(moveCmd{}, false, false)
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100 on final suit tic", g.stats.Health)
	}
	if g.inventory.RadSuitTics != 0 {
		t.Fatalf("radsuit tics=%d want=0 after final protected tic", g.inventory.RadSuitTics)
	}
	for i := 0; i < 32; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
	}
	if g.stats.Health != 90 {
		t.Fatalf("health=%d want=90 once suit expires on next damage tic", g.stats.Health)
	}
}

func TestDamagePlayerFrom_BlockedByInvulnerabilityPowerup(t *testing.T) {
	g := &game{
		stats:            playerStats{Health: 100},
		playerMobjHealth: 100,
		inventory:        playerInventory{InvulnTics: 10},
	}
	g.damagePlayerFrom(20, "ouch", 0, 0, false, -1)
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100", g.stats.Health)
	}
}

func TestDamagePlayerFrom_ClampsPlayerHealthButNotMobjHealth(t *testing.T) {
	g := &game{
		stats:            playerStats{Health: 2},
		playerMobjHealth: 2,
	}
	g.damagePlayerFrom(3, "ouch", 0, 0, false, -1)
	if g.stats.Health != 0 {
		t.Fatalf("health=%d want=0", g.stats.Health)
	}
	if g.playerMobjHealth != -1 {
		t.Fatalf("playerMobjHealth=%d want=-1", g.playerMobjHealth)
	}
}

func TestDamagePlayerFrom_FatalHitConsumesDoomDeathTicRandom(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:            playerStats{Health: 5},
		playerMobjHealth: 5,
		soundQueue:       make([]soundEvent, 0, 2),
	}
	_, before := doomrand.State()
	g.damagePlayerFrom(12, "ouch", 0, 0, false, -1)
	_, after := doomrand.State()
	if got := after - before; got != 1 {
		t.Fatalf("prnd advanced by %d want=1 on fatal hit", got)
	}
	if !g.isDead {
		t.Fatal("player should be dead after fatal hit")
	}
	if g.playerMobjState != 0 || g.playerMobjTics != 0 {
		t.Fatalf("player mobj state/tics=%d/%d want=0/0 on fatal hit", g.playerMobjState, g.playerMobjTics)
	}
}

func TestHazardDamageSpecial16WithoutSuit(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 16}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 100},
	}
	for i := 0; i < 32; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
	}
	if g.stats.Health != 80 {
		t.Fatalf("health=%d want=80", g.stats.Health)
	}
}

func TestHazardDamageSpecial11RequestsExitAtTenHealthOrLess(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 11}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 30},
	}
	g.applySectorHazardDamage()
	if g.stats.Health != 10 {
		t.Fatalf("health=%d want=10", g.stats.Health)
	}
	if !g.levelExitRequested {
		t.Fatal("special 11 should request a level exit once health reaches 10")
	}
	if g.secretLevelExit {
		t.Fatal("special 11 should request a normal exit")
	}
}

func TestHazardDamageSpecial11RequestsExitEvenOnFatalTick(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 11}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 15},
		soundQueue:  make([]soundEvent, 0, 2),
	}
	g.applySectorHazardDamage()
	if g.stats.Health != 0 {
		t.Fatalf("health=%d want=0", g.stats.Health)
	}
	if !g.isDead {
		t.Fatal("special 11 fatal tick should still kill the player")
	}
	if !g.levelExitRequested {
		t.Fatal("special 11 fatal tick should still request the level exit")
	}
}

func TestTrackSecrets_RequiresPlayerOnFloorAndClearsSectorSpecial(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Sectors: []mapdata.Sector{{Special: 9}}},
		p: player{
			x:      0,
			y:      0,
			z:      fracUnit,
			floorz: 0,
		},
		secretFound:        make([]bool, 1),
		hudMessagesEnabled: true,
	}
	g.trackSecrets()
	if g.secretsFound != 0 {
		t.Fatalf("secretsFound=%d want=0 while above floor", g.secretsFound)
	}
	if g.m.Sectors[0].Special != 9 {
		t.Fatalf("sector special=%d want=9 before touching floor", g.m.Sectors[0].Special)
	}
	g.p.z = 0
	g.trackSecrets()
	if g.secretsFound != 1 {
		t.Fatalf("secretsFound=%d want=1 after touching floor", g.secretsFound)
	}
	if !g.secretFound[0] {
		t.Fatal("secret sector should be marked found")
	}
	if g.m.Sectors[0].Special != 0 {
		t.Fatalf("sector special=%d want=0 after finding secret", g.m.Sectors[0].Special)
	}
	if got := g.useText; got != "" {
		t.Fatalf("message=%q want empty", got)
	}
}

func TestTrackSecrets_UsesCachedPlayerSectorLikeDoomMobj(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Sectors: []mapdata.Sector{{Special: 0}, {Special: 9}}},
		p: player{
			x:         0,
			y:         0,
			z:         0,
			floorz:    0,
			subsector: 0,
			sector:    1,
		},
		secretFound:        make([]bool, 2),
		hudMessagesEnabled: true,
	}
	g.trackSecrets()
	if g.secretsFound != 1 {
		t.Fatalf("secretsFound=%d want=1 from cached sector", g.secretsFound)
	}
	if !g.secretFound[1] {
		t.Fatal("cached secret sector should be marked found")
	}
}

func TestApplySectorHazardDamage_UsesCachedPlayerSectorLikeDoomMobj(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 0}, {Special: 7}}},
		sectorFloor: []int64{0, 0},
		p: player{
			x:         0,
			y:         0,
			z:         0,
			floorz:    0,
			subsector: 0,
			sector:    1,
		},
		stats: playerStats{Health: 100},
	}
	g.applySectorHazardDamage()
	if g.stats.Health != 95 {
		t.Fatalf("health=%d want=95 from cached hazard sector", g.stats.Health)
	}
}

func TestApplySectorHazardDamage_UsesSectorFloorNotLocalSupportFloor(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 7}}},
		sectorFloor: []int64{-24 * fracUnit},
		p: player{
			z:         0,
			floorz:    0,
			subsector: 0,
			sector:    0,
		},
		stats: playerStats{Health: 100},
	}
	g.applySectorHazardDamage()
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100 when above sector floor", g.stats.Health)
	}
}

func TestTrackSecrets_UsesSectorFloorNotLocalSupportFloor(t *testing.T) {
	g := &game{
		m: &mapdata.Map{Sectors: []mapdata.Sector{{Special: 9}}},
		p: player{
			z:         0,
			floorz:    0,
			subsector: 0,
			sector:    0,
		},
		sectorFloor:        []int64{-24 * fracUnit},
		secretFound:        make([]bool, 1),
		hudMessagesEnabled: true,
	}
	g.trackSecrets()
	if g.secretsFound != 0 {
		t.Fatalf("secretsFound=%d want=0 when above sector floor", g.secretsFound)
	}
	if g.m.Sectors[0].Special != 9 {
		t.Fatalf("sector special=%d want=9 when above sector floor", g.m.Sectors[0].Special)
	}
}

func TestPickupRadSuitSetsTimer(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	msg, _, ok := g.applyPickup(2025, false)
	if !ok {
		t.Fatal("radsuit pickup should succeed")
	}
	if msg == "" {
		t.Fatal("radsuit pickup should return a message")
	}
	if g.inventory.RadSuitTics != 60*doomTicsPerSecond {
		t.Fatalf("radsuit tics=%d want=%d", g.inventory.RadSuitTics, 60*doomTicsPerSecond)
	}
}

func TestDamagePlayerSetsDeathStateAndFlash(t *testing.T) {
	g := &game{
		stats:              playerStats{Health: 5},
		soundQueue:         make([]soundEvent, 0, 2),
		hudMessagesEnabled: true,
	}
	g.damagePlayer(10, "ouch")
	if g.stats.Health != 0 {
		t.Fatalf("health=%d want=0", g.stats.Health)
	}
	if !g.isDead {
		t.Fatal("player should be dead at zero health")
	}
	if g.damageFlashTic == 0 {
		t.Fatal("damage flash should be active")
	}
	if g.useText != "You Died" {
		t.Fatalf("message=%q want=%q", g.useText, "You Died")
	}
	if got := len(g.soundQueue); got != 1 {
		t.Fatalf("soundQueue len=%d want=1", got)
	}
	if got := g.soundQueue[0]; got != soundEventPlayerDeath {
		t.Fatalf("sound event=%v want=%v", got, soundEventPlayerDeath)
	}
}

func TestDamagePlayerFromConsumesPlayerPainChancePRandomAndStartsPainState(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:      playerStats{Health: 100},
		soundQueue: make([]soundEvent, 0, 2),
	}
	_, before := doomrand.State()
	g.damagePlayerFrom(2, "ouch", 0, 0, false, -1)
	_, after := doomrand.State()
	if got := after - before; got != 1 {
		t.Fatalf("prnd advanced by %d want=1", got)
	}
	if g.playerMobjState != doomStatePlayerPain1 || g.playerMobjTics != 4 {
		t.Fatalf("player mobj state/tics=%d/%d want=%d/4", g.playerMobjState, g.playerMobjTics, doomStatePlayerPain1)
	}
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("soundQueue len=%d want=0 before A_Pain frame", got)
	}
}

func TestDamagePlayerFromSkipsPainStateWhenPainChanceRollFails(t *testing.T) {
	doomrand.SetState(0, 157) // next PRandom is table[158]=255
	g := &game{
		stats:      playerStats{Health: 100},
		soundQueue: make([]soundEvent, 0, 2),
	}
	g.damagePlayerFrom(2, "ouch", 0, 0, false, -1)
	if g.playerMobjState != 0 || g.playerMobjTics != 0 {
		t.Fatalf("player mobj state/tics=%d/%d want=0/0", g.playerMobjState, g.playerMobjTics)
	}
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("soundQueue len=%d want=0", got)
	}
}

func TestTickPlayerCounters_EmitsPainSoundOnSecondPainFrame(t *testing.T) {
	g := &game{
		playerMobjState: doomStatePlayerPain1,
		playerMobjTics:  1,
		soundQueue:      make([]soundEvent, 0, 2),
	}
	g.tickPlayerCounters()
	if g.playerMobjState != doomStatePlayerPain2 || g.playerMobjTics != 4 {
		t.Fatalf("player mobj state/tics=%d/%d want=%d/4", g.playerMobjState, g.playerMobjTics, doomStatePlayerPain2)
	}
	if got := len(g.soundQueue); got != 1 {
		t.Fatalf("soundQueue len=%d want=1", got)
	}
	if got := g.soundQueue[0]; got != soundEventPain {
		t.Fatalf("sound=%v want=%v", got, soundEventPain)
	}
}

func TestTickPlayerCounters_IncrementsBerserkFadeCounter(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			Strength:      true,
			StrengthCount: 1,
		},
	}
	g.tickPlayerCounters()
	if g.inventory.StrengthCount != 2 {
		t.Fatalf("strength count=%d want=2", g.inventory.StrengthCount)
	}
}

func TestDamagePlayerBlockedByInvulnerability(t *testing.T) {
	g := &game{
		stats:        playerStats{Health: 100},
		invulnerable: true,
	}
	g.damagePlayer(25, "ouch")
	if g.stats.Health != 100 {
		t.Fatalf("health=%d want=100", g.stats.Health)
	}
	if g.damageFlashTic != 0 {
		t.Fatalf("damage flash=%d want=0", g.damageFlashTic)
	}
}

func TestDamagePlayerFromAppliesThrustFromAttacker(t *testing.T) {
	g := &game{
		stats: playerStats{Health: 100},
		p: player{
			x: 0,
			y: 0,
		},
	}
	g.damagePlayerFrom(8, "ouch", -64*fracUnit, 0, true, -1)
	if g.p.momx <= 0 {
		t.Fatalf("momx=%d want > 0 after left-side hit", g.p.momx)
	}
	if abs(g.p.momy) > fracUnit/256 {
		t.Fatalf("momy=%d want near 0 for horizontal hit", g.p.momy)
	}
}

func TestPlayerDeathViewFallsTowardFloor(t *testing.T) {
	g := &game{
		p: player{
			z:      0,
			floorz: 0,
		},
		playerViewZ: 41 * fracUnit,
		isDead:      true,
	}
	g.tickPlayerViewHeight()
	if got := g.playerViewZ; got != 40*fracUnit {
		t.Fatalf("view z=%d want=%d after one tic", got, 40*fracUnit)
	}
	for i := 0; i < 80; i++ {
		g.tickPlayerViewHeight()
	}
	if got := g.playerViewZ; got != 0 {
		t.Fatalf("view z=%d want=%d at death floor target", got, 0)
	}
}

func TestRunGameplayTicDeadStillTicksWorldLogic(t *testing.T) {
	g := &game{
		m:      &mapdata.Map{},
		isDead: true,
		p: player{
			x:    0,
			y:    0,
			momx: 3 * fracUnit,
			momy: -2 * fracUnit,
		},
	}
	g.runGameplayTic(moveCmd{}, false, false)
	if got := g.worldTic; got != 1 {
		t.Fatalf("worldTic=%d want=1", got)
	}
	if g.p.momx == 0 && g.p.momy == 0 {
		t.Fatalf("dead player momentum should not be force-cleared, got momx=%d momy=%d", g.p.momx, g.p.momy)
	}
}

func TestRunGameplayTicDeadTurnsTowardAttacker(t *testing.T) {
	g := &game{
		m:      &mapdata.Map{},
		isDead: true,
		p: player{
			x:     0,
			y:     0,
			angle: doomAng5 * 2,
		},
		statusHasAttacker: true,
		statusAttackerX:   64 * fracUnit,
		statusAttackerY:   0,
	}
	g.runGameplayTic(moveCmd{}, false, false)
	if got, want := g.p.angle, uint32(doomAng5); got != want {
		t.Fatalf("angle=%d want=%d after one death turn tic", got, want)
	}
}

func TestRunGameplayTicDeadKeepsTurningAfterDamageFlashExpires(t *testing.T) {
	g := &game{
		m:      &mapdata.Map{},
		isDead: true,
		p: player{
			x:     0,
			y:     0,
			angle: doomAng5 * 3,
		},
		statusHasAttacker: true,
		statusAttackerX:   64 * fracUnit,
		statusAttackerY:   0,
		statusDamageCount: 1,
	}

	g.runGameplayTic(moveCmd{}, false, false)
	if got, want := g.p.angle, uint32(doomAng5*2); got != want {
		t.Fatalf("angle=%d want=%d after first death turn tic", got, want)
	}
	if !g.statusHasAttacker {
		t.Fatal("dead player should keep attacker latched after damage flash expires")
	}

	g.runGameplayTic(moveCmd{}, false, false)
	if got, want := g.p.angle, uint32(doomAng5); got != want {
		t.Fatalf("angle=%d want=%d after second death turn tic", got, want)
	}
}

func TestRunGameplayTicDeadTracksMovingAttackerThing(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3001, X: 64, Y: 0}},
		},
		isDead: true,
		p: player{
			x:     0,
			y:     0,
			angle: doomAng5 * 3,
		},
		thingX:              []int64{64 * fracUnit},
		thingY:              []int64{0},
		statusHasAttacker:   true,
		statusAttackerX:     64 * fracUnit,
		statusAttackerY:     0,
		statusAttackerThing: 0,
	}

	g.runGameplayTic(moveCmd{}, false, false)
	if got, want := g.p.angle, uint32(doomAng5*2); got != want {
		t.Fatalf("angle=%d want=%d after first death turn tic", got, want)
	}

	g.setThingPosFixed(0, 64*fracUnit, 64*fracUnit)
	g.runGameplayTic(moveCmd{}, false, false)
	if got, want := g.p.angle, uint32(doomAng5*3); got != want {
		t.Fatalf("angle=%d want=%d after tracking moved attacker", got, want)
	}
}

func TestInitSectorLightEffects_ResetsSpecialsLikeDoom(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{Light: 160, Special: 1},
				{Light: 160, Special: 2},
				{Light: 160, Special: 3},
				{Light: 160, Special: 4},
				{Light: 160, Special: 8},
				{Light: 160, Special: 12},
				{Light: 160, Special: 13},
				{Light: 160, Special: 17},
			},
		},
	}
	g.initSectorLightEffects()
	if got := len(g.sectorLightFx); got != len(g.m.Sectors) {
		t.Fatalf("sectorLightFx len=%d want=%d", got, len(g.m.Sectors))
	}
	for i, s := range g.m.Sectors {
		want := int16(0)
		if i == 3 {
			want = 4
		}
		if s.Special != want {
			t.Fatalf("sector %d special=%d want=%d", i, s.Special, want)
		}
	}
}

func TestSectorLightGlow_TicksBetweenMinAndMax(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{Light: 160, Special: 8},
				{Light: 96},
			},
			Linedefs: []mapdata.Linedef{{SideNum: [2]int16{0, 1}}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
		},
	}
	g.initSectorLightEffects()
	g.tickSectorLightEffects()
	if got := g.m.Sectors[0].Light; got != 152 {
		t.Fatalf("glow first tick light=%d want=152", got)
	}
	for i := 0; i < 7; i++ {
		g.tickSectorLightEffects()
	}
	if got := g.m.Sectors[0].Light; got != 104 {
		t.Fatalf("glow lower bound bounce light=%d want=104", got)
	}
	if got := g.sectorLightFx[0].direction; got != 1 {
		t.Fatalf("glow direction=%d want=1 after lower-bound bounce", got)
	}
}

func TestSectorLightStrobeSync_StartsImmediately(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{Light: 160, Special: 12},
			},
		},
	}
	g.initSectorLightEffects()
	g.tickSectorLightEffects()
	if got := g.m.Sectors[0].Light; got != 0 {
		t.Fatalf("sync strobe first tick light=%d want=0", got)
	}
}
