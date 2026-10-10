package doomruntime

import (
	"errors"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
)

func predictionTestMap() *mapdata.Map {
	m := &mapdata.Map{
		Name: "MAP01", Things: []mapdata.Thing{{Type: 1}},
		Vertexes: []mapdata.Vertex{{X: -128, Y: -128}, {X: -128, Y: 128}, {X: 128, Y: 128}, {X: 128, Y: -128}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}}, Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		SubSectors: []mapdata.SubSector{{SegCount: 4}},
	}
	for i := range 4 {
		m.Linedefs = append(m.Linedefs, mapdata.Linedef{V1: uint16(i), V2: uint16((i + 1) % 4), Flags: mlBlocking, SideNum: [2]int16{0, -1}})
		m.Segs = append(m.Segs, mapdata.Seg{StartVertex: uint16(i), EndVertex: uint16((i + 1) % 4), Linedef: uint16(i)})
	}
	return m
}

func predictionTestWorld(t *testing.T) (*Authority, *ClientPrediction) {
	t.Helper()
	m := predictionTestMap()
	a, err := NewAuthority(m, Options{SkillLevel: 3, NoMonsters: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	g := newGame(cloneMapForRestart(m), Options{Headless: true, SkillLevel: 3, NoMonsters: true})
	p, err := newClientPrediction(g, netgame.Welcome{Epoch: 7, PlayerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Reconcile(predictionSnapshot(t, a, 1, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	return a, p
}

func predictionSnapshot(t *testing.T, a *Authority, id uint32, ack netgame.InputAck) netgame.Snapshot {
	t.Helper()
	data, err := a.Snapshot(1)
	if err != nil {
		t.Fatal(err)
	}
	return netgame.Snapshot{Epoch: 7, ID: id, Tick: a.Tic(), Finalized: ack, State: data}
}

func TestPredictionReconcilesWallAndExpiresLostCommands(t *testing.T) {
	a, p := predictionTestWorld(t)
	for tic := uint32(1); tic <= 40; tic++ {
		if err := p.Predict(netgame.Input{Sequence: tic, Tick: tic, Command: demo.Tic{Forward: 50}}); err != nil {
			t.Fatal(err)
		}
	}
	if p.g.worldTic != 0 || p.PredictedTic() != 40 {
		t.Fatal("prediction advanced authoritative world clock")
	}
	// Missing packets finalize as neutral on the server, even though all forty
	// commands were predicted locally. Correct a body that has drifted into a wall.
	for range 20 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := predictionSnapshot(t, a, 2, netgame.InputAck{HasTick: true, Tick: 20, HasSequence: true, Sequence: 5})
	p.g.p.x = 140 * fracUnit
	if applied, err := p.Reconcile(snapshot); err != nil || !applied {
		t.Fatalf("reconcile: applied=%v err=%v", applied, err)
	}
	for tic := uint32(21); tic <= 40; tic++ {
		if err := a.Step(map[byte]demo.Tic{1: {Forward: 50}}); err != nil {
			t.Fatal(err)
		}
	}
	if p.g.p != a.players[1].p {
		t.Fatalf("replayed body differs from server: got=%+v want=%+v", p.g.p, a.players[1].p)
	}
	if p.g.p.x > (128-playerRadius/fracUnit)*fracUnit {
		t.Fatal("correction replay penetrated wall")
	}
	if len(p.history) != 20 || p.history[0].Tick != 21 || p.AuthoritativeTic() != 20 || p.PredictedTic() != 40 || p.g.worldTic != 20 {
		t.Fatalf("expired deadline/history clocks incorrect: pending=%d auth=%d predicted=%d", len(p.history), p.AuthoritativeTic(), p.PredictedTic())
	}
}

func TestPredictionIgnoresStaleAndRejectsInvalidSnapshotsWithoutMutation(t *testing.T) {
	a, p := predictionTestWorld(t)
	old := predictionSnapshot(t, a, 1, netgame.InputAck{})
	if err := p.Predict(netgame.Input{Sequence: 1, Tick: 1, Command: demo.Tic{Forward: 25}}); err != nil {
		t.Fatal(err)
	}
	body, history := p.g.p, p.PendingInputs()
	if applied, err := p.Reconcile(old); err != nil || applied || p.g.p != body {
		t.Fatal("stale snapshot changed prediction")
	}
	bad := old
	bad.ID = 2
	bad.Epoch++
	if _, err := p.Reconcile(bad); err == nil {
		t.Fatal("accepted wrong epoch")
	}
	bad = old
	bad.ID = 2
	bad.Finalized = netgame.InputAck{HasTick: true, Tick: 0, HasSequence: true, Sequence: 2}
	if _, err := p.Reconcile(bad); err == nil {
		t.Fatal("accepted acknowledgment for unsent input")
	}
	bad = old
	bad.ID = 2
	bad.State = append([]byte(nil), old.State...)
	bad.State[len(bad.State)/2] ^= 1
	if _, err := p.Reconcile(bad); err == nil {
		t.Fatal("accepted corrupt snapshot")
	}
	if p.g.p != body || !reflect.DeepEqual(history, p.PendingInputs()) {
		t.Fatal("rejected snapshot mutated prediction")
	}
}

func TestPredictionClearsHistoryAcrossRespawnAndTeleport(t *testing.T) {
	for _, teleport := range []bool{false, true} {
		t.Run(map[bool]string{false: "respawn", true: "teleport"}[teleport], func(t *testing.T) {
			a, p := predictionTestWorld(t)
			for tic := uint32(1); tic <= 5; tic++ {
				if err := p.Predict(netgame.Input{Sequence: tic, Tick: tic, Command: demo.Tic{Forward: 50}}); err != nil {
					t.Fatal(err)
				}
			}
			if teleport {
				a.g.authorityRules.Scores[1].MovementEpoch++
			} else {
				a.g.authorityRules.Scores[1].Generation++
			}
			// The actual teleport tic flag may already be cleared before a sparse
			// snapshot; its persistent movement epoch must still clear old intent.
			a.players[1].p.teleportedThisTic = false
			a.players[1].p.x = -64 * fracUnit
			if _, err := p.Reconcile(predictionSnapshot(t, a, 2, netgame.InputAck{})); err != nil {
				t.Fatal(err)
			}
			if len(p.history) != 0 || p.PredictedTic() != 0 || p.g.p.x != -64*fracUnit {
				t.Fatal("old-life movement replayed across discontinuity")
			}
			if next, ok := p.NextInputTic(); !ok || next != 6 {
				t.Fatalf("reused submitted future tic: next=%d ok=%v", next, ok)
			}
		})
	}
}

func TestPredictionWindowIsBoundedAndInputGapsAreNeutral(t *testing.T) {
	a, p := predictionTestWorld(t)
	if err := p.Predict(netgame.Input{Sequence: 1, Tick: 3, Command: demo.Tic{Forward: 25}}); err != nil {
		t.Fatal(err)
	}
	// With no initial momentum, two neutral gap tics leave exactly one thrust.
	for range 2 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Step(map[byte]demo.Tic{1: {Forward: 25}}); err != nil {
		t.Fatal(err)
	}
	if p.g.p != a.players[1].p {
		t.Fatalf("gap repeated command: body=%+v, want=%+v", p.g.p, a.players[1].p)
	}
	for tic := uint32(4); tic <= netgame.MaxInputWindow; tic++ {
		if err := p.Predict(netgame.Input{Sequence: tic, Tick: tic}); err != nil {
			t.Fatal(err)
		}
	}
	body, count := p.g.p, len(p.history)
	if err := p.Predict(netgame.Input{Sequence: 257, Tick: 257}); !errors.Is(err, ErrPredictionWindow) {
		t.Fatalf("unbounded prediction accepted: %v", err)
	}
	if p.g.p != body || len(p.history) != count {
		t.Fatal("overflow mutated prediction")
	}
	copy := p.PendingInputs()
	copy[0].Command.Forward = 50
	if p.history[0].Command.Forward != 25 {
		t.Fatal("pending history aliases caller")
	}
}

func TestPredictionDoesNotReplayWorldEffectsOrRNG(t *testing.T) {
	m := predictionTestMap()
	m.Things = append(m.Things, mapdata.Thing{X: 12, Type: 2014, Flags: skillMediumBits})
	// A walk-over exit inside the room must remain server-owned.
	m.Vertexes = append(m.Vertexes, mapdata.Vertex{X: 16, Y: 64}, mapdata.Vertex{X: 16, Y: -64})
	m.Linedefs = append(m.Linedefs, mapdata.Linedef{V1: 4, V2: 5, Flags: mlTwoSided, Special: 52, SideNum: [2]int16{0, 0}})
	g := newGame(m, Options{Headless: true, SkillLevel: 3, NoMonsters: true, PlayerSlot: 1})
	g.stats.Health = 90
	before, playBefore := doomrand.State()
	blockOrder, thinkerOrder := g.nextBlockmapOrder, g.nextThinkerOrder
	for range 20 {
		g.predictAuthoritativeMovement(demo.Tic{Forward: 50, Buttons: demo.ButtonAttack | demo.ButtonUse})
	}
	if g.worldTic != 0 || g.stats.Health != 90 || g.stats.Bullets != 50 || g.thingCollected[1] || g.levelExitRequested || len(g.projectiles) != 0 || len(g.soundQueue) != 0 || len(g.doors) != 0 {
		t.Fatal("prediction changed authoritative gameplay or emitted effects")
	}
	if g.nextBlockmapOrder != blockOrder || g.nextThinkerOrder != thinkerOrder {
		t.Fatal("prediction allocated authoritative actor order")
	}
	if after, playAfter := doomrand.State(); after != before || playAfter != playBefore {
		t.Fatal("prediction consumed RNG")
	}
	g.p.z, g.p.momz = 60*fracUnit, -10*fracUnit
	for range 8 {
		g.predictAuthoritativeMovement(demo.Tic{})
	}
	if len(g.soundQueue) != 0 {
		t.Fatal("replayed landing emitted sound")
	}
}

func TestAuthoritativeTeleportEpochSurvivesLaterTics(t *testing.T) {
	m := predictionTestMap()
	m.Things = append(m.Things, mapdata.Thing{X: 64, Y: 64, Type: teleportThingType, Flags: skillMediumBits})
	m.Sectors[0].Tag = 7
	m.Vertexes = append(m.Vertexes, mapdata.Vertex{X: 16, Y: 64}, mapdata.Vertex{X: 16, Y: -64})
	m.Linedefs = append(m.Linedefs, mapdata.Linedef{V1: 4, V2: 5, Flags: mlTwoSided, Special: 97, Tag: 7, SideNum: [2]int16{0, 0}})
	g := newGame(m, Options{Headless: true, SkillLevel: 3, NoMonsters: true, PlayerSlot: 1})
	g.authorityRules = &authorityRulesState{}
	if !g.activateTeleportLine(4, 0, mapdata.TeleportInfo{UsesTag: true}, -1, true) {
		t.Fatal("teleport fixture could not activate destination")
	}
	if !g.p.teleportedThisTic || g.authorityRules.Scores[1].MovementEpoch != 1 {
		t.Fatal("authoritative teleport did not create discontinuity epoch")
	}
	g.tickPlayerBody()
	if g.p.teleportedThisTic || g.authorityRules.Scores[1].MovementEpoch != 1 {
		t.Fatal("teleport identity was lost before a later snapshot")
	}
}
