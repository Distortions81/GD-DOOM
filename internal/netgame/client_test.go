package netgame

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func pipeClient(t *testing.T, serve func(net.Conn)) (*Client, <-chan struct{}) {
	t.Helper()
	local, remote := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer remote.Close()
		hello, err := ReadClientMessage(remote)
		if err != nil {
			return
		}
		if _, ok := hello.(Hello); !ok {
			return
		}
		if err := writeStreamMessage(remote, Welcome{Epoch: 12, PlayerID: 1, InputLead: 3}); err != nil {
			return
		}
		serve(remote)
	}()
	c, err := Connect(context.Background(), &streamTransport{local}, Hello{Compatibility: "test", Name: "client"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("pipe server leaked")
		}
	})
	return c, done
}

func TestClientKeepsNewestFullSnapshotAndFinalState(t *testing.T) {
	c, _ := pipeClient(t, func(remote net.Conn) {
		for _, id := range []uint32{10, 9, 11} {
			if err := writeStreamMessage(remote, Snapshot{Epoch: 12, ID: id, Tick: id, State: []byte{byte(id)}}); err != nil {
				return
			}
		}
		_ = writeStreamMessage(remote, Disconnect{Reason: "match complete"})
	})
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("client did not see end of match")
	}
	snapshot, ok, err := c.PollSnapshot()
	if err != nil || !ok || snapshot.ID != 11 {
		t.Fatalf("latest=%+v ok=%v err=%v", snapshot, ok, err)
	}
	if _, ok, err := c.PollSnapshot(); ok || err == nil {
		t.Fatal("missing termination after final state")
	}
}

func TestClientRejectsWrongEpochBaseline(t *testing.T) {
	c, _ := pipeClient(t, func(remote net.Conn) {
		_ = writeStreamMessage(remote, Snapshot{Epoch: 99, ID: 1, Tick: 1, State: []byte{1}})
	})
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("invalid server did not close")
	}
	if !errors.Is(c.Err(), ErrProtocol) {
		t.Fatalf("err=%v", c.Err())
	}
}

func TestClientSendNeverWaitsForBlockedNetwork(t *testing.T) {
	release := make(chan struct{})
	c, _ := pipeClient(t, func(remote net.Conn) { <-release })
	defer close(release)
	finished := make(chan error, 1)
	go func() {
		for i := uint32(1); i <= 10000; i++ {
			if err := c.SendInputs(InputBatch{Epoch: 12, Inputs: []Input{{Sequence: i, Tick: i}}}); err != nil {
				finished <- err
				return
			}
		}
		finished <- nil
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("game input waited for socket")
	}
	if len(c.outgoing) > 1 {
		t.Fatal("unbounded outgoing input")
	}
}

func TestClientCancellationInterruptsHandshake(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Connect(ctx, &streamTransport{local}, Hello{Compatibility: "test", Name: "client"})
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled handshake succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled handshake blocked")
	}
}

func TestClientMapTransitionPrecedesNewBaseline(t *testing.T) {
	c, _ := pipeClient(t, func(remote net.Conn) {
		_ = writeStreamMessage(remote, Snapshot{Epoch: 12, ID: 99, Tick: 99, State: []byte{1}})
		_ = writeStreamMessage(remote, MapChange{PreviousEpoch: 12, Welcome: Welcome{Epoch: 13, PlayerID: 1, InputLead: 3}, Map: "E1M2", Compatibility: "next"})
		_ = writeStreamMessage(remote, Snapshot{Epoch: 13, ID: 1, State: []byte{2}})
		_ = writeStreamMessage(remote, Disconnect{Reason: "done"})
	})
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("connection did not finish")
	}
	if got := c.Welcome().Epoch; got != 12 {
		t.Fatalf("epoch published before map load: %d", got)
	}
	if _, ok, err := c.PollSnapshot(); ok || err != nil {
		t.Fatalf("baseline before transition: %v %v", ok, err)
	}
	change, ok, err := c.PollTransition()
	if !ok || err != nil || change.Map != "E1M2" || c.Welcome().Epoch != 13 {
		t.Fatalf("transition=%+v ok=%v err=%v", change, ok, err)
	}
	snapshot, ok, err := c.PollSnapshot()
	if !ok || err != nil || snapshot.Epoch != 13 || snapshot.ID != 1 {
		t.Fatalf("baseline=%+v ok=%v err=%v", snapshot, ok, err)
	}
	if _, ok, err := c.PollSnapshot(); ok || err == nil {
		t.Fatal("missing session end after new-map baseline")
	}
}

func TestClientRejectsMapChangeRebindingPlayer(t *testing.T) {
	c, _ := pipeClient(t, func(remote net.Conn) {
		_ = writeStreamMessage(remote, MapChange{PreviousEpoch: 12, Welcome: Welcome{Epoch: 13, PlayerID: 2, InputLead: 3}, Map: "E1M2", Compatibility: "next"})
	})
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("connection did not finish")
	}
	if !errors.Is(c.Err(), ErrProtocol) {
		t.Fatalf("err=%v", c.Err())
	}
}
