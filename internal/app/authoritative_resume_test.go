package app

import (
	"bytes"
	"context"
	"io"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
)

// The wrapper observes the real Doom body under the same lock as the match
// owner. Tests never read a live Authority concurrently with its simulation.
type resumeAuditWorld struct {
	*doomruntime.Authority
	mu            sync.Mutex
	adds, removes int
}

func (w *resumeAuditWorld) AddPlayer(id byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.Authority.AddPlayer(id)
	if err == nil {
		w.adds++
	}
	return err
}
func (w *resumeAuditWorld) RemovePlayer(id byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.removes++
	w.Authority.RemovePlayer(id)
}
func (w *resumeAuditWorld) Step(inputs map[byte]demo.Tic) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Authority.Step(inputs)
}
func (w *resumeAuditWorld) state() ([]doomruntime.AuthorityPlayerState, doomruntime.AuthorityMatchState, int, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.PlayerStates(), w.MatchState(), w.adds, w.removes
}

type resumeProxyPair struct {
	client, server net.Conn
	hello          netgame.Hello
}

func (p resumeProxyPair) close() { p.client.Close(); p.server.Close() }

// The test proxy forwards the real framed protocol and only reads the opening
// frame to distinguish discovery from joins. Closing both halves simulates a
// lost transport, without sending the explicit leave message.
func startResumeCutProxy(t *testing.T, target string) (string, <-chan resumeProxyPair, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	joins := make(chan resumeProxyPair, 4)
	queries := &atomic.Int32{}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer client.Close()
				stopClient := context.AfterFunc(ctx, func() { client.Close() })
				defer stopClient()
				_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
				first, err := netgame.ReadClientMessage(client)
				if err != nil {
					return
				}
				_ = client.SetReadDeadline(time.Time{})
				upstream, err := (&net.Dialer{}).DialContext(ctx, "tcp", target)
				if err != nil {
					return
				}
				defer upstream.Close()
				stopServer := context.AfterFunc(ctx, func() { upstream.Close() })
				defer stopServer()
				data, err := netgame.MarshalMessage(first)
				if err != nil {
					return
				}
				if _, err := io.Copy(upstream, bytes.NewReader(data)); err != nil {
					return
				}
				switch message := first.(type) {
				case netgame.Query:
					queries.Add(1)
				case netgame.Hello:
					joins <- resumeProxyPair{client, upstream, message}
				}
				copied := make(chan struct{})
				go func() { defer close(copied); _, _ = io.Copy(upstream, client); upstream.Close() }()
				_, _ = io.Copy(client, upstream)
				client.Close()
				<-copied
			}()
		}
	}()
	t.Cleanup(func() { cancel(); listener.Close(); wg.Wait() })
	return listener.Addr().String(), joins, queries
}

func TestAuthorityLaunchResumesRealBodyAfterTCPPathLoss(t *testing.T) {
	file, paths, err := openWADStack(filepath.Join("..", "..", "DOOM1.WAD"), nil)
	if err != nil {
		t.Fatal(err)
	}
	hashes, err := launchcatalog.HashWADStackSHA256(paths)
	if err != nil {
		t.Fatal(err)
	}
	manifest := netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: hashes, Map: "E1M1", Mode: "coop", Skill: 3, NoMonsters: true, RespawnDelayTics: 35}
	key, _ := manifest.Key()
	content, _ := manifest.ContentKey()
	m, err := mapdata.LoadMap(file, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := doomruntime.NewAuthority(m, doomruntime.Options{GameMode: "coop", SkillLevel: 3, NoMonsters: true, WADHash: content})
	if err != nil {
		t.Fatal(err)
	}
	world := &resumeAuditWorld{Authority: authority}
	match, err := netgame.NewMatch(world, world, netgame.MatchConfig{Epoch: 7, Compatibility: key, Manifest: &manifest, PlayerLimit: 4, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 2, ResumeGraceTicks: 1050})
	if err != nil {
		t.Fatal(err)
	}
	server, err := netgame.NewServer(match)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server failed to stop")
		}
	})
	address, joins, queries := startResumeCutProxy(t, listener.Addr().String())
	opts := runtimecfg.Options{}
	client, joined, err := connectAuthority(ctx, authorityLaunch{Address: address, Name: "resume-integration"}, paths, file, &opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	if joined.Name != "E1M1" {
		t.Fatal("wrong initial map")
	}
	first := <-joins
	welcome := client.Welcome()
	if first.hello.ResumeToken != ([32]byte{}) || welcome.ResumeToken == ([32]byte{}) || queries.Load() != 1 {
		t.Fatal("initial discovery/join did not create resumable session")
	}
	poll := func(predicate func(netgame.Snapshot) bool) netgame.Snapshot {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			snapshot, ok, err := client.PollSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if ok && predicate(snapshot) {
				return snapshot
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("snapshot condition timed out; status=%+v", client.Status())
		return netgame.Snapshot{}
	}
	ready := poll(func(s netgame.Snapshot) bool { return s.Tick >= 20 })
	fireTick := ready.Tick + 4
	if err := client.SendInputs(netgame.InputBatch{Epoch: 7, SnapshotAck: ready.ID, Inputs: []netgame.Input{{Sequence: 1, Tick: fireTick, Command: demo.Tic{Buttons: demo.ButtonAttack}}}}); err != nil {
		t.Fatal(err)
	}
	poll(func(s netgame.Snapshot) bool { return s.Tick >= fireTick+4 && s.Finalized.HasSequence })
	before, beforeRules, adds, removes := world.state()
	if len(before) != 1 || before[0].Bullets != 49 || adds != 1 || removes != 0 {
		t.Fatalf("test did not modify original body inventory: %+v adds=%d removes=%d", before, adds, removes)
	}
	first.close()
	deadline := time.Now().Add(4 * time.Second)
	var resumed netgame.ClientSessionChange
	for time.Now().Before(deadline) {
		_, _, _ = client.PollSnapshot()
		if change, ok := client.PollSessionChange(); ok {
			resumed = change
			break
		}
		time.Sleep(time.Millisecond)
	}
	if resumed.Generation != 2 || resumed.Welcome.Epoch != welcome.Epoch || resumed.Welcome.PlayerID != welcome.PlayerID || resumed.Welcome.ResumeToken == welcome.ResumeToken || queries.Load() != 2 {
		t.Fatalf("app did not freshly discover/resume original slot: generation=%d epoch=%d player=%d queries=%d status=%+v", resumed.Generation, resumed.Welcome.Epoch, resumed.Welcome.PlayerID, queries.Load(), client.Status())
	}
	second := <-joins
	if second.hello.ResumeToken != welcome.ResumeToken || second.hello.Compatibility != key {
		t.Fatal("resume handshake did not use exact session token and verified compatibility")
	}
	baseline := poll(func(s netgame.Snapshot) bool { return s.Tick >= resumed.Welcome.ServerTick })
	if baseline.Finalized.HasSequence {
		t.Fatal("resumed baseline retained old input sequence acknowledgment")
	}
	after, afterRules, adds, removes := world.state()
	if len(after) != 1 || after[0].ID != before[0].ID || after[0].Bullets != 49 || after[0].Health != before[0].Health || adds != 1 || removes != 0 || afterRules.Scores[0].Generation != beforeRules.Scores[0].Generation {
		t.Fatalf("resume replaced live body/inventory: before=%+v after=%+v adds=%d removes=%d", before, after, adds, removes)
	}
	if err := client.SendInputs(netgame.InputBatch{Epoch: 7, SnapshotAck: baseline.ID, Inputs: []netgame.Input{{Sequence: 1, Tick: baseline.Tick + 4}}}); err != nil {
		t.Fatal(err)
	}
	poll(func(s netgame.Snapshot) bool { return s.Finalized.HasSequence && s.Finalized.Sequence == 1 })
}
