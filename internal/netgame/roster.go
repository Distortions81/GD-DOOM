package netgame

import (
	"cmp"
	"slices"
	"time"
)

const MaxParticipants = MaxPlayers + MaxSpectators
const rosterBodyMaxBytes = 8 + 1 + 1 + MaxParticipants*(8+1+1+2+2+64)

// PlayerPresence contains public session identity only. ID survives reconnects
// and map changes; a reused player slot receives a new ID. Zero ping is unknown.
type PlayerPresence struct {
	ID         uint64
	PlayerID   byte
	Name       string
	Connected  bool
	Spectator  bool
	PingMillis uint16
}

// Roster is a bounded value so reading it on every render update allocates
// nothing. Only Players[:Count] is populated; the unused tail is always zero.
type Roster struct {
	Revision    uint64
	PlayerLimit uint8
	Count       uint8
	Players     [MaxParticipants]PlayerPresence
}

func validateRoster(r Roster) error {
	if r.Revision == 0 || int(r.Count) > MaxParticipants || r.PlayerLimit < 1 || r.PlayerLimit > MaxPlayers {
		return ErrProtocol
	}
	var slots [MaxPlayers + 1]bool
	var previous uint64
	spectators := 0
	for _, p := range r.Players[:r.Count] {
		if p.ID <= previous || p.PlayerID > MaxPlayers || p.Spectator != (p.PlayerID == 0) ||
			!validChatText(p.Name, 64, 64) || p.PingMillis > uint16(maximumRoundTrip.Milliseconds()) || (!p.Connected && p.PingMillis != 0) {
			return ErrProtocol
		}
		if p.Spectator {
			spectators++
			if spectators > MaxSpectators || !p.Connected {
				return ErrProtocol
			}
		} else {
			if slots[p.PlayerID] {
				return ErrProtocol
			}
			slots[p.PlayerID] = true
		}
		previous = p.ID
	}
	for _, p := range r.Players[r.Count:] {
		if p != (PlayerPresence{}) {
			return ErrProtocol
		}
	}
	if int(r.Count)-spectators > int(r.PlayerLimit) {
		return ErrProtocol
	}
	return nil
}

// Roster returns the latest server-authored view, not client-supplied names or
// timing estimates. False means the initial roster has not arrived yet.
func (c *Client) Roster() (Roster, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.roster, c.roster.Revision != 0
}

func (c *ReconnectingClient) Roster() (Roster, bool) {
	active := c.current()
	if active == nil {
		return Roster{}, false
	}
	return active.Roster()
}

// publishRoster runs on the match owner. Membership changes are immediate;
// timing-only updates occur once per second and unsent rosters are replaced.
func (s *Server) publishRoster(now time.Time) {
	if s.rosterMembership == s.match.rosterRevision && !s.rosterStamp.IsZero() && now.Sub(s.rosterStamp) < pingInterval {
		return
	}
	s.rosterMembership, s.rosterStamp = s.match.rosterRevision, now
	s.rosterRevision++
	roster := Roster{Revision: s.rosterRevision, PlayerLimit: uint8(s.match.config.PlayerLimit)}
	for handle, player := range s.match.players {
		entry := PlayerPresence{ID: player.presenceID, PlayerID: player.id, Name: player.name, Connected: !player.suspended, Spectator: player.spectator}
		if peer := s.peers[handle]; peer != nil && entry.Connected {
			peer.latencyMu.Lock()
			rtt := peer.latency.roundTrip
			peer.latencyMu.Unlock()
			if rtt > 0 {
				entry.PingMillis = uint16(max(1, rtt.Milliseconds()))
			}
		}
		roster.Players[roster.Count] = entry
		roster.Count++
	}
	slices.SortFunc(roster.Players[:roster.Count], func(a, b PlayerPresence) int { return cmp.Compare(a.ID, b.ID) })
	for _, peer := range s.peers {
		peer.offerRoster(roster)
	}
}

func (p *streamPeer) offerRoster(roster Roster) {
	select {
	case p.rosters <- roster:
		return
	default:
	}
	select {
	case <-p.rosters:
	default:
	}
	select {
	case p.rosters <- roster:
	default:
	}
}
