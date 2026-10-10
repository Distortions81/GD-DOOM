package doomruntime

import (
	"context"
	"net"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

// This exercises real map physics and real TCP framing, rather than a mock
// world. One client temporarily sends no inputs while the other keeps playing.
func TestAuthoritativeTCPRealMapContinuesAndRecoversSilentClient(t *testing.T) {
	a := testAuthority(t)
	match, err := netgame.NewMatch(a, a, netgame.MatchConfig{Epoch: 17, Compatibility: "test-map", PlayerLimit: 4, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 2})
	if err != nil {
		t.Fatal(err)
	}
	server, err := netgame.NewServer(match)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, ln) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server shutdown stalled")
		}
	}()
	send := func(conn net.Conn, message any) {
		t.Helper()
		data, err := netgame.MarshalMessage(message)
		if err != nil {
			t.Fatal(err)
		}
		for len(data) > 0 {
			n, err := conn.Write(data)
			if err != nil {
				t.Fatal(err)
			}
			data = data[n:]
		}
	}
	dial := func() (net.Conn, netgame.Welcome) {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		send(conn, netgame.Hello{Compatibility: "test-map", Name: "test"})
		message, err := netgame.ReadMessage(conn)
		if err != nil {
			t.Fatal(err)
		}
		welcome, ok := message.(netgame.Welcome)
		if !ok {
			t.Fatalf("handshake=%T %+v", message, message)
		}
		return conn, welcome
	}
	first, w1 := dial()
	second, w2 := dial()
	// Drain second client's state without producing any inputs during the gap.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for {
			if _, err := netgame.ReadMessage(second); err != nil {
				return
			}
		}
	}()
	defer func() { second.Close(); <-drained }()
	var initial, latest authorityReplica
	decoder, err := netgame.NewSnapshotDecoder()
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	var previous uint32
	for previous < 56 {
		message, err := netgame.ReadMessage(first)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, ok := message.(netgame.Snapshot)
		if !ok {
			t.Fatalf("message=%T", message)
		}
		snapshot, err = decoder.Decode(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Tick <= previous {
			t.Fatal("server clock stalled or went backward")
		}
		previous = snapshot.Tick
		latest, err = decodeAuthorityReplica(snapshot.State)
		if err != nil {
			t.Fatal(err)
		}
		if len(latest.Players) != 2 {
			continue
		}
		if len(initial.Players) == 0 {
			initial = latest
		}
		if snapshot.Finalized.Tick != snapshot.Tick || latest.Tic != snapshot.Tick {
			t.Fatalf("snapshot and input deadlines disagree: %+v", snapshot.Finalized)
		}
		// This low-level framing test decodes full JSON on the reader goroutine.
		// Allow bounded instrumentation delay under -race. The production client
		// instead measures RTT and schedules its lead adaptively.
		const testLead = 12
		send(first, netgame.InputBatch{Epoch: w1.Epoch, SnapshotAck: snapshot.ID, Inputs: []netgame.Input{{Sequence: snapshot.Tick, Tick: snapshot.Tick + testLead, Command: demo.Tic{Forward: 25}}}})
		if snapshot.Tick >= 20 {
			send(second, netgame.InputBatch{Epoch: w2.Epoch, Inputs: []netgame.Input{{Sequence: snapshot.Tick, Tick: snapshot.Tick + testLead, Command: demo.Tic{Forward: 25}}}})
		} else {
			if latest.Players[1].P != initial.Players[1].P {
				t.Fatal("silent client received unsubmitted movement")
			}
		}
	}
	if len(initial.Players) != 2 || len(latest.Players) != 2 {
		t.Fatal("missing two-player baseline")
	}
	if initial.Players[0].P.X == latest.Players[0].P.X && initial.Players[0].P.Y == latest.Players[0].P.Y {
		t.Fatal("active client did not move")
	}
	if initial.Players[1].P.X == latest.Players[1].P.X && initial.Players[1].P.Y == latest.Players[1].P.Y {
		t.Fatal("silent client did not resume movement")
	}
	t.Logf("received real-map baselines through tic %d; final full baseline %d bytes", latest.Tic, lenMustSnapshot(t, latest))
}

func lenMustSnapshot(t *testing.T, r authorityReplica) int {
	t.Helper()
	data, err := encodeAuthorityReplica(r)
	if err != nil {
		t.Fatal(err)
	}
	return len(data)
}
