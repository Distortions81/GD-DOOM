package netplay

import (
	"sync"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/runtimecfg"
)

// maxPeerTicBuffer is the maximum number of tics buffered per peer before
// older entries are dropped. This bounds memory under adversarial or stalled
// conditions.
const maxPeerTicBuffer = 35 * 10 // ~10 seconds at 35 tics/sec

// LockstepCoordinator wraps a PlayerPeer connection and implements
// runtimecfg.CoopPeerSource. It:
//   - forwards local tics to the relay via PlayerPeer.SendTic
//   - drains incoming PeerTics from the relay into per-player queues
//   - exposes ReadyTics so the game loop knows when all peers have
//     submitted their tic for the current world tic
//   - exposes PollPeerTic for the game to consume each remote player's tic
type LockstepCoordinator struct {
	peer *PlayerPeer

	mu      sync.Mutex
	queues  map[byte][]demo.Tic // player_id → buffered tics
	roster  []byte              // current remote peer IDs (excludes local)
	pending *runtimecfg.RosterUpdate
	err     error
}

// NewLockstepCoordinator creates a coordinator from an already-connected
// PlayerPeer. It starts a background goroutine that drains inbound frames.
func NewLockstepCoordinator(peer *PlayerPeer) *LockstepCoordinator {
	c := &LockstepCoordinator{
		peer:   peer,
		queues: make(map[byte][]demo.Tic),
	}
	go c.drainLoop()
	return c
}

func (c *LockstepCoordinator) drainLoop() {
	for {
		pt, ok, err := c.peer.PollPeerTic()
		if err != nil {
			c.mu.Lock()
			if c.err == nil {
				c.err = err
			}
			c.mu.Unlock()
			return
		}
		if ok {
			c.mu.Lock()
			q := c.queues[pt.PlayerID]
			if len(q) < maxPeerTicBuffer {
				c.queues[pt.PlayerID] = append(q, pt.Tic)
			}
			c.mu.Unlock()
			continue
		}

		// Also drain roster updates.
		roster, hasRoster, rosterErr := c.peer.PollRoster()
		if rosterErr != nil {
			c.mu.Lock()
			if c.err == nil {
				c.err = rosterErr
			}
			c.mu.Unlock()
			return
		}
		if hasRoster {
			c.mu.Lock()
			// Build remote roster: exclude our own player ID.
			localID := c.peer.PlayerID()
			remote := make([]byte, 0, len(roster.PlayerIDs))
			for _, id := range roster.PlayerIDs {
				if id != localID {
					remote = append(remote, id)
				}
			}
			c.roster = remote
			c.pending = &runtimecfg.RosterUpdate{PlayerIDs: append([]byte(nil), remote...)}
			c.mu.Unlock()
			continue
		}

		// Nothing available yet — yield briefly to avoid busy-spin.
		time.Sleep(time.Millisecond)
	}
}

// LocalPlayerID implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) LocalPlayerID() byte {
	if c == nil {
		return 0
	}
	return c.peer.PlayerID()
}

// ActivePeerIDs implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) ActivePeerIDs() []byte {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]byte, len(c.roster))
	copy(out, c.roster)
	return out
}

// SendLocalTic implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) SendLocalTic(tc demo.Tic) error {
	if c == nil {
		return nil
	}
	if err := c.peer.SendTic(tc); err != nil {
		return err
	}
	return c.peer.Flush()
}

// ReadyTics returns the number of complete tics available across all currently
// active remote peers. The game must not advance more tics than this value.
func (c *LockstepCoordinator) ReadyTics() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.roster) == 0 {
		// No remote peers — advance freely (solo with relay connected).
		return maxPeerTicBuffer
	}
	min := maxPeerTicBuffer
	for _, id := range c.roster {
		n := len(c.queues[id])
		if n < min {
			min = n
		}
	}
	return min
}

// PollPeerTic implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) PollPeerTic(playerID byte) (demo.Tic, bool, error) {
	if c == nil {
		return demo.Tic{}, false, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return demo.Tic{}, false, c.err
	}
	q := c.queues[playerID]
	if len(q) == 0 {
		return demo.Tic{}, false, nil
	}
	tc := q[0]
	c.queues[playerID] = q[1:]
	return tc, true, nil
}

// PollRosterUpdate implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) PollRosterUpdate() (runtimecfg.RosterUpdate, bool) {
	if c == nil {
		return runtimecfg.RosterUpdate{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending == nil {
		return runtimecfg.RosterUpdate{}, false
	}
	r := *c.pending
	c.pending = nil
	return r, true
}

// SendKeyframe implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) SendKeyframe(tic uint32, blob []byte) error {
	if c == nil {
		return nil
	}
	return c.peer.SendKeyframe(tic, blob)
}

// PollKeyframe implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) PollKeyframe() ([]byte, bool, bool) {
	if c == nil {
		return nil, false, false
	}
	kf, ok, _ := c.peer.PollKeyframe()
	if !ok {
		return nil, false, false
	}
	return kf.Blob, kf.MandatoryApply, true
}

// SendCheckpoint implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) SendCheckpoint(tic uint32, hash uint32) error {
	if c == nil {
		return nil
	}
	return c.peer.SendCheckpoint(tic, hash)
}

// PollCheckpoint implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) PollCheckpoint() (runtimecfg.Checkpoint, bool) {
	if c == nil {
		return runtimecfg.Checkpoint{}, false
	}
	cp, ok, _ := c.peer.PollCheckpoint()
	if !ok {
		return runtimecfg.Checkpoint{}, false
	}
	return runtimecfg.Checkpoint{Tic: cp.Tic, Hash: cp.Hash}, true
}

// SendDesyncNotify implements runtimecfg.CoopPeerSource.
func (c *LockstepCoordinator) SendDesyncNotify(tic uint32, localHash uint32) error {
	if c == nil {
		return nil
	}
	return c.peer.SendDesyncRequest(DesyncRequest{Tic: tic, LocalHash: localHash})
}
