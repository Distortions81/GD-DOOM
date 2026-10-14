package netgame

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// The production read deadline is recorded and mapped to a shorter real socket
// deadline. This tests blocked-read interruption without a five-second sleep or
// global clock/timeout mutations that would race other clients.
type livenessTestTransport struct {
	*streamTransport
	scale     time.Duration
	requested chan time.Time
}

func (t *livenessTestTransport) SetReadDeadline(deadline time.Time) error {
	select {
	case t.requested <- deadline:
	default:
	}
	if !deadline.IsZero() {
		deadline = time.Now().Add(t.scale)
	}
	return t.Conn.SetReadDeadline(deadline)
}

func newLivenessTestClient(t *testing.T, scale time.Duration) (*Client, net.Conn, *livenessTestTransport) {
	t.Helper()
	local, remote := net.Pipe()
	transport := &livenessTestTransport{streamTransport: &streamTransport{local}, scale: scale, requested: make(chan time.Time, 32)}
	handshake := make(chan error, 1)
	go func() {
		_, err := ReadClientMessage(remote)
		if err == nil {
			err = writeStreamMessage(remote, Welcome{Epoch: 1, PlayerID: 1, ResumeToken: [32]byte{1}, ResumeGraceTicks: 1050})
		}
		handshake <- err
	}()
	client, err := Connect(context.Background(), transport, Hello{Compatibility: "test", Name: "liveness"})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-handshake; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { remote.Close(); client.Close() })
	return client, remote, transport
}

func TestClientSilentSocketTimesOutAndStartsResume(t *testing.T) {
	client, _, transport := newLivenessTestClient(t, 25*time.Millisecond)
	dialed := make(chan struct{})
	var once sync.Once
	resumed, err := NewReconnectingClient(context.Background(), client, discoveryTestManifest(), func(ctx context.Context, _ [32]byte) (*Client, CompatibilityManifest, error) {
		once.Do(func() { close(dialed) })
		<-ctx.Done()
		return nil, CompatibilityManifest{}, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	select {
	case deadline := <-transport.requested:
		remaining := time.Until(deadline)
		if remaining < 4*time.Second || remaining > serverSilenceTimeout {
			t.Fatalf("server liveness bound=%s", remaining)
		}
	case <-time.After(time.Second):
		t.Fatal("reader has no silence deadline")
	}
	select {
	case <-client.done:
	case <-time.After(time.Second):
		t.Fatal("silent open socket did not fail")
	}
	var timeout net.Error
	if !errors.As(client.Err(), &timeout) || !timeout.Timeout() {
		t.Fatalf("read error=%v", client.Err())
	}
	_, _, _ = resumed.PollSnapshot()
	select {
	case <-dialed:
	case <-time.After(time.Second):
		t.Fatal("blackholed connection did not start resume")
	}
	if resumed.Status().State != ConnectionReconnecting {
		t.Fatalf("status=%+v", resumed.Status())
	}
}

func TestClientReliablePongsKeepLostSnapshotPathAlive(t *testing.T) {
	client, remote, transport := newLivenessTestClient(t, 50*time.Millisecond)
	// No world snapshots arrive. Every reliable Pong extends the next read's
	// deadline, as it does for an idle spectator or a lossy datagram path.
	for nonce := uint64(1); nonce <= 8; nonce++ {
		select {
		case <-transport.requested:
		case <-client.done:
			t.Fatalf("Pong-only path disconnected: %v", client.Err())
		case <-time.After(time.Second):
			t.Fatal("reader did not re-arm")
		}
		time.Sleep(10 * time.Millisecond)
		if err := writeStreamMessage(remote, Pong{Nonce: nonce}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-transport.requested:
	case <-time.After(time.Second):
		t.Fatal("last Pong did not refresh liveness")
	}
	select {
	case <-client.done:
		t.Fatalf("live reliable control rejected: %v", client.Err())
	default:
	}
}
