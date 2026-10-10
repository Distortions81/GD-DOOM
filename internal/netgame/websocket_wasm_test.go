//go:build js && wasm

package netgame

import (
	"context"
	"os"
	"testing"
	"time"
)

// Executed by the opt-in native TestWebSocketWASMClient test against a real
// server using Node's WebSocket browser-compatible API and this library's JS
// implementation. Browser UI integration remains a separate application check.
func TestWebSocketWASMBridge(t *testing.T) {
	url := os.Getenv("GD_TEST_WEBSOCKET_URL")
	if url == "" {
		t.Skip("requires the native WebSocket integration fixture")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := DialWebSocket(ctx, url, Hello{Compatibility: "test-content", Name: "wasm"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	next := func() Snapshot {
		select {
		case snapshot := <-client.snapshots:
			return snapshot
		case <-client.done:
			t.Fatalf("WASM client disconnected: %v", client.Err())
		case <-time.After(3 * time.Second):
			t.Fatal("WASM client did not receive a snapshot")
		}
		return Snapshot{}
	}
	snapshot := next()
	target := snapshot.Tick + 6
	if err := client.SendInputs(InputBatch{Epoch: client.Welcome().Epoch, SnapshotAck: snapshot.ID, Inputs: []Input{{Sequence: target, Tick: target}}}); err != nil {
		t.Fatal(err)
	}
	for snapshot.Tick < target {
		snapshot = next()
	}
	if !snapshot.Finalized.HasSequence || snapshot.Finalized.Sequence != target {
		t.Fatalf("WASM command was not consumed: %+v", snapshot.Finalized)
	}
	cancel()
	select {
	case <-client.done:
	case <-time.After(3 * time.Second):
		t.Fatal("WASM cancellation did not release client")
	}
}
