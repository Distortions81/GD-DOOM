package netgame

import (
	"errors"
	"math"
)

// PlayerID zero cycles to the next present body. A spectator follows a real
// authoritative player viewpoint without owning that player or sending inputs.
type FollowPlayer struct{ PlayerID byte }

var ErrSpectatorsFull = errors.New("spectator capacity is full")
var ErrCameraUnavailable = errors.New("requested player camera is unavailable")

func (m *Match) joinSpectator(hello Hello) (ConnectionID, Welcome, error) {
	count := 0
	for _, p := range m.players {
		if p.spectator {
			count++
		}
	}
	if count >= MaxSpectators {
		return 0, Welcome{}, ErrSpectatorsFull
	}
	if m.world.Tic() == math.MaxUint32 || m.nextConnection == ConnectionID(math.MaxUint64) {
		return 0, Welcome{}, ErrInputEpochExhausted
	}
	buffer, err := NewInputBuffer(InputBufferConfig{FirstTick: m.world.Tic() + 1, FutureTicks: m.config.FutureTicks, HoldTicks: m.config.HoldTicks})
	if err != nil {
		return 0, Welcome{}, err
	}
	p := &matchPlayer{name: hello.Name, spectator: true, input: buffer}
	m.nextConnection++
	m.players[m.nextConnection] = p
	return m.nextConnection, m.welcome(p), nil
}

func (m *Match) playerSlotPresent(id byte) bool {
	if id == 0 {
		return false
	}
	for _, p := range m.players {
		if !p.spectator && p.id == id {
			return true
		}
	}
	return false
}

func (m *Match) spectatorView(p *matchPlayer) byte {
	if m.playerSlotPresent(p.viewer) {
		return p.viewer
	}
	p.viewer = 0
	for id := byte(1); id <= MaxPlayers; id++ {
		if m.playerSlotPresent(id) {
			p.viewer = id
			break
		}
	}
	return p.viewer
}

func (m *Match) follow(handle ConnectionID, target byte) error {
	p := m.players[handle]
	if p == nil || p.suspended {
		return ErrUnknownConnection
	}
	if !p.spectator || target > MaxPlayers {
		return ErrProtocol
	}
	if target != 0 {
		if !m.playerSlotPresent(target) {
			return ErrCameraUnavailable
		}
		p.viewer = target
		return nil
	}
	current := m.spectatorView(p)
	for offset := byte(1); offset <= MaxPlayers; offset++ {
		id := (current+offset-1)%MaxPlayers + 1
		if m.playerSlotPresent(id) {
			p.viewer = id
			return nil
		}
	}
	return ErrCameraUnavailable
}

type followRequest struct {
	id     ConnectionID
	target byte
	peer   *streamPeer
}

func (s *Server) acceptFollow(request followRequest) {
	if request.peer != nil {
		<-request.peer.inputSlots
	}
	if err := s.match.follow(request.id, request.target); err == nil {
		if peer := s.peers[request.id]; peer != nil {
			peer.requestFull.Store(true)
		}
	} else if errors.Is(err, ErrProtocol) {
		s.drop(request.id)
	}
}

// FollowPlayer is optional spectator control and never carries movement intent.
func (c *Client) FollowPlayer(id byte) error {
	if id > MaxPlayers {
		return ErrProtocol
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.welcome.PlayerID != 0 {
		return ErrProtocol
	}
	select {
	case <-c.done:
		return c.err
	default:
	}
	select {
	case c.following <- FollowPlayer{PlayerID: id}:
		return nil
	default:
		return ErrControlQueueFull
	}
}

var ErrControlQueueFull = errors.New("spectator control queue is full")
