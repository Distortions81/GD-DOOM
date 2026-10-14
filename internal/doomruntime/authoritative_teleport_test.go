package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestAuthoritativeTeleportStompsOtherPlayerAndCreditsTeleporter(t *testing.T) {
	for _, blockmap := range []bool{false, true} {
		t.Run(map[bool]string{false: "linear", true: "blockmap"}[blockmap], func(t *testing.T) {
			g, victim, teleporter := authorityCombatTestWorld()
			g.opts.GameMode = gameModeDeathmatch
			g.authorityRules = &authorityRulesState{}
			g.authorityRules.Scores[1].Generation, g.authorityRules.Scores[2].Generation = 1, 1
			g.inventory.InvulnTics = 100
			*victim = g.captureAuthoritativePlayer()
			if blockmap {
				g.bmapWidth, g.bmapHeight = 1, 1
				g.m.BlockMap = &mapdata.BlockMap{Width: 1, Height: 1}
				g.thingBlockCells = [][]int{{}}
			}
			g.withAuthoritativePlayer(teleporter, func() {
				if !g.teleportStompDestinationThings(0, 0, playerRadius, -1, true, g.p.x, g.p.y) {
					t.Fatal("player teleport rejected stompable destination")
				}
				if g.localSlot != 2 || g.stats.Health != 100 {
					t.Fatal("stomping another player damaged the teleporter")
				}
			})
			if !victim.isDead || !g.isDead || g.authorityRules.Scores[2].Frags != 1 || g.authorityRules.Scores[1].Deaths != 1 {
				t.Fatalf("telefrag result dead=%v scores=%+v", victim.isDead, g.authorityRules.Scores)
			}
			if g.authorityDamageSource != 0 {
				t.Fatal("telefrag source leaked into subsequent world damage")
			}
		})
	}
}

func TestAuthoritativeTeleportProtectedTeammateBlocksWithoutDamage(t *testing.T) {
	g, teleporter, teammate := authorityCombatTestWorld()
	g.opts.GameMode = gameModeCoop
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation, g.authorityRules.Scores[2].Generation = 1, 1
	if g.teleportStompDestinationThings(teammate.p.x, 0, playerRadius, -1, true, 0, 0) {
		t.Fatal("protected teammate allowed overlapping teleport")
	}
	if teammate.stats.Health != 100 || teleporter.stats.Health != 100 {
		t.Fatal("rejected co-op teleport damaged a player")
	}
	g.authorityRules.Config.FriendlyFire = true
	if !g.teleportStompDestinationThings(teammate.p.x, 0, playerRadius, -1, true, 0, 0) || !teammate.isDead {
		t.Fatal("enabled friendly fire did not permit telefrag")
	}
	// A vacated or never-joined map start remains just a marker.
	g.authorityPlayers = []*authoritativePlayerState{teleporter}
	if !g.teleportStompDestinationThings(64*fracUnit, 0, playerRadius, -1, true, 0, 0) || g.stats.Health != 100 {
		t.Fatal("absent player start became a telefrag victim")
	}
}

func TestAuthoritativeMonsterTeleportRespectsRemotePlayer(t *testing.T) {
	g, _, remote := authorityCombatTestWorld()
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation, g.authorityRules.Scores[2].Generation = 1, 1
	if g.teleportStompDestinationThings(remote.p.x, 0, 20*fracUnit, 9, false, 256*fracUnit, 0) || remote.stats.Health != 100 {
		t.Fatal("ordinary monster teleport ignored remote player occupant")
	}
	g.m.Name = "MAP30"
	if !g.teleportStompDestinationThings(remote.p.x, 0, 20*fracUnit, 9, false, 256*fracUnit, 0) || !remote.isDead {
		t.Fatal("MAP30 monster telefrag did not kill remote player")
	}
	if g.authorityRules.Scores[1].Frags != 0 {
		t.Fatal("monster telefrag credited the active anchor")
	}
}
