package netgame

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServerQueuedInputCannotRevokeResumeReservation(t *testing.T) {
	m, _ := newResumeTestMatch(t, 35)
	old, welcome := joinTestMatch(t, m)
	s, err := NewServer(m)
	if err != nil {
		t.Fatal(err)
	}
	s.suspend(old)
	// These messages entered the bounded owner queue before the reader's
	// loss notification but were dispatched afterward.
	s.acceptClientInput(clientMessage{id: old, batch: InputBatch{Epoch: 99}})
	fresh, _ := resumeTestMatch(t, m, welcome.ResumeToken)
	s.acceptClientInput(clientMessage{id: old, batch: InputBatch{Epoch: 99}})
	if m.players[fresh] == nil || m.players[fresh].suspended || m.PlayerCount() != 1 {
		t.Fatal("old socket input revoked the reserved or resumed body")
	}
}

func TestServerResumeReplacesActiveSocketAndKeepsSlot(t *testing.T) {
	m, _ := newResumeTestMatch(t, 35)
	m.config.PlayerLimit, m.config.DisconnectTicks = 1, 350
	s, err := NewServer(m)
	if err != nil {
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
	dial := func(token [32]byte) (net.Conn, Welcome) {
		t.Helper()
		conn := listener.dial(t)
		t.Cleanup(func() { conn.Close() })
		if err := writeStreamMessage(conn, Hello{Compatibility: "test-content", Name: "player", ResumeToken: token}); err != nil {
			t.Fatal(err)
		}
		message, err := readGameplayTestMessage(conn)
		if err != nil {
			t.Fatal(err)
		}
		welcome, ok := message.(Welcome)
		if !ok {
			t.Fatalf("resume response=%T", message)
		}
		return conn, welcome
	}
	old, first := dial([32]byte{})
	fresh, resumed := dial(first.ResumeToken)
	if resumed.PlayerID != first.PlayerID || resumed.ResumeToken == first.ResumeToken {
		t.Fatal("active socket resume changed slot or retained bearer")
	}
	_ = old.SetReadDeadline(time.Now().Add(time.Second))
	for {
		if _, err := readGameplayTestMessage(old); err != nil {
			break
		}
	}
	message, err := readGameplayTestMessage(fresh)
	baseline, ok := message.(Snapshot)
	if err != nil || !ok || baseline.BaselineID != 0 || baseline.Finalized.HasSequence {
		t.Fatalf("resume lacks independent baseline: %T %v", message, err)
	}
	if err := writeStreamMessage(fresh, InputBatch{Epoch: resumed.Epoch, SnapshotAck: baseline.ID}); err != nil {
		t.Fatal(err)
	}
	// A second loss resumes using the new bearer; the first bearer is retired.
	fresh.Close()
	latest, again := dial(resumed.ResumeToken)
	if again.PlayerID != first.PlayerID {
		t.Fatal("transport loss created a new player")
	}
	if _, err := readGameplayTestMessage(latest); err != nil {
		t.Fatalf("late previous-socket cleanup killed resumed connection: %v", err)
	}
}
