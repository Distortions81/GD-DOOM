package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/mapdata"
)

func TestAuthoritativePlayerCollisionUsesOnlyLiveBodies(t *testing.T) {
	g := newAuthoritativePlayersTestGame()
	first := g.captureAuthoritativePlayer()
	second := secondAuthoritativeTestPlayer(g)
	g.authorityPlayers = []*authoritativePlayerState{&first, &second}
	if !g.actorBlockedByThings(64*fracUnit, 0, playerRadius, -1, false) {
		t.Fatal("player can move through another live player")
	}
	if g.actorBlockedByThings(0, 0, playerRadius, -1, false) {
		t.Fatal("player collides with its own body")
	}
	if !g.actorBlockedByThings(64*fracUnit, 0, 20*fracUnit, 7, true) {
		t.Fatal("monster can move through the non-anchor player")
	}
	// The current active body may have moved since its roster checkpoint.
	g.p.x = 128 * fracUnit
	if !g.actorBlockedByThings(128*fracUnit, 0, 20*fracUnit, 7, true) || g.actorBlockedByThings(0, 0, 20*fracUnit, 7, true) {
		t.Fatal("monster collision used a stale anchor body")
	}
	second.isDead = true
	if g.actorBlockedByThings(64*fracUnit, 0, playerRadius, -1, false) {
		t.Fatal("dead player is still solid")
	}
	second.isDead = false
	g.authorityPlayers = []*authoritativePlayerState{&first}
	if g.actorBlockedByThings(64*fracUnit, 0, playerRadius, -1, false) {
		t.Fatal("absent slot's map start became a ghost collider")
	}
}

func TestAuthoritativeSectorMovementClipsEveryPlayer(t *testing.T) {
	for _, blockmap := range []bool{false, true} {
		t.Run(map[bool]string{false: "without blockmap", true: "with blockmap"}[blockmap], func(t *testing.T) {
			g := newAuthoritativePlayersTestGame()
			first := g.captureAuthoritativePlayer()
			second := secondAuthoritativeTestPlayer(g)
			// Distinct cached supports exercise per-player oldFloor handling.
			second.p.floorz, second.p.z = 8*fracUnit, 8*fracUnit
			g.authorityPlayers = []*authoritativePlayerState{&first, &second}
			if blockmap {
				g.bmapWidth, g.bmapHeight = 1, 1
				g.m.BlockMap = &mapdata.BlockMap{Width: 1, Height: 1}
				g.thingBlockCells = [][]int{{}}
				g.sectorBBox = []worldBBox{{minX: 0, minY: 0, maxX: 127, maxY: 127}}
			}
			if g.setSectorFloorHeightWithCrush(0, 16*fracUnit, false) {
				t.Fatal("open rising floor incorrectly blocked")
			}
			if first.p.z != 16*fracUnit || second.p.z != 16*fracUnit || first.p.floorz != 16*fracUnit || second.p.floorz != 16*fracUnit {
				t.Fatalf("floor did not carry both players: first=%+v second=%+v", first.p, second.p)
			}
			g.worldTic = 1 // a crusher damage pulse
			if !g.setSectorCeilingHeightWithCrush(0, 24*fracUnit, true) {
				t.Fatal("crusher did not report blocked players")
			}
			if first.stats.Health != 90 || second.stats.Health != 90 || g.stats.Health != 90 {
				t.Fatalf("crusher damage must apply once per body: first=%d second=%d active=%d", first.stats.Health, second.stats.Health, g.stats.Health)
			}
			if first.p.ceilz != 24*fracUnit || second.p.ceilz != 24*fracUnit {
				t.Fatal("ceiling support was not refreshed for every player")
			}
		})
	}
}

func TestAuthoritativeActorBlockLinksInterleaveBodiesAndThings(t *testing.T) {
	g := newAuthoritativePlayersTestGame()
	first := g.captureAuthoritativePlayer()
	second := secondAuthoritativeTestPlayer(g)
	second.playerBlockOrder = 3
	g.authorityPlayers = []*authoritativePlayerState{&first, &second}
	g.m.Things = append(g.m.Things, mapdata.Thing{Type: 25, X: 32})
	g.bmapWidth, g.bmapHeight = 1, 1
	g.thingBlockCells = [][]int{{2}}
	g.thingBlockCell = []int{-1, -1, 0}
	g.thingBlockOrder = []int64{1, 3, 2}
	var visits []int
	g.walkAuthoritativeActorBlockCell(0, func(i int) {
		visits = append(visits, 100+i)
	}, func() {
		visits = append(visits, g.localSlot)
	})
	if want := []int{2, 102, 1}; !reflect.DeepEqual(visits, want) {
		t.Fatalf("blocklink order=%v want=%v", visits, want)
	}
	if g.localSlot != 1 {
		t.Fatal("actor traversal changed the selected player")
	}
}
