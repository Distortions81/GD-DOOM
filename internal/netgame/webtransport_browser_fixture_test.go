//go:build !js

package netgame

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Opt-in fixture for a real browser operated through the normal UI. The test
// serves a Run button and waits for its result; it never automates a browser or
// changes certificate trust. The browser pins only this fixture's generated key.
func TestWebTransportBrowserFixture(t *testing.T) {
	if os.Getenv("GD_TEST_RUN_WT_BROWSER") != "1" {
		t.Skip("set GD_TEST_RUN_WT_BROWSER=1 and open the logged local browser probe")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:18082")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	origin := "http://" + listener.Addr().String()
	match, world := newDiscoveryTestMatch(t)
	match.snapshots = codecSnapshotWorld{world, noisySnapshot(1).State}
	fixture := startNativeWebTransportFixture(t, match, WebTransportOptions{OriginPatterns: []string{origin}})
	directory := t.TempDir()
	wasm := filepath.Join(directory, "netgame.test.wasm")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "test", "-c", "-o", wasm, ".")
	command.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build browser probe: %v\n%s", err, output)
	}
	result := make(chan int, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "testdata/webtransport_probe.html") })
	mux.HandleFunc("/wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"))
	})
	mux.HandleFunc("/netgame.test.wasm", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/wasm")
		http.ServeFile(w, r, wasm)
	})
	mux.HandleFunc("/config.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": fixture.address, "hash": hex.EncodeToString(fixture.pin[:])})
	})
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var report struct{ Code int }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&report) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		select {
		case result <- report.Code:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	web := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer web.Close()
	go web.Serve(listener)
	t.Logf("WebTransport browser probe ready: %s/ (press Run WebTransport tests)", origin)
	select {
	case code := <-result:
		if code != 0 {
			t.Fatalf("browser WebTransport tests exited %d", code)
		}
	case <-ctx.Done():
		t.Fatal("browser probe was not completed within ten minutes")
	}
}
