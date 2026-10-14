//go:build !js

package netgame

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Opt in because this compiles another target and requires Node with the
// standard WebSocket global (Node 22+). Ordinary Go tests stay self-contained.
func TestWebSocketWASMClient(t *testing.T) {
	if os.Getenv("GD_TEST_RUN_WASM") != "1" {
		t.Skip("set GD_TEST_RUN_WASM=1 for the executed JS/WASM transport test")
	}
	_, tcpURL, wsURL, cancel, done := startWebSocketTestServer(t, WebSocketOptions{})
	defer func() { cancel(); waitServerStopped(t, done) }()
	native, err := DialTCP(context.Background(), tcpURL, Hello{Compatibility: "test-content", Name: "native-peer"})
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	ctx, stop := context.WithTimeout(context.Background(), time.Minute)
	defer stop()
	wasm := filepath.Join(t.TempDir(), "netgame.test.wasm")
	build := exec.CommandContext(ctx, "go", "test", "-c", "-o", wasm, ".")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile WASM bridge: %v\n%s", err, output)
	}
	runner := filepath.Join(runtime.GOROOT(), "lib", "wasm", "go_js_wasm_exec")
	run := exec.CommandContext(ctx, runner, wasm, "-test.run=^Test(WebSocketWASMBridge|SnapshotCodecFullAndAcknowledgedDelta)$", "-test.v")
	run.Env = append(os.Environ(), "GD_TEST_WEBSOCKET_URL="+wsURL)
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("execute WASM bridge: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
	if snapshot := nextWebSocketTestSnapshot(t, native); snapshot.Tick == 0 {
		t.Fatal("native peer did not share advancing room with WASM peer")
	}
}
