package netgame

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"math"
)

var ErrResumeUnavailable = errors.New("multiplayer resume token is invalid or expired")

func (m *Match) newResumeToken() ([32]byte, error) {
	var token [32]byte
	if m.config.ResumeGraceTicks == 0 {
		return token, nil
	}
	_, err := rand.Read(token[:])
	return token, err
}

func (m *Match) welcome(p *matchPlayer) Welcome {
	if p.spectator {
		return Welcome{Epoch: m.config.Epoch, PlayerID: 0, ServerTick: m.world.Tic(), InputLead: m.config.InputLead}
	}
	return Welcome{Epoch: m.config.Epoch, PlayerID: p.id, ServerTick: m.world.Tic(), InputLead: m.config.InputLead,
		ResumeToken: p.resumeToken, ResumeGraceTicks: m.config.ResumeGraceTicks}
}

// Suspend keeps a disconnected player's existing body vulnerable in the world
// with neutral inputs for a bounded grace period. Repeated stale socket notices
// never extend that period. Leave instead removes the player immediately.
func (m *Match) Suspend(handle ConnectionID) {
	p := m.players[handle]
	if p == nil || p.suspended {
		return
	}
	if p.spectator {
		m.Leave(handle)
		return
	}
	if m.config.ResumeGraceTicks == 0 {
		m.Leave(handle)
		return
	}
	p.suspended, p.resumeTicks = true, m.config.ResumeGraceTicks
}

// A successful resume rotates both the transport handle and the bearer token,
// including when the former socket has not yet noticed its failure. The body
// and slot stay intact while all input/snapshot history belongs to the new
// connection. No old queued input can run after this owner operation.
func (m *Match) resume(token [32]byte) (ConnectionID, Welcome, error) {
	if m.config.ResumeGraceTicks == 0 {
		return 0, Welcome{}, ErrResumeUnavailable
	}
	if m.world.Tic() == math.MaxUint32 || m.nextConnection == ConnectionID(math.MaxUint64) {
		return 0, Welcome{}, ErrInputEpochExhausted
	}
	for old, p := range m.players {
		if subtle.ConstantTimeCompare(token[:], p.resumeToken[:]) != 1 && subtle.ConstantTimeCompare(token[:], p.previousResumeToken[:]) != 1 {
			continue
		}
		buffer, err := NewInputBuffer(InputBufferConfig{FirstTick: m.world.Tic() + 1, FutureTicks: m.config.FutureTicks, HoldTicks: m.config.HoldTicks})
		if err != nil {
			return 0, Welcome{}, err
		}
		nextToken, err := m.newResumeToken()
		if err != nil {
			return 0, Welcome{}, err
		}
		delete(m.players, old)
		m.nextConnection++
		p.input, p.resumeToken, p.suspended, p.resumeTicks = buffer, nextToken, false, 0
		p.previousResumeToken = token
		p.lastActivity, p.lastSnapshot, p.ackedSnapshot = m.world.Tic(), 0, 0
		m.players[m.nextConnection] = p
		return m.nextConnection, m.welcome(p), nil
	}
	return 0, Welcome{}, ErrResumeUnavailable
}
