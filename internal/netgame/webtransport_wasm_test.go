//go:build js && wasm

package netgame

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"syscall/js"
	"testing"
	"time"
)

func installBrowserWTMock(t *testing.T) js.Value {
	t.Helper()
	previous := js.Global().Get("WebTransport")
	factory := js.Global().Get("Function").New(`
class MockWebTransport {
 constructor(url) {
  globalThis.__mockWT = this;
  this.url = url; this.hardClosed = false; this.fin = false;
  this.streamWrites = []; this.datagramWrites = [];
  this.ready = Promise.resolve();
  this.closed = new Promise(resolve => { this.finish = resolve; });
  this.stream = {
   readable: new ReadableStream({start: c => {this.streamControl = c;}}),
   writable: new WritableStream({write: b => {this.streamWrites.push(new Uint8Array(b));}, close: () => {this.fin = true;}})
  };
  this.datagrams = {maxDatagramSize:1100, incomingMaxAge:null, outgoingMaxAge:null,
   incomingMaxBufferedDatagrams:32, outgoingMaxBufferedDatagrams:32,
   readable: new ReadableStream({start:c => {this.datagramControl = c;}}),
   writable: new WritableStream({write:b => {
    this.datagramWrites.push(new Uint8Array(b));
    if(this.holdDatagrams) return new Promise(resolve => {this.resumeDatagram = resolve;});
   }})
  };
 }
 createBidirectionalStream() { return Promise.resolve(this.stream); }
 close() {
  if(this.hardClosed) return;
  this.hardClosed = true;
  try { this.streamControl.close(); } catch {}
  try { this.datagramControl.close(); } catch {}
  this.finish();
 }
}

return MockWebTransport;
`)
	js.Global().Set("WebTransport", factory.Invoke())
	t.Cleanup(func() { js.Global().Set("WebTransport", previous); js.Global().Delete("__mockWT") })
	return previous
}

func waitBrowserWT(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("browser WebTransport operation did not complete")
		}
		time.Sleep(time.Millisecond)
	}
}

func wtTestBytes(data []byte) js.Value {
	value := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(value, data)
	return value
}

func TestWebTransportWASMStreamAndDatagrams(t *testing.T) {
	installBrowserWTMock(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	transport, err := OpenWebTransport(ctx, "wt://localhost:4433/netplay")
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	mock := js.Global().Get("__mockWT")
	if mock.Get("url").String() != "https://localhost:4433/netplay" {
		t.Fatal("URL was not normalized")
	}
	hello := Hello{Compatibility: "test", Name: "WASM bridge"}
	if err := transport.WriteMessage(hello); err != nil {
		t.Fatal(err)
	}
	encoded, _ := MarshalMessage(hello)
	got, err := wtCopyBytes(mock.Get("streamWrites").Index(0), MaxSnapshotBytes)
	if err != nil || !bytes.Equal(got, encoded) {
		t.Fatal("reliable bytes changed")
	}
	welcome := Welcome{Epoch: 7, PlayerID: 1, InputLead: 3}
	encoded, _ = MarshalMessage(welcome)
	mock.Get("streamControl").Call("enqueue", wtTestBytes(encoded[:6]))
	mock.Get("streamControl").Call("enqueue", wtTestBytes(encoded[6:]))
	message, err := transport.ReadMessage()
	if err != nil || message != welcome {
		t.Fatalf("fragmented reliable message: %+v %v", message, err)
	}
	batch := InputBatch{Epoch: 7, Inputs: []Input{{Sequence: 1, Tick: 5}}}
	if err := transport.WriteMessage(batch); err != nil {
		t.Fatal(err)
	}
	waitBrowserWT(t, func() bool { return mock.Get("datagramWrites").Length() == 1 })
	mock.Get("datagrams").Set("maxDatagramSize", 1)
	if err := transport.WriteMessage(batch); err != nil || mock.Get("datagramWrites").Length() != 1 {
		t.Fatal("MTU refusal was not safe datagram loss")
	}
	if err := transport.Close(); err != nil {
		t.Fatal(err)
	}
	// WritableStream close is queued after accepted writes; give its Promise a
	// turn before inspecting FIN, without needing a browser/native socket.
	deadline := time.After(time.Second)
	for !mock.Get("fin").Bool() {
		select {
		case <-deadline:
			t.Fatal("write FIN never emitted")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if mock.Get("hardClosed").Bool() {
		t.Fatal("half-close discarded reliable drain")
	}
	mock.Get("streamControl").Call("close")
	transport.(*webTransportTransport).waitClosed()
	if !mock.Get("hardClosed").Bool() {
		t.Fatal("drained session not closed")
	}
}

func TestWebTransportWASMBackpressureDoesNotBlockControls(t *testing.T) {
	installBrowserWTMock(t)
	transport, err := OpenWebTransport(context.Background(), "https://localhost:4433/netplay")
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	mock := js.Global().Get("__mockWT")
	for name, want := range map[string]int{"incomingMaxAge": 100, "outgoingMaxAge": 100, "incomingMaxBufferedDatagrams": 4, "outgoingMaxBufferedDatagrams": 1} {
		if got := mock.Get("datagrams").Get(name).Int(); got != want {
			t.Fatalf("%s=%d want %d", name, got, want)
		}
	}
	mock.Set("holdDatagrams", true)
	batch := InputBatch{Epoch: 1, Inputs: []Input{{Sequence: 1, Tick: 1}}}
	if err := transport.WriteMessage(batch); err != nil {
		t.Fatal(err)
	}
	waitBrowserWT(t, func() bool { return mock.Get("datagramWrites").Length() == 1 })
	for range 100 {
		if err := transport.WriteMessage(batch); err != nil {
			t.Fatal(err)
		}
	}
	if mock.Get("datagramWrites").Length() != 1 {
		t.Fatal("browser datagram backlog grew")
	}
	if err := transport.WriteMessage(Ping{Nonce: 1}); err != nil {
		t.Fatal(err)
	}
	if mock.Get("streamWrites").Length() != 1 {
		t.Fatal("reliable control blocked by datagram backpressure")
	}
	mock.Call("resumeDatagram")
	session := transport.(*webTransportTransport).session.(*browserWTSession)
	waitBrowserWT(t, func() bool {
		session.writeMu.Lock()
		defer session.writeMu.Unlock()
		return !session.datagramPending
	})
	mock.Set("holdDatagrams", false)
	if err := transport.WriteMessage(batch); err != nil {
		t.Fatal(err)
	}
	waitBrowserWT(t, func() bool { return mock.Get("datagramWrites").Length() == 2 })
}

func TestWebTransportWASMCancellationAndUnavailable(t *testing.T) {
	previous := installBrowserWTMock(t)
	_ = previous
	ctx, cancel := context.WithCancel(context.Background())
	transport, err := OpenWebTransport(ctx, "https://localhost:4433/netplay")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := transport.ReadMessage(); err == nil {
		t.Fatal("cancel did not interrupt pending reliable read")
	}
	transport.Close()
	transport.(*webTransportTransport).waitClosed()
	js.Global().Set("WebTransport", js.Undefined())
	if _, err := OpenWebTransport(context.Background(), "https://localhost:4433/netplay"); err == nil {
		t.Fatal("missing browser API accepted")
	}
	if _, err := wtCopyBytes(wtTestBytes([]byte{1, 2}), 1); !errors.Is(err, ErrProtocol) {
		t.Fatal("oversized browser chunk accepted")
	}
}

// Executed in the opt-in real browser fixture. Pins authenticate only the
// fixture's generated short-lived certificate; production uses normal TLS.
func TestWebTransportWASMLiveBridge(t *testing.T) {
	url := os.Getenv("GD_TEST_WEBTRANSPORT_URL")
	if url == "" {
		t.Skip("requires browser WebTransport fixture")
	}
	pin, err := hex.DecodeString(os.Getenv("GD_TEST_WEBTRANSPORT_HASH"))
	if err != nil || len(pin) != 32 {
		t.Fatal("invalid fixture certificate hash")
	}
	opts := js.ValueOf(map[string]any{"serverCertificateHashes": []any{map[string]any{"algorithm": "sha-256", "value": wtTestBytes(pin)}}})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	transport, err := openBrowserWebTransport(ctx, url, opts)
	if err != nil {
		t.Fatal(err)
	}
	info, err := QueryServer(ctx, transport)
	if err != nil {
		t.Fatal(err)
	}
	key, err := info.Manifest.Key()
	if err != nil {
		t.Fatal(err)
	}
	transport, err = openBrowserWebTransport(ctx, url, opts)
	if err != nil {
		t.Fatal(err)
	}
	client, err := Connect(ctx, transport, Hello{Compatibility: key, Name: "browser WebTransport probe"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	next := func() Snapshot {
		select {
		case s := <-client.snapshots:
			return s
		case <-client.done:
			t.Fatalf("transport ended: %v", client.Err())
		case <-ctx.Done():
			t.Fatal("browser snapshot timeout")
		}
		return Snapshot{}
	}
	snapshot := next()
	if len(snapshot.State) != 128<<10 {
		t.Fatal("full reliable baseline truncated")
	}
	target := snapshot.Tick + 6
	if err := client.SendInputs(InputBatch{Epoch: client.Welcome().Epoch, SnapshotAck: snapshot.ID, Inputs: []Input{{Sequence: 1, Tick: target}}}); err != nil {
		t.Fatal(err)
	}
	for snapshot.Tick < target {
		snapshot = next()
	}
	if !snapshot.Finalized.HasSequence || snapshot.Finalized.Sequence != 1 {
		t.Fatalf("browser datagram input not finalized: %+v", snapshot.Finalized)
	}
	if snapshot.State[0] != byte(snapshot.Tick) {
		t.Fatal("browser delta reconstruction differs from server")
	}
	t.Log("PASS: browser HTTP/3 TLS pin, discovery, reliable full baseline, input datagram, compressed snapshot correction")
}
