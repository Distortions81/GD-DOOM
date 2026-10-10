package doomruntime

import (
	"bytes"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
)

func snapshotFixture(t *testing.T) (*Authority, *game, []byte) {
	t.Helper()
	a := testAuthority(t)
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	for range 10 {
		if err := a.Step(map[byte]demo.Tic{2: {Forward: 25}}); err != nil {
			t.Fatal(err)
		}
	}
	// Model an authoritative moving-sector update and projectile ownership.
	sec := a.players[2].p.sector
	a.g.sectorFloor[sec] += 8 * fracUnit
	a.players[2].p.floorz += 8 * fracUnit
	a.players[2].p.z += 8 * fracUnit
	a.players[2].p.teleportedThisTic = true
	a.players[2].useButtonDown = true
	a.g.projectiles = append(a.g.projectiles, projectile{
		kind: projectileRocket, sourcePlayer: true, sourcePlayerSlot: 2,
		sourcePlayerGeneration: 7, tracerPlayer: true, tracerPlayerSlot: 1,
		tracerPlayerGeneration: 9, order: 500,
	})
	for i, thing := range a.g.m.Things {
		if thing.Type == barrelThingType {
			a.g.authorityBarrelSources = map[int]authorityBarrelSource{i: {PlayerSlot: 2, PlayerGeneration: 7, Thing: -1}}
			break
		}
	}
	data, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	client := newGame(cloneMapForRestart(a.g.restartTemplate), Options{
		Headless: true, SourcePortMode: true, PlayerSlot: 1,
		SkillLevel: 3, NoMonsters: true,
	})
	return a, client, data
}

func TestAuthoritySnapshotRestoresViewerWorldAndPreservesClientSettings(t *testing.T) {
	a, client, data := snapshotFixture(t)
	client.gammaLevel, client.crtEnabled, client.showGrid = 3, true, true
	client.paletteLUTEnabled, client.hudMessagesEnabled = false, false
	for i := range client.m.Linedefs {
		client.m.Linedefs[i].Flags &^= mlMapped
	}
	beforeRNG, beforePlayRNG := doomrand.State()
	if err := client.applyAuthoritySnapshot(data); err != nil {
		t.Fatal(err)
	}
	if client.localSlot != 2 || client.worldTic != 10 || client.p != a.players[2].p {
		t.Fatalf("viewer/body/tic mismatch: slot=%d, tic=%d, body=%+v", client.localSlot, client.worldTic, client.p)
	}
	if !client.useButtonDown || client.currentMoveCmd != a.players[2].currentMoveCmd || client.inventory.ReadyWeapon != a.players[2].inventory.ReadyWeapon {
		t.Fatal("player activation/input state was omitted from baseline")
	}
	if !reflect.DeepEqual(client.sectorFloor, a.g.sectorFloor) || !reflect.DeepEqual(client.thingBlockOrder, a.g.thingBlockOrder) {
		t.Fatal("world collision state does not match baseline")
	}
	if len(client.remotePlayers) != 1 || client.remotePlayers[1].p != a.players[1].p || len(client.authorityPlayers) != 2 {
		t.Fatal("remote player roster was not restored")
	}
	if client.projectiles[0].sourcePlayerSlot != 2 || client.projectiles[0].sourcePlayerGeneration != 7 || client.projectiles[0].tracerPlayerSlot != 1 || client.projectiles[0].tracerPlayerGeneration != 9 {
		t.Fatal("projectile identity was not restored")
	}
	if len(client.authorityBarrelSources) != 1 || !reflect.DeepEqual(client.authorityBarrelSources, a.g.authorityBarrelSources) {
		t.Fatal("delayed barrel ownership was not restored")
	}
	for i := range client.authorityBarrelSources {
		client.authorityBarrelSources[i] = authorityBarrelSource{}
		if a.g.authorityBarrelSources[i].PlayerSlot != 2 {
			t.Fatal("barrel source maps alias")
		}
	}
	if !client.opts.SourcePortMode || client.gammaLevel != 3 || !client.crtEnabled || !client.showGrid || client.paletteLUTEnabled || client.hudMessagesEnabled {
		t.Fatal("server baseline replaced client presentation preferences")
	}
	mapped := 0
	for _, line := range client.m.Linedefs {
		if line.Flags&mlMapped != 0 {
			mapped++
		}
	}
	if mapped == 0 {
		t.Fatal("confirmed player surroundings were not revealed on the automap")
	}
	if after, playAfter := doomrand.State(); after != beforeRNG || playAfter != beforePlayRNG {
		t.Fatal("applying baseline changed RNG")
	}
	// Neither the encoded bytes nor the decoded inventory aliases server state.
	client.inventory.Weapons[2001] = true
	if a.players[2].inventory.Weapons[2001] {
		t.Fatal("client inventory aliases authority")
	}
	data[0] ^= 1
	if client.localSlot != 2 {
		t.Fatal("client state aliases received bytes")
	}
}

func TestAuthoritySnapshotCaptureDoesNotMutateSimulation(t *testing.T) {
	a, _, _ := snapshotFixture(t)
	before := captureGameSaveState(a.g)
	player := a.g.captureAuthoritativePlayer()
	rng, playRNG := doomrand.State()
	first, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same state produced different full baselines")
	}
	if !reflect.DeepEqual(before, captureGameSaveState(a.g)) || !reflect.DeepEqual(player, a.g.captureAuthoritativePlayer()) {
		t.Fatal("snapshot capture changed server state")
	}
	if after, playAfter := doomrand.State(); after != rng || playAfter != playRNG {
		t.Fatal("capture changed RNG")
	}
	first[0] ^= 1
	if bytes.Equal(first, second) {
		t.Fatal("snapshot buffers alias")
	}
}

func TestAuthoritySnapshotRejectsInvalidDataWithoutMutation(t *testing.T) {
	_, client, valid := snapshotFixture(t)
	before := captureGameSaveState(client)
	check := func(data []byte) {
		t.Helper()
		if err := client.applyAuthoritySnapshot(data); err == nil {
			t.Fatal("accepted invalid baseline")
		}
		if !reflect.DeepEqual(before, captureGameSaveState(client)) {
			t.Fatal("invalid baseline mutated client")
		}
	}
	for _, length := range []int{0, 1, len(valid) / 2, len(valid) - 1} {
		check(valid[:length])
	}
	corrupt := bytes.Clone(valid)
	corrupt[len(corrupt)/2] ^= 1
	check(corrupt)
	check(make([]byte, MaxAuthoritySnapshotBytes+1))
	mutations := []func(*authorityReplica){
		func(r *authorityReplica) { r.Version++ },
		func(r *authorityReplica) { r.MapHash[0] ^= 1 },
		func(r *authorityReplica) { r.Game.ThingX = nil },
		func(r *authorityReplica) { r.Game.SectorFloor = r.Game.SectorFloor[:1] },
		func(r *authorityReplica) { r.Players[1].LocalSlot = r.Players[0].LocalSlot },
		func(r *authorityReplica) { r.Players[1].P.X = 1 << 40 },
		func(r *authorityReplica) { r.Viewer = 4; r.Game.Session.PlayerSlot = 4 },
		func(r *authorityReplica) { r.ProjectilePlayers = nil },
		func(r *authorityReplica) {
			r.BarrelSources = map[int]authorityBarrelSource{-1: {PlayerSlot: 1, Thing: -1}}
		},
	}
	for _, mutate := range mutations {
		r, err := decodeAuthorityReplica(valid)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&r)
		data, err := encodeAuthorityReplica(r)
		if err != nil {
			t.Fatal(err)
		}
		check(data)
	}
}

func TestReplicaPlayerIncludesEveryActivationField(t *testing.T) {
	a, _, data := snapshotFixture(t)
	r, err := decodeAuthorityReplica(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, wire := range r.Players {
		want := *a.players[wire.LocalSlot]
		got := restoreReplicaPlayer(wire)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("player %d activation state differs after wire roundtrip", wire.LocalSlot)
		}
	}
}
