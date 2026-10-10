package netgame

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServerMapTransitionKeepsTwoClientsAndIgnoresOldEpochInput(t *testing.T) {
	m, _ := newTestMatch(t)
	w := &completingTestWorld{completeAt: 4}
	m.world, m.snapshots, m.config.DisconnectTicks = w, w, 350
	s, err := NewServer(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetTransitionHandler(func() (*MapTransition, error) {
		w.tick, w.complete, w.completeAt = 0, false, 1000
		return &MapTransition{Epoch: 100, Map: "E1M2", Compatibility: "next-content"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})
	dial := func() (net.Conn, Welcome) {
		conn := listener.dial(t)
		if err := writeStreamMessage(conn, Hello{Compatibility: "test-content", Name: "player"}); err != nil {
			t.Fatal(err)
		}
		message, err := ReadMessage(conn)
		if err != nil {
			t.Fatal(err)
		}
		welcome, ok := message.(Welcome)
		if !ok {
			t.Fatalf("unexpected handshake %T", message)
		}
		return conn, welcome
	}
	a, wa := dial()
	b, wb := dial()
	for _, peer := range []struct {
		conn net.Conn
		old  Welcome
	}{{a, wa}, {b, wb}} {
		var change MapChange
		for {
			message, err := ReadMessage(peer.conn)
			if err != nil {
				t.Fatal(err)
			}
			if c, ok := message.(MapChange); ok {
				change = c
				break
			}
			if snapshot, ok := message.(Snapshot); !ok || snapshot.Epoch != peer.old.Epoch {
				t.Fatalf("new baseline appeared before map control: %+v", message)
			}
		}
		if change.PreviousEpoch != peer.old.Epoch || change.Welcome.Epoch != 100 || change.Welcome.PlayerID != peer.old.PlayerID || change.Map != "E1M2" || change.Compatibility != "next-content" {
			t.Fatalf("wrong transition: %+v", change)
		}
		message, err := ReadMessage(peer.conn)
		if err != nil {
			t.Fatal(err)
		}
		baseline, ok := message.(Snapshot)
		if !ok || baseline.Epoch != 100 || baseline.ID != 1 || baseline.Tick != 0 || baseline.Finalized.HasTick {
			t.Fatalf("transition did not deliver initial baseline: %+v", message)
		}
		// This old packet was already in flight when MapChange arrived. It
		// must not kick the authenticated connection or alter the new epoch.
		if err := writeStreamMessage(peer.conn, InputBatch{Epoch: peer.old.Epoch, SnapshotAck: 999}); err != nil {
			t.Fatal(err)
		}
		if err := writeStreamMessage(peer.conn, InputBatch{Epoch: 100, SnapshotAck: baseline.ID}); err != nil {
			t.Fatal(err)
		}
	}
	for _, conn := range []net.Conn{a, b} {
		message, err := ReadMessage(conn)
		if err != nil {
			t.Fatalf("stale epoch disconnected client: %v", err)
		}
		if snapshot, ok := message.(Snapshot); !ok || snapshot.Epoch != 100 || snapshot.Tick == 0 {
			t.Fatalf("new level did not continue: %+v", message)
		}
	}
}

func TestServerPendingMapControlPrecedesFastNextRoundCompletion(t *testing.T) {
	m, _ := newTestMatch(t)
	w := &completingTestWorld{completeAt: 2}
	m.world, m.snapshots, m.config.DisconnectTicks = w, w, 350
	s, _ := NewServer(m)
	transitions := 0
	if err := s.SetTransitionHandler(func() (*MapTransition, error) {
		if transitions > 0 {
			return nil, nil
		}
		transitions++
		w.tick, w.complete, w.completeAt = 0, false, 1
		return &MapTransition{Epoch: 100, Map: "E1M2", Compatibility: "next-content"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	conn := listener.dial(t)
	if err := writeStreamMessage(conn, Hello{Compatibility: "test-content", Name: "player"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMessage(conn); err != nil {
		t.Fatal(err)
	}
	// The first replaceable snapshot blocks this pipe's writer. Let the
	// owner enqueue the map change and finish its one-tic replacement round.
	time.Sleep(150 * time.Millisecond)
	sawChange, sawInitial, sawFinal := false, false, false
	for {
		message, err := ReadMessage(conn)
		if err != nil {
			t.Fatal(err)
		}
		switch msg := message.(type) {
		case MapChange:
			sawChange = true
		case Snapshot:
			if msg.Epoch == 100 {
				if !sawChange {
					t.Fatal("new round baseline overtook reliable map control")
				}
				if msg.Tick == 0 {
					sawInitial = true
				} else if msg.Tick == 1 {
					sawFinal = true
				}
			}
		case Disconnect:
			if !sawChange || !sawInitial || !sawFinal {
				t.Fatalf("incomplete terminal transition delivery: change=%t initial=%t final=%t", sawChange, sawInitial, sawFinal)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("completed server did not exit")
			}
			return
		}
	}
}
