//go:build !js

package app

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

type fallbackTestWorld struct{ tick uint32 }

func (w *fallbackTestWorld) Tic() uint32                  { return w.tick }
func (*fallbackTestWorld) AddPlayer(byte) error           { return nil }
func (*fallbackTestWorld) RemovePlayer(byte)              {}
func (w *fallbackTestWorld) Step(map[byte]demo.Tic) error { w.tick++; return nil }
func (w *fallbackTestWorld) Snapshot(viewer byte) ([]byte, error) {
	return []byte{byte(w.tick), viewer}, nil
}

type fallbackReadyListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

func (l *fallbackReadyListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })
	return l.Listener.Accept()
}

// A subprocess is required because crypto/x509 caches system roots process-wide.
// Only the child trusts the generated fixture certificate; production trust and
// the parent test process are never changed or bypassed.
func TestAuthorityHTTPSFallsBackToTrustedWSSAndRetainsSelection(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("fixture requires the Unix SSL_CERT_FILE trust override")
	}
	world := &fallbackTestWorld{}
	manifest := netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: []string{strings.Repeat("ab", 32)}, Map: "E1M1", Mode: "coop", Skill: 3}
	compatibility, err := manifest.Key()
	if err != nil {
		t.Fatal(err)
	}
	match, err := netgame.NewMatch(world, world, netgame.MatchConfig{Epoch: 77, Compatibility: compatibility, Manifest: &manifest, PlayerLimit: 1, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 1})
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
	ready := &fallbackReadyListener{Listener: listener, ready: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	ownerDone := make(chan error, 1)
	go func() { ownerDone <- server.Serve(ctx, ready) }()
	<-ready.ready
	defer func() {
		cancel()
		select {
		case err := <-ownerDone:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("fallback match did not stop")
		}
	}()
	var packets atomic.Int64
	mux := http.NewServeMux()
	mux.Handle("/netplay", server.WebSocketHandler(netgame.WebSocketOptions{}))
	mux.HandleFunc("/udp-count", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, packets.Load()) })
	web := httptest.NewTLSServer(mux)
	defer web.Close()
	// UDP exists but never responds, reproducing a network that blocks QUIC
	// while the authenticated HTTPS/WSS listener remains reachable.
	udp, err := net.ListenPacket("udp", strings.TrimPrefix(web.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	go func() {
		buffer := make([]byte, 2048)
		for {
			if _, _, err := udp.ReadFrom(buffer); err != nil {
				return
			}
			packets.Add(1)
		}
	}()
	directory := t.TempDir()
	emptyRoots := filepath.Join(directory, "empty-roots")
	if err := os.Mkdir(emptyRoots, 0700); err != nil {
		t.Fatal(err)
	}
	trusted := filepath.Join(directory, "fixture-ca.pem")
	if err := os.WriteFile(trusted, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: web.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	untrusted := filepath.Join(directory, "empty-ca.pem")
	if err := os.WriteFile(untrusted, nil, 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ mode, roots string }{{"untrusted", untrusted}, {"fallback", trusted}} {
		t.Run(test.mode, func(t *testing.T) {
			childCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			command := exec.CommandContext(childCtx, executable, "-test.run=^TestAuthorityFallbackSubprocess$", "-test.v")
			env := make([]string, 0, len(os.Environ())+5)
			for _, value := range os.Environ() {
				if !strings.HasPrefix(value, "SSL_CERT_FILE=") && !strings.HasPrefix(value, "SSL_CERT_DIR=") && !strings.HasPrefix(value, "GD_FALLBACK_") {
					env = append(env, value)
				}
			}
			command.Env = append(env, "SSL_CERT_FILE="+test.roots, "SSL_CERT_DIR="+emptyRoots, "GD_FALLBACK_MODE="+test.mode, "GD_FALLBACK_URL="+web.URL, "GD_FALLBACK_KEY="+compatibility)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("fallback subprocess: %v\n%s", err, output)
			}
			t.Logf("%s", output)
		})
	}
	if packets.Load() == 0 {
		t.Fatal("fixture never attempted actual QUIC/UDP")
	}
}

func TestAuthorityFallbackSubprocess(t *testing.T) {
	mode := os.Getenv("GD_FALLBACK_MODE")
	if mode == "" {
		t.Skip("launched by TLS fallback integration")
	}
	endpoint := os.Getenv("GD_FALLBACK_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if mode == "untrusted" {
		transport, err := netgame.OpenWebSocket(ctx, strings.Replace(endpoint, "https://", "wss://", 1)+"/netplay")
		if err == nil {
			transport.Close()
			t.Fatal("untrusted TLS fixture accepted")
		}
		var authorityError x509.UnknownAuthorityError
		if !errors.As(err, &authorityError) {
			t.Fatalf("expected certificate trust failure, got %v", err)
		}
		return
	}
	connector := &authorityConnector{address: endpoint + "/netplay"}
	transport, err := connector.open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	info, err := netgame.QueryServer(ctx, transport)
	if err != nil {
		t.Fatal(err)
	}
	key, err := info.Manifest.Key()
	if err != nil || key != os.Getenv("GD_FALLBACK_KEY") {
		t.Fatalf("verified discovery mismatch: %v", err)
	}
	selected := "wss" + strings.TrimPrefix(endpoint, "https") + "/netplay"
	connector.mu.Lock()
	actual := connector.address
	connector.mu.Unlock()
	if actual != selected {
		t.Fatalf("fallback selection not retained: %q", actual)
	}
	packetCount := func() int64 {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/udp-count", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		bytes, err := io.ReadAll(io.LimitReader(response.Body, 64))
		if err != nil {
			t.Fatal(err)
		}
		count, err := strconv.ParseInt(string(bytes), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return count
	}
	before := packetCount()
	if before == 0 {
		t.Fatal("HTTPS path never attempted UDP")
	}
	started := time.Now()
	transport, err = connector.open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, err := netgame.Connect(ctx, transport, netgame.Hello{Compatibility: key, Name: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("join retried unavailable UDP: %s", elapsed)
	}
	if after := packetCount(); after != before {
		t.Fatalf("join sent more QUIC packets after WSS selection: %d -> %d", before, after)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok, err := client.PollSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			if snapshot.Epoch != 77 || snapshot.Tick == 0 || len(snapshot.State) != 2 {
				t.Fatalf("invalid joined state: %+v", snapshot)
			}
			t.Logf("verified HTTPS→WSS discovery/join; %d initial UDP packets; no UDP retry for join; authoritative tic%d", before, snapshot.Tick)
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("fallback join did not receive authoritative state")
}
