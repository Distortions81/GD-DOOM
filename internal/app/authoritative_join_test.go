package app

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

type authorityJoinFixture struct {
	file     *wad.File
	paths    []string
	manifest netgame.CompatibilityManifest
	world    *resumeAuditWorld
	address  string
}

func newAuthorityJoinFixture(t *testing.T, mismatch bool) authorityJoinFixture {
	t.Helper()
	file, paths, err := openWADStack(filepath.Join("..", "..", "DOOM1.WAD"), nil)
	if err != nil {
		t.Fatal(err)
	}
	hashes, err := launchcatalog.HashWADStackSHA256(paths)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch {
		hashes[0] = strings.Repeat("ab", 32)
	}
	manifest := netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: hashes, Map: "E1M2", Mode: "coop", Skill: 4, NoMonsters: true, RespawnDelayTics: 35}
	key, err := manifest.Key()
	if err != nil {
		t.Fatal(err)
	}
	content, _ := manifest.ContentKey()
	m, err := mapdata.LoadMap(file, "E1M2")
	if err != nil {
		t.Fatal(err)
	}
	a, err := doomruntime.NewAuthority(m, doomruntime.Options{GameMode: "coop", SkillLevel: 4, NoMonsters: true, WADHash: content})
	if err != nil {
		t.Fatal(err)
	}
	world := &resumeAuditWorld{Authority: a}
	match, err := netgame.NewMatch(world, world, netgame.MatchConfig{Epoch: 17, Compatibility: key, Manifest: &manifest, PlayerLimit: 1, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 2, ResumeGraceTicks: 1050})
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
			t.Error("join fixture server did not stop")
		}
	})
	return authorityJoinFixture{file, paths, manifest, world, listener.Addr().String()}
}

func waitAuthorityJoinCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("join lifecycle condition timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

func joinedClientLeave(t *testing.T, client runtimecfg.AuthorityClient) func() {
	t.Helper()
	leaver, ok := client.(interface{ Leave() error })
	if !ok {
		t.Fatal("joined client has no explicit Leave operation")
	}
	return func() {
		if err := leaver.Leave(); err != nil {
			t.Error(err)
		}
	}
}

func TestAuthorityJoinerLoadedWADLeaveAndRejoin(t *testing.T) {
	f := newAuthorityJoinFixture(t, false)
	address, joins, queries := startResumeCutProxy(t, f.address)
	loaded, err := mapdata.LoadMap(f.file, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	paths := append([]string(nil), f.paths...)
	join := authorityJoiner(paths, f.file)
	paths[0] = filepath.Join(t.TempDir(), "not-the-loaded-wad.wad")
	// Options remain owned by the game loop. The callback prepares a result;
	// loading a server map must not mutate the already running local game.
	opts := runtimecfg.Options{GameMode: "single", SkillLevel: 1, PlayerSlot: 4, WADHash: "local-content", AuthorityJoin: join}
	original := opts
	original.AuthorityJoin = nil
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	first, err := opts.AuthorityJoin(ctx, runtimecfg.AuthorityJoinRequest{Address: " " + address + " ", Name: " Alice "})
	if err != nil {
		t.Fatal(err)
	}
	leaveFirst := joinedClientLeave(t, first.Client)
	t.Cleanup(leaveFirst)
	hello := <-joins
	if hello.hello.Name != "Alice" || hello.hello.Spectator || hello.hello.ResumeToken != ([32]byte{}) || queries.Load() != 1 {
		t.Fatal("menu join did not discover and establish the requested fresh identity")
	}
	after := opts
	after.AuthorityJoin = nil
	if !reflect.DeepEqual(after, original) || loaded.Name != "E1M1" || first.Map == loaded || first.Map.Name != "E1M2" || !reflect.DeepEqual(first.Manifest, f.manifest) || first.Client.Welcome().PlayerID != 1 {
		t.Fatal("join mutated existing options/map or returned incorrect server content")
	}
	waitAuthorityJoinCondition(t, func() bool {
		snapshot, ok, err := first.Client.PollSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		return ok && snapshot.Epoch == 17 && len(snapshot.State) > 0
	})
	// Metadata returned to the game is owned. Editing it cannot change the
	// content/rules check captured by the loader or future reconnect attempt.
	nextManifest := f.manifest
	nextManifest.Map = "E1M3"
	nextKey, _ := nextManifest.Key()
	first.Manifest.WADHashes[0] = strings.Repeat("cd", 32)
	next, err := first.MapLoader(netgame.MapChange{Map: "E1M3", Compatibility: nextKey})
	if err != nil || next.Name != "E1M3" {
		t.Fatalf("owned result metadata corrupted verified map loader: %v", err)
	}
	if _, err := first.MapLoader(netgame.MapChange{Map: "E1M3", Compatibility: "unverified"}); err == nil {
		t.Fatal("menu join supplied a loader accepting mismatched map content")
	}
	leaveFirst()
	// A dropped socket would reserve this only slot for 30 seconds. Explicit
	// menu leave must remove the body now, allowing a new identity to join.
	waitAuthorityJoinCondition(t, func() bool {
		players, _, adds, removes := f.world.state()
		return len(players) == 0 && adds == 1 && removes == 1
	})
	second, err := join(ctx, runtimecfg.AuthorityJoinRequest{Address: address, Name: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(joinedClientLeave(t, second.Client))
	hello = <-joins
	if hello.hello.Name != "Bob" || hello.hello.ResumeToken != ([32]byte{}) || second.Client.Welcome().PlayerID != 1 || queries.Load() != 2 {
		t.Fatal("rejoin reused old name/resume identity or failed to reclaim released slot")
	}
	players, _, adds, removes := f.world.state()
	if len(players) != 1 || adds != 2 || removes != 1 || players[0].Health != 100 || !reflect.DeepEqual(second.Manifest, f.manifest) {
		t.Fatal("fresh join did not create a fresh body with original verified content")
	}
	observer, err := join(ctx, runtimecfg.AuthorityJoinRequest{Address: address, Name: "Watcher", Spectator: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(joinedClientLeave(t, observer.Client))
	hello = <-joins
	players, _, adds, removes = f.world.state()
	if !hello.hello.Spectator || hello.hello.Name != "Watcher" || observer.Client.Welcome().PlayerID != 0 || len(players) != 1 || adds != 2 || removes != 1 {
		t.Fatal("spectator menu join took a player slot or lost the selected identity")
	}
}

func TestAuthorityJoinerRejectsCanceledAndMismatchedAttemptsWithoutSlot(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		name := "canceled"
		if mismatch {
			name = "content-mismatch"
		}
		t.Run(name, func(t *testing.T) {
			f := newAuthorityJoinFixture(t, mismatch)
			address, joins, queries := startResumeCutProxy(t, f.address)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if !mismatch {
				cancel()
			}
			result, err := authorityJoiner(f.paths, f.file)(ctx, runtimecfg.AuthorityJoinRequest{Address: address, Name: "rejected"})
			if err == nil || result.Client != nil || result.Map != nil || result.MapLoader != nil {
				t.Fatal("failed join returned a usable or partially owned session")
			}
			if !mismatch && !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled join lost its cancellation cause: %v", err)
			}
			players, _, adds, removes := f.world.state()
			if len(players) != 0 || adds != 0 || removes != 0 {
				t.Fatal("rejected attempt allocated a gameplay slot")
			}
			select {
			case <-joins:
				t.Fatal("rejected attempt submitted a gameplay Hello")
			default:
			}
			wantQueries := int32(0)
			if mismatch {
				wantQueries = 1
			}
			if queries.Load() != wantQueries {
				t.Fatalf("discovery count=%d want=%d", queries.Load(), wantQueries)
			}
		})
	}
}

func TestAuthorityJoinerRetainsLoadedContentIdentityAfterDiskChange(t *testing.T) {
	f := newAuthorityJoinFixture(t, false)
	bytes, err := os.ReadFile(f.paths[0])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "loaded.wad")
	if err := os.WriteFile(path, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := wad.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	join := authorityJoiner([]string{path}, loaded)
	// A long-running game may outlive its source file. Neither its advertised
	// content identity nor later map loading may switch to these new disk bytes.
	if err := os.WriteFile(path, []byte("replaced after the game loaded the WAD"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result, err := join(ctx, runtimecfg.AuthorityJoinRequest{Address: f.address, Name: "loaded-content"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(joinedClientLeave(t, result.Client))
	if !reflect.DeepEqual(result.Manifest, f.manifest) || result.Map.Name != "E1M2" {
		t.Fatal("menu join advertised different bytes from its already loaded WAD")
	}
	nextManifest := f.manifest
	nextManifest.Map = "E1M3"
	nextKey, _ := nextManifest.Key()
	next, err := result.MapLoader(netgame.MapChange{Map: "E1M3", Compatibility: nextKey})
	if err != nil || next.Name != "E1M3" {
		t.Fatalf("map transition reread the replaced file instead of loaded content: %v", err)
	}
}

func TestAuthorityJoinerCancelsBlockedDiscovery(t *testing.T) {
	file, paths, err := openWADStack(filepath.Join("..", "..", "DOOM1.WAD"), nil)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := authorityJoiner(paths, file)(ctx, runtimecfg.AuthorityJoinRequest{Address: listener.Addr().String()})
		if result.Client != nil {
			err = errors.New("canceled discovery returned a client")
		}
		done <- err
	}()
	var conn net.Conn
	select {
	case conn = <-accepted:
		defer conn.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("join did not open discovery")
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	message, err := netgame.ReadClientMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := message.(netgame.Query); !ok {
		t.Fatalf("opening record=%T, expected discovery Query", message)
	}
	// No ServerInfo arrives. Cancel must interrupt the worker's blocked read.
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("blocked discovery ignored cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt blocked discovery promptly")
	}
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("canceled discovery left its socket open")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("canceled discovery only stopped waiting; socket remained open")
	}
}
