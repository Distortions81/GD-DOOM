package netgame

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"testing"
	"time"
)

func TestLatencyProbeSinglePendingNonceAndEWMA(t *testing.T) {
	start := time.Unix(100, 0)
	var probe latencyProbe
	ping, send := probe.begin(start)
	if !send || ping.Nonce == 0 {
		t.Fatal("missing initial probe")
	}
	if _, send := probe.begin(start.Add(time.Second)); send {
		t.Fatal("created a second pending probe")
	}
	probe.receive(Pong{Nonce: ping.Nonce + 1}, start.Add(50*time.Millisecond))
	if probe.roundTrip != 0 || probe.pending != ping.Nonce {
		t.Fatal("unmatched pong altered RTT")
	}
	probe.receive(Pong{Nonce: ping.Nonce}, start.Add(80*time.Millisecond))
	if probe.roundTrip != 80*time.Millisecond || probe.pending != 0 {
		t.Fatalf("first RTT=%s pending=%d", probe.roundTrip, probe.pending)
	}
	probe.receive(Pong{Nonce: ping.Nonce}, start.Add(time.Second))
	if probe.roundTrip != 80*time.Millisecond {
		t.Fatal("duplicate pong altered RTT")
	}
	next, send := probe.begin(start.Add(time.Second))
	if !send || next.Nonce <= ping.Nonce {
		t.Fatal("nonce did not advance")
	}
	probe.receive(Pong{Nonce: next.Nonce}, start.Add(time.Second+160*time.Millisecond))
	if probe.roundTrip != 90*time.Millisecond {
		t.Fatalf("EWMA=%s", probe.roundTrip)
	}
}

func TestLatencyReplyLimitBoundsUnsolicitedPongs(t *testing.T) {
	var limit latencyReplyLimit
	now := time.Unix(10, 0)
	for range 4 {
		if !limit.allow(now) {
			t.Fatal("small delayed burst rejected")
		}
	}
	for range 100 {
		if limit.allow(now.Add(pingInterval - time.Nanosecond)) {
			t.Fatal("Pong flood accepted")
		}
	}
	if !limit.allow(now.Add(pingInterval)) {
		t.Fatal("next probe interval remained throttled")
	}
}

func TestLatencyProbeExpiryCapAndCounterExhaustion(t *testing.T) {
	start := time.Unix(100, 0)
	var probe latencyProbe
	old, _ := probe.begin(start)
	next, send := probe.begin(start.Add(pingExpiry))
	if !send || next.Nonce == old.Nonce {
		t.Fatal("expired probe blocked replacement")
	}
	probe.receive(Pong{Nonce: old.Nonce}, start.Add(pingExpiry+time.Millisecond))
	if probe.roundTrip != 0 {
		t.Fatal("stale nonce changed RTT")
	}
	probe.receive(Pong{Nonce: next.Nonce}, start.Add(pingExpiry+1500*time.Millisecond))
	if probe.roundTrip != maximumRoundTrip {
		t.Fatalf("RTT cap=%s", probe.roundTrip)
	}
	next, _ = probe.begin(start.Add(4 * time.Second))
	probe.receive(Pong{Nonce: next.Nonce}, start.Add(7*time.Second))
	if probe.roundTrip != maximumRoundTrip || probe.pending != 0 {
		t.Fatal("expired pong changed sample")
	}
	probe.nonce = math.MaxUint64
	if _, send := probe.begin(start.Add(8 * time.Second)); send {
		t.Fatal("probe nonce wrapped")
	}
}

func TestClientPeriodicPingIsSerializedAndMeasured(t *testing.T) {
	received := make(chan Ping, 1)
	client, _ := pipeClient(t, func(remote net.Conn) {
		message, err := ReadClientMessage(remote)
		if err != nil {
			return
		}
		ping, ok := message.(Ping)
		if !ok {
			return
		}
		received <- ping
		_ = writeStreamMessage(remote, Pong{Nonce: ping.Nonce})
		_ = writeStreamMessage(remote, Disconnect{Reason: "probe complete"})
	})
	select {
	case ping := <-received:
		if ping.Nonce == 0 {
			t.Fatal("zero ping")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client did not send periodic ping")
	}
	select {
	case <-client.done:
	case <-time.After(time.Second):
		t.Fatal("client did not process pong and completion")
	}
	client.mu.Lock()
	pending := client.latency.pending
	client.mu.Unlock()
	if pending != 0 || client.RoundTripTime() <= 0 || client.RoundTripTime() > maximumRoundTrip {
		t.Fatalf("RTT=%s pending=%d", client.RoundTripTime(), pending)
	}
}

func TestServerPingsDoNotKeepInactivePlayerAlive(t *testing.T) {
	match, _ := newTestMatch(t)
	match.config.DisconnectTicks = 40
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
	if err := writeStreamMessage(conn, Hello{Compatibility: "test-content", Name: "ping only"}); err != nil {
		t.Fatal(err)
	}
	if _, err := readGameplayTestMessage(conn); err != nil {
		t.Fatal(err)
	}
	if err := writeStreamMessage(conn, Ping{Nonce: 1}); err != nil {
		t.Fatal(err)
	}
	pings, pongs := 0, 0
	var lastTick uint32
	for {
		message, err := ReadMessage(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(err)
			}
			break
		}
		switch m := message.(type) {
		case Roster:
			continue
		case Ping:
			pings++
			if err := writeStreamMessage(conn, Pong{Nonce: m.Nonce}); err != nil {
				t.Fatal(err)
			}
		case Pong:
			pongs++
			if m.Nonce == 0 {
				t.Fatal("invalid nonce")
			}
		case Snapshot:
			lastTick = m.Tick
			if err := writeStreamMessage(conn, Ping{Nonce: uint64(m.Tick) + 1}); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unexpected message %T", message)
		}
	}
	if pings == 0 || pongs == 0 || lastTick < 39 {
		t.Fatalf("closed before bidirectional probes/deadline: pings=%d pongs=%d tic=%d", pings, pongs, lastTick)
	}
}
