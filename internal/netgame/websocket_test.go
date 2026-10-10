//go:build !js

package netgame

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type readyListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })
	return l.Listener.Accept()
}

func startWebSocketTestServer(t *testing.T, options WebSocketOptions) (*Server, string, string, context.CancelFunc, <-chan error) {
	t.Helper()
	m, _ := newTestMatch(t)
	m.config.DisconnectTicks = 1000
	s, err := NewServer(m)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ready := &readyListener{Listener: listener, ready: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ready) }()
	select {
	case <-ready.ready:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("owner did not start")
	}
	httpServer := httptest.NewServer(s.WebSocketHandler(options))
	t.Cleanup(func() { cancel(); httpServer.Close() })
	return s, listener.Addr().String(), "ws" + strings.TrimPrefix(httpServer.URL, "http"), cancel, done
}

func waitServerStopped(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(2 * time.Second):
		t.Error("server did not stop all transports")
	}
}

func nextWebSocketTestSnapshot(t *testing.T, client *Client) Snapshot {
	t.Helper()
	select {
	case snapshot := <-client.snapshots:
		return snapshot
	case <-client.done:
		t.Fatalf("client disconnected: %v", client.Err())
	case <-time.After(time.Second):
		t.Fatal("client received no authoritative snapshot")
	}
	return Snapshot{}
}

func TestWebSocketAndTCPClientsShareOneAuthoritativeMatch(t *testing.T) {
	_, tcpURL, wsURL, cancel, done := startWebSocketTestServer(t, WebSocketOptions{})
	defer func() { cancel(); waitServerStopped(t, done) }()
	tcp, err := DialTCP(context.Background(), tcpURL, Hello{Compatibility: "test-content", Name: "desktop"})
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	ws, err := DialWebSocket(context.Background(), wsURL, Hello{Compatibility: "test-content", Name: "browser"})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if tcp.Welcome().Epoch != ws.Welcome().Epoch || tcp.Welcome().PlayerID == ws.Welcome().PlayerID {
		t.Fatalf("separate/duplicate session identities: tcp=%+v ws=%+v", tcp.Welcome(), ws.Welcome())
	}
	for _, client := range []*Client{tcp, ws} {
		snapshot := nextWebSocketTestSnapshot(t, client)
		target := snapshot.Tick + 3
		if err := client.SendInputs(InputBatch{Epoch: client.Welcome().Epoch, SnapshotAck: snapshot.ID, Inputs: []Input{{Sequence: target, Tick: target}}}); err != nil {
			t.Fatal(err)
		}
		for snapshot.Tick < target {
			snapshot = nextWebSocketTestSnapshot(t, client)
		}
		if !snapshot.Finalized.HasSequence || snapshot.Finalized.Sequence != target {
			t.Fatalf("transport command was not finalized: %+v", snapshot.Finalized)
		}
	}
	// Losing the browser connection removes only its player; TCP keeps running.
	if err := ws.Close(); err != nil {
		t.Fatal(err)
	}
	first := nextWebSocketTestSnapshot(t, tcp)
	second := nextWebSocketTestSnapshot(t, tcp)
	if second.Tick <= first.Tick {
		t.Fatal("WebSocket disconnect stalled native simulation")
	}
}

func TestWebSocketRejectsContentMismatch(t *testing.T) {
	_, _, url, cancel, done := startWebSocketTestServer(t, WebSocketOptions{})
	defer func() { cancel(); waitServerStopped(t, done) }()
	client, err := DialWebSocket(context.Background(), url, Hello{Compatibility: "wrong-content", Name: "player"})
	if err == nil {
		client.Close()
		t.Fatal("incompatible WebSocket player joined")
	}
	if !strings.Contains(err.Error(), "refused") {
		t.Fatalf("refusal was not delivered over WebSocket: %v", err)
	}
}

func TestWebSocketOriginPolicyAndSubprotocol(t *testing.T) {
	_, _, url, cancel, done := startWebSocketTestServer(t, WebSocketOptions{OriginPatterns: []string{"https://play.example.test"}})
	defer func() { cancel(); waitServerStopped(t, done) }()
	for _, test := range []struct {
		origin string
		allow  bool
	}{
		{"", true},
		{strings.Replace(url, "ws://", "http://", 1), true},
		{"https://play.example.test", true},
		{"http://play.example.test", false},
		{"https://other.example.test", false},
	} {
		ctx, stop := context.WithTimeout(context.Background(), time.Second)
		connection, response, err := websocket.Dial(ctx, url, &websocket.DialOptions{
			Subprotocols: []string{WebSocketSubprotocol}, HTTPHeader: http.Header{"Origin": []string{test.origin}},
		})
		if test.allow {
			if err != nil {
				stop()
				t.Fatalf("allowed origin %q: %v", test.origin, err)
			}
			if connection.Subprotocol() != WebSocketSubprotocol {
				t.Fatal("game subprotocol was not negotiated")
			}
			connection.CloseNow()
		} else if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
			if connection != nil {
				connection.CloseNow()
			}
			stop()
			t.Fatalf("forbidden origin %q: response=%v error=%v", test.origin, response, err)
		}
		stop()
	}
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	connection, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	if _, _, err := connection.Read(ctx); err == nil {
		t.Fatal("missing game subprotocol was accepted")
	}
}

func TestWebSocketServerCancellationClosesGameAndIncompleteHandshake(t *testing.T) {
	_, _, url, cancel, done := startWebSocketTestServer(t, WebSocketOptions{})
	client, err := DialWebSocket(context.Background(), url, Hello{Compatibility: "test-content", Name: "active"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	incomplete, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{Subprotocols: []string{WebSocketSubprotocol}})
	if err != nil {
		t.Fatal(err)
	}
	defer incomplete.CloseNow()
	cancel()
	waitServerStopped(t, done)
	select {
	case <-client.done:
	case <-time.After(time.Second):
		t.Fatal("active WebSocket survived owner cancellation")
	}
	if _, _, err := incomplete.Read(ctx); err == nil {
		t.Fatal("unfinished WebSocket handshake survived owner cancellation")
	}
}

func TestWebSocketRejectsTextAndRestoresReadLimit(t *testing.T) {
	_, _, url, cancel, done := startWebSocketTestServer(t, WebSocketOptions{})
	defer func() { cancel(); waitServerStopped(t, done) }()
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	connection, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{Subprotocols: []string{WebSocketSubprotocol}})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	frame := marshalProtocol(t, Hello{Compatibility: "test-content", Name: "text"})
	if err := connection.Write(ctx, websocket.MessageText, frame); err != nil {
		t.Fatal(err)
	}
	if _, _, err := connection.Read(ctx); err == nil {
		t.Fatal("text WebSocket frame reached binary game protocol")
	}
	// NetConn disables limits by default. Verify our adapter restores one,
	// independently of the logical game frame length validation.
	result := make(chan error, 1)
	limitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			result <- err
			return
		}
		stream := newWebSocketStream(context.Background(), c, 8)
		defer stream.Close()
		// Reader permits one lookahead byte to find FIN; read beyond it so the
		// configured message limit, rather than our destination length, ends IO.
		data := make([]byte, 10)
		_, err = io.ReadFull(stream, data)
		result <- err
	}))
	defer limitServer.Close()
	limited, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(limitServer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.CloseNow()
	if err := limited.Write(ctx, websocket.MessageBinary, []byte("123456789")); err != nil {
		t.Fatal(err)
	}
	// Consume the close frame so the library can finish its bounded handshake.
	_, _, _ = limited.Read(ctx)
	select {
	case err := <-result:
		if !errors.Is(err, websocket.ErrMessageTooBig) {
			t.Fatalf("WebSocket limit was disabled: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("oversized message was not rejected")
	}
}

func TestWebSocketHandlerUnavailableWithoutOwner(t *testing.T) {
	m, _ := newTestMatch(t)
	s, _ := NewServer(m)
	response := httptest.NewRecorder()
	s.WebSocketHandler(WebSocketOptions{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/play", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unowned WebSocket handler response = %d", response.Code)
	}
}
