package netgame

import (
	"errors"
	"time"
)

// MapTransition describes a World that the owner has just advanced. Epoch must
// increase and the World must have reset to tic zero with the same active slots.
type MapTransition struct {
	Epoch         uint64
	Map           string
	Compatibility string
}

// TransitionHandler runs synchronously on the simulation owner after completion.
// It may select/load content and advance the World, but must never perform socket
// IO. Returning nil finishes the session normally; returning metadata continues
// the existing authenticated connections on the new map. Errors stop the server.
type TransitionHandler func() (*MapTransition, error)

// SetTransitionHandler configures optional campaign/rotation before Serve starts.
func (s *Server) SetTransitionHandler(handler TransitionHandler) error {
	if s.started.Load() {
		return errors.New("map transition handler must be set before Serve")
	}
	s.transition = handler
	return nil
}

type transitionMessage struct {
	control  MapChange
	baseline Snapshot
}

func (s *Server) advanceLevel() (bool, error) {
	if s.transition == nil {
		return false, nil
	}
	previous := s.match.Epoch()
	transition, err := s.transition()
	if err != nil || transition == nil {
		return false, err
	}
	// Validate even when no peers remain, before changing match input history.
	check := MapChange{PreviousEpoch: previous, Welcome: Welcome{Epoch: transition.Epoch, PlayerID: 1}, Map: transition.Map, Compatibility: transition.Compatibility}
	if _, err := validateMessage(check); err != nil {
		return false, err
	}
	welcomes, err := s.match.ResetEpoch(transition.Epoch, transition.Compatibility, transition.Map)
	if err != nil {
		return false, err
	}
	initial, err := s.match.snapshotResult(TickResult{Tick: s.match.Tic()})
	if err != nil {
		return false, err
	}
	for id, welcome := range welcomes {
		peer := s.peers[id]
		if peer == nil {
			continue
		}
		control := MapChange{PreviousEpoch: previous, Welcome: welcome, Map: transition.Map, Compatibility: transition.Compatibility}
		message := transitionMessage{control: control, baseline: initial.Snapshots[id]}
		select {
		case peer.transitions <- message:
		case <-peer.done:
			s.drop(id)
		default:
			// Reliable controls cannot be coalesced. A peer unable to drain
			// several map changes is dropped without delaying other players.
			s.drop(id)
		}
	}
	return true, nil
}

func (p *streamPeer) writeTransition(change transitionMessage, encoder *SnapshotEncoder) error {
	_ = p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := writeStreamMessage(p.conn, change.control); err != nil {
		return err
	}
	if change.baseline.ID == 0 {
		return nil
	}
	return p.writeSnapshot(encoder, change.baseline, true)
}
