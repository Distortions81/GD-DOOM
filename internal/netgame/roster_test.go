package netgame

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRosterRejectsInvalidIdentityAndBounds(t *testing.T) {
	valid := Roster{Revision: 1, PlayerLimit: 4, Count: 2, Players: [MaxParticipants]PlayerPresence{
		{ID: 5, PlayerID: 1, Name: "Alice", Connected: true, PingMillis: 25},
		{ID: 7, PlayerID: 2, Name: "Bob", Connected: true},
	}}
	for name, edit := range map[string]func(*Roster){
		"revision":       func(r *Roster) { r.Revision = 0 },
		"count":          func(r *Roster) { r.Count = MaxParticipants + 1 },
		"limit":          func(r *Roster) { r.PlayerLimit = 0 },
		"overfull":       func(r *Roster) { r.PlayerLimit = 1 },
		"id zero":        func(r *Roster) { r.Players[0].ID = 0 },
		"id duplicate":   func(r *Roster) { r.Players[1].ID = r.Players[0].ID },
		"id unordered":   func(r *Roster) { r.Players[1].ID = 1 },
		"slot duplicate": func(r *Roster) { r.Players[1].PlayerID = 1 },
		"slot invalid":   func(r *Roster) { r.Players[1].PlayerID = MaxPlayers + 1 },
		"spectator flag": func(r *Roster) { r.Players[1].Spectator = true },
		"empty name":     func(r *Roster) { r.Players[1].Name = "" },
		"name controls":  func(r *Roster) { r.Players[1].Name = "bad\nname" },
		"name size":      func(r *Roster) { r.Players[1].Name = strings.Repeat("x", 65) },
		"invalid utf8":   func(r *Roster) { r.Players[1].Name = "\xff" },
		"ping bounds":    func(r *Roster) { r.Players[1].PingMillis = 1001 },
		"offline ping":   func(r *Roster) { r.Players[0].Connected = false },
		"unused tail":    func(r *Roster) { r.Players[2].ID = 20 },
		"offline spectator": func(r *Roster) {
			r.Players[1].PlayerID, r.Players[1].Spectator, r.Players[1].Connected = 0, true, false
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := valid
			edit(&r)
			if _, err := MarshalMessage(r); !errors.Is(err, ErrProtocol) {
				t.Fatalf("invalid roster accepted: %v", err)
			}
		})
	}
	frame := marshalProtocol(t, valid)
	// Flags have only connected/spectator bits; reject unknown bits on wire.
	frame[messageHeaderBytes+10+8+1] = 4
	if _, err := UnmarshalMessage(frame); !errors.Is(err, ErrProtocol) {
		t.Fatalf("flags accepted: %v", err)
	}
}

func TestRosterMaximumAndClientForgery(t *testing.T) {
	r := Roster{Revision: 1, PlayerLimit: 4, Count: MaxParticipants}
	for i := range int(r.Count) {
		p := PlayerPresence{ID: uint64(i + 1), Name: strings.Repeat("x", 64), Connected: true, PingMillis: 1000}
		if i < MaxPlayers {
			p.PlayerID = byte(i + 1)
		} else {
			p.Spectator = true
		}
		r.Players[i] = p
	}
	frame := marshalProtocol(t, r)
	if len(frame) != messageHeaderBytes+rosterBodyMaxBytes {
		t.Fatal("wrong maximum roster size")
	}
	got, err := UnmarshalMessage(frame)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("maximum roster: %v %v", got, err)
	}
	if _, err := ReadClientMessage(bytes.NewReader(frame[:messageHeaderBytes])); !errors.Is(err, ErrProtocol) {
		t.Fatalf("client roster payload accepted: %v", err)
	}
}

func TestRosterCoalescesAndAccessorDoesNotAllocate(t *testing.T) {
	p := &streamPeer{rosters: make(chan Roster, 1)}
	for i := uint64(1); i <= 100; i++ {
		p.offerRoster(Roster{Revision: i, PlayerLimit: 4})
	}
	r := <-p.rosters
	if r.Revision != 100 || len(p.rosters) != 0 {
		t.Fatal("rosters were not coalesced")
	}
	c := &Client{roster: r}
	if count := testing.AllocsPerRun(100, func() { _, _ = c.Roster() }); count != 0 {
		t.Fatalf("accessor allocates %v", count)
	}
	copy, _ := c.Roster()
	copy.Players[0].Name = "modified"
	if again, _ := c.Roster(); again.Players[0].Name != "" {
		t.Fatal("roster accessor exposes mutable state")
	}
}

func TestRosterIdentitySurvivesResumeAndSlotReuseDoesNot(t *testing.T) {
	m, _ := newTestMatch(t)
	m.config.ResumeGraceTicks = 350
	id, welcome := joinTestMatch(t, m)
	presence := m.players[id].presenceID
	previous := m.rosterRevision
	m.Suspend(id)
	if !m.players[id].suspended || m.rosterRevision <= previous {
		t.Fatal("suspension did not change roster")
	}
	next, resumed, err := m.Join(Hello{Compatibility: m.config.Compatibility, Name: "ignored", ResumeToken: welcome.ResumeToken})
	if err != nil || resumed.PlayerID != welcome.PlayerID || m.players[next].presenceID != presence {
		t.Fatalf("resume identity: %v", err)
	}
	m.Leave(next)
	replacement, joined := joinTestMatch(t, m)
	if joined.PlayerID != welcome.PlayerID || m.players[replacement].presenceID == presence {
		t.Fatal("replacement reused presence identity")
	}
}

func TestRosterBroadcastCadenceAndImmediateMembership(t *testing.T) {
	m, _ := newTestMatch(t)
	id, _ := joinTestMatch(t, m)
	p := &streamPeer{rosters: make(chan Roster, 1)}
	s := &Server{match: m, peers: map[ConnectionID]*streamPeer{id: p}}
	now := time.Unix(100, 0)
	s.publishRoster(now)
	initial := <-p.rosters
	p.latency.roundTrip = 42 * time.Millisecond
	s.publishRoster(now.Add(time.Second - time.Nanosecond))
	if len(p.rosters) != 0 {
		t.Fatal("timing-only update exceeded1Hz")
	}
	s.publishRoster(now.Add(time.Second))
	updated := <-p.rosters
	if updated.Revision <= initial.Revision || updated.Players[0].PingMillis != 42 {
		t.Fatal("timing update missing")
	}
	joinTestMatch(t, m)
	s.publishRoster(now.Add(time.Second + time.Nanosecond))
	if joined := <-p.rosters; joined.Count != 2 || joined.Revision <= updated.Revision {
		t.Fatal("membership update was delayed")
	}
}

func waitClientRoster(t *testing.T, c *Client, check func(Roster) bool) Roster {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if r, ok := c.Roster(); ok && check(r) {
			return r
		}
		if err := c.Err(); err != nil {
			t.Fatalf("connection failed: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	r, _ := c.Roster()
	t.Fatalf("roster condition not reached: %+v", r)
	return Roster{}
}

func TestServerRosterNamesPingJoinLeaveAndSuspend(t *testing.T) {
	m, _ := newTestMatch(t)
	m.config.DisconnectTicks, m.config.ResumeGraceTicks = 350, 350
	s, err := NewServer(m)
	if err != nil {
		t.Fatal(err)
	}
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	join := func(name string, spectator bool) *Client {
		c, err := Connect(ctx, &streamTransport{listener.dial(t)}, Hello{Compatibility: m.config.Compatibility, Name: name, Spectator: spectator})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	alice, bob, observer := join("Alice", false), join("Bob", false), join("Observer", true)
	r := waitClientRoster(t, alice, func(r Roster) bool {
		return r.Count == 3 && r.Players[0].PingMillis > 0 && r.Players[1].PingMillis > 0 && r.Players[2].PingMillis > 0
	})
	if r.PlayerLimit != 4 || r.Players[0].Name != "Alice" || r.Players[1].Name != "Bob" || r.Players[2].Name != "Observer" || !r.Players[2].Spectator {
		t.Fatalf("roster identity mismatch: %+v", r)
	}
	bobID := r.Players[1].ID
	if err := observer.Leave(); err != nil {
		t.Fatal(err)
	}
	waitClientRoster(t, alice, func(r Roster) bool { return r.Count == 2 })
	_ = bob.Close()
	r = waitClientRoster(t, alice, func(r Roster) bool { return r.Count == 2 && !r.Players[1].Connected })
	if r.Players[1].ID != bobID || r.Players[1].PingMillis != 0 {
		t.Fatal("suspended identity/latency is wrong")
	}
	if err := alice.Leave(); err != nil {
		t.Fatal(err)
	}
}

// Existing wire-level gameplay tests still check snapshots directly, while
// servicing the independent roster and ping controls used by real clients.
func readGameplayTestMessage(conn io.ReadWriter) (any, error) {
	for {
		message, err := ReadMessage(conn)
		if err != nil {
			return nil, err
		}
		switch message := message.(type) {
		case Roster:
			continue
		case Ping:
			if err := writeStreamMessage(conn, Pong{Nonce: message.Nonce}); err != nil {
				return nil, err
			}
		default:
			return message, nil
		}
	}
}
