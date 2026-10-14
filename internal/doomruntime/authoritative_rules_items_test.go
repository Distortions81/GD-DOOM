package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func authoritativeItemsTestGame(types ...int16) *game {
	g := &game{
		m:              &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}, SubSectors: []mapdata.SubSector{{}}},
		opts:           Options{Headless: true, SkillLevel: 3, GameMode: gameModeDeathmatch},
		subSectorSec:   []int{0},
		authorityRules: &authorityRulesState{},
	}
	g.initPhysics()
	g.initPlayerState()
	for _, typ := range types {
		th := mapdata.Thing{Type: typ, X: 64, Flags: skillMediumBits}
		i := g.appendRuntimeThing(th, false)
		g.thingSpawnPoint[i] = th
	}
	return g
}

func takeAuthorityTestPickup(g *game, i int) bool {
	x, y := g.thingPosFixed(i, g.m.Things[i])
	z, _, _ := g.thingSupportState(i, g.m.Things[i])
	return g.processThingPickupAtIndex(i, g.m.Things[i], x, y, z, playerRadius, playerHeight, false)
}

func TestAuthorityDeathmatchPickupRespawnsAfterThirtySeconds(t *testing.T) {
	g := authoritativeItemsTestGame(2001)
	g.bmapWidth, g.bmapHeight = 2, 1
	g.m.BlockMap = &mapdata.BlockMap{Width: 2, Height: 1}
	g.rebuildThingBlockmap()
	g.worldTic = 17
	if !takeAuthorityTestPickup(g, 0) || !g.thingCollected[0] || !g.inventory.Weapons[2001] {
		t.Fatal("weapon pickup did not consume its map object")
	}
	if takeAuthorityTestPickup(g, 0) {
		t.Fatal("collected weapon stayed available before the respawn delay")
	}
	// Sector movement and old cached placement must not move the spawnpoint.
	g.setThingPosFixed(0, 200*fracUnit, 0)
	g.setThingMomentum(0, fracUnit, fracUnit, fracUnit)
	g.sectorFloor[0] = 24 * fracUnit
	deadline := 17 + authorityItemRespawnTics
	g.worldTic = deadline - 1
	g.tickAuthoritativeItemRespawns()
	if !g.thingCollected[0] {
		t.Fatal("item respawned before 30 seconds elapsed")
	}
	g.worldTic++
	// The real authority end-of-tic hook owns this once-per-world operation.
	(&Authority{g: g}).finishAuthoritativeRulesTic()
	if g.thingCollected[0] || len(g.authorityItemRespawns) != 0 || g.thingBlockCell[0] != 0 {
		t.Fatal("due pickup did not become active in its original blockmap cell")
	}
	x, y := g.thingPosFixed(0, g.m.Things[0])
	z, floor, ceil := g.thingSupportState(0, g.m.Things[0])
	if x != 64*fracUnit || y != 0 || z != 24*fracUnit || floor != z || ceil != 128*fracUnit || g.prevThingZ[0] != z {
		t.Fatalf("respawn placement=(%d,%d,%d) support=(%d,%d)", x, y, z, floor, ceil)
	}
	if g.thingMomX[0] != 0 || g.thingMomY[0] != 0 || g.thingMomZ[0] != 0 {
		t.Fatal("respawn retained stale item momentum")
	}
	oldOrder := g.thingBlockOrder[0]
	g.tickAuthoritativeItemRespawns()
	if oldOrder != g.thingBlockOrder[0] {
		t.Fatal("already respawned item relinked a second time")
	}
	g.stats.Shells = 0
	if !takeAuthorityTestPickup(g, 0) || g.authorityItemRespawns[0] != uint64(deadline+authorityItemRespawnTics) {
		t.Fatal("second pickup did not start its own full respawn delay")
	}
}

func TestAuthorityPickupRespawnEligibility(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		typ                            int16
		mode                           string
		dropped, runtime, legacy, want bool
	}{
		{name: "ammo", typ: 2007, want: true},
		{name: "health", typ: 2014, want: true},
		{name: "armor", typ: 2015, want: true},
		{name: "weapon", typ: 2003, want: true},
		{name: "backpack", typ: 8, want: true},
		{name: "soul sphere", typ: 2013, want: true},
		{name: "radiation suit", typ: 2025, want: true},
		{name: "invulnerability", typ: 2022},
		{name: "invisibility", typ: 2024},
		{name: "dropped", typ: 2007, dropped: true},
		{name: "runtime", typ: 2007, runtime: true},
		{name: "coop", typ: 2007, mode: gameModeCoop},
		{name: "singleplayer", typ: 2007, mode: gameModeSingle},
		{name: "legacy deathmatch", typ: 2007, legacy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := authoritativeItemsTestGame(tc.typ)
			if tc.mode != "" {
				g.opts.GameMode = tc.mode
			}
			if tc.legacy {
				g.authorityRules = nil
			}
			g.thingDropped[0] = tc.dropped
			if tc.runtime {
				g.thingSpawnPoint[0] = mapdata.Thing{}
			}
			if !takeAuthorityTestPickup(g, 0) {
				t.Fatal("test pickup could not be collected")
			}
			if got := len(g.authorityItemRespawns) == 1; got != tc.want {
				t.Fatalf("scheduled=%v want=%v", got, tc.want)
			}
			g.worldTic = authorityItemRespawnTics
			g.tickAuthoritativeItemRespawns()
			if g.thingCollected[0] == tc.want {
				t.Fatalf("collected=%v want=%v", g.thingCollected[0], !tc.want)
			}
		})
	}
}

func TestAuthoritySimultaneousItemRespawnsUseMapOrder(t *testing.T) {
	g := authoritativeItemsTestGame(2014, 2014)
	if !takeAuthorityTestPickup(g, 1) || !takeAuthorityTestPickup(g, 0) {
		t.Fatal("pickups failed")
	}
	g.worldTic = authorityItemRespawnTics
	g.tickAuthoritativeItemRespawns()
	if g.thingBlockOrder[0] >= g.thingBlockOrder[1] || g.thingThinkerOrder[0] >= g.thingThinkerOrder[1] {
		t.Fatal("simultaneous respawn ordering depends on map iteration or pickup order")
	}
}
