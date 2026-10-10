package netgame

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxChatRunes    = 160
	MaxChatBytes    = MaxChatRunes * utf8.UTFMax
	chatBurst       = 4
	chatTokenPeriod = time.Second / 2
)

// ChatSay has no sender identity. The match supplies that from the connection.
type ChatSay struct{ Text string }

// IDs increase throughout a server session, including map changes. Chat travels
// on reliable control paths; it does not wait for or alter a simulation tick.
type ChatEvent struct {
	ID       uint64
	PlayerID byte
	Name     string
	Text     string
}

var (
	ErrChatThrottled = errors.New("chat rate limit reached")
	ErrChatQueueFull = errors.New("chat queue is full")
)

func validChatText(text string, maxBytes, maxRunes int) bool {
	if len(text) == 0 || len(text) > maxBytes || !utf8.ValidString(text) || utf8.RuneCountInString(text) > maxRunes || strings.TrimSpace(text) == "" {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return false
		}
	}
	return true
}

type chatRateLimit struct {
	stamp  time.Time
	credit time.Duration
}

func (r *chatRateLimit) allow(now time.Time) bool {
	if r.stamp.IsZero() {
		r.stamp, r.credit = now, chatBurst*chatTokenPeriod
	} else if elapsed := now.Sub(r.stamp); elapsed > 0 {
		r.credit = min(chatBurst*chatTokenPeriod, r.credit+min(elapsed, chatBurst*chatTokenPeriod))
		r.stamp = now
	}
	if r.credit < chatTokenPeriod {
		return false
	}
	r.credit -= chatTokenPeriod
	return true
}

func (m *Match) chat(handle ConnectionID, say ChatSay, now time.Time) (ChatEvent, error) {
	p := m.players[handle]
	if p == nil || p.suspended {
		return ChatEvent{}, ErrUnknownConnection
	}
	if _, err := validateMessage(say); err != nil {
		return ChatEvent{}, err
	}
	if !p.chatRate.allow(now) {
		return ChatEvent{}, ErrChatThrottled
	}
	if m.chatID == math.MaxUint64 {
		return ChatEvent{}, ErrInputEpochExhausted
	}
	m.chatID++
	return ChatEvent{ID: m.chatID, PlayerID: p.id, Name: p.name, Text: strings.Join(strings.Fields(say.Text), " ")}, nil
}

type chatRequest struct {
	id   ConnectionID
	say  ChatSay
	peer *streamPeer
}

func (s *Server) acceptChat(request chatRequest, now time.Time) {
	if request.peer != nil {
		<-request.peer.chatSlots
	}
	event, err := s.match.chat(request.id, request.say, now)
	if err != nil {
		return
	}
	for id, peer := range s.peers {
		select {
		case peer.chats <- event:
		case <-peer.done:
		default:
			// Never discard one chat line while pretending reliable delivery.
			// A slow recipient reconnects independently of the simulation.
			s.suspend(id)
		}
	}
}
