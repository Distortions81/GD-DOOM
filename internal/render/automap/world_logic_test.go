package automap

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
	for i := 0; i < 31; i++ {
		g.tickWorldLogic()
	}
	if g.stats.Health != 100 {
		t.Fatalf("health before period=%d want=100", g.stats.Health)
	}
	g.tickWorldLogic()
	if g.stats.Health != 90 {
		t.Fatalf("health after period=%d want=90", g.stats.Health)
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
		g.tickWorldLogic()
	}
	if g.stats.Health != 100 {
		t.Fatalf("health with suit=%d want=100", g.stats.Health)
	}
	if g.inventory.RadSuitTics != 8 {
		t.Fatalf("radsuit tics=%d want=8", g.inventory.RadSuitTics)
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
		g.tickWorldLogic()
	}
	if g.stats.Health != 80 {
		t.Fatalf("health=%d want=80", g.stats.Health)
	}
}

func TestPickupRadSuitSetsTimer(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	msg, _, ok := g.applyPickup(2025)
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

func TestUpdatePlayerDeadStillTicksWorldLogic(t *testing.T) {
	g := &game{
		m:      &mapdata.Map{},
		isDead: true,
		p: player{
			momx: 3 * fracUnit,
			momy: -2 * fracUnit,
		},
	}
	g.updatePlayer(moveCmd{})
	if got := g.worldTic; got != 1 {
		t.Fatalf("worldTic=%d want=1", got)
	}
	if g.p.momx != 0 || g.p.momy != 0 {
		t.Fatalf("dead player momentum should clear, got momx=%d momy=%d", g.p.momx, g.p.momy)
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
