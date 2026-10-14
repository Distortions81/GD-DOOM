package netgame

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func resumePipeClient(t *testing.T, welcome Welcome) (*Client, net.Conn, <-chan any) {
	t.Helper()
	local, remote := net.Pipe()
	input := make(chan any, 8)
	go func() {
		defer close(input)
		if _, err := ReadClientMessage(remote); err != nil {
			return
		}
		if err := writeStreamMessage(remote, welcome); err != nil {
			return
		}
		for {
			message, err := ReadClientMessage(remote)
			if err != nil {
				return
			}
			input <- message
		}
	}()
	client, err := Connect(context.Background(), &streamTransport{local}, Hello{Compatibility: "test", Name: "resume-test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { remote.Close(); client.Close() })
	return client, remote, input
}

func waitReconnectState(t *testing.T, c *ReconnectingClient, want ConnectionState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, _, _ = c.PollSnapshot()
		if c.Status().State == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("status=%+v want=%s", c.Status(), want)
}

func TestReconnectingClientGatesFreshSessionAndDropsOfflineIntent(t *testing.T) {
	welcome := Welcome{Epoch: 7, PlayerID: 1, InputLead: 3, ResumeGraceTicks: 1050, ResumeToken: [32]byte{1}}
	first, remote, _ := resumePipeClient(t, welcome)
	welcome.ServerTick, welcome.ResumeToken = 60, [32]byte{2}
	next, nextRemote, inputs := resumePipeClient(t, welcome)
	dialed := make(chan struct{}, 1)
	c, err := NewReconnectingClient(context.Background(), first, discoveryTestManifest(), func(ctx context.Context, token [32]byte) (*Client, CompatibilityManifest, error) {
		if token != ([32]byte{1}) {
			return nil, CompatibilityManifest{}, errors.New("incorrect resume token")
		}
		if _, bounded := ctx.Deadline(); bounded {
			return nil, CompatibilityManifest{}, errors.New("successful session would inherit opening deadline")
		}
		dialed <- struct{}{}
		return next, discoveryTestManifest(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	remote.Close()
	waitReconnectState(t, c, ConnectionReconnecting)
	select {
	case <-dialed:
	case <-time.After(time.Second):
		t.Fatal("retry not started")
	}
	if err := writeStreamMessage(nextRemote, Snapshot{Epoch: 7, ID: 1, Tick: 60, State: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.PollSnapshot(); err != nil || ok {
		t.Fatal("new baseline escaped before explicit session reset")
	}
	if err := c.SendInputs(InputBatch{Epoch: 7, Inputs: []Input{{Sequence: 999, Tick: 61}}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	var change ClientSessionChange
	for time.Now().Before(deadline) {
		if value, ok := c.PollSessionChange(); ok {
			change = value
			break
		}
		time.Sleep(time.Millisecond)
	}
	if change.Generation != 2 || change.Welcome.ServerTick != 60 || c.Welcome().ResumeToken != ([32]byte{2}) {
		t.Fatalf("resume install=%+v", change)
	}
	if err := c.SendInputs(InputBatch{Epoch: 7, Inputs: []Input{{Sequence: 1, Tick: 64}}}); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-inputs:
		batch, ok := message.(InputBatch)
		if !ok || len(batch.Inputs) != 1 || batch.Inputs[0].Sequence != 1 {
			t.Fatalf("old intent leaked: %+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("resumed input not delivered")
	}
}

func TestReconnectingClientRetainsFinalSnapshotAndDoesNotResumeCompletion(t *testing.T) {
	first, remote, _ := resumePipeClient(t, Welcome{Epoch: 7, PlayerID: 1, ResumeToken: [32]byte{1}, ResumeGraceTicks: 1050})
	dialed := false
	c, err := NewReconnectingClient(context.Background(), first, discoveryTestManifest(), func(context.Context, [32]byte) (*Client, CompatibilityManifest, error) {
		dialed = true
		return nil, CompatibilityManifest{}, errors.New("unexpected retry")
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := writeStreamMessage(remote, Snapshot{Epoch: 7, ID: 9, Tick: 90, State: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if err := writeStreamMessage(remote, Disconnect{Reason: "Match complete: frag_limit"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.done:
	case <-time.After(time.Second):
		t.Fatal("completion not received")
	}
	snapshot, ok, err := c.PollSnapshot()
	if err != nil || !ok || snapshot.Tick != 90 {
		t.Fatal("completion replaced final baseline")
	}
	_, _, _ = c.PollSnapshot()
	if c.Status().State != ConnectionComplete || dialed {
		t.Fatalf("completion status=%+v retried=%v", c.Status(), dialed)
	}
}

func TestReconnectingClientBoundsStalledAttemptByGrace(t *testing.T) {
	first, remote, _ := resumePipeClient(t, Welcome{Epoch: 7, PlayerID: 1, ResumeToken: [32]byte{1}, ResumeGraceTicks: 1})
	cancelled := make(chan struct{})
	c, err := NewReconnectingClient(context.Background(), first, discoveryTestManifest(), func(ctx context.Context, _ [32]byte) (*Client, CompatibilityManifest, error) {
		<-ctx.Done()
		close(cancelled)
		return nil, CompatibilityManifest{}, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	remote.Close()
	waitReconnectState(t, c, ConnectionDisconnected)
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("expired retry IO not cancelled")
	}
	if c.Status().Attempt != 1 {
		t.Fatal("retry exceeded server grace")
	}
}
