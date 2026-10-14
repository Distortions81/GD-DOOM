package roomhost

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gddoom/internal/lobby"
	"gddoom/internal/netgame"
)

func TestWebTransportRoomsSharePortAndAuthorityWithWSSFallback(t *testing.T) {
	worker := filepath.Join(t.TempDir(), "gdserver")
	build := exec.Command("go", "build", "-o", worker, "./cmd/gdserver")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build worker: %v\n%s", err, output)
	}
	for _, gateway := range []bool{false, true} {
		name := "lobby UDP listener"
		if gateway {
			name = "existing public UDP gateway"
		}
		t.Run(name, func(t *testing.T) { testWebTransportRoomGateway(t, worker, gateway) })
	}
}

func testWebTransportRoomGateway(t *testing.T, worker string, gateway bool) {
	web := httptest.NewUnstartedServer(nil)
	manager, err := New(context.Background(), Config{
		WorkerPath: worker, PublicURL: "https://" + web.Listener.Addr().String(),
		Packs: testPacks(t), WebTransport: true, MaxRooms: 2,
		PollInterval:   20 * time.Millisecond,
		TrustedProxies: []string{"127.0.0.1"},
	})
	if err != nil {
		web.Close()
		t.Fatal(err)
	}
	web.Config.Handler = manager.Handler()
	web.StartTLS()
	defer web.Close()
	defer manager.Close()
	udp, err := net.ListenPacket("udp", web.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	server, err := manager.NewWebTransportServer(web.TLS)
	if gateway {
		private := httptest.NewServer(manager.Handler())
		defer private.Close()
		server, err = netgame.NewWebTransportServer(web.TLS, netgame.WebTransportOptions{})
		if err == nil {
			upstream, _ := url.Parse(private.URL + "/rooms/")
			server.Handle("/rooms/", server.ProxyHandler(manager.ctx, upstream, "/rooms/"))
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(web.Certificate())
	trust := &tls.Config{RootCAs: roots}
	// WSS uses ordinary HTTP trust while the native QUIC client receives the
	// same test CA explicitly. Neither path disables certificate verification.
	previous := http.DefaultTransport
	http.DefaultTransport = web.Client().Transport
	defer func() { http.DefaultTransport = previous }()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	first, err := lobby.Create(ctx, web.URL, testRequest(t, "Co-op datagrams", "E1M1", "coop"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := lobby.Create(ctx, web.URL, testRequest(t, "Deathmatch datagrams", "E1M2", "deathmatch"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.Address, web.URL+"/rooms/") || first.Address == second.Address {
		t.Fatal("rooms did not advertise distinct HTTPS routes on the shared port")
	}
	// The same public listener can route an always-running match directly to
	// its fixed native TCP endpoint, alongside dynamically selected rooms.
	manager.mu.Lock()
	nativeAddress := manager.rooms[second.ID].tcpAddress
	manager.mu.Unlock()
	nativeTarget, _ := url.Parse("tcp://" + nativeAddress)
	server.Handle("/deathmatch", server.ProxyHandler(manager.ctx, nativeTarget, "/deathmatch"))
	done := make(chan error, 1)
	go func() { done <- server.Serve(udp) }()
	defer func() { server.Close(); <-done }()
	probe, err := netgame.OpenWebTransportWithTLS(ctx, web.URL+"/deathmatch", trust)
	if err != nil {
		t.Fatal(err)
	}
	info, err := netgame.QueryServer(ctx, probe)
	if err != nil || info.Manifest.Map != second.Manifest.Map || info.Manifest.Mode != "deathmatch" {
		t.Fatalf("native deathmatch route: %+v %v", info, err)
	}
	// A public CONNECT forwarded through the trusted HTTP proxy must not
	// acquire the private binary tunnel, including with a spoofed IP chain.
	for _, forwarded := range []string{"203.0.113.8", "127.0.0.1, 203.0.113.8"} {
		request := httptest.NewRequest(http.MethodConnect, strings.TrimPrefix(first.Address, web.URL), nil)
		request.RemoteAddr = "203.0.113.8:4000"
		request.Header.Set("X-Forwarded-For", forwarded)
		if forwarded == "203.0.113.8" {
			request.RemoteAddr = "127.0.0.1:4000"
			request.Header.Set("X-Forwarded-For", forwarded)
		}
		recorder := httptest.NewRecorder()
		manager.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("public native tunnel status=%d want 403", recorder.Code)
		}
	}
	dial := func(room lobby.Room, datagrams bool) *netgame.Client {
		t.Helper()
		var transport netgame.MessageTransport
		var err error
		if datagrams {
			transport, err = netgame.OpenWebTransportWithTLS(ctx, room.Address, trust)
		} else {
			transport, err = netgame.OpenWebSocket(ctx, "wss"+strings.TrimPrefix(room.Address, "https"))
		}
		if err != nil {
			t.Fatal(err)
		}
		key, _ := room.Manifest.Key()
		client, err := netgame.Connect(ctx, transport, netgame.Hello{Name: "room player", Compatibility: key})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { client.Close() })
		return client
	}
	one, fallback, two := dial(first, true), dial(first, false), dial(second, true)
	if one.Welcome().Epoch != fallback.Welcome().Epoch || one.Welcome().Epoch == two.Welcome().Epoch || one.Welcome().PlayerID == fallback.Welcome().PlayerID || two.Welcome().PlayerID != 1 {
		t.Fatal("QUIC/WSS did not share one authority, or separate rooms leaked state")
	}
	for _, client := range []*netgame.Client{one, fallback, two} {
		var initial netgame.Snapshot
		waitFor(t, 2*time.Second, func() bool {
			snapshot, ok, err := client.PollSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				initial = snapshot
			}
			return ok
		})
		if err := client.SendInputs(netgame.InputBatch{Epoch: client.Welcome().Epoch, SnapshotAck: initial.ID, Inputs: []netgame.Input{{Tick: initial.Tick + 6, Sequence: 1}}}); err != nil {
			t.Fatal(err)
		}
		waitFor(t, 2*time.Second, func() bool {
			snapshot, ok, err := client.PollSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			return ok && snapshot.Finalized.HasSequence && snapshot.Finalized.Sequence == 1 && len(snapshot.State) > 0
		})
	}
	for _, client := range []*netgame.Client{one, fallback, two} {
		if err := client.Leave(); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, 2*time.Second, func() bool {
		return roomByID(manager, first.ID).Players == 0 && roomByID(manager, second.ID).Players == 0 && roomByID(manager, first.ID).ReservedPlayers == 0 && roomByID(manager, second.ID).ReservedPlayers == 0
	})
	// Active datagram sessions must also terminate when their room owner stops.
	active := dial(first, true)
	manager.Close()
	waitFor(t, 2*time.Second, func() bool { return active.Err() != nil })
}
