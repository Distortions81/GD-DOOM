package doomruntime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
)

type fakeResumingAuthorityClient struct {
	*fakeAuthorityClient
	status netgame.ConnectionStatus
	resume *netgame.ClientSessionChange
}

func (f *fakeResumingAuthorityClient) Status() netgame.ConnectionStatus { return f.status }
func (f *fakeResumingAuthorityClient) PollSessionChange() (netgame.ClientSessionChange, bool) {
	if f.resume == nil {
		return netgame.ClientSessionChange{}, false
	}
	change := *f.resume
	f.resume = nil
	f.welcome = change.Welcome
	f.status = netgame.ConnectionStatus{State: netgame.ConnectionConnected}
	return change, true
}

func authorityResumeTestManifest(mapName string) netgame.CompatibilityManifest {
	return netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: []string{strings.Repeat("0", 64)}, Map: mapName, Mode: "coop", Skill: 3}
}

func TestAuthoritySessionResumeSameEpochResetsInputAndFreezesOffline(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	resuming := &fakeResumingAuthorityClient{fakeAuthorityClient: connection, status: netgame.ConnectionStatus{State: netgame.ConnectionConnected}}
	g.opts.AuthorityClient = resuming
	sg := &sessionGame{g: g, rt: g, opts: g.opts, current: g.m.Name}
	sg.frontend.Active = true
	now := time.Unix(100, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 9, netgame.InputAck{})}
	if err := sg.updateAuthoritySessionAt(now); err != nil {
		t.Fatal(err)
	}
	if err := sg.updateAuthoritySessionAt(now.Add(time.Second / 35 * 3)); err != nil {
		t.Fatal(err)
	}
	old := g.clientPrediction
	oldSequence, oldBody, oldTic, sent := g.clientUpdate.sequence, g.p, old.PredictedTic(), len(connection.sent)
	if oldSequence < 2 || old.snapshotID != 9 || len(old.PendingInputs()) < 2 {
		t.Fatal("test did not establish old input and ack history")
	}
	resuming.status = netgame.ConnectionStatus{State: netgame.ConnectionReconnecting, Attempt: 2}
	if err := sg.updateAuthoritySessionAt(now.Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if g.p != oldBody || old.PredictedTic() != oldTic || g.clientUpdate.sequence != oldSequence || len(connection.sent) != sent {
		t.Fatal("offline session predicted or transmitted gameplay")
	}
	if got := strings.Join(g.authorityStatusLines(), " "); !strings.Contains(got, "RECONNECTING (2/6)") {
		t.Fatalf("reconnect state not visible: %q", got)
	}
	for range 10 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	welcome := connection.welcome
	welcome.ServerTick = 10
	resuming.resume = &netgame.ClientSessionChange{Generation: 2, Welcome: welcome, Manifest: authorityResumeTestManifest(string(g.m.Name))}
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := sg.updateAuthoritySessionAt(now.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	latest := connection.sent[len(connection.sent)-1]
	if sg.g != g || g.clientPrediction == old || g.worldTic != 10 || latest.SnapshotAck != 1 || len(latest.Inputs) != 1 || latest.Inputs[0].Sequence != 1 || latest.Inputs[0].Tick != 11 {
		t.Fatalf("resume reused old prediction or rejected fresh baseline: %+v", latest)
	}
	if len(g.authorityStatusLines()) != 0 {
		t.Fatal("reconnect overlay remained after resumed baseline")
	}
}

func TestAuthoritySessionResumeNewEpochValidatesMapBeforeBaseline(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 0)
	welcome := connection.welcome
	welcome.Epoch = 8
	manifest := authorityResumeTestManifest("MAP02")
	resuming := &fakeResumingAuthorityClient{fakeAuthorityClient: connection, resume: &netgame.ClientSessionChange{Generation: 2, Welcome: welcome, Manifest: manifest}}
	g.opts.AuthorityClient = resuming
	sg := &sessionGame{g: g, rt: g, opts: g.opts, current: g.m.Name}
	sg.frontend.Active = true
	next := predictionTestMap()
	next.Name = "MAP02"
	key, _ := manifest.Key()
	loaded := false
	sg.opts.AuthorityMapLoader = func(change netgame.MapChange) (*mapdata.Map, error) {
		if connection.snapshotPolls != 0 || change.PreviousEpoch != 7 || change.Welcome != welcome || change.Compatibility != key || change.Map != "MAP02" {
			t.Fatalf("resume map not verified before baseline: %+v", change)
		}
		loaded = true
		return cloneMapForRestart(next), nil
	}
	a, err := NewAuthority(next, Options{SkillLevel: 3, NoMonsters: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	baseline := predictionSnapshot(t, a, 1, netgame.InputAck{})
	baseline.Epoch = 8
	connection.snapshots = []netgame.Snapshot{baseline}
	if err := sg.updateAuthoritySessionAt(time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if !loaded || sg.g == g || sg.current != "MAP02" || sg.g.clientPrediction.epoch != 8 || len(connection.sent) != 1 || connection.sent[0].Epoch != 8 {
		t.Fatal("resumed map epoch was not installed before prediction")
	}
}

func TestAuthoritySessionCompletionPreservesWorldAndDisplaysScores(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	a.g.authorityRules.Ended = true
	a.g.authorityRules.EndReason = "frag_limit"
	a.g.authorityRules.Scores[1].Frags = 7
	a.g.authorityRules.Scores[1].Deaths = 2
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	sampled := 0
	if err := g.updateAuthoritativeClientAt(time.Unix(100, 0), func() demo.Tic { sampled++; return demo.Tic{Forward: 50} }); err != nil {
		t.Fatal(err)
	}
	if !g.clientPrediction.Ready() || g.p != a.players[1].p || sampled != 0 || len(connection.sent) != 0 {
		t.Fatal("completion failed to preserve final world or kept submitting input")
	}
	if lines := strings.Join(g.authorityStatusLines(), " "); !strings.Contains(lines, "MATCH COMPLETE FRAG LIMIT") || !strings.Contains(lines, "PLAYER 1  FRAGS 7  DEATHS 2") {
		t.Fatalf("completion scores not visible: %q", lines)
	}
}

func TestAuthoritySessionMenuContinuesNeutralAndAppliesWorld(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	sg := &sessionGame{g: g, rt: g, opts: g.opts}
	sg.frontend.Active = true
	now := time.Unix(100, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := sg.updateAuthoritySessionAt(now); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 2, netgame.InputAck{HasTick: true, Tick: 3})}
	if err := sg.updateAuthoritySessionAt(now.Add(time.Second / 35 * 3)); err != nil {
		t.Fatal(err)
	}
	if g.worldTic != 3 || len(connection.sent) != 2 || connection.sent[1].SnapshotAck != 2 {
		t.Fatal("menu stopped server baseline/ack pump")
	}
	for _, batch := range connection.sent {
		for _, input := range batch.Inputs {
			if input.Command != (demo.Tic{}) {
				t.Fatal("menu sent active gameplay controls")
			}
		}
	}
}

func TestAuthoritySessionValidatesAndRebuildsMapBeforeSnapshot(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 0)
	sg := &sessionGame{g: g, rt: g, opts: g.opts, current: g.m.Name, currentTemplate: cloneMapForRestart(g.m)}
	sg.frontend.Active = true
	next := predictionTestMap()
	next.Name = "MAP02"
	change := netgame.MapChange{PreviousEpoch: 7, Welcome: netgame.Welcome{Epoch: 8, PlayerID: 1}, Map: "MAP02", Compatibility: "validated-content"}
	connection.transitions = []netgame.MapChange{change}
	loaded := false
	sg.opts.AuthorityMapLoader = func(received netgame.MapChange) (*mapdata.Map, error) {
		if received != change || connection.snapshotPolls != 0 {
			t.Fatal("snapshot consumed before map compatibility validation")
		}
		loaded = true
		return cloneMapForRestart(next), nil
	}
	a, err := NewAuthority(next, Options{SkillLevel: 3, NoMonsters: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	baseline := predictionSnapshot(t, a, 1, netgame.InputAck{})
	baseline.Epoch = 8
	connection.snapshots = []netgame.Snapshot{baseline}
	if err := sg.updateAuthoritySessionAt(time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if !loaded || sg.g == g || sg.current != "MAP02" || sg.g.clientPrediction.epoch != 8 || sg.g.worldTic != 0 || len(connection.sent) != 1 || connection.sent[0].Epoch != 8 {
		t.Fatal("new map/epoch was not installed before its baseline")
	}
}

func TestAuthoritySessionRejectsMapAndLocalStateReplacement(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 0)
	sg := &sessionGame{g: g, rt: g, opts: g.opts, current: g.m.Name}
	sg.frontend.Active = true
	failure := errors.New("compatibility mismatch")
	sg.opts.AuthorityMapLoader = func(netgame.MapChange) (*mapdata.Map, error) { return nil, failure }
	connection.transitions = []netgame.MapChange{{PreviousEpoch: 7, Welcome: netgame.Welcome{Epoch: 8, PlayerID: 1}, Map: "MAP02"}}
	if err := sg.updateAuthoritySessionAt(time.Unix(100, 0)); !errors.Is(err, failure) {
		t.Fatalf("map validation error lost: %v", err)
	}
	if sg.g != g || connection.snapshotPolls != 0 {
		t.Fatal("rejected transition changed game or consumed baseline")
	}
	if err := sg.SaveGameToSlot(0); !errors.Is(err, errSaveGameUnavailable) {
		t.Fatalf("network save allowed: %v", err)
	}
	if err := sg.LoadGameFromSlot(0); !errors.Is(err, errSaveGameUnavailable) {
		t.Fatalf("network load allowed: %v", err)
	}
	sg.startGameFromFrontend(5)
	if sg.g != g || sg.opts.SkillLevel == 5 {
		t.Fatal("frontend replaced authoritative match")
	}
}
