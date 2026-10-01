package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestLevelStatsUseOriginalCountFlagsAndSpawnFiltering(t *testing.T) {
	g := &game{opts: Options{SkillLevel: 3}, m: &mapdata.Map{Things: []mapdata.Thing{
		{Type: 3001, Flags: skillMediumBits}, {Type: 72, Flags: skillMediumBits},
		{Type: 3006, Flags: skillMediumBits}, {Type: 3001, Flags: skillHardBits},
		{Type: 2014, Flags: skillMediumBits}, {Type: 2026, Flags: skillMediumBits},
		{Type: 2025, Flags: skillMediumBits}, {Type: 2007, Flags: skillMediumBits},
		{Type: 5, Flags: skillMediumBits}, {Type: 2001, Flags: skillMediumBits},
		{Type: 2015, Flags: skillHardBits},
	}}, thingCollected: make([]bool, 11)}
	g.applyThingSpawnFiltering()
	g.initLevelStats()
	if g.levelKillsTotal != 2 || g.levelItemsTotal != 2 {
		t.Fatalf("map totals=%d kills/%d items, want 2/2", g.levelKillsTotal, g.levelItemsTotal)
	}
}

func TestLevelStatsCreditInfightingAndRepeatedDeaths(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	for _, tc := range []struct {
		mode   string
		player bool
		want   int
	}{
		{gameModeSingle, false, 2}, {gameModeCoop, false, 0}, {gameModeCoop, true, 2},
	} {
		g := &game{opts: Options{GameMode: tc.mode}, m: &mapdata.Map{
			Things:  []mapdata.Thing{{Type: 3001}, {Type: 3006}},
			Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		}, thingHP: []int{60, 100}, thingCollected: make([]bool, 2), levelKillsTotal: 1}
		g.initPhysics()
		g.ensureMonsterAIState()
		g.damageMonsterFrom(0, 60, tc.player, -1, 0, 0, false)
		g.damageMonsterFrom(0, 60, tc.player, -1, 0, 0, false) // Already dead: no extra credit.
		// A restored monster can be killed again without changing map totals.
		g.thingHP[0], g.thingDead[0] = 60, false
		g.damageMonsterFrom(0, 60, tc.player, -1, 0, 0, false)
		g.damageMonsterFrom(1, 100, tc.player, -1, 0, 0, false) // Lost Souls do not count.
		if g.playerKillCount != tc.want || g.levelKillsTotal != 1 {
			t.Fatalf("mode=%s player source=%v count=%d total=%d want %d/1", tc.mode, tc.player, g.playerKillCount, g.levelKillsTotal, tc.want)
		}
		if tc.want == 2 && collectIntermissionStats(g, "MAP01", "MAP02").KillsPct != 200 {
			t.Fatal("repeated counted deaths must produce 200%, as in the original")
		}
	}
}

func TestLevelStatsCountOnlySuccessfullyConsumedCountItems(t *testing.T) {
	g := &game{opts: Options{SkillLevel: 3}, m: &mapdata.Map{
		Things: []mapdata.Thing{{Type: 2014, Flags: skillMediumBits}, {Type: 2007, Flags: skillMediumBits}, {Type: 2013, Flags: skillMediumBits}},
	}, thingCollected: make([]bool, 3)}
	g.initPlayerState()
	g.initLevelStats()
	if !g.processThingPickupAtIndex(0, g.m.Things[0], 0, 0, 0, playerRadius, playerHeight, false) || g.playerItemCount != 1 {
		t.Fatalf("health bonus count=%d want 1", g.playerItemCount)
	}
	if g.processThingPickupAtIndex(0, g.m.Things[0], 0, 0, 0, playerRadius, playerHeight, false) {
		t.Fatal("removed bonus must not be collected twice")
	}
	if !g.processThingPickupAtIndex(1, g.m.Things[1], 0, 0, 0, playerRadius, playerHeight, false) || g.playerItemCount != 1 {
		t.Fatal("ammo pickup must not increase item count")
	}
	if g.processThingPickupAtIndex(2, g.m.Things[2], 128*fracUnit, 0, 0, playerRadius, playerHeight, false) || g.playerItemCount != 1 {
		t.Fatal("unreachable soul sphere must not increase item count")
	}
	stats := collectIntermissionStats(g, "MAP01", "MAP02")
	if stats.ItemsTotal != 2 || stats.ItemsFound != 1 || stats.ItemsPct != 50 {
		t.Fatalf("item stats=%+v want 1/2=50%%", stats)
	}
}
