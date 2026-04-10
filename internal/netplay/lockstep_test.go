package netplay

import (
	"testing"
	"time"

	"gddoom/internal/demo"
)

// dialCoordinator spins up a server, connects two PlayerPeer clients,
// and returns LockstepCoordinators for each.
func dialCoordinator(t *testing.T) (c1, c2 *LockstepCoordinator, cleanup func()) {
	t.Helper()
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer: %v", err)
	}

	cfg1 := SessionConfig{WADHash: "abc", MapName: "E1M1", GameMode: "coop", PlayerSlot: 1}
	cfg2 := SessionConfig{WADHash: "abc", MapName: "E1M1", GameMode: "coop", PlayerSlot: 2}

	peer1, err := DialPlayerPeer(srv.Addr(), 0, cfg1)
	if err != nil {
		srv.Close()
		t.Fatalf("DialPlayerPeer 1: %v", err)
	}
	peer2, err := DialPlayerPeer(srv.Addr(), peer1.SessionID(), cfg2)
	if err != nil {
		peer1.Close()
		srv.Close()
		t.Fatalf("DialPlayerPeer 2: %v", err)
	}

	c1 = NewLockstepCoordinator(peer1)
	c2 = NewLockstepCoordinator(peer2)

	cleanup = func() {
		peer1.Close()
		peer2.Close()
		srv.Close()
	}
	return c1, c2, cleanup
}

func waitReady(t *testing.T, c *LockstepCoordinator, minTics int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.ReadyTics() >= minTics {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d ready tics, got %d", minTics, c.ReadyTics())
}

func waitRoster(t *testing.T, c *LockstepCoordinator, wantLen int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(c.ActivePeerIDs()) == wantLen {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for roster len=%d, got %v", wantLen, c.ActivePeerIDs())
}

func TestLockstepReadyTicsGatesOnSlowestPeer(t *testing.T) {
	c1, c2, cleanup := dialCoordinator(t)
	defer cleanup()

	// Wait for both coordinators to see each other in the roster.
	waitRoster(t, c1, 1)
	waitRoster(t, c2, 1)

	// Before any tics are sent, neither side is ready.
	if got := c1.ReadyTics(); got != 0 {
		t.Fatalf("c1.ReadyTics()=%d want=0 before any sends", got)
	}

	// peer2 sends one tic; c1 should now have 1 ready tic.
	tc := demo.Tic{Forward: 5, Side: -3, AngleTurn: 64, Buttons: 0}
	if err := c2.SendLocalTic(tc); err != nil {
		t.Fatalf("c2.SendLocalTic: %v", err)
	}
	waitReady(t, c1, 1)

	if got := c1.ReadyTics(); got < 1 {
		t.Fatalf("c1.ReadyTics()=%d want>=1 after peer2 sent", got)
	}
}

func TestLockstepPollPeerTicRoundTrip(t *testing.T) {
	c1, c2, cleanup := dialCoordinator(t)
	defer cleanup()

	waitRoster(t, c1, 1)
	waitRoster(t, c2, 1)

	sent := demo.Tic{Forward: 10, Side: 0, AngleTurn: -256, Buttons: 1}
	if err := c2.SendLocalTic(sent); err != nil {
		t.Fatalf("SendLocalTic: %v", err)
	}
	waitReady(t, c1, 1)

	got, ok, err := c1.PollPeerTic(c2.LocalPlayerID())
	if err != nil {
		t.Fatalf("PollPeerTic: %v", err)
	}
	if !ok {
		t.Fatal("PollPeerTic: expected tic, got none")
	}
	// AngleTurn is packed/unpacked with lossy compression; check forward/side exactly.
	if got.Forward != sent.Forward || got.Side != sent.Side {
		t.Fatalf("tic mismatch: got %+v want forward=%d side=%d", got, sent.Forward, sent.Side)
	}
}

func TestLockstepLocalPlayerID(t *testing.T) {
	c1, c2, cleanup := dialCoordinator(t)
	defer cleanup()

	if c1.LocalPlayerID() != 1 {
		t.Fatalf("c1 playerID=%d want=1", c1.LocalPlayerID())
	}
	if c2.LocalPlayerID() != 2 {
		t.Fatalf("c2 playerID=%d want=2", c2.LocalPlayerID())
	}
}

func TestLockstepRosterUpdate(t *testing.T) {
	c1, c2, cleanup := dialCoordinator(t)
	defer cleanup()

	waitRoster(t, c1, 1) // c1 sees c2
	waitRoster(t, c2, 1) // c2 sees c1

	peers1 := c1.ActivePeerIDs()
	if len(peers1) != 1 || peers1[0] != c2.LocalPlayerID() {
		t.Fatalf("c1 peers=%v want=[%d]", peers1, c2.LocalPlayerID())
	}
}
