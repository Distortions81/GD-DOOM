package netgame

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

type codecSnapshotWorld struct {
	*testWorld
	state []byte
}

func (w codecSnapshotWorld) Snapshot(id byte) ([]byte, error) {
	s := bytes.Clone(w.state)
	s[0], s[1] = byte(w.tick), id
	return s, nil
}

func TestServerWritesAcknowledgedSnapshotDelta(t *testing.T) {
	match, world := newTestMatch(t)
	match.snapshots = codecSnapshotWorld{world, noisySnapshot(1).State}
	server, err := NewServer(match)
	if err != nil {
		t.Fatal(err)
	}
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	}()
	conn := listener.dial(t)
	if err := writeStreamMessage(conn, Hello{Compatibility: "test-content", Name: "codec"}); err != nil {
		t.Fatal(err)
	}
	message, err := ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	welcome := message.(Welcome)
	_, decoder := snapshotCodecs(t)
	sawDelta := false
	for range 6 {
		message, err = ReadMessage(conn)
		if err != nil {
			t.Fatal(err)
		}
		wire, ok := message.(Snapshot)
		if !ok {
			t.Fatalf("message=%T", message)
		}
		full, err := decoder.Decode(wire)
		if err != nil {
			t.Fatal(err)
		}
		if full.State[0] != byte(full.Tick) || full.State[1] != welcome.PlayerID {
			t.Fatal("wrong reconstructed world")
		}
		if wire.Encoding == SnapshotDeltaZstd {
			sawDelta = true
			break
		}
		if err := writeStreamMessage(conn, InputBatch{Epoch: welcome.Epoch, SnapshotAck: full.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if !sawDelta {
		t.Fatal("server did not use acknowledged sent baseline")
	}
}

type failedSnapshotConn struct{ net.Conn }

func (failedSnapshotConn) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestServerDoesNotCommitFailedSnapshotWrite(t *testing.T) {
	encoder, _ := snapshotCodecs(t)
	peer := streamPeer{conn: failedSnapshotConn{}}
	if err := peer.writeSnapshot(encoder, noisySnapshot(1)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error=%v", err)
	}
	if len(encoder.history.states) != 0 {
		t.Fatal("failed write became a baseline")
	}
}
