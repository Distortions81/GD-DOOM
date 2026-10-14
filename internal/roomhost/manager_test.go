package roomhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gddoom/internal/lobby"
	"gddoom/internal/netgame"
)

func testPacks(t *testing.T) []ContentPack {
	t.Helper()
	base, err := filepath.Abs(filepath.Join("..", "..", "DOOM1.WAD"))
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "catalog.json")
	data, _ := json.Marshal(Catalog{Packs: []CatalogPack{{ID: "doom", Name: "Doom", WADs: []string{base}}}})
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	packs, err := LoadCatalog(config)
	if err != nil {
		t.Fatal(err)
	}
	return packs
}

func testRequest(t *testing.T, name, mapName, mode string) lobby.CreateRequest {
	t.Helper()
	id, err := lobby.NewRequestID()
	if err != nil {
		t.Fatal(err)
	}
	return lobby.CreateRequest{RequestID: id, Name: name, Settings: lobby.Settings{PackID: "doom", Map: mapName, Mode: mode, Skill: 3, PlayerLimit: 4, NoMonsters: true, FastMonsters: true, RespawnMonsters: true}}
}

func testManager(t *testing.T, config Config) (*Manager, *httptest.Server) {
	t.Helper()
	server := httptest.NewUnstartedServer(nil)
	config.PublicURL = "http://" + server.Listener.Addr().String()
	if config.WorkerPath == "" {
		config.WorkerPath = os.Args[0]
	}
	if config.Packs == nil {
		config.Packs = testPacks(t)
	}
	manager, err := New(context.Background(), config)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	server.Config.Handler = manager.Handler()
	server.Start()
	t.Cleanup(func() { manager.Close(); server.Close() })
	return manager, server
}

func waitFor(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not complete before deadline")
}

func roomByID(manager *Manager, id string) lobby.Room {
	for _, room := range manager.State().Rooms {
		if room.ID == id {
			return room
		}
	}
	return lobby.Room{}
}

func TestProcessRoomsIsolationCancellationAndCleanup(t *testing.T) {
	worker := filepath.Join(t.TempDir(), "gdserver")
	build := exec.Command("go", "build", "-o", worker, "./cmd/gdserver")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build worker: %v\n%s", err, output)
	}
	private := t.TempDir()
	manager, server := testManager(t, Config{WorkerPath: worker, MaxRooms: 2, IdleTimeout: 300 * time.Millisecond, PollInterval: 20 * time.Millisecond, TempDir: private})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	firstRequest := testRequest(t, "Cooperative room", "E1M1", "coop")
	first, err := lobby.Create(ctx, server.URL, firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	firstKey, _ := first.Manifest.Key()
	one, err := netgame.DialWebSocket(ctx, first.Address, netgame.Hello{Name: "first", Compatibility: firstKey})
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()
	secondRequest := testRequest(t, "Deathmatch room", "E1M2", "deathmatch")
	second, err := lobby.Create(ctx, server.URL, secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, _ := second.Manifest.Key()
	two, err := netgame.DialWebSocket(ctx, second.Address, netgame.Hello{Name: "second", Compatibility: secondKey})
	if err != nil {
		t.Fatal(err)
	}
	defer two.Close()
	if first.ID == second.ID || one.Welcome().Epoch == two.Welcome().Epoch || one.Welcome().PlayerID != 1 || two.Welcome().PlayerID != 1 {
		t.Fatal("rooms did not have isolated match identities/player slots")
	}
	if first.Manifest.Map != "E1M1" || second.Manifest.Map != "E1M2" || !first.Manifest.FastMonsters || !second.Manifest.RespawnMonsters {
		t.Fatal("worker manifest lost requested rules")
	}
	for _, client := range []*netgame.Client{one, two} {
		waitFor(t, 2*time.Second, func() bool {
			snapshot, ok, err := client.PollSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			return ok && len(snapshot.State) > 0
		})
	}
	waitFor(t, time.Second, func() bool {
		return roomByID(manager, first.ID).Players == 1 && roomByID(manager, second.ID).Players == 1
	})
	duplicate, err := lobby.Create(ctx, server.URL, firstRequest)
	if err != nil || duplicate.ID != first.ID {
		t.Fatalf("idempotent retry: %+v %v", duplicate, err)
	}
	if _, err := lobby.Create(ctx, server.URL, testRequest(t, "Too many", "E1M1", "coop")); err == nil {
		t.Fatal("room capacity was not enforced")
	}
	if err := one.Leave(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 2*time.Second, func() bool { return roomByID(manager, first.ID).State == "ended" })
	if roomByID(manager, second.ID).State != "ready" {
		t.Fatal("ending one room stopped the other")
	}
	// A dropped socket reserves its body for reconnection and must not look idle.
	two.Close()
	waitFor(t, time.Second, func() bool { return roomByID(manager, second.ID).ReservedPlayers == 1 })
	time.Sleep(400 * time.Millisecond)
	if roomByID(manager, second.ID).State != "ready" {
		t.Fatal("idle cleanup discarded a reserved reconnect body")
	}
	// Cancellation of the HTTP wait leaves the startup managed and discoverable.
	thirdRequest := testRequest(t, "Canceled wait", "E1M3", "coop")
	createCtx, stopCreate := context.WithCancel(ctx)
	created := make(chan error, 1)
	go func() { _, err := manager.Create(createCtx, "different-creator", thirdRequest); created <- err }()
	var thirdID string
	waitFor(t, time.Second, func() bool {
		for _, r := range manager.State().Rooms {
			if r.Name == thirdRequest.Name {
				thirdID = r.ID
				return true
			}
		}
		return false
	})
	stopCreate()
	if err := <-created; !errors.Is(err, context.Canceled) {
		t.Fatalf("create cancellation: %v", err)
	}
	third, err := manager.Create(ctx, "different-creator", thirdRequest)
	if err != nil || third.ID != thirdID {
		t.Fatalf("canceled startup was orphaned or duplicated: %+v %v", third, err)
	}
	thirdKey, _ := third.Manifest.Key()
	three, err := netgame.DialWebSocket(ctx, third.Address, netgame.Hello{Name: "shutdown", Compatibility: thirdKey})
	if err != nil {
		t.Fatal(err)
	}
	defer three.Close()
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { _, _, err := three.PollSnapshot(); return err != nil })
	for _, r := range manager.State().Rooms {
		if activeRoom(r.State) {
			t.Fatalf("room remained active after shutdown: %+v", r)
		}
	}
	files, err := os.ReadDir(private)
	if err != nil || len(files) != 0 {
		t.Fatalf("private worker directories remain after wait/reap: %v %v", files, err)
	}
	transport, err := netgame.OpenWebSocket(ctx, third.Address)
	if transport != nil {
		transport.Close()
	}
	if err == nil {
		t.Fatal("shutdown left a room route accepting WebSockets")
	}
	t.Run("changed-content-fails-readiness", func(t *testing.T) {
		packs := testPacks(t)
		packs[0].Pack.WADHashes[0] = strings.Repeat("a", 64)
		private := t.TempDir()
		m, _ := testManager(t, Config{WorkerPath: worker, Packs: packs, TempDir: private})
		if _, err := m.Create(ctx, "content-test", testRequest(t, "Mismatch", "E1M1", "coop")); err == nil {
			t.Fatal("worker with changed resources became ready")
		}
		if rooms := m.State().Rooms; len(rooms) != 1 || rooms[0].State != "failed" {
			t.Fatalf("startup failure not published: %+v", rooms)
		}
		files, _ := os.ReadDir(private)
		if len(files) != 0 {
			t.Fatal("failed worker was not cleaned up")
		}
	})
}

func TestWorkerStartupDeadlineReapsChild(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("test helper requires a POSIX shell")
	}
	worker := filepath.Join(t.TempDir(), "blocked-worker")
	if err := os.WriteFile(worker, []byte("#!/bin/sh\nexec sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	manager, _ := testManager(t, Config{WorkerPath: worker, TempDir: private, StartupTimeout: 50 * time.Millisecond, ShutdownTimeout: time.Second})
	started := time.Now()
	if _, err := manager.Create(context.Background(), "startup", testRequest(t, "Not ready", "E1M1", "coop")); err == nil {
		t.Fatal("worker without readiness was accepted")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("startup deadline did not bound worker lifetime")
	}
	if rooms := manager.State().Rooms; len(rooms) != 1 || rooms[0].State != "failed" {
		t.Fatalf("deadline failure not published: %+v", rooms)
	}
	files, _ := os.ReadDir(private)
	if len(files) != 0 {
		t.Fatal("timed-out worker was not reaped/cleaned up")
	}
}

func TestRoomAPIValidationOriginAndIdempotency(t *testing.T) {
	manager, server := testManager(t, Config{StartupTimeout: 50 * time.Millisecond})
	state, err := lobby.Fetch(context.Background(), server.URL)
	if err != nil || len(state.Packs) != 1 || len(state.Rooms) != 0 {
		t.Fatalf("lobby state: %+v %v", state, err)
	}
	request := testRequest(t, "Origin denied", "E1M1", "coop")
	data, _ := json.Marshal(request)
	httpRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/rooms", bytes.NewReader(data))
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Origin", "https://untrusted.example")
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden || len(manager.State().Rooms) != 0 {
		t.Fatal("foreign origin created a room")
	}
	for _, body := range []string{string(data) + "{}", `{"request_id":"bad"}`, `{"path":"/etc/passwd"}`} {
		response, err := http.Post(server.URL+"/api/v1/rooms", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid body accepted: %s %d", body, response.StatusCode)
		}
	}
	if len(manager.State().Rooms) != 0 {
		t.Fatal("invalid requests started processes")
	}
	// Directly exercise bounded idempotency and rates without starting a child.
	manager.mu.Lock()
	now := time.Now()
	if !manager.allowCreateLocked("rate", now) || !manager.allowCreateLocked("rate", now) || manager.allowCreateLocked("rate", now) || !manager.allowCreateLocked("rate", now.Add(30*time.Second)) {
		t.Fatal("creation rate is not bounded/refilled")
	}
	manager.mu.Unlock()
	for _, path := range []string{"/rooms/unknown/netplay", "/rooms/unknown/netplay/extra", "/api/v1/lobby/", "/%61pi/v1/lobby"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("inexact route accepted: %s %d", path, response.StatusCode)
		}
	}
}
