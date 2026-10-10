package netgame

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func discoveryTestManifest() CompatibilityManifest {
	return CompatibilityManifest{Simulation: SimulationVersion, WADHashes: []string{strings.Repeat("ab", 32)}, Map: "E1M1", Mode: "coop", Skill: 3, RespawnDelayTics: 35}
}

func newDiscoveryTestMatch(t *testing.T) (*Match, *testWorld) {
	t.Helper()
	base, w := newTestMatch(t)
	manifest := discoveryTestManifest()
	key, _ := manifest.Key()
	config := base.config
	config.Compatibility, config.Manifest = key, &manifest
	config.PlayerLimit, config.DisconnectTicks = 2, 350
	m, err := NewMatch(w, w, config)
	if err != nil {
		t.Fatal(err)
	}
	return m, w
}

func TestDiscoveryProtocolRoundTripAndBounds(t *testing.T) {
	info := ServerInfo{Manifest: discoveryTestManifest()}
	for _, message := range []any{Query{}, info} {
		frame := marshalProtocol(t, message)
		got, err := UnmarshalMessage(frame)
		if err != nil || !reflect.DeepEqual(got, message) {
			t.Fatalf("roundtrip=%+v err=%v", got, err)
		}
	}
	if _, err := ReadClientMessage(bytes.NewReader(marshalProtocol(t, Query{}))); err != nil {
		t.Fatal(err)
	}
	frame := marshalProtocol(t, info)
	if _, err := ReadClientMessage(bytes.NewReader(frame[:messageHeaderBytes])); !errors.Is(err, ErrProtocol) {
		t.Fatalf("client server-info header was accepted: %v", err)
	}
	invalid := [][]byte{
		protocolFrame(KindQuery, []byte{0}),
		protocolFrame(KindServerInfo, []byte(`{"Manifest":{},"unexpected":1}`)),
		protocolFrame(KindServerInfo, []byte(`{"Manifest":{}}`)),
		protocolFrame(KindServerInfo, append(bytes.Clone(frame[messageHeaderBytes:]), []byte(`{}`)...)),
	}
	header := bytes.Clone(frame[:messageHeaderBytes])
	binary.LittleEndian.PutUint32(header[6:], MaxServerInfoBytes+1)
	invalid = append(invalid, header)
	for _, data := range invalid {
		if _, err := UnmarshalMessage(data); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted malformed discovery: %v", err)
		}
	}
	for _, change := range []func(*CompatibilityManifest){
		func(m *CompatibilityManifest) { m.WADHashes = make([]string, 65) },
		func(m *CompatibilityManifest) { m.Map = "../../E1M1" },
		func(m *CompatibilityManifest) { m.Simulation = strings.Repeat("s", 129) },
		func(m *CompatibilityManifest) { m.Skill = 6 },
		func(m *CompatibilityManifest) { m.FragLimit = -1 },
	} {
		bad := ServerInfo{Manifest: discoveryTestManifest()}
		change(&bad.Manifest)
		if _, err := MarshalMessage(bad); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted invalid manifest: %v", err)
		}
	}
}

func TestMatchDiscoveryOwnsManifestAndUpdatesMapAtomically(t *testing.T) {
	m, _ := newDiscoveryTestMatch(t)
	original, _ := m.ServerInfo()
	copyInfo, _ := m.ServerInfo()
	copyInfo.Manifest.WADHashes[0], copyInfo.Manifest.Map = "changed", "MAP30"
	unchanged, _ := m.ServerInfo()
	if !reflect.DeepEqual(original, unchanged) {
		t.Fatal("public info aliases match metadata")
	}
	config := m.config
	input := discoveryTestManifest()
	config.Manifest = &input
	independent, err := NewMatch(m.world, m.snapshots, config)
	if err != nil {
		t.Fatal(err)
	}
	input.WADHashes[0] = "changed"
	cloned, _ := independent.ServerInfo()
	if !reflect.DeepEqual(original, cloned) {
		t.Fatal("match aliases caller metadata")
	}
	if _, err := NewMatch(m.world, m.snapshots, config); !errors.Is(err, ErrCompatibility) {
		t.Fatal("invalid manifest accepted")
	}
	config.Manifest = &original.Manifest
	config.Compatibility = "wrong key"
	if _, err := NewMatch(m.world, m.snapshots, config); !errors.Is(err, ErrCompatibility) {
		t.Fatal("manifest/key mismatch accepted")
	}
	m.completed = true
	next := cloneCompatibilityManifest(original.Manifest)
	next.Map = "E1M2"
	key, _ := next.Key()
	for _, tc := range []struct {
		key  string
		maps []string
	}{{key, nil}, {"wrong", []string{"E1M2"}}, {key, []string{"MAP30"}}} {
		if _, err := m.ResetEpoch(100, tc.key, tc.maps...); !errors.Is(err, ErrCompatibility) {
			t.Fatalf("invalid reset=%v", err)
		}
		info, _ := m.ServerInfo()
		if m.Epoch() != 99 || !reflect.DeepEqual(info, original) {
			t.Fatal("failed map reset changed published metadata")
		}
	}
	if _, err := m.ResetEpoch(100, key, "E1M2"); err != nil {
		t.Fatal(err)
	}
	info, _ := m.ServerInfo()
	if !reflect.DeepEqual(info.Manifest, next) {
		t.Fatal("new epoch published stale map metadata")
	}
}

func TestServerDiscoveryDoesNotReserveSlotsAndWorksWhenFull(t *testing.T) {
	m, _ := newDiscoveryTestMatch(t)
	expect, _ := m.ServerInfo()
	key, _ := expect.Manifest.Key()
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
	query := func() {
		t.Helper()
		conn := listener.dial(t)
		info, err := QueryServer(ctx, &streamTransport{conn})
		if err != nil || !reflect.DeepEqual(info, expect) {
			t.Fatalf("discovery=%+v err=%v", info, err)
		}
	}
	for range 3 {
		query()
	}
	for want := byte(1); want <= 2; want++ {
		conn := listener.dial(t)
		t.Cleanup(func() { conn.Close() })
		if err := writeStreamMessage(conn, Hello{Name: "player", Compatibility: key}); err != nil {
			t.Fatal(err)
		}
		got, err := ReadMessage(conn)
		welcome, ok := got.(Welcome)
		if err != nil || !ok || welcome.PlayerID != want || welcome.ServerTick != 0 {
			t.Fatalf("query changed roster/timeline: %+v %v", got, err)
		}
	}
	query()
}

func TestQueryServerCancelsBlockedReadAndClosesTransport(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := QueryServer(ctx, &streamTransport{local}); done <- err }()
	if got, err := ReadMessage(remote); err != nil {
		t.Fatal(err)
	} else if _, ok := got.(Query); !ok {
		t.Fatalf("first frame=%T", got)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("discovery ignored cancellation")
	}
	if _, err := remote.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("discovery transport stayed open: %v", err)
	}
}
