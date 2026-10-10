// Package netgame implements the authoritative multiplayer session protocol.
// It is independent of rendering, socket transports, and the Doom simulation.
package netgame

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const (
	ProtocolVersion         = 3
	TickRate                = 35
	MaxPlayers              = 4
	MaxSpectators           = 16
	MaxInputBatch           = 8
	MaxSnapshotBytes        = 8 << 20
	MaxServerInfoBytes      = 8 << 10
	messageHeaderBytes      = 10
	snapshotBodyHeaderBytes = 70
	inputBodyHeaderBytes    = 13
	inputCommandBytes       = 13
	welcomeBodyBytes        = 15 + 32 + 4
)

type MessageKind byte

const (
	KindHello MessageKind = iota + 1
	KindWelcome
	KindInput
	KindSnapshot
	KindDisconnect
	KindMapChange
	KindQuery
	KindServerInfo
	KindPing
	KindPong
	KindChatSay
	KindChatEvent
	KindFollowPlayer
	KindStatusQuery
	KindServerStatus
	KindRoster
)

// Hello verifies the exact ordered WAD set, simulation build, and rules digest.
// The server binds the connection to an assigned slot; inputs carry no slot ID.
type Hello struct {
	Compatibility string
	Name          string
	ResumeToken   [32]byte // zero requests a fresh player
	Spectator     bool
}

// Query requests public match metadata before content loading and slot binding.
// It is legal only as the first frame of a short-lived discovery connection.
type Query struct{}

type ServerInfo struct{ Manifest CompatibilityManifest }

// StatusQuery is a separate discovery extension so legacy Query/ServerInfo
// bytes remain unchanged. Older servers may reject this kind; clients can open
// a fresh connection and issue Query when live occupancy is unavailable.
type StatusQuery struct{}

// ServerStatus is advisory discovery data, never a reservation or admission
// guarantee. Players includes ReservedPlayers whose bodies occupy reconnect
// slots; spectators have a separate capacity. No names or bearer tokens escape.
type ServerStatus struct {
	Manifest        CompatibilityManifest
	Players         int
	PlayerLimit     int
	Spectators      int
	SpectatorLimit  int
	ReservedPlayers int
}

type Welcome struct {
	Epoch            uint64
	PlayerID         byte
	ServerTick       uint32
	InputLead        uint16
	ResumeToken      [32]byte
	ResumeGraceTicks uint32
}

// MapChange is reliable control and precedes the new epoch's initial baseline.
// The connection keeps its authenticated slot. Clients must verify Compatibility
// against local content/rules and load Map before applying the new baseline.
type MapChange struct {
	PreviousEpoch uint64
	Welcome       Welcome
	Map           string
	Compatibility string
}

// InputBatch may contain no inputs when only acknowledging a snapshot.
// The match authenticates Epoch and validates tic/sequence scheduling.
type InputBatch struct {
	Epoch       uint64
	SnapshotAck uint32
	Inputs      []Input
}

// Snapshot carries raw, compressed, or dictionary-compressed state. Raw API
// snapshots use zero DecodedSize/Digest; framing fills and verifies these on the
// wire. Decoding compressed states requires SnapshotDecoder before presentation.
type Snapshot struct {
	Epoch       uint64
	ID          uint32
	BaselineID  uint32
	Tick        uint32
	Finalized   InputAck
	Encoding    SnapshotEncoding
	DecodedSize uint32
	Digest      [32]byte
	State       []byte
}

type Disconnect struct{ Reason string }

var ErrProtocol = errors.New("invalid authoritative multiplayer message")

// validateMessage is shared by encoders and decoders without constructing a
// second frame or copying snapshot payloads just to validate a decoded message.
func validateMessage(message any) (MessageKind, error) {
	validString := func(value string, max int) bool {
		return len(value) > 0 && len(value) <= max && utf8.ValidString(value)
	}
	switch m := message.(type) {
	case Roster:
		if err := validateRoster(m); err != nil {
			return 0, err
		}
		return KindRoster, nil
	case FollowPlayer:
		if m.PlayerID > MaxPlayers {
			return 0, ErrProtocol
		}
		return KindFollowPlayer, nil
	case ChatSay:
		if !validChatText(m.Text, MaxChatBytes, MaxChatRunes) {
			return 0, ErrProtocol
		}
		return KindChatSay, nil
	case ChatEvent:
		if m.ID == 0 || m.PlayerID > MaxPlayers || !validChatText(m.Name, 64, 64) || !validChatText(m.Text, MaxChatBytes, MaxChatRunes) {
			return 0, ErrProtocol
		}
		return KindChatEvent, nil
	case Ping:
		if m.Nonce == 0 {
			return 0, ErrProtocol
		}
		return KindPing, nil
	case Pong:
		if m.Nonce == 0 {
			return 0, ErrProtocol
		}
		return KindPong, nil
	case Query:
		return KindQuery, nil
	case StatusQuery:
		return KindStatusQuery, nil
	case ServerStatus:
		if m.PlayerLimit < 1 || m.PlayerLimit > MaxPlayers || m.Players < 0 || m.Players > m.PlayerLimit ||
			m.SpectatorLimit < 0 || m.SpectatorLimit > MaxSpectators || m.Spectators < 0 || m.Spectators > m.SpectatorLimit ||
			m.ReservedPlayers < 0 || m.ReservedPlayers > m.Players {
			return 0, ErrProtocol
		}
		if _, err := m.Manifest.Key(); err != nil {
			return 0, fmt.Errorf("%w: %w", ErrProtocol, err)
		}
		return KindServerStatus, nil
	case ServerInfo:
		if _, err := m.Manifest.Key(); err != nil {
			return 0, fmt.Errorf("%w: %w", ErrProtocol, err)
		}
		return KindServerInfo, nil
	case Hello:
		if !validString(m.Compatibility, 256) || !validChatText(m.Name, 64, 64) {
			return 0, ErrProtocol
		}
		return KindHello, nil
	case Welcome:
		if m.Epoch == 0 || m.PlayerID > MaxPlayers || m.InputLead > TickRate {
			return 0, ErrProtocol
		}
		if m.ResumeGraceTicks > 5*60*TickRate || (m.ResumeGraceTicks == 0) != (m.ResumeToken == ([32]byte{})) {
			return 0, ErrProtocol
		}
		return KindWelcome, nil
	case InputBatch:
		if m.Epoch == 0 || len(m.Inputs) > MaxInputBatch {
			return 0, ErrProtocol
		}
		for _, input := range m.Inputs {
			if err := ValidateInputCommand(input.Command); err != nil {
				return 0, fmt.Errorf("%w: %w", ErrProtocol, err)
			}
		}
		return KindInput, nil
	case MapChange:
		if _, err := validateMessage(m.Welcome); err != nil || m.PreviousEpoch == 0 || m.Welcome.Epoch <= m.PreviousEpoch || !validString(m.Map, 32) || !validString(m.Compatibility, 256) {
			return 0, ErrProtocol
		}
		return KindMapChange, nil
	case Snapshot:
		if m.Epoch == 0 || m.ID == 0 || len(m.State) == 0 || len(m.State) > MaxSnapshotBytes ||
			(m.BaselineID != 0 && m.BaselineID >= m.ID) {
			return 0, ErrProtocol
		}
		switch m.Encoding {
		case SnapshotRaw:
			if m.BaselineID != 0 || m.DecodedSize != 0 || m.Digest != ([32]byte{}) {
				return 0, ErrProtocol
			}
		case SnapshotZstd, SnapshotDeltaZstd:
			if m.DecodedSize == 0 || m.DecodedSize > MaxSnapshotBytes || m.Digest == ([32]byte{}) {
				return 0, ErrProtocol
			}
			if (m.Encoding == SnapshotZstd && m.BaselineID != 0) || (m.Encoding == SnapshotDeltaZstd && m.BaselineID == 0) {
				return 0, ErrProtocol
			}
		default:
			return 0, ErrProtocol
		}
		if (m.Finalized.HasTick && m.Finalized.Tick > m.Tick) ||
			(m.Finalized.HasSequence && !m.Finalized.HasTick) ||
			(!m.Finalized.HasTick && m.Finalized.Tick != 0) ||
			(!m.Finalized.HasSequence && m.Finalized.Sequence != 0) {
			return 0, ErrProtocol
		}
		return KindSnapshot, nil
	case Disconnect:
		if !validString(m.Reason, 256) {
			return 0, ErrProtocol
		}
		return KindDisconnect, nil
	default:
		return 0, fmt.Errorf("%w: unsupported message type %T", ErrProtocol, message)
	}
}

func MarshalMessage(message any) ([]byte, error) {
	kind, err := validateMessage(message)
	if err != nil {
		return nil, err
	}
	switch m := message.(type) {
	case Snapshot:
		return marshalSnapshot(m), nil
	case InputBatch:
		return marshalInput(m), nil
	}
	var body bytes.Buffer
	put := func(v any) { _ = binary.Write(&body, binary.LittleEndian, v) }
	putString := func(s string) {
		put(uint16(len(s)))
		body.WriteString(s)
	}
	switch m := message.(type) {
	case FollowPlayer:
		put(m.PlayerID)
	case Roster:
		put(m.Revision)
		put(m.PlayerLimit)
		put(m.Count)
		for _, p := range m.Players[:m.Count] {
			put(p.ID)
			put(p.PlayerID)
			var flags byte
			if p.Connected {
				flags |= 1
			}
			if p.Spectator {
				flags |= 2
			}
			put(flags)
			put(p.PingMillis)
			putString(p.Name)
		}
	case ChatSay:
		putString(m.Text)
	case ChatEvent:
		put(m.ID)
		put(m.PlayerID)
		putString(m.Name)
		putString(m.Text)
	case Ping:
		put(m.Nonce)
	case Pong:
		put(m.Nonce)
	case Query, StatusQuery:
	case ServerInfo, ServerStatus:
		data, err := json.Marshal(m)
		if err != nil || len(data) > MaxServerInfoBytes {
			return nil, ErrProtocol
		}
		body.Write(data)
	case Hello:
		putString(m.Compatibility)
		putString(m.Name)
		put(m.ResumeToken)
		var spectator byte
		if m.Spectator {
			spectator = 1
		}
		put(spectator)
	case Welcome:
		put(m.Epoch)
		put(m.PlayerID)
		put(m.ServerTick)
		put(m.InputLead)
		put(m.ResumeToken)
		put(m.ResumeGraceTicks)
	case MapChange:
		put(m.PreviousEpoch)
		put(m.Welcome.Epoch)
		put(m.Welcome.PlayerID)
		put(m.Welcome.ServerTick)
		put(m.Welcome.InputLead)
		put(m.Welcome.ResumeToken)
		put(m.Welcome.ResumeGraceTicks)
		putString(m.Map)
		putString(m.Compatibility)
	case Disconnect:
		putString(m.Reason)
	}
	out := make([]byte, messageHeaderBytes+body.Len())
	copy(out, "GDMP")
	out[4], out[5] = ProtocolVersion, byte(kind)
	binary.LittleEndian.PutUint32(out[6:10], uint32(body.Len()))
	copy(out[10:], body.Bytes())
	return out, nil
}

// ReadMessage bounds allocation before reading a payload. A stream is closed
// after any decode error; callers must not attempt to recover framing in-place.
func ReadMessage(r io.Reader) (any, error) {
	return readMessage(r, false)
}

// ReadClientMessage rejects server-only frames from untrusted clients before
// allocating or reading their payloads. In particular, a client cannot make a
// server allocate a snapshot-sized buffer merely by sending its frame header.
// Session state still determines whether Hello/Input/Disconnect is legal now.
func ReadClientMessage(r io.Reader) (any, error) {
	return readMessage(r, true)
}

func readMessage(r io.Reader, clientOnly bool) (any, error) {
	var header [messageHeaderBytes]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	if string(header[:4]) != "GDMP" || header[4] != ProtocolVersion {
		return nil, ErrProtocol
	}
	n := binary.LittleEndian.Uint32(header[6:])
	kind := MessageKind(header[5])
	if clientOnly && kind != KindHello && kind != KindInput && kind != KindDisconnect && kind != KindQuery && kind != KindStatusQuery && kind != KindPing && kind != KindPong && kind != KindChatSay && kind != KindFollowPlayer {
		return nil, ErrProtocol
	}
	var minimum, maximum uint32
	switch kind {
	case KindRoster:
		minimum, maximum = 10, rosterBodyMaxBytes
	case KindFollowPlayer:
		minimum, maximum = 1, 1
	case KindChatSay:
		minimum, maximum = 3, 2+MaxChatBytes
	case KindChatEvent:
		minimum, maximum = 8+1+3+3, 8+1+2+64+2+MaxChatBytes
	case KindPing, KindPong:
		minimum, maximum = 8, 8
	case KindQuery, KindStatusQuery:
		minimum, maximum = 0, 0
	case KindServerInfo, KindServerStatus:
		minimum, maximum = 2, MaxServerInfoBytes
	case KindHello:
		minimum, maximum = 6+32+1, 2+256+2+64+32+1
	case KindWelcome:
		minimum, maximum = welcomeBodyBytes, welcomeBodyBytes
	case KindInput:
		minimum, maximum = inputBodyHeaderBytes, inputBodyHeaderBytes+MaxInputBatch*inputCommandBytes
	case KindSnapshot:
		minimum, maximum = snapshotBodyHeaderBytes+1, snapshotBodyHeaderBytes+MaxSnapshotBytes
	case KindDisconnect:
		minimum, maximum = 3, 2+256
	case KindMapChange:
		minimum, maximum = 8+welcomeBodyBytes+3+3, 8+welcomeBodyBytes+2+32+2+256
	default:
		return nil, ErrProtocol
	}
	if n < minimum || n > maximum {
		return nil, ErrProtocol
	}
	body := make([]byte, int(n))
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return decodeBody(kind, body)
}

// UnmarshalMessage requires exactly one frame, including on datagram transports.
func UnmarshalMessage(data []byte) (any, error) {
	r := bytes.NewReader(data)
	m, err := ReadMessage(r)
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, ErrProtocol
	}
	return m, nil
}

func decodeBody(kind MessageKind, body []byte) (any, error) {
	switch kind {
	case KindSnapshot:
		return decodeSnapshot(body)
	case KindInput:
		return decodeInput(body)
	}
	r := bytes.NewReader(body)
	var readErr error
	get := func(v any) {
		if readErr == nil {
			readErr = binary.Read(r, binary.LittleEndian, v)
		}
	}
	getString := func(max int) string {
		var n uint16
		get(&n)
		if readErr != nil {
			return ""
		}
		if n == 0 || int(n) > max || int(n) > r.Len() {
			readErr = ErrProtocol
			return ""
		}
		value := make([]byte, int(n))
		if _, err := io.ReadFull(r, value); err != nil {
			readErr = err
			return ""
		}
		if !utf8.Valid(value) {
			readErr = ErrProtocol
		}
		return string(value)
	}
	var result any
	switch kind {
	case KindRoster:
		var m Roster
		get(&m.Revision)
		get(&m.PlayerLimit)
		get(&m.Count)
		if int(m.Count) > MaxParticipants {
			return nil, ErrProtocol
		}
		for i := range int(m.Count) {
			p := &m.Players[i]
			get(&p.ID)
			get(&p.PlayerID)
			var flags byte
			get(&flags)
			if flags > 3 {
				return nil, ErrProtocol
			}
			p.Connected, p.Spectator = flags&1 != 0, flags&2 != 0
			get(&p.PingMillis)
			p.Name = getString(64)
		}
		result = m
	case KindFollowPlayer:
		var m FollowPlayer
		get(&m.PlayerID)
		result = m
	case KindChatSay:
		result = ChatSay{Text: getString(MaxChatBytes)}
	case KindChatEvent:
		var m ChatEvent
		get(&m.ID)
		get(&m.PlayerID)
		m.Name, m.Text = getString(64), getString(MaxChatBytes)
		result = m
	case KindPing:
		var m Ping
		get(&m.Nonce)
		result = m
	case KindPong:
		var m Pong
		get(&m.Nonce)
		result = m
	case KindQuery:
		result = Query{}
	case KindStatusQuery:
		result = StatusQuery{}
	case KindServerInfo, KindServerStatus:
		var info any = &ServerInfo{}
		if kind == KindServerStatus {
			info = &ServerStatus{}
		}
		decoder := json.NewDecoder(r)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(info); err != nil {
			return nil, ErrProtocol
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, ErrProtocol
		}
		switch info := info.(type) {
		case *ServerInfo:
			result = *info
		case *ServerStatus:
			result = *info
		}
	case KindHello:
		m := Hello{Compatibility: getString(256), Name: getString(64)}
		get(&m.ResumeToken)
		var spectator byte
		get(&spectator)
		if spectator > 1 {
			return nil, ErrProtocol
		}
		m.Spectator = spectator == 1
		result = m
	case KindWelcome:
		var m Welcome
		get(&m.Epoch)
		get(&m.PlayerID)
		get(&m.ServerTick)
		get(&m.InputLead)
		get(&m.ResumeToken)
		get(&m.ResumeGraceTicks)
		result = m
	case KindMapChange:
		var m MapChange
		get(&m.PreviousEpoch)
		get(&m.Welcome.Epoch)
		get(&m.Welcome.PlayerID)
		get(&m.Welcome.ServerTick)
		get(&m.Welcome.InputLead)
		get(&m.Welcome.ResumeToken)
		get(&m.Welcome.ResumeGraceTicks)
		m.Map, m.Compatibility = getString(32), getString(256)
		result = m
	case KindDisconnect:
		result = Disconnect{Reason: getString(256)}
	default:
		return nil, ErrProtocol
	}
	if readErr != nil || r.Len() != 0 {
		return nil, ErrProtocol
	}
	// Apply the same value validation on both sides of the wire.
	if _, err := validateMessage(result); err != nil {
		return nil, err
	}
	return result, nil
}
