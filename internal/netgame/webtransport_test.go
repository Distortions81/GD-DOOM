package netgame

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

type testWTSession struct {
	sent     chan []byte
	receive  chan []byte
	done     chan struct{}
	once     sync.Once
	drained  chan struct{}
	receives int
}

func (s *testWTSession) SendDatagram(data []byte) error {
	select {
	case s.sent <- bytes.Clone(data):
		return nil
	case <-s.done:
		return net.ErrClosed
	}
}
func (s *testWTSession) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	s.receives++
	if s.receives == 65 && s.drained != nil {
		close(s.drained)
	}
	select {
	case data := <-s.receive:
		return data, nil
	case <-s.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (s *testWTSession) Close() error { s.once.Do(func() { close(s.done) }); return nil }
func testWTTransport(t *testing.T, serverSide bool) (*webTransportTransport, *testWTSession, net.Conn) {
	t.Helper()
	local, remote := net.Pipe()
	session := &testWTSession{sent: make(chan []byte, 64), receive: make(chan []byte, 64), done: make(chan struct{}), drained: make(chan struct{})}
	transport := newWebTransportTransport(context.Background(), local, session, serverSide)
	t.Cleanup(func() { transport.Close(); remote.Close(); transport.waitClosed() })
	return transport, session, remote
}

func TestWebTransportDatagramRoutingAndSizeBound(t *testing.T) {
	transport, session, remote := testWTTransport(t, false)
	input := InputBatch{Epoch: 1, SnapshotAck: 3, Inputs: []Input{{Tick: 4, Sequence: 4}}}
	if err := transport.WriteMessage(input); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-session.sent:
		if len(data) > MaxGameDatagramBytes {
			t.Fatal("oversized datagram")
		}
		if _, err := decodeGameDatagram(data, true); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("input blocked behind reliable stream")
	}
	encoder, _ := snapshotCodecs(t)
	baseline := noisySnapshot(1)
	if err := encoder.Commit(baseline); err != nil {
		t.Fatal(err)
	}
	delta, err := encoder.Encode(noisySnapshot(2), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := transport.WriteMessage(delta); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-session.sent:
		if _, err := decodeGameDatagram(data, false); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("small delta not sent as datagram")
	}
	for _, message := range []any{Ping{Nonce: 1}, InputBatch{Epoch: 1}, Snapshot{Epoch: 1, ID: 1, State: []byte("full")}, Snapshot{Epoch: 1, ID: 2, BaselineID: 1, Encoding: SnapshotDeltaZstd, DecodedSize: 4096, Digest: [32]byte{1}, State: make([]byte, MaxGameDatagramBytes)}} {
		received := make(chan transportRead, 1)
		go func() { value, err := ReadMessage(remote); received <- transportRead{value, err} }()
		if err := transport.WriteMessage(message); err != nil {
			t.Fatal(err)
		}
		select {
		case result := <-received:
			if result.err != nil {
				t.Fatal(result.err)
			}
		case <-time.After(time.Second):
			t.Fatal("reliable message missing")
		}
		if len(session.sent) != 0 {
			t.Fatalf("%T incorrectly sent as datagram", message)
		}
	}
}

func TestWebTransportBoundsQueueAndPreservesReliableControl(t *testing.T) {
	transport, session, remote := testWTTransport(t, true)
	for id := uint32(1); id <= 64; id++ {
		session.receive <- marshalProtocol(t, InputBatch{Epoch: 1, Inputs: []Input{{Tick: id, Sequence: id}}})
	}
	select {
	case <-session.drained:
	case <-time.After(time.Second):
		t.Fatal("datagram pump stopped draining")
	}
	if len(transport.datagrams) != 32 {
		t.Fatalf("unbounded datagram queue: %d", len(transport.datagrams))
	}
	if err := writeStreamMessage(remote, Hello{Compatibility: "key", Name: "reliable"}); err != nil {
		t.Fatal(err)
	}
	// ReadReliable queues the parsed message immediately after the pipe write.
	// Wait for its publication so this test measures priority, not arrival races.
	ready := time.Now().Add(time.Second)
	for len(transport.reliable) == 0 && time.Now().Before(ready) {
		time.Sleep(time.Millisecond)
	}
	message, err := transport.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := message.(Hello); !ok {
		t.Fatalf("reliable control evicted/starved: %T", message)
	}
	for id := uint32(33); id <= 64; id++ {
		message, err := transport.ReadMessage()
		if err != nil || message.(InputBatch).Inputs[0].Sequence != id {
			t.Fatalf("stale datagram retained: sequence=%d message=%+v error=%v", id, message, err)
		}
	}
}

type blockedWTStream struct {
	net.Conn
	writing chan struct{}
}

func (s *blockedWTStream) Write(data []byte) (int, error) {
	select {
	case s.writing <- struct{}{}:
	default:
	}
	return s.Conn.Write(data)
}

func TestWebTransportDatagramBypassesBlockedReliableWrite(t *testing.T) {
	local, remote := net.Pipe()
	stream := &blockedWTStream{Conn: local, writing: make(chan struct{}, 1)}
	session := &testWTSession{sent: make(chan []byte, 1), receive: make(chan []byte), done: make(chan struct{})}
	transport := newWebTransportTransport(context.Background(), stream, session, false)
	t.Cleanup(func() { transport.Close(); remote.Close(); transport.waitClosed() })
	written := make(chan error, 1)
	go func() { written <- transport.WriteMessage(Hello{Compatibility: "test", Name: "blocked"}) }()
	<-stream.writing
	input := make(chan error, 1)
	go func() { input <- transport.WriteMessage(InputBatch{Epoch: 1, Inputs: []Input{{Tick: 1, Sequence: 1}}}) }()
	select {
	case err := <-input:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("datagram blocked behind reliable write")
	}
	if _, err := ReadClientMessage(remote); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
}

func TestWebTransportRejectsInvalidDatagramDirections(t *testing.T) {
	for _, test := range []struct {
		message any
		server  bool
	}{
		{Ping{Nonce: 1}, true}, {Hello{Compatibility: "key", Name: "name"}, true},
		{Snapshot{Epoch: 1, ID: 1, State: []byte{1}}, true}, {Snapshot{Epoch: 1, ID: 1, State: []byte{1}}, false},
		{InputBatch{Epoch: 1}, false},
	} {
		if _, err := decodeGameDatagram(marshalProtocol(t, test.message), test.server); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted %T server=%v: %v", test.message, test.server, err)
		}
	}
	for _, data := range [][]byte{make([]byte, MaxGameDatagramBytes+1), []byte("truncated"), append(marshalProtocol(t, InputBatch{Epoch: 1}), 0)} {
		if _, err := decodeGameDatagram(data, true); !errors.Is(err, ErrProtocol) {
			t.Fatal("accepted malformed datagram")
		}
	}
	transport, session, _ := testWTTransport(t, true)
	session.receive <- marshalProtocol(t, Pong{Nonce: 1})
	if _, err := transport.ReadMessage(); !errors.Is(err, ErrProtocol) {
		t.Fatalf("invalid peer datagram error=%v", err)
	}
}

func TestWebTransportReadDeadlineAndCancellation(t *testing.T) {
	transport, _, _ := testWTTransport(t, false)
	_ = transport.SetReadDeadline(time.Now().Add(-time.Second))
	if _, err := transport.ReadMessage(); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("deadline=%v", err)
	}
	_ = transport.SetReadDeadline(time.Time{})
	done := make(chan error, 1)
	go func() { _, err := transport.ReadMessage(); done <- err }()
	_ = transport.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	select {
	case err := <-done:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("deadline update did not wake pending read")
	}
	_ = transport.SetReadDeadline(time.Time{})
	go func() { _, err := transport.ReadMessage(); done <- err }()
	transport.Close()
	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not cancel read")
	}
}

func TestWebTransportLostBaselineReorderAndEpochRecovery(t *testing.T) {
	transport, session, remote := testWTTransport(t, false)
	encoder, _ := snapshotCodecs(t)
	baseline := noisySnapshot(1)
	if err := encoder.Commit(baseline); err != nil {
		t.Fatal(err)
	}
	lost := noisySnapshot(2)
	lost.State[10] ^= 1
	if err := encoder.Commit(lost); err != nil {
		t.Fatal(err)
	}
	dependent, err := encoder.Encode(noisySnapshot(3), 2)
	if err != nil {
		t.Fatal(err)
	}
	recovered := noisySnapshot(4)
	recovered.State[11] ^= 1
	if _, err := encoder.Encode(recovered, 0); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Commit(recovered); err != nil {
		t.Fatal(err)
	}
	latest := noisySnapshot(5)
	latest.State[12] ^= 1
	delta, err := encoder.Encode(latest, 4)
	if err != nil {
		t.Fatal(err)
	}
	dependentFrame := marshalProtocol(t, dependent)
	latestFrame := marshalProtocol(t, delta)
	staleEpoch := delta
	staleEpoch.Epoch = 99
	staleEpoch.ID = 99
	staleEpochFrame := marshalProtocol(t, staleEpoch)
	serverDone := make(chan error, 1)
	go func() {
		if _, err := ReadClientMessage(remote); err != nil {
			serverDone <- err
			return
		}
		if err := writeStreamMessage(remote, Welcome{Epoch: 1, PlayerID: 1}); err != nil {
			serverDone <- err
			return
		}
		if err := writeStreamMessage(remote, baseline); err != nil {
			serverDone <- err
			return
		}
		// Snapshot2 was lost; snapshot3 arrives but cannot reconstruct it.
		session.receive <- dependentFrame
		request, err := ReadClientMessage(remote)
		if err != nil {
			serverDone <- err
			return
		}
		batch, ok := request.(InputBatch)
		if !ok || batch.Epoch != 1 || batch.SnapshotAck != 0 || len(batch.Inputs) != 0 {
			serverDone <- errors.New("missing reliable baseline recovery request")
			return
		}
		if err := writeStreamMessage(remote, recovered); err != nil {
			serverDone <- err
			return
		}
		// A real sender may use baseline4 only after the client acknowledges
		// decoding it; reliable bytes and QUIC datagrams can arrive reordered.
		select {
		case data := <-session.sent:
			value, err := decodeGameDatagram(data, true)
			if err != nil || value.(InputBatch).SnapshotAck != 4 {
				serverDone <- errors.New("missing reconstructed baseline acknowledgment")
				return
			}
		case <-time.After(time.Second):
			serverDone <- errors.New("baseline acknowledgment timed out")
			return
		}
		// Delayed previous-epoch and older snapshots must not overwrite recovery.
		session.receive <- staleEpochFrame
		session.receive <- dependentFrame
		session.receive <- latestFrame
		serverDone <- nil
	}()
	client, err := Connect(context.Background(), transport, Hello{Compatibility: "test", Name: "faults"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for {
		select {
		case snapshot := <-client.snapshots:
			if snapshot.ID == 4 {
				if err := client.SendInputs(InputBatch{Epoch: 1, SnapshotAck: 4}); err != nil {
					t.Fatal(err)
				}
			}
			if snapshot.ID == 5 {
				if !bytes.Equal(snapshot.State, latest.State) {
					t.Fatal("recovery bytes mismatch")
				}
				if err := <-serverDone; err != nil {
					t.Fatal(err)
				}
				return
			}
		case <-client.done:
			t.Fatalf("packet loss terminated client: %v", client.Err())
		case <-timeout.C:
			t.Fatal("full baseline recovery failed")
		}
	}
}
