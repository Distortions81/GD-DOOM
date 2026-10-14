package netgame

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestTCPServerKeepsAdvancingAfterOneClientLosesInput(t *testing.T) {
	m, _ := newTestMatch(t)
	s, err := NewServer(m)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}()
	dial := func() (net.Conn, Welcome) {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		if err := writeStreamMessage(c, Hello{Compatibility: "test-content", Name: "test"}); err != nil {
			t.Fatal(err)
		}
		message, err := readGameplayTestMessage(c)
		if err != nil {
			t.Fatal(err)
		}
		w, ok := message.(Welcome)
		if !ok {
			t.Fatalf("handshake=%T", message)
		}
		return c, w
	}
	a, wa := dial()
	b, wb := dial()
	if wa.PlayerID == wb.PlayerID {
		t.Fatal("duplicate slot")
	}
	// B goes silent immediately. A stays active while reading real wire snapshots.
	_ = b
	var previous uint32
	for previous < 15 {
		message, err := readGameplayTestMessage(a)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, ok := message.(Snapshot)
		if !ok {
			t.Fatalf("message=%T", message)
		}
		if snapshot.Tick <= previous {
			t.Fatalf("snapshot went backward: %d <= %d", snapshot.Tick, previous)
		}
		previous = snapshot.Tick
		if !snapshot.Finalized.HasTick || snapshot.Finalized.Tick != snapshot.Tick {
			t.Fatalf("ack=%+v", snapshot.Finalized)
		}
		if err := writeStreamMessage(a, InputBatch{Epoch: wa.Epoch, SnapshotAck: snapshot.ID, Inputs: []Input{{Sequence: snapshot.Tick, Tick: snapshot.Tick + 2}}}); err != nil {
			t.Fatal(err)
		}
	}
}

// pipeListener exercises the full accept/session owner without depending on
// the host's permission to bind TCP sockets. Real loopback tests above remain.
type pipeListener struct {
	connections chan net.Conn
	done        chan struct{}
	once        sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{connections: make(chan net.Conn), done: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return &net.TCPAddr{} }

func (l *pipeListener) dial(t *testing.T) net.Conn {
	t.Helper()
	server, client := net.Pipe()
	select {
	case l.connections <- server:
	case <-l.done:
		server.Close()
		client.Close()
		t.Fatal("listener closed")
	case <-time.After(time.Second):
		server.Close()
		client.Close()
		t.Fatal("server did not accept")
	}
	t.Cleanup(func() { client.Close() })
	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	return client
}

func TestServerCompletionFlushesFinalSnapshotBeforeDisconnect(t *testing.T) {
	m, _ := newTestMatch(t)
	w := &completingTestWorld{completeAt: 2}
	m.world, m.snapshots, m.config.SnapshotInterval = w, w, 5
	s, err := NewServer(m)
	if err != nil {
		t.Fatal(err)
	}
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	c := listener.dial(t)
	if err := writeStreamMessage(c, Hello{Compatibility: "test-content", Name: "player"}); err != nil {
		t.Fatal(err)
	}
	if message, err := readGameplayTestMessage(c); err != nil {
		t.Fatal(err)
	} else if _, ok := message.(Welcome); !ok {
		t.Fatalf("first message = %T", message)
	}
	message, err := readGameplayTestMessage(c)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, ok := message.(Snapshot)
	if !ok || snapshot.Tick != 2 || len(snapshot.State) == 0 || snapshot.Finalized.Tick != 2 {
		t.Fatalf("terminal snapshot = %+v", message)
	}
	message, err = readGameplayTestMessage(c)
	if err != nil {
		t.Fatal(err)
	}
	if disconnect, ok := message.(Disconnect); !ok || disconnect.Reason != "Match complete: frag limit" {
		t.Fatalf("terminal reason = %+v", message)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("finished session did not exit")
	}
	if w.steps != 2 {
		t.Fatalf("completed simulation advanced %d tics", w.steps)
	}
}

func standalonePeer(t *testing.T, s *Server, id ConnectionID) (net.Conn, <-chan struct{}) {
	t.Helper()
	server, client := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.serveConnection(ctx, server) }()
	t.Cleanup(func() {
		cancel()
		client.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("peer did not stop")
		}
	})
	_ = client.SetDeadline(time.Now().Add(time.Second))
	if err := writeStreamMessage(client, Hello{Compatibility: "test-content", Name: "player"}); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-s.joins:
		request.reply <- joinReply{id: id, welcome: Welcome{Epoch: 99, PlayerID: byte(id)}}
	case <-time.After(time.Second):
		t.Fatal("peer did not request join")
	}
	if _, err := readGameplayTestMessage(client); err != nil {
		t.Fatal(err)
	}
	return client, done
}

func TestServerBoundsFloodingPeerWithoutDroppingOtherClient(t *testing.T) {
	m, _ := newTestMatch(t)
	s, _ := NewServer(m)
	flood, floodDone := standalonePeer(t, s, 1)
	for range maxQueuedPeerInputs + 1 {
		if err := writeStreamMessage(flood, InputBatch{Epoch: 99}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-floodDone:
	case <-time.After(time.Second):
		t.Fatal("flooding sender retained an unbounded queue")
	}
	if len(s.incoming) != maxQueuedPeerInputs {
		t.Fatalf("flood queue size = %d", len(s.incoming))
	}
	other, otherDone := standalonePeer(t, s, 2)
	if err := writeStreamMessage(other, InputBatch{Epoch: 99}); err != nil {
		t.Fatal(err)
	}
	for range maxQueuedPeerInputs + 1 {
		select {
		case message := <-s.incoming:
			<-message.peer.inputSlots
			if message.id == 2 {
				select {
				case <-otherDone:
					t.Fatal("innocent peer was dropped")
				default:
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("innocent peer message was lost behind flood")
		}
	}
	t.Fatal("innocent peer input missing")
}

func TestServerSharedQueueBackpressuresInsteadOfDroppingPeer(t *testing.T) {
	m, _ := newTestMatch(t)
	s, _ := NewServer(m)
	s.incoming = make(chan clientMessage, 1)
	first, _ := standalonePeer(t, s, 1)
	second, secondDone := standalonePeer(t, s, 2)
	if err := writeStreamMessage(first, InputBatch{Epoch: 99}); err != nil {
		t.Fatal(err)
	}
	firstQueued := <-s.incoming
	s.incoming <- firstQueued
	if err := writeStreamMessage(second, InputBatch{Epoch: 99}); err != nil {
		t.Fatal(err)
	}
	// The first sender's queue occupies the only shared slot. Free it and
	// verify the second sender's already-decoded input survives backpressure.
	firstMessage := <-s.incoming
	<-firstMessage.peer.inputSlots
	select {
	case message := <-s.incoming:
		<-message.peer.inputSlots
		if message.id != 2 {
			t.Fatalf("delayed input belongs to %d", message.id)
		}
	case <-secondDone:
		t.Fatal("temporary shared queue pressure dropped second sender")
	case <-time.After(time.Second):
		t.Fatal("second sender did not recover from shared queue pressure")
	}
}

type shortStreamWriter struct{ bytes int }

func (w *shortStreamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.bytes++
	return 1, nil
}

type stalledStreamWriter struct{}

func (stalledStreamWriter) Write([]byte) (int, error) { return 0, nil }

func TestServerStreamWritesHandlePartialAndStalledWriters(t *testing.T) {
	message := Hello{Compatibility: "build", Name: "player"}
	frame := marshalProtocol(t, message)
	short := &shortStreamWriter{}
	if err := writeStreamMessage(short, message); err != nil || short.bytes != len(frame) {
		t.Fatalf("partial writes bytes=%d error=%v", short.bytes, err)
	}
	if err := writeStreamMessage(stalledStreamWriter{}, message); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("stalled writer = %v", err)
	}
}

func TestTCPServerRejectsContentMismatch(t *testing.T) {
	m, _ := newTestMatch(t)
	s, _ := NewServer(m)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	if err := writeStreamMessage(c, Hello{Compatibility: "another-wad", Name: "test"}); err != nil {
		t.Fatal(err)
	}
	message, err := readGameplayTestMessage(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := message.(Disconnect); !ok {
		t.Fatalf("expected refusal, got %T", message)
	}
}

func TestServerSnapshotQueueReplacesObsoleteStateWithoutBlocking(t *testing.T) {
	p := &streamPeer{snapshots: make(chan Snapshot, 1)}
	for i := uint32(1); i <= 10000; i++ {
		p.offer(Snapshot{ID: i})
	}
	if len(p.snapshots) != 1 {
		t.Fatal("unbounded snapshot queue")
	}
	if got := (<-p.snapshots).ID; got != 10000 {
		t.Fatalf("retained snapshot %d", got)
	}
}

func TestTCPServerCancellationClosesUnfinishedHandshake(t *testing.T) {
	m, _ := newTestMatch(t)
	s, _ := NewServer(m)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("handshake prevented shutdown")
	}
}
