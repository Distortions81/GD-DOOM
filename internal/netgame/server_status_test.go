package netgame

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestServerStatusProtocolPreservesLegacyDiscovery(t *testing.T) {
	manifest := discoveryTestManifest()
	legacy := ServerInfo{Manifest: manifest}
	// This is the original v2 shape, intentionally with no live-status fields.
	oldJSON, err := json.Marshal(struct{ Manifest CompatibilityManifest }{manifest})
	if err != nil {
		t.Fatal(err)
	}
	if got := marshalProtocol(t, legacy); !bytes.Equal(got, protocolFrame(KindServerInfo, oldJSON)) {
		t.Fatal("legacy ServerInfo bytes changed")
	}
	if got := marshalProtocol(t, Query{}); !bytes.Equal(got, protocolFrame(KindQuery, nil)) {
		t.Fatal("legacy Query bytes changed")
	}
	status := ServerStatus{Manifest: manifest, Players: 3, PlayerLimit: 4, Spectators: 2, SpectatorLimit: 16, ReservedPlayers: 1}
	for _, message := range []any{StatusQuery{}, status} {
		got, err := UnmarshalMessage(marshalProtocol(t, message))
		if err != nil || !reflect.DeepEqual(got, message) {
			t.Fatalf("status roundtrip=%+v err=%v", got, err)
		}
	}
	if _, err := ReadClientMessage(bytes.NewReader(marshalProtocol(t, StatusQuery{}))); err != nil {
		t.Fatal(err)
	}
	frame := marshalProtocol(t, status)
	if _, err := ReadClientMessage(bytes.NewReader(frame[:messageHeaderBytes])); !errors.Is(err, ErrProtocol) {
		t.Fatal("client sent a server status body")
	}
	invalid := [][]byte{
		protocolFrame(KindStatusQuery, []byte{1}),
		protocolFrame(KindServerStatus, []byte(`{"Manifest":{},"Players":0,"PlayerLimit":4}`)),
		protocolFrame(KindServerStatus, append(bytes.Clone(frame[messageHeaderBytes:]), []byte(`{}`)...)),
		protocolFrame(KindServerStatus, bytes.Replace(frame[messageHeaderBytes:], []byte(`"Players":3`), []byte(`"Players":3,"Names":["private"]`), 1)),
	}
	header := bytes.Clone(frame[:messageHeaderBytes])
	binary.LittleEndian.PutUint32(header[6:], MaxServerInfoBytes+1)
	invalid = append(invalid, header)
	for _, frame := range invalid {
		if _, err := UnmarshalMessage(frame); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted malformed status: %v", err)
		}
	}
	for _, mutate := range []func(*ServerStatus){
		func(s *ServerStatus) { s.Players = -1 },
		func(s *ServerStatus) { s.Players = 5 },
		func(s *ServerStatus) { s.PlayerLimit = 0 },
		func(s *ServerStatus) { s.PlayerLimit = MaxPlayers + 1 },
		func(s *ServerStatus) { s.Spectators = -1 },
		func(s *ServerStatus) { s.Spectators = 17 },
		func(s *ServerStatus) { s.SpectatorLimit = -1 },
		func(s *ServerStatus) { s.SpectatorLimit = MaxSpectators + 1 },
		func(s *ServerStatus) { s.ReservedPlayers = -1 },
		func(s *ServerStatus) { s.ReservedPlayers = s.Players + 1 },
	} {
		bad := status
		mutate(&bad)
		if _, err := MarshalMessage(bad); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted invalid live counts: %+v, err=%v", bad, err)
		}
	}
}

func TestMatchServerStatusOwnsMetadataAndCountsReservedBodies(t *testing.T) {
	m, world := newDiscoveryTestMatch(t)
	m.config.ResumeGraceTicks = 35
	original, _ := m.ServerInfo()
	key, _ := original.Manifest.Key()
	join := func(spectator bool) (ConnectionID, Welcome) {
		handle, welcome, err := m.Join(Hello{Compatibility: key, Name: "private display name", Spectator: spectator})
		if err != nil {
			t.Fatal(err)
		}
		return handle, welcome
	}
	one, welcome := join(false)
	_, _ = join(false)
	watcher, _ := join(true)
	m.Suspend(one)
	status, err := m.ServerStatus()
	if err != nil || status.Players != 2 || status.PlayerLimit != 2 || status.Spectators != 1 || status.SpectatorLimit != 16 || status.ReservedPlayers != 1 {
		t.Fatalf("incorrect occupied capacity: %+v %v", status, err)
	}
	frame := marshalProtocol(t, status)
	if bytes.Contains(frame, []byte("private display name")) || bytes.Contains(frame, welcome.ResumeToken[:]) {
		t.Fatal("public status leaked private session identity")
	}
	status.Manifest.WADHashes[0] = "mutated client copy"
	unchanged, _ := m.ServerInfo()
	if !reflect.DeepEqual(original, unchanged) {
		t.Fatal("status metadata aliases match compatibility identity")
	}
	resumed, _, err := m.Join(Hello{Compatibility: key, Name: "resumed", ResumeToken: welcome.ResumeToken})
	if err != nil {
		t.Fatal(err)
	}
	m.Leave(watcher)
	after, _ := m.ServerStatus()
	if after.Players != 2 || after.ReservedPlayers != 0 || after.Spectators != 0 || len(world.players) != 2 || world.tick != 0 {
		t.Fatal("status/resume disturbed admission or simulation")
	}
	m.Leave(resumed)
	after, _ = m.ServerStatus()
	if after.Players != 1 {
		t.Fatal("explicit leave did not free advertised capacity")
	}
}

func TestServerStatusQueriesDoNotJoinAndRemainAvailableWhenFull(t *testing.T) {
	m, _ := newDiscoveryTestMatch(t)
	m.config.ResumeGraceTicks = 350
	original, _ := m.ServerInfo()
	key, _ := original.Manifest.Key()
	server, err := NewServer(m)
	if err != nil {
		t.Fatal(err)
	}
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("status server did not stop")
		}
	})
	query := func() ServerStatus {
		t.Helper()
		status, err := QueryServerStatus(ctx, &streamTransport{listener.dial(t)})
		if err != nil {
			t.Fatal(err)
		}
		return status
	}
	for range 3 {
		if status := query(); status.Players != 0 || status.Spectators != 0 || status.PlayerLimit != 2 {
			t.Fatalf("empty match query changed occupancy: %+v", status)
		}
	}
	join := func(spectator bool) *Client {
		t.Helper()
		client, err := Connect(ctx, &streamTransport{listener.dial(t)}, Hello{Compatibility: key, Name: "player", Spectator: spectator})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { client.Leave() })
		return client
	}
	one := join(false)
	if one.Welcome().PlayerID != 1 || one.Welcome().ServerTick != 0 {
		t.Fatal("status queries allocated a slot or advanced an empty world")
	}
	_ = join(false)
	_ = join(true)
	if status := query(); status.Players != 2 || status.Spectators != 1 || status.ReservedPlayers != 0 {
		t.Fatalf("full match status=%+v", status)
	}
	// Old browsers still receive the exact legacy metadata shape when full.
	old, err := QueryServer(ctx, &streamTransport{listener.dial(t)})
	if err != nil || !reflect.DeepEqual(old, original) {
		t.Fatalf("legacy discovery changed: %+v %v", old, err)
	}
	one.Close()
	deadline := time.Now().Add(time.Second)
	for {
		status := query()
		if status.ReservedPlayers == 1 {
			if status.Players != 2 || status.Spectators != 1 {
				t.Fatal("reserved body was advertised as a free player slot")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("status never reflected transport-loss reservation")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestQueryServerStatusCancelsBlockedReadAndRejectsWrongResponse(t *testing.T) {
	for _, cancelQuery := range []bool{true, false} {
		local, remote := net.Pipe()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { _, err := QueryServerStatus(ctx, &streamTransport{local}); done <- err }()
		message, err := ReadClientMessage(remote)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := message.(StatusQuery); !ok {
			t.Fatalf("query record=%T", message)
		}
		want := ErrProtocol
		if cancelQuery {
			cancel()
			want = context.Canceled
		} else if err := writeStreamMessage(remote, ServerInfo{Manifest: discoveryTestManifest()}); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if !errors.Is(err, want) {
				t.Fatalf("query failure=%v, want %v", err, want)
			}
		case <-time.After(time.Second):
			t.Fatal("status query remained blocked")
		}
		if _, err := remote.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
			t.Fatal("status query did not close its transport")
		}
		cancel()
		remote.Close()
	}
}
