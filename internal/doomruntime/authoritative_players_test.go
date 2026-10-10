package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func newAuthoritativePlayersTestGame() *game {
	g := &game{
		m: &mapdata.Map{
			Sectors:    []mapdata.Sector{{CeilingHeight: 128}},
			SubSectors: []mapdata.SubSector{{}},
			Things:     []mapdata.Thing{{Type: 1}, {Type: 2, X: 64}},
		},
		localSlot:             1,
		localPlayerThingIndex: 0,
		playerBlockOrder:      1,
		nextBlockmapOrder:     10,
		subSectorSec:          []int{0},
		p: player{
			ceilz: 128 * fracUnit, viewHeight: playerViewHeight,
			subsector: -1, sector: -1,
		},
	}
	g.initPhysics()
	g.initPlayerState()
	g.clearWeaponOverlay()
	return g
}

func secondAuthoritativeTestPlayer(g *game) authoritativePlayerState {
	p := g.captureAuthoritativePlayer()
	p.localSlot = 2
	p.localPlayerThingIndex = 1
	p.playerBlockOrder = 2
	p.p.x = 64 * fracUnit
	return p
}

func TestAuthoritativePlayerActivationKeepsIndependentState(t *testing.T) {
	g := newAuthoritativePlayersTestGame()
	g.stats.Health = 91
	g.inventory.BlueKey = true
	g.inventory.Weapons[2001] = true
	g.weaponStateTics = 7
	g.useButtonDown = true
	g.statusAttackerThing = 9
	g.playerMobjHealth = 91
	first := g.captureAuthoritativePlayer()
	second := secondAuthoritativeTestPlayer(g)
	second.stats.Health = 23
	second.stats.Bullets = 2
	second.inventory.BlueKey = false
	second.inventory.Weapons[2001] = false
	second.weaponStateTics = 2
	second.useButtonDown = false
	second.statusAttackerThing = 4
	second.playerMobjHealth = 23

	g.withAuthoritativePlayer(&second, func() {
		if g.stats.Health != 23 || g.stats.Bullets != 2 || g.inventory.BlueKey || g.inventory.Weapons[2001] || g.useButtonDown || g.weaponStateTics != 2 || g.statusAttackerThing != 4 || g.playerMobjHealth != 23 {
			t.Fatal("activation mixed independent player fields")
		}
		g.stats.Health--
		g.inventory.Weapons[2003] = true
		g.useButtonDown = true
	})
	if got := g.captureAuthoritativePlayer(); !reflect.DeepEqual(got, first) {
		t.Fatalf("switching back changed the first player:\n got %+v\nwant %+v", got, first)
	}
	if second.stats.Health != 22 || !second.inventory.Weapons[2003] || !second.useButtonDown {
		t.Fatal("second player's mutations were lost")
	}
	// A snapshot is an owned value, including the inventory map.
	g.inventory.Weapons[2005] = true
	if first.inventory.Weapons[2005] || second.inventory.Weapons[2005] {
		t.Fatal("player snapshots alias the runtime inventory")
	}
}

func TestAuthoritativePlayerActivationPreservesWorldDamage(t *testing.T) {
	g := newAuthoritativePlayersTestGame()
	first := g.captureAuthoritativePlayer()
	second := secondAuthoritativeTestPlayer(g)
	// World thinkers may damage the active anchor after its input phase.
	g.stats.Health = 40
	g.withAuthoritativePlayer(&second, func() { g.stats.Health = 80 })
	g.withAuthoritativePlayer(&first, func() { g.stats.Health-- })
	if first.stats.Health != 39 || second.stats.Health != 80 {
		t.Fatalf("world damage was overwritten: first=%d second=%d", first.stats.Health, second.stats.Health)
	}
}

func TestAuthoritativeNestedPlayerActivationPreservesOtherPlayerChanges(t *testing.T) {
	g := newAuthoritativePlayersTestGame()
	first := g.captureAuthoritativePlayer()
	second := secondAuthoritativeTestPlayer(g)
	g.authorityPlayers = []*authoritativePlayerState{&first, &second}
	g.withAuthoritativePlayer(&second, func() {
		g.stats.Health = 80
		g.withAuthoritativePlayer(&first, func() { g.stats.Health = 40 })
		if g.stats.Health != 80 {
			t.Fatal("nested switch lost the active second player's changes")
		}
	})
	if g.stats.Health != 40 || first.stats.Health != 40 || second.stats.Health != 80 {
		t.Fatalf("nested switch restored stale state: active=%d first=%d second=%d", g.stats.Health, first.stats.Health, second.stats.Health)
	}
}

func TestAuthoritativePlayersAdvanceWorldOnce(t *testing.T) {
	g := newAuthoritativePlayersTestGame()
	first := g.captureAuthoritativePlayer()
	second := secondAuthoritativeTestPlayer(g)
	first.inventory.InvulnTics = 7
	second.inventory.InvulnTics = 3
	g.doors[0] = &doorThinker{order: 3, sector: 0, typ: doorOpen, direction: 1, speed: fracUnit, topHeight: 200 * fracUnit}
	if err := g.runAuthoritativePlayersTic([]*authoritativePlayerState{&second, &first}, map[int]DemoTic{1: {Forward: 25}, 2: {Forward: -25}}); err != nil {
		t.Fatal(err)
	}
	if g.worldTic != 1 || g.sectorCeil[0] != 129*fracUnit {
		t.Fatalf("world advanced more than once: tic=%d ceiling=%d", g.worldTic, g.sectorCeil[0])
	}
	if first.p.x <= 0 || second.p.x >= 64*fracUnit {
		t.Fatalf("both bodies must move: first=%d second=%d", first.p.x, second.p.x)
	}
	if first.inventory.InvulnTics != 6 || second.inventory.InvulnTics != 2 {
		t.Fatalf("player counters advanced incorrectly: first=%d second=%d", first.inventory.InvulnTics, second.inventory.InvulnTics)
	}
	if g.localSlot != 1 || g.p != first.p {
		t.Fatal("selected player viewpoint was not preserved")
	}
}

func TestAuthoritativeSinglePlayerMatchesLegacyTic(t *testing.T) {
	commands := []DemoTic{{Forward: 25}, {Side: 12, AngleTurn: 128}, {}, {Forward: -25}, {Buttons: 1}, {}, {}}
	legacy := newAuthoritativePlayersTestGame()
	authority := newAuthoritativePlayersTestGame()
	player := authority.captureAuthoritativePlayer()
	for _, g := range []*game{legacy, authority} {
		g.doors[0] = &doorThinker{order: 3, sector: 0, typ: doorOpen, direction: 1, speed: fracUnit, topHeight: 200 * fracUnit}
	}
	for i, tc := range commands {
		// The legacy runtime has process-global RNG. Give each path exactly
		// the same state; these tests intentionally never use t.Parallel.
		doomrand.Clear()
		cmd, use, fire := demoTicCommand(tc)
		legacy.runGameplayTic(cmd, use, fire)
		doomrand.Clear()
		if err := authority.runAuthoritativePlayersTic([]*authoritativePlayerState{&player}, map[int]DemoTic{1: tc}); err != nil {
			t.Fatal(err)
		}
		if got, want := authority.captureAuthoritativePlayer(), legacy.captureAuthoritativePlayer(); !reflect.DeepEqual(got, want) {
			t.Fatalf("tic %d changed single-player behavior:\n got %+v\nwant %+v", i, got, want)
		}
		if authority.worldTic != legacy.worldTic || !reflect.DeepEqual(authority.doors, legacy.doors) || !reflect.DeepEqual(authority.sectorCeil, legacy.sectorCeil) {
			t.Fatalf("tic %d changed world thinker behavior", i)
		}
	}
}

func TestAuthoritativePlayerBodiesKeepMapThinkerOrder(t *testing.T) {
	g := newAuthoritativePlayersTestGame()
	first := g.captureAuthoritativePlayer()
	second := secondAuthoritativeTestPlayer(g)
	// Body order follows the map's starts even when it differs from slot
	// order. Input order remains player-slot order independently.
	g.m.Things = []mapdata.Thing{{Type: 2}, {Type: 1}}
	first.localPlayerThingIndex = 1
	second.localPlayerThingIndex = 0
	roster := []*authoritativePlayerState{&first, &second}
	ref, ok := g.nextWorldThinkerAfterForPlayers(0, roster)
	if !ok || ref.kind != worldThinkerPlayer || ref.key != 1 || ref.order != 1 {
		t.Fatalf("first thinker=%+v ok=%t", ref, ok)
	}
	ref, ok = g.nextWorldThinkerAfterForPlayers(ref.order, roster)
	if !ok || ref.kind != worldThinkerPlayer || ref.key != 0 || ref.order != 2 {
		t.Fatalf("second thinker=%+v ok=%t", ref, ok)
	}
}

func TestAuthoritativePlayersRejectInvalidRosterWithoutAdvancing(t *testing.T) {
	for _, name := range []string{"duplicate slot", "duplicate body order", "inactive command", "empty", "nil player"} {
		t.Run(name, func(t *testing.T) {
			g := newAuthoritativePlayersTestGame()
			first := g.captureAuthoritativePlayer()
			second := secondAuthoritativeTestPlayer(g)
			roster := []*authoritativePlayerState{&first, &second}
			commands := map[int]DemoTic{}
			switch name {
			case "duplicate slot":
				second.localSlot = first.localSlot
			case "duplicate body order":
				second.localPlayerThingIndex = first.localPlayerThingIndex
			case "inactive command":
				commands[3] = DemoTic{}
			case "empty":
				roster = nil
			case "nil player":
				roster[0] = nil
			}
			before := g.captureAuthoritativePlayer()
			if err := g.runAuthoritativePlayersTic(roster, commands); err == nil {
				t.Fatal("invalid roster accepted")
			}
			if g.worldTic != 0 || !reflect.DeepEqual(g.captureAuthoritativePlayer(), before) {
				t.Fatal("invalid roster mutated the simulation")
			}
		})
	}
}
