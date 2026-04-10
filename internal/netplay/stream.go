package netplay

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/voicecodec"

	"github.com/klauspost/compress/zstd"
)

const (
	protocolVersion byte = 2
	protocolMagic        = "GDSF"

	helloRoleBroadcaster      byte = 1
	helloRoleViewer           byte = 2
	helloRoleServer           byte = 3
	helloRoleAudioBroadcaster byte = 4
	helloRoleAudioViewer      byte = 5
	helloRolePlayerPeer       byte = 6

	frameTypeKeyframe            byte = 1
	frameTypeTicBatch            byte = 4
	frameTypeIntermissionAdvance byte = 8
	frameTypeAudioConfig         byte = 16
	frameTypeAudioChunk          byte = 17
	frameTypeChat                byte = 32
	frameTypePeerTicBatch        byte = 33
	frameTypeRosterUpdate        byte = 34
	frameTypeCheckpoint          byte = 35
	frameTypeDesyncRequest       byte = 36
)

const (
	helloFlagGameplayCompactV1 uint16 = 1 << 14
	helloFlagAudioCompactV1    uint16 = 1 << 15
)

const (
	sessionFlagShowNoSkillItems uint16 = 1 << iota
	sessionFlagShowAllItems
	sessionFlagFastMonsters
	sessionFlagRespawnMonsters
	sessionFlagNoMonsters
	sessionFlagAutoWeaponSwitch
	sessionFlagInvulnerable
	sessionFlagSourcePortMode
)

const (
	frameHeaderSize      = 10
	ticBatchOverhead     = 2
	ticBatchSize         = 4
	peerTicBatchOverhead = 3 // player_id[1] + count[2]
)

const (
	keyframeFlagMandatoryApply byte = 1 << iota
	keyframeFlagZstdCompressed
)

const (
	audioChunkFlagSilence byte = 1 << iota
)

const (
	audioCodecPCM16Mono   byte = 2
	audioCodecG72632      byte = 3
	audioCodecSilkV3      byte = 4
	audioViewerChunkQueue      = 24
)

const (
	audioRecordTypeFormat byte = 0x80
	audioFormatPayloadLen      = 17
	audioChunkHeaderSize       = 1
	audioChunkVarLenSize       = 2
)

const (
	gameplayRecordTicBatchBase byte = 0x40
	gameplayRecordTicBatchMax       = 63
)

var (
	keyframeZstdEncOnce sync.Once
	keyframeZstdEnc     *zstd.Encoder
	keyframeZstdEncErr  error

	keyframeZstdDecOnce sync.Once
	keyframeZstdDec     *zstd.Decoder
	keyframeZstdDecErr  error
)

type SessionConfig struct {
	WADHash          string
	MapName          string
	MaxPlayers       int
	PlayerSlot       int
	SkillLevel       int
	GameMode         string
	ShowNoSkillItems bool
	ShowAllItems     bool
	FastMonsters     bool
	RespawnMonsters  bool
	NoMonsters       bool
	AutoWeaponSwitch bool
	CheatLevel       int
	Invulnerable     bool
	SourcePortMode   bool
}

type frameHeader struct {
	Type   byte
	Flags  byte
	Length uint32
	Tic    uint32
}

type Keyframe struct {
	Tic            uint32
	Blob           []byte
	MandatoryApply bool
}

type AudioFormat struct {
	Codec                byte
	BitsPerSample        byte
	SampleRateChoice     byte
	SampleRate           int
	Channels             int
	PacketDurationMillis int
	PacketSamples        int
	Bitrate              int
}

type AudioChunk struct {
	GameTic     uint32
	StartSample uint64
	Silence     bool
	Payload     []byte
}

type ChatMessage struct {
	Name string
	Text string
}

const (
	chatMaxNameLen  = 128
	chatMaxTextLen  = 512
	maxPeerPlayerID = 4
)

// PeerTic is a tic command tagged with the originating player's slot.
type PeerTic struct {
	PlayerID byte
	Tic      demo.Tic
}

// RosterUpdate describes a change to the active peer roster.
type RosterUpdate struct {
	// PlayerIDs is the full set of active player slot IDs after this update.
	PlayerIDs []byte
}

// Checkpoint is a periodic hash broadcast by the server so peers can detect
// simulation divergence. The hash covers key sim state at the given tic.
type Checkpoint struct {
	Tic  uint32
	Hash uint32
}

// DesyncRequest is sent by a peer to the server when its local hash does not
// match a received checkpoint. The server responds by pushing a mandatory
// keyframe from a canonical peer.
type DesyncRequest struct {
	Tic       uint32
	LocalHash uint32
}

const checkpointPayloadSize = 8  // tic[4] + hash[4]
const desyncRequestPayloadSize = 8 // tic[4] + hash[4]

type RelayBroadcaster struct {
	conn      net.Conn
	sessionID uint64
	meter     *bandwidthMeter
	chats     chan ChatMessage

	mu           sync.Mutex
	closed       bool
	err          error
	pendingTic   []demo.Tic
	ticBatchSize int
	wg           sync.WaitGroup
}

type AudioBroadcaster struct {
	conn      net.Conn
	sessionID uint64
	meter     *bandwidthMeter

	mu          sync.Mutex
	closed      bool
	audioFormat AudioFormat
}

// PlayerPeer is a bidirectional co-op gameplay connection. It sends this
// player's tics upstream and receives tagged tics from all other peers.
type PlayerPeer struct {
	conn      net.Conn
	sessionID uint64
	session   SessionConfig
	playerID  byte
	meter     *bandwidthMeter

	// inbound channels
	peerTics    chan PeerTic
	rosters     chan RosterUpdate
	keyframes   chan Keyframe
	chats       chan ChatMessage
	checkpoints chan Checkpoint

	mu           sync.Mutex
	closed       bool
	err          error
	pendingTic   []demo.Tic
	ticBatchSize int
	wg           sync.WaitGroup
}

func DialRelayBroadcaster(addr string, sessionID uint64, session SessionConfig) (*RelayBroadcaster, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("broadcast relay address is required")
	}
	rawConn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial relay %s: %w", addr, err)
	}
	if tcp, ok := rawConn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	meter := newBandwidthMeter()
	conn := &countingConn{Conn: rawConn, meter: meter}
	if err := writeHello(conn, helloRoleBroadcaster, helloFlagGameplayCompactV1, sessionID, session); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write relay hello: %w", err)
	}
	role, flags, assignedID, _, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read relay hello ack: %w", err)
	}
	if role != helloRoleServer {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected relay hello ack role %d", role)
	}
	if flags&helloFlagGameplayCompactV1 == 0 {
		_ = rawConn.Close()
		return nil, fmt.Errorf("relay hello ack missing compact gameplay flag")
	}
	b := &RelayBroadcaster{
		conn:         conn,
		sessionID:    assignedID,
		meter:        meter,
		chats:        make(chan ChatMessage, 32),
		pendingTic:   make([]demo.Tic, 0, ticBatchSize),
		ticBatchSize: ticBatchSize,
	}
	b.wg.Add(1)
	go b.readLoop()
	return b, nil
}

func (b *RelayBroadcaster) SessionID() uint64 {
	if b == nil {
		return 0
	}
	return b.sessionID
}

func (b *AudioBroadcaster) SessionID() uint64 {
	if b == nil {
		return 0
	}
	return b.sessionID
}

func (b *RelayBroadcaster) BandwidthStats() (float64, float64) {
	if b == nil || b.meter == nil {
		return 0, 0
	}
	return b.meter.stats()
}

func (b *AudioBroadcaster) BandwidthStats() (float64, float64) {
	if b == nil || b.meter == nil {
		return 0, 0
	}
	return b.meter.stats()
}

func (b *RelayBroadcaster) SetLowLatency(enabled bool) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if enabled {
		b.ticBatchSize = 1
	} else {
		b.ticBatchSize = ticBatchSize
	}
	if len(b.pendingTic) >= b.ticBatchSize {
		_ = b.flushPendingTicsLocked()
	}
}

func DialRelayAudioBroadcaster(addr string, sessionID uint64) (*AudioBroadcaster, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("audio relay address is required")
	}
	if sessionID == 0 {
		return nil, fmt.Errorf("audio session id is required")
	}
	rawConn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial audio relay %s: %w", addr, err)
	}
	if tcp, ok := rawConn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	meter := newBandwidthMeter()
	conn := &countingConn{Conn: rawConn, meter: meter}
	if err := writeHello(conn, helloRoleAudioBroadcaster, helloFlagAudioCompactV1, sessionID, SessionConfig{}); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write audio relay hello: %w", err)
	}
	role, flags, assignedID, _, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read audio relay hello ack: %w", err)
	}
	if role != helloRoleServer {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected audio relay hello ack role %d", role)
	}
	if flags&helloFlagAudioCompactV1 == 0 {
		_ = rawConn.Close()
		return nil, fmt.Errorf("audio relay hello ack missing compact transport flag")
	}
	return &AudioBroadcaster{
		conn:      conn,
		sessionID: assignedID,
		meter:     meter,
	}, nil
}

func DialPlayerPeer(addr string, sessionID uint64, session SessionConfig) (*PlayerPeer, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("peer relay address is required")
	}
	rawConn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial peer relay %s: %w", addr, err)
	}
	if tcp, ok := rawConn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	meter := newBandwidthMeter()
	conn := &countingConn{Conn: rawConn, meter: meter}
	if err := writeHello(conn, helloRolePlayerPeer, helloFlagGameplayCompactV1, sessionID, session); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write peer hello: %w", err)
	}
	role, flags, assignedID, serverSession, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read peer hello ack: %w", err)
	}
	if role != helloRoleServer {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected peer hello ack role %d", role)
	}
	if flags&helloFlagGameplayCompactV1 == 0 {
		_ = rawConn.Close()
		return nil, fmt.Errorf("peer hello ack missing compact gameplay flag")
	}
	if sessionID != 0 && assignedID != sessionID {
		_ = rawConn.Close()
		return nil, fmt.Errorf("peer session mismatch: requested=%d got=%d", sessionID, assignedID)
	}
	if session.WADHash != "" && serverSession.WADHash != "" && serverSession.WADHash != session.WADHash {
		_ = rawConn.Close()
		return nil, fmt.Errorf("peer WAD hash mismatch: local=%s server=%s", session.WADHash, serverSession.WADHash)
	}
	p := &PlayerPeer{
		conn:         conn,
		sessionID:    assignedID,
		session:      serverSession,
		playerID:     byte(serverSession.PlayerSlot),
		meter:        meter,
		peerTics:     make(chan PeerTic, 512),
		rosters:      make(chan RosterUpdate, 8),
		keyframes:    make(chan Keyframe, 4),
		checkpoints:  make(chan Checkpoint, 16),
		chats:        make(chan ChatMessage, 32),
		pendingTic:   make([]demo.Tic, 0, ticBatchSize),
		ticBatchSize: ticBatchSize,
	}
	p.wg.Add(1)
	go p.readLoop()
	return p, nil
}

func (p *PlayerPeer) SessionID() uint64 {
	if p == nil {
		return 0
	}
	return p.sessionID
}

func (p *PlayerPeer) PlayerID() byte {
	if p == nil {
		return 0
	}
	return p.playerID
}

func (p *PlayerPeer) Session() SessionConfig {
	if p == nil {
		return SessionConfig{}
	}
	return p.session
}

func (p *PlayerPeer) BandwidthStats() (float64, float64) {
	if p == nil || p.meter == nil {
		return 0, 0
	}
	return p.meter.stats()
}

func (p *PlayerPeer) SendTic(tc demo.Tic) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return net.ErrClosed
	}
	p.pendingTic = append(p.pendingTic, tc)
	if len(p.pendingTic) < p.ticBatchSize {
		return nil
	}
	return p.flushPendingTicsLocked()
}

// Flush immediately sends any buffered tics upstream. The game loop should
// call this once per tic after SendTic so peers receive inputs without delay.
func (p *PlayerPeer) Flush() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	return p.flushPendingTicsLocked()
}

func (p *PlayerPeer) flushPendingTicsLocked() error {
	if p == nil || len(p.pendingTic) == 0 {
		return nil
	}
	payload := marshalPeerTicBatchPayload(p.playerID, p.pendingTic)
	err := writeFrame(p.conn, frameHeader{
		Type:   frameTypePeerTicBatch,
		Length: uint32(len(payload)),
	}, payload)
	if err != nil {
		return err
	}
	p.pendingTic = p.pendingTic[:0]
	return nil
}

func (p *PlayerPeer) PollPeerTic() (PeerTic, bool, error) {
	if p == nil {
		return PeerTic{}, false, nil
	}
	select {
	case pt, ok := <-p.peerTics:
		if ok {
			return pt, true, nil
		}
		return PeerTic{}, false, p.readErr()
	default:
		return PeerTic{}, false, p.readErr()
	}
}

func (p *PlayerPeer) PollRoster() (RosterUpdate, bool, error) {
	if p == nil {
		return RosterUpdate{}, false, nil
	}
	select {
	case r, ok := <-p.rosters:
		if ok {
			return r, true, nil
		}
		return RosterUpdate{}, false, p.readErr()
	default:
		return RosterUpdate{}, false, p.readErr()
	}
}

func (p *PlayerPeer) PollKeyframe() (Keyframe, bool, error) {
	if p == nil {
		return Keyframe{}, false, nil
	}
	select {
	case kf, ok := <-p.keyframes:
		if ok {
			return kf, true, nil
		}
		return Keyframe{}, false, p.readErr()
	default:
		return Keyframe{}, false, p.readErr()
	}
}

func (p *PlayerPeer) SendChat(msg ChatMessage) error {
	if p == nil {
		return nil
	}
	payload := marshalChatPayload(msg)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return net.ErrClosed
	}
	if err := p.flushPendingTicsLocked(); err != nil {
		return err
	}
	return writeFrame(p.conn, frameHeader{Type: frameTypeChat}, payload)
}

func (p *PlayerPeer) PollChat() (ChatMessage, bool, error) {
	if p == nil {
		return ChatMessage{}, false, nil
	}
	select {
	case msg, ok := <-p.chats:
		if ok {
			return msg, true, nil
		}
		return ChatMessage{}, false, p.readErr()
	default:
		return ChatMessage{}, false, p.readErr()
	}
}

func (p *PlayerPeer) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	_ = p.flushPendingTicsLocked()
	p.mu.Unlock()
	err := p.conn.Close()
	p.wg.Wait()
	return err
}

func (p *PlayerPeer) readLoop() {
	defer p.wg.Done()
	defer close(p.peerTics)
	defer close(p.rosters)
	defer close(p.keyframes)
	defer close(p.chats)
	defer close(p.checkpoints)
	for {
		header, payload, err := readFrame(p.conn)
		if err != nil {
			p.setErr(err)
			return
		}
		switch header.Type {
		case frameTypePeerTicBatch:
			playerID, tics, err := unmarshalPeerTicBatchPayload(payload)
			if err != nil {
				p.setErr(err)
				return
			}
			for _, tc := range tics {
				select {
				case p.peerTics <- PeerTic{PlayerID: playerID, Tic: tc}:
				default:
				}
			}
		case frameTypeRosterUpdate:
			roster, err := unmarshalRosterUpdatePayload(payload)
			if err != nil {
				p.setErr(err)
				return
			}
			select {
			case p.rosters <- roster:
			default:
			}
		case frameTypeKeyframe:
			mandatory := header.Flags&keyframeFlagMandatoryApply != 0
			if header.Flags&keyframeFlagZstdCompressed != 0 {
				payload, err = decompressKeyframe(payload)
				if err != nil {
					p.setErr(fmt.Errorf("decompress keyframe: %w", err))
					return
				}
			}
			select {
			case p.keyframes <- Keyframe{
				Tic:            header.Tic,
				Blob:           append([]byte(nil), payload...),
				MandatoryApply: mandatory,
			}:
			default:
			}
		case frameTypeChat:
			msg, err := readChatFrame(bytes.NewReader(payload))
			if err != nil {
				p.setErr(fmt.Errorf("read chat frame: %w", err))
				return
			}
			select {
			case p.chats <- msg:
			default:
			}
		case frameTypeCheckpoint:
			if len(payload) < checkpointPayloadSize {
				p.setErr(fmt.Errorf("short checkpoint frame: %d bytes", len(payload)))
				return
			}
			cp := Checkpoint{
				Tic:  binary.LittleEndian.Uint32(payload[0:4]),
				Hash: binary.LittleEndian.Uint32(payload[4:8]),
			}
			select {
			case p.checkpoints <- cp:
			default:
			}
		default:
			p.setErr(fmt.Errorf("unexpected peer frame type %d", header.Type))
			return
		}
	}
}

// PollCheckpoint returns the next checkpoint received from the server, if any.
func (p *PlayerPeer) PollCheckpoint() (Checkpoint, bool, error) {
	if p == nil {
		return Checkpoint{}, false, nil
	}
	select {
	case cp, ok := <-p.checkpoints:
		if ok {
			return cp, true, nil
		}
		return Checkpoint{}, false, p.readErr()
	default:
		return Checkpoint{}, false, p.readErr()
	}
}

// SendCheckpoint sends this peer's simulation hash to the server for relay.
// Only the canonical peer (slot 1) should call this.
func (p *PlayerPeer) SendCheckpoint(tic uint32, hash uint32) error {
	if p == nil {
		return nil
	}
	payload := make([]byte, checkpointPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], tic)
	binary.LittleEndian.PutUint32(payload[4:8], hash)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return net.ErrClosed
	}
	if err := p.flushPendingTicsLocked(); err != nil {
		return err
	}
	return writeFrame(p.conn, frameHeader{Type: frameTypeCheckpoint, Tic: tic, Length: uint32(checkpointPayloadSize)}, payload)
}

// SendDesyncRequest informs the server that our local hash at the given tic
// does not match the checkpoint. The server will push a mandatory keyframe.
func (p *PlayerPeer) SendDesyncRequest(req DesyncRequest) error {
	if p == nil {
		return nil
	}
	payload := make([]byte, desyncRequestPayloadSize)
	binary.LittleEndian.PutUint32(payload[0:4], req.Tic)
	binary.LittleEndian.PutUint32(payload[4:8], req.LocalHash)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return net.ErrClosed
	}
	if err := p.flushPendingTicsLocked(); err != nil {
		return err
	}
	return writeFrame(p.conn, frameHeader{Type: frameTypeDesyncRequest, Tic: req.Tic}, payload)
}

func (p *PlayerPeer) readErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *PlayerPeer) setErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err == nil {
		p.err = err
	}
}

func (b *RelayBroadcaster) BroadcastTic(tc demo.Tic) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	b.pendingTic = append(b.pendingTic, tc)
	if len(b.pendingTic) < b.ticBatchSize {
		return nil
	}
	return b.flushPendingTicsLocked()
}

func (b *RelayBroadcaster) BroadcastKeyframe(tic uint32, blob []byte) error {
	return b.BroadcastKeyframeWithFlags(tic, blob, 0)
}

func (b *RelayBroadcaster) BroadcastKeyframeWithFlags(tic uint32, blob []byte, flags byte) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	if err := b.flushPendingTicsLocked(); err != nil {
		return err
	}
	payload, err := compressKeyframe(blob)
	if err != nil {
		return fmt.Errorf("compress keyframe: %w", err)
	}
	return writeFrame(b.conn, frameHeader{
		Type:   frameTypeKeyframe,
		Flags:  flags | keyframeFlagZstdCompressed,
		Length: uint32(len(payload)),
		Tic:    tic,
	}, payload)
}

func (b *RelayBroadcaster) BroadcastIntermissionAdvance() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	if err := b.flushPendingTicsLocked(); err != nil {
		return err
	}
	return writeFrame(b.conn, frameHeader{Type: frameTypeIntermissionAdvance}, nil)
}

func (b *AudioBroadcaster) BroadcastAudioFormat(format AudioFormat) error {
	if b == nil {
		return nil
	}
	format, err := normalizeAudioFormat(format)
	if err != nil {
		return err
	}
	payload, err := marshalAudioFormat(format)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	b.audioFormat = format
	return writeAudioFormatRecord(b.conn, payload)
}

func (b *AudioBroadcaster) BroadcastAudioConfig(cfg AudioFormat) error {
	return b.BroadcastAudioFormat(cfg)
}

func (b *AudioBroadcaster) BroadcastAudioChunk(chunk AudioChunk) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	if b.audioFormat.PacketSamples <= 0 {
		return fmt.Errorf("audio packet samples are not configured")
	}
	return writeAudioChunkRecord(b.conn, b.audioFormat, chunk)
}

func (b *RelayBroadcaster) readLoop() {
	defer b.wg.Done()
	defer close(b.chats)
	for {
		header, payload, err := readFrame(b.conn)
		if err != nil {
			b.mu.Lock()
			if b.err == nil {
				b.err = err
			}
			b.mu.Unlock()
			return
		}
		if header.Type != frameTypeChat {
			continue
		}
		msg, err := readChatFrame(bytes.NewReader(payload))
		if err != nil {
			continue
		}
		select {
		case b.chats <- msg:
		default:
		}
	}
}

func (b *RelayBroadcaster) SendChat(msg ChatMessage) error {
	if b == nil {
		return nil
	}
	payload := marshalChatPayload(msg)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	return writeFrame(b.conn, frameHeader{Type: frameTypeChat}, payload)
}

func (b *RelayBroadcaster) PollChat() (ChatMessage, bool, error) {
	if b == nil {
		return ChatMessage{}, false, nil
	}
	select {
	case msg, ok := <-b.chats:
		if ok {
			return msg, true, nil
		}
		b.mu.Lock()
		err := b.err
		b.mu.Unlock()
		return ChatMessage{}, false, err
	default:
		b.mu.Lock()
		err := b.err
		b.mu.Unlock()
		return ChatMessage{}, false, err
	}
}

func (b *RelayBroadcaster) SendRuntimeChat(msg runtimecfg.ChatMessage) error {
	if b == nil {
		return nil
	}
	return b.SendChat(ChatMessage{Name: msg.Name, Text: msg.Text})
}

func (b *RelayBroadcaster) PollRuntimeChat() (runtimecfg.ChatMessage, bool, error) {
	if b == nil {
		return runtimecfg.ChatMessage{}, false, nil
	}
	msg, ok, err := b.PollChat()
	return runtimecfg.ChatMessage{Name: msg.Name, Text: msg.Text}, ok, err
}

func (b *RelayBroadcaster) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	err := b.flushPendingTicsLocked()
	b.mu.Unlock()
	closeErr := b.conn.Close()
	b.wg.Wait()
	if err != nil {
		return err
	}
	return closeErr
}

func (b *AudioBroadcaster) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	return b.conn.Close()
}

func (b *RelayBroadcaster) flushPendingTicsLocked() error {
	if b == nil || len(b.pendingTic) == 0 {
		return nil
	}
	payload := make([]byte, ticBatchOverhead+len(b.pendingTic)*4)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(len(b.pendingTic)))
	for i, tc := range b.pendingTic {
		copy(payload[ticBatchOverhead+i*4:], packDemoTic(tc))
	}
	err := writeFrame(b.conn, frameHeader{
		Type:   frameTypeTicBatch,
		Length: uint32(len(payload)),
	}, payload)
	if err != nil {
		return err
	}
	b.pendingTic = b.pendingTic[:0]
	return nil
}

type Viewer struct {
	conn      net.Conn
	session   SessionConfig
	tics      chan demo.Tic
	keyframes chan Keyframe
	advance   chan struct{}
	chats     chan ChatMessage
	meter     *bandwidthMeter

	mu     sync.Mutex
	err    error
	closed bool
	wg     sync.WaitGroup
}

type AudioViewer struct {
	conn    net.Conn
	session SessionConfig
	formats chan AudioFormat
	chunks  chan AudioChunk
	meter   *bandwidthMeter

	mu     sync.Mutex
	err    error
	closed bool
	wg     sync.WaitGroup
}

type countingConn struct {
	net.Conn
	meter *bandwidthMeter
}

func (c *countingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if c.meter != nil && n > 0 {
		c.meter.downloadBytes.Add(uint64(n))
	}
	return n, err
}

func (c *countingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if c.meter != nil && n > 0 {
		c.meter.uploadBytes.Add(uint64(n))
	}
	return n, err
}

type bandwidthMeter struct {
	uploadBytes   atomic.Uint64
	downloadBytes atomic.Uint64

	mu           sync.Mutex
	lastSampleAt time.Time
	lastUpload   uint64
	lastDownload uint64
	uploadPerSec float64
	downPerSec   float64
}

func newBandwidthMeter() *bandwidthMeter {
	return &bandwidthMeter{lastSampleAt: time.Now()}
}

func (m *bandwidthMeter) stats() (float64, float64) {
	if m == nil {
		return 0, 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if m.lastSampleAt.IsZero() {
		m.lastSampleAt = now
		m.lastUpload = m.uploadBytes.Load()
		m.lastDownload = m.downloadBytes.Load()
		return m.uploadPerSec, m.downPerSec
	}
	if elapsed := now.Sub(m.lastSampleAt); elapsed >= time.Second {
		up := m.uploadBytes.Load()
		down := m.downloadBytes.Load()
		secs := elapsed.Seconds()
		m.uploadPerSec = float64(up-m.lastUpload) / secs
		m.downPerSec = float64(down-m.lastDownload) / secs
		m.lastSampleAt = now
		m.lastUpload = up
		m.lastDownload = down
	}
	return m.uploadPerSec, m.downPerSec
}

func DialRelayViewer(addr string, sessionID uint64, localWADHash string) (*Viewer, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("watch relay address is required")
	}
	if sessionID == 0 {
		return nil, fmt.Errorf("watch session id is required")
	}
	rawConn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial relay %s: %w", addr, err)
	}
	if tcp, ok := rawConn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	meter := newBandwidthMeter()
	conn := &countingConn{Conn: rawConn, meter: meter}
	if err := writeHello(conn, helloRoleViewer, helloFlagGameplayCompactV1, sessionID, SessionConfig{}); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write relay hello: %w", err)
	}
	role, flags, resolvedID, session, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read relay session hello: %w", err)
	}
	if role != helloRoleBroadcaster {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected relay session role %d", role)
	}
	if flags&helloFlagGameplayCompactV1 == 0 {
		_ = rawConn.Close()
		return nil, fmt.Errorf("relay session hello missing compact gameplay flag")
	}
	if resolvedID != sessionID {
		_ = rawConn.Close()
		return nil, fmt.Errorf("relay session mismatch: requested=%d got=%d", sessionID, resolvedID)
	}
	if localWADHash != "" && session.WADHash != "" && session.WADHash != localWADHash {
		_ = rawConn.Close()
		return nil, fmt.Errorf("broadcast WAD hash mismatch: local=%s host=%s", localWADHash, session.WADHash)
	}
	v := &Viewer{
		conn:      conn,
		session:   session,
		tics:      make(chan demo.Tic, 256),
		keyframes: make(chan Keyframe, 4),
		advance:   make(chan struct{}, 8),
		chats:     make(chan ChatMessage, 32),
		meter:     meter,
	}
	v.wg.Add(1)
	go v.readLoop()
	return v, nil
}

func DialRelayAudioViewer(addr string, sessionID uint64, localWADHash string) (*AudioViewer, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("audio watch relay address is required")
	}
	if sessionID == 0 {
		return nil, fmt.Errorf("audio watch session id is required")
	}
	rawConn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial audio relay %s: %w", addr, err)
	}
	if tcp, ok := rawConn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	meter := newBandwidthMeter()
	conn := &countingConn{Conn: rawConn, meter: meter}
	if err := writeHello(conn, helloRoleAudioViewer, helloFlagAudioCompactV1, sessionID, SessionConfig{}); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write audio relay hello: %w", err)
	}
	role, flags, resolvedID, session, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read audio relay session hello: %w", err)
	}
	if role != helloRoleAudioBroadcaster {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected audio relay session role %d", role)
	}
	if resolvedID != sessionID {
		_ = rawConn.Close()
		return nil, fmt.Errorf("audio relay session mismatch: requested=%d got=%d", sessionID, resolvedID)
	}
	if flags&helloFlagAudioCompactV1 == 0 {
		_ = rawConn.Close()
		return nil, fmt.Errorf("audio relay session hello missing compact transport flag")
	}
	if localWADHash != "" && session.WADHash != "" && session.WADHash != localWADHash {
		_ = rawConn.Close()
		return nil, fmt.Errorf("broadcast WAD hash mismatch: local=%s host=%s", localWADHash, session.WADHash)
	}
	v := &AudioViewer{
		conn:    conn,
		session: session,
		formats: make(chan AudioFormat, 4),
		chunks:  make(chan AudioChunk, audioViewerChunkQueue),
		meter:   meter,
	}
	v.wg.Add(1)
	go v.readLoop()
	return v, nil
}

func (v *Viewer) BandwidthStats() (float64, float64) {
	if v == nil || v.meter == nil {
		return 0, 0
	}
	return v.meter.stats()
}

func (v *Viewer) Session() SessionConfig {
	if v == nil {
		return SessionConfig{}
	}
	return v.session
}

func (v *AudioViewer) BandwidthStats() (float64, float64) {
	if v == nil || v.meter == nil {
		return 0, 0
	}
	return v.meter.stats()
}

func (v *AudioViewer) Session() SessionConfig {
	if v == nil {
		return SessionConfig{}
	}
	return v.session
}

func (v *Viewer) PollTic() (demo.Tic, bool, error) {
	if v == nil {
		return demo.Tic{}, false, nil
	}
	select {
	case tc, ok := <-v.tics:
		if ok {
			return tc, true, nil
		}
		return demo.Tic{}, false, v.readErr()
	default:
		return demo.Tic{}, false, v.readErr()
	}
}

func (v *Viewer) PendingTics() int {
	if v == nil {
		return 0
	}
	return len(v.tics)
}

func (v *Viewer) PollKeyframe() (Keyframe, bool, error) {
	if v == nil {
		return Keyframe{}, false, nil
	}
	select {
	case kf, ok := <-v.keyframes:
		if ok {
			return kf, true, nil
		}
		return Keyframe{}, false, v.readErr()
	default:
		return Keyframe{}, false, v.readErr()
	}
}

func (v *Viewer) PollRuntimeKeyframe() (runtimecfg.RuntimeKeyframe, bool, error) {
	kf, ok, err := v.PollKeyframe()
	if !ok || err != nil {
		return runtimecfg.RuntimeKeyframe{}, ok, err
	}
	return runtimecfg.RuntimeKeyframe{
		Tic:            kf.Tic,
		Blob:           kf.Blob,
		MandatoryApply: kf.MandatoryApply,
	}, true, nil
}

func (v *Viewer) PollIntermissionAdvance() (bool, error) {
	if v == nil {
		return false, nil
	}
	select {
	case _, ok := <-v.advance:
		if ok {
			return true, nil
		}
		return false, v.readErr()
	default:
		return false, v.readErr()
	}
}

func (v *AudioViewer) PollAudioFormat() (AudioFormat, bool, error) {
	if v == nil {
		return AudioFormat{}, false, nil
	}
	select {
	case format, ok := <-v.formats:
		if ok {
			return format, true, nil
		}
		return AudioFormat{}, false, v.readErr()
	default:
		return AudioFormat{}, false, v.readErr()
	}
}

func (v *AudioViewer) PollAudioConfig() (AudioFormat, bool, error) {
	return v.PollAudioFormat()
}

func (v *AudioViewer) PollAudioChunk() (AudioChunk, bool, error) {
	if v == nil {
		return AudioChunk{}, false, nil
	}
	select {
	case chunk, ok := <-v.chunks:
		if ok {
			return chunk, true, nil
		}
		return AudioChunk{}, false, v.readErr()
	default:
		return AudioChunk{}, false, v.readErr()
	}
}

func (v *Viewer) SendChat(msg ChatMessage) error {
	if v == nil {
		return nil
	}
	payload := marshalChatPayload(msg)
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return fmt.Errorf("viewer closed")
	}
	return writeFrame(v.conn, frameHeader{Type: frameTypeChat}, payload)
}

func (v *Viewer) PollChat() (ChatMessage, bool, error) {
	if v == nil {
		return ChatMessage{}, false, nil
	}
	select {
	case msg, ok := <-v.chats:
		if ok {
			return msg, true, nil
		}
		return ChatMessage{}, false, v.readErr()
	default:
		return ChatMessage{}, false, v.readErr()
	}
}

func (v *Viewer) SendRuntimeChat(msg runtimecfg.ChatMessage) error {
	if v == nil {
		return nil
	}
	return v.SendChat(ChatMessage{Name: msg.Name, Text: msg.Text})
}

func (v *Viewer) PollRuntimeChat() (runtimecfg.ChatMessage, bool, error) {
	if v == nil {
		return runtimecfg.ChatMessage{}, false, nil
	}
	msg, ok, err := v.PollChat()
	return runtimecfg.ChatMessage{Name: msg.Name, Text: msg.Text}, ok, err
}

func (v *Viewer) Close() error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	v.closed = true
	v.mu.Unlock()
	err := v.conn.Close()
	v.wg.Wait()
	return err
}

func (v *AudioViewer) Close() error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	v.closed = true
	v.mu.Unlock()
	err := v.conn.Close()
	v.wg.Wait()
	return err
}

func (v *Viewer) readLoop() {
	defer v.wg.Done()
	defer close(v.tics)
	defer close(v.keyframes)
	defer close(v.advance)
	defer close(v.chats)
	for {
		header, payload, err := readFrame(v.conn)
		if err != nil {
			v.setErr(err)
			return
		}
		switch header.Type {
		case frameTypeKeyframe:
			mandatory := header.Flags&keyframeFlagMandatoryApply != 0
			if header.Flags&keyframeFlagZstdCompressed != 0 {
				payload, err = decompressKeyframe(payload)
				if err != nil {
					v.setErr(fmt.Errorf("decompress keyframe: %w", err))
					return
				}
			}
			if mandatory {
				v.drainPendingTics()
			}
			v.keyframes <- Keyframe{
				Tic:            header.Tic,
				Blob:           append([]byte(nil), payload...),
				MandatoryApply: mandatory,
			}
		case frameTypeTicBatch:
			if err := v.consumeTicBatch(payload); err != nil {
				v.setErr(err)
				return
			}
		case frameTypeIntermissionAdvance:
			if len(payload) != 0 {
				v.setErr(fmt.Errorf("intermission advance payload len=%d want=0", len(payload)))
				return
			}
			v.advance <- struct{}{}
		case frameTypeChat:
			msg, err := readChatFrame(bytes.NewReader(payload))
			if err != nil {
				v.setErr(fmt.Errorf("read chat frame: %w", err))
				return
			}
			select {
			case v.chats <- msg:
			default:
			}
		default:
			v.setErr(fmt.Errorf("unexpected broadcast frame type %d", header.Type))
			return
		}
	}
}

func (v *AudioViewer) readLoop() {
	defer v.wg.Done()
	defer close(v.formats)
	defer close(v.chunks)
	packetSamples := 0
	currentFormat := AudioFormat{}
	var nextStartSample uint64
	for {
		kind, format, chunk, err := readAudioRecord(v.conn, currentFormat)
		if err != nil {
			v.setErr(err)
			return
		}
		switch kind {
		case audioRecordTypeFormat:
			for {
				select {
				case <-v.chunks:
				default:
					goto drainedChunks
				}
			}
		drainedChunks:
			packetSamples = format.PacketSamples
			currentFormat = format
			nextStartSample = 0
			v.formats <- format
		default:
			if packetSamples <= 0 {
				v.setErr(fmt.Errorf("audio chunk received before format"))
				return
			}
			chunk.StartSample = nextStartSample
			v.chunks <- chunk
			nextStartSample += uint64(packetSamples)
		}
	}
}

func (v *Viewer) drainPendingTics() {
	if v == nil {
		return
	}
	for {
		select {
		case <-v.tics:
		default:
			return
		}
	}
}

func (v *Viewer) consumeTicBatch(payload []byte) error {
	if len(payload) < ticBatchOverhead {
		return fmt.Errorf("tic batch payload too short")
	}
	count := int(binary.LittleEndian.Uint16(payload[0:2]))
	want := ticBatchOverhead + count*4
	if len(payload) != want {
		return fmt.Errorf("tic batch payload len=%d want=%d", len(payload), want)
	}
	for i := 0; i < count; i++ {
		offset := ticBatchOverhead + i*4
		v.tics <- unpackDemoTic(payload[offset : offset+4])
	}
	return nil
}

func compressKeyframe(src []byte) ([]byte, error) {
	enc, err := keyframeEncoder()
	if err != nil {
		return nil, err
	}
	return enc.EncodeAll(src, nil), nil
}

func decompressKeyframe(src []byte) ([]byte, error) {
	dec, err := keyframeDecoder()
	if err != nil {
		return nil, err
	}
	return dec.DecodeAll(src, nil)
}

func keyframeEncoder() (*zstd.Encoder, error) {
	keyframeZstdEncOnce.Do(func() {
		keyframeZstdEnc, keyframeZstdEncErr = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	})
	return keyframeZstdEnc, keyframeZstdEncErr
}

func keyframeDecoder() (*zstd.Decoder, error) {
	keyframeZstdDecOnce.Do(func() {
		keyframeZstdDec, keyframeZstdDecErr = zstd.NewReader(nil)
	})
	return keyframeZstdDec, keyframeZstdDecErr
}

func normalizeAudioFormat(format AudioFormat) (AudioFormat, error) {
	if format.Codec == 0 {
		format.Codec = audioCodecSilkV3
	}
	if format.Codec != audioCodecPCM16Mono && format.Codec != audioCodecG72632 && format.Codec != audioCodecSilkV3 {
		return AudioFormat{}, fmt.Errorf("unsupported audio codec %d", format.Codec)
	}
	rate, err := voicecodec.ResolveSampleRate(voicecodec.SampleRateChoice(format.SampleRateChoice), format.SampleRate)
	if err != nil {
		return AudioFormat{}, err
	}
	format.SampleRate = rate
	if format.Channels <= 0 {
		return AudioFormat{}, fmt.Errorf("audio channels must be > 0")
	}
	if format.PacketDurationMillis <= 0 {
		return AudioFormat{}, fmt.Errorf("audio packet duration must be > 0")
	}
	expectedPacketSamples, err := voicecodec.PacketSamplesFor(format.SampleRate, format.PacketDurationMillis)
	if err != nil {
		return AudioFormat{}, err
	}
	if format.PacketSamples <= 0 {
		format.PacketSamples = expectedPacketSamples
	}
	if format.PacketSamples != expectedPacketSamples {
		return AudioFormat{}, fmt.Errorf("audio packet samples=%d want=%d", format.PacketSamples, expectedPacketSamples)
	}
	if format.Bitrate < 0 {
		return AudioFormat{}, fmt.Errorf("audio bitrate must be >= 0")
	}
	switch format.Codec {
	case audioCodecPCM16Mono:
		if format.BitsPerSample == 0 {
			format.BitsPerSample = 16
		}
	case audioCodecG72632:
		format.BitsPerSample = byte(voicecodec.NormalizeG726BitsPerSample(int(format.BitsPerSample)))
	case audioCodecSilkV3:
		format.BitsPerSample = 0
	}
	return format, nil
}

func marshalAudioFormat(format AudioFormat) ([]byte, error) {
	format, err := normalizeAudioFormat(format)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, audioFormatPayloadLen)
	payload[0] = format.Codec
	payload[1] = format.BitsPerSample
	payload[2] = format.SampleRateChoice
	binary.LittleEndian.PutUint32(payload[3:7], uint32(format.SampleRate))
	binary.LittleEndian.PutUint16(payload[7:9], uint16(format.Channels))
	binary.LittleEndian.PutUint16(payload[9:11], uint16(format.PacketDurationMillis))
	binary.LittleEndian.PutUint16(payload[11:13], uint16(format.PacketSamples))
	binary.LittleEndian.PutUint32(payload[13:17], uint32(format.Bitrate))
	return payload, nil
}

func unmarshalAudioFormat(payload []byte) (AudioFormat, error) {
	if len(payload) != audioFormatPayloadLen {
		return AudioFormat{}, fmt.Errorf("audio format payload len=%d want=%d", len(payload), audioFormatPayloadLen)
	}
	return normalizeAudioFormat(AudioFormat{
		Codec:                payload[0],
		BitsPerSample:        payload[1],
		SampleRateChoice:     payload[2],
		SampleRate:           int(binary.LittleEndian.Uint32(payload[3:7])),
		Channels:             int(binary.LittleEndian.Uint16(payload[7:9])),
		PacketDurationMillis: int(binary.LittleEndian.Uint16(payload[9:11])),
		PacketSamples:        int(binary.LittleEndian.Uint16(payload[11:13])),
		Bitrate:              int(binary.LittleEndian.Uint32(payload[13:17])),
	})
}

func writeAudioFormatRecord(w io.Writer, payload []byte) error {
	if len(payload) != audioFormatPayloadLen {
		return fmt.Errorf("audio format payload len=%d want=%d", len(payload), audioFormatPayloadLen)
	}
	var buf [1 + audioFormatPayloadLen]byte
	buf[0] = audioRecordTypeFormat
	copy(buf[1:], payload)
	_, err := w.Write(buf[:])
	return err
}

func writeAudioChunkRecord(w io.Writer, format AudioFormat, chunk AudioChunk) error {
	flags, err := audioChunkWireFlags(format, chunk)
	if err != nil {
		return err
	}
	var header [audioChunkHeaderSize]byte
	header[0] = flags
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if audioCodecUsesVariablePayload(format.Codec) && !chunk.Silence {
		var lenBuf [audioChunkVarLenSize]byte
		binary.LittleEndian.PutUint16(lenBuf[:], uint16(len(chunk.Payload)))
		if _, err := w.Write(lenBuf[:]); err != nil {
			return err
		}
	}
	if chunk.Silence || len(chunk.Payload) == 0 {
		return nil
	}
	_, err = w.Write(chunk.Payload)
	return err
}

func readAudioRecord(r io.Reader, currentFormat AudioFormat) (kind byte, format AudioFormat, chunk AudioChunk, err error) {
	var tag [1]byte
	if _, err = io.ReadFull(r, tag[:]); err != nil {
		return 0, AudioFormat{}, AudioChunk{}, err
	}
	if tag[0] == audioRecordTypeFormat {
		payload := make([]byte, audioFormatPayloadLen)
		if _, err = io.ReadFull(r, payload); err != nil {
			return 0, AudioFormat{}, AudioChunk{}, err
		}
		format, err = unmarshalAudioFormat(payload)
		return audioRecordTypeFormat, format, AudioChunk{}, err
	}
	if currentFormat.Codec == 0 {
		return 0, AudioFormat{}, AudioChunk{}, fmt.Errorf("audio chunk received before format")
	}
	payloadLen := 0
	if tag[0]&audioChunkFlagSilence == 0 {
		if audioCodecUsesVariablePayload(currentFormat.Codec) {
			var lenBuf [audioChunkVarLenSize]byte
			if _, err = io.ReadFull(r, lenBuf[:]); err != nil {
				return 0, AudioFormat{}, AudioChunk{}, err
			}
			payloadLen = int(binary.LittleEndian.Uint16(lenBuf[:]))
		} else {
			payloadLen, err = audioChunkPayloadLen(currentFormat, tag[0])
			if err != nil {
				return 0, AudioFormat{}, AudioChunk{}, err
			}
		}
	}
	payload := make([]byte, payloadLen)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, AudioFormat{}, AudioChunk{}, err
	}
	return tag[0], AudioFormat{}, AudioChunk{
		Silence: tag[0]&audioChunkFlagSilence != 0,
		Payload: payload,
	}, nil
}

func audioChunkWireFlags(format AudioFormat, chunk AudioChunk) (byte, error) {
	if format.Codec == 0 || format.PacketSamples <= 0 || format.Channels <= 0 {
		return 0, fmt.Errorf("audio config is required for chunk encoding")
	}
	var flags byte
	if chunk.Silence {
		if len(chunk.Payload) != 0 {
			return 0, fmt.Errorf("silent audio chunk payload len=%d want=0", len(chunk.Payload))
		}
		flags |= audioChunkFlagSilence
		return flags, nil
	}
	switch format.Codec {
	case audioCodecPCM16Mono:
		want := format.PacketSamples * format.Channels * 2
		if len(chunk.Payload) != want {
			return 0, fmt.Errorf("raw pcm payload len=%d want=%d", len(chunk.Payload), want)
		}
	case audioCodecG72632:
		want, err := voicecodec.G726PacketBytes(format.PacketSamples, format.Channels, int(format.BitsPerSample))
		if err != nil {
			return 0, err
		}
		if len(chunk.Payload) != want {
			return 0, fmt.Errorf("g726 payload len=%d want=%d", len(chunk.Payload), want)
		}
	case audioCodecSilkV3:
		if len(chunk.Payload) == 0 {
			return 0, fmt.Errorf("silk payload must not be empty")
		}
		if len(chunk.Payload) > 0xffff {
			return 0, fmt.Errorf("silk payload len=%d exceeds %d", len(chunk.Payload), 0xffff)
		}
	default:
		return 0, fmt.Errorf("unsupported audio codec %d", format.Codec)
	}
	return flags, nil
}

func audioChunkPayloadLen(format AudioFormat, flags byte) (int, error) {
	if flags&audioChunkFlagSilence != 0 {
		return 0, nil
	}
	switch format.Codec {
	case audioCodecPCM16Mono:
		return format.PacketSamples * format.Channels * 2, nil
	case audioCodecG72632:
		return voicecodec.G726PacketBytes(format.PacketSamples, format.Channels, int(format.BitsPerSample))
	default:
		return 0, fmt.Errorf("unsupported audio codec %d", format.Codec)
	}
}

func audioCodecUsesVariablePayload(codec byte) bool {
	return codec == audioCodecSilkV3
}

func (v *Viewer) readErr() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.err
}

func (v *Viewer) setErr(err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.err == nil {
		v.err = err
	}
}

func (v *AudioViewer) readErr() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.err
}

func (v *AudioViewer) setErr(err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.err == nil {
		v.err = err
	}
}

func writeHello(w io.Writer, role byte, flags uint16, sessionID uint64, session SessionConfig) error {
	payload, err := marshalSessionConfig(session)
	if err != nil {
		return err
	}
	var header [20]byte
	copy(header[:4], protocolMagic)
	header[4] = protocolVersion
	header[5] = role
	binary.LittleEndian.PutUint16(header[6:8], flags)
	binary.LittleEndian.PutUint64(header[8:16], sessionID)
	binary.LittleEndian.PutUint32(header[16:20], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func readHello(r io.Reader) (role byte, flags uint16, sessionID uint64, session SessionConfig, err error) {
	var header [20]byte
	if _, err = io.ReadFull(r, header[:]); err != nil {
		return 0, 0, 0, SessionConfig{}, err
	}
	if string(header[:4]) != protocolMagic {
		return 0, 0, 0, SessionConfig{}, fmt.Errorf("unexpected hello magic %q", string(header[:4]))
	}
	if header[4] != protocolVersion {
		return 0, 0, 0, SessionConfig{}, fmt.Errorf("unsupported broadcast protocol %d", header[4])
	}
	role = header[5]
	flags = binary.LittleEndian.Uint16(header[6:8])
	sessionID = binary.LittleEndian.Uint64(header[8:16])
	payloadLen := binary.LittleEndian.Uint32(header[16:20])
	payload := make([]byte, payloadLen)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, 0, 0, SessionConfig{}, err
	}
	session, err = unmarshalSessionConfig(payload)
	return role, flags, sessionID, session, err
}

func writeFrame(w io.Writer, header frameHeader, payload []byte) error {
	switch header.Type {
	case frameTypeKeyframe:
		var buf [frameHeaderSize]byte
		buf[0] = header.Type
		buf[1] = header.Flags
		binary.LittleEndian.PutUint32(buf[2:6], uint32(len(payload)))
		binary.LittleEndian.PutUint32(buf[6:10], header.Tic)
		if _, err := w.Write(buf[:]); err != nil {
			return err
		}
		if len(payload) == 0 {
			return nil
		}
		_, err := w.Write(payload)
		return err
	case frameTypeTicBatch:
		if len(payload) < ticBatchOverhead {
			return fmt.Errorf("tic batch payload too short")
		}
		count := int(binary.LittleEndian.Uint16(payload[0:2]))
		want := ticBatchOverhead + count*ticBatchSize
		if len(payload) != want {
			return fmt.Errorf("tic batch payload len=%d want=%d", len(payload), want)
		}
		if count <= 0 || count > gameplayRecordTicBatchMax {
			return fmt.Errorf("tic batch count=%d want 1..%d", count, gameplayRecordTicBatchMax)
		}
		if _, err := w.Write([]byte{gameplayRecordTicBatchBase | byte(count)}); err != nil {
			return err
		}
		_, err := w.Write(payload[ticBatchOverhead:])
		return err
	case frameTypeIntermissionAdvance:
		if len(payload) != 0 {
			return fmt.Errorf("intermission advance payload len=%d want=0", len(payload))
		}
		_, err := w.Write([]byte{frameTypeIntermissionAdvance})
		return err
	case frameTypeChat:
		if _, err := w.Write([]byte{frameTypeChat}); err != nil {
			return err
		}
		_, err := w.Write(payload)
		return err
	case frameTypePeerTicBatch:
		// payload: player_id[1] + count[2 LE] + tics[count*4]
		if len(payload) < peerTicBatchOverhead {
			return fmt.Errorf("peer tic batch payload too short")
		}
		count := int(binary.LittleEndian.Uint16(payload[1:3]))
		want := peerTicBatchOverhead + count*ticBatchSize
		if len(payload) != want {
			return fmt.Errorf("peer tic batch payload len=%d want=%d", len(payload), want)
		}
		if count <= 0 || count > gameplayRecordTicBatchMax {
			return fmt.Errorf("peer tic batch count=%d want 1..%d", count, gameplayRecordTicBatchMax)
		}
		if _, err := w.Write([]byte{frameTypePeerTicBatch}); err != nil {
			return err
		}
		_, err := w.Write(payload)
		return err
	case frameTypeRosterUpdate:
		if _, err := w.Write([]byte{frameTypeRosterUpdate}); err != nil {
			return err
		}
		_, err := w.Write(payload)
		return err
	default:
		return fmt.Errorf("unsupported gameplay frame type %d", header.Type)
	}
}

func readFrame(r io.Reader) (frameHeader, []byte, error) {
	var kind [1]byte
	if _, err := io.ReadFull(r, kind[:]); err != nil {
		return frameHeader{}, nil, err
	}
	switch {
	case kind[0] == frameTypeKeyframe:
		var buf [frameHeaderSize - 1]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return frameHeader{}, nil, err
		}
		header := frameHeader{
			Type:   frameTypeKeyframe,
			Flags:  buf[0],
			Length: binary.LittleEndian.Uint32(buf[1:5]),
			Tic:    binary.LittleEndian.Uint32(buf[5:9]),
		}
		payload := make([]byte, header.Length)
		if _, err := io.ReadFull(r, payload); err != nil {
			return frameHeader{}, nil, err
		}
		return header, payload, nil
	case kind[0]&0xC0 == gameplayRecordTicBatchBase:
		count := int(kind[0] & 0x3F)
		if count <= 0 {
			return frameHeader{}, nil, fmt.Errorf("tic batch count=%d want >0", count)
		}
		payload := make([]byte, ticBatchOverhead+count*ticBatchSize)
		binary.LittleEndian.PutUint16(payload[0:2], uint16(count))
		if _, err := io.ReadFull(r, payload[2:]); err != nil {
			return frameHeader{}, nil, err
		}
		return frameHeader{
			Type:   frameTypeTicBatch,
			Length: uint32(len(payload)),
		}, payload, nil
	case kind[0] == frameTypeIntermissionAdvance:
		return frameHeader{Type: frameTypeIntermissionAdvance}, nil, nil
	case kind[0] == frameTypeChat:
		// 1 byte name_len + 2 byte text_len + 1 byte reserved = 4 bytes header
		var hdr [4]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return frameHeader{}, nil, err
		}
		nameLen := int(hdr[0])
		textLen := int(binary.LittleEndian.Uint16(hdr[1:3]))
		if nameLen > chatMaxNameLen || textLen > chatMaxTextLen {
			return frameHeader{}, nil, fmt.Errorf("chat frame lengths out of range name=%d text=%d", nameLen, textLen)
		}
		body := make([]byte, 4+nameLen+textLen)
		copy(body, hdr[:])
		if nameLen+textLen > 0 {
			if _, err := io.ReadFull(r, body[4:]); err != nil {
				return frameHeader{}, nil, err
			}
		}
		return frameHeader{Type: frameTypeChat}, body, nil
	case kind[0] == frameTypePeerTicBatch:
		// player_id[1] + count[2 LE] + tics[count*4]
		var hdr [peerTicBatchOverhead]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return frameHeader{}, nil, err
		}
		playerID := hdr[0]
		count := int(binary.LittleEndian.Uint16(hdr[1:3]))
		if count <= 0 || count > gameplayRecordTicBatchMax {
			return frameHeader{}, nil, fmt.Errorf("peer tic batch count=%d want 1..%d", count, gameplayRecordTicBatchMax)
		}
		payload := make([]byte, peerTicBatchOverhead+count*ticBatchSize)
		payload[0] = playerID
		binary.LittleEndian.PutUint16(payload[1:3], uint16(count))
		if _, err := io.ReadFull(r, payload[peerTicBatchOverhead:]); err != nil {
			return frameHeader{}, nil, err
		}
		return frameHeader{Type: frameTypePeerTicBatch}, payload, nil
	case kind[0] == frameTypeRosterUpdate:
		// count[1] + player_ids[count]
		var lenBuf [1]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			return frameHeader{}, nil, err
		}
		n := int(lenBuf[0])
		payload := make([]byte, 1+n)
		payload[0] = lenBuf[0]
		if n > 0 {
			if _, err := io.ReadFull(r, payload[1:]); err != nil {
				return frameHeader{}, nil, err
			}
		}
		return frameHeader{Type: frameTypeRosterUpdate}, payload, nil
	default:
		return frameHeader{}, nil, fmt.Errorf("unexpected broadcast frame type %d", kind[0])
	}
}

// marshalChatPayload encodes a ChatMessage into the payload bytes used by writeFrame/readFrame.
// Format: [1 name_len][2 text_len LE][1 reserved][name...][text...]
func marshalChatPayload(msg ChatMessage) []byte {
	name := []byte(msg.Name)
	text := []byte(msg.Text)
	if len(name) > chatMaxNameLen {
		name = name[:chatMaxNameLen]
	}
	if len(text) > chatMaxTextLen {
		text = text[:chatMaxTextLen]
	}
	buf := make([]byte, 4+len(name)+len(text))
	buf[0] = byte(len(name))
	binary.LittleEndian.PutUint16(buf[1:3], uint16(len(text)))
	buf[3] = 0 // reserved
	copy(buf[4:], name)
	copy(buf[4+len(name):], text)
	return buf
}

func readChatFrame(r io.Reader) (ChatMessage, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return ChatMessage{}, err
	}
	nameLen := int(hdr[0])
	textLen := int(binary.LittleEndian.Uint16(hdr[1:3]))
	// hdr[3] reserved
	if nameLen > chatMaxNameLen || textLen > chatMaxTextLen {
		return ChatMessage{}, fmt.Errorf("chat frame lengths out of range name=%d text=%d", nameLen, textLen)
	}
	buf := make([]byte, nameLen+textLen)
	if len(buf) > 0 {
		if _, err := io.ReadFull(r, buf); err != nil {
			return ChatMessage{}, err
		}
	}
	return ChatMessage{
		Name: string(buf[:nameLen]),
		Text: string(buf[nameLen:]),
	}, nil
}

func marshalPeerTicBatchPayload(playerID byte, tics []demo.Tic) []byte {
	payload := make([]byte, peerTicBatchOverhead+len(tics)*ticBatchSize)
	payload[0] = playerID
	binary.LittleEndian.PutUint16(payload[1:3], uint16(len(tics)))
	for i, tc := range tics {
		copy(payload[peerTicBatchOverhead+i*ticBatchSize:], packDemoTic(tc))
	}
	return payload
}

func unmarshalPeerTicBatchPayload(payload []byte) (playerID byte, tics []demo.Tic, err error) {
	if len(payload) < peerTicBatchOverhead {
		return 0, nil, fmt.Errorf("peer tic batch payload too short")
	}
	playerID = payload[0]
	count := int(binary.LittleEndian.Uint16(payload[1:3]))
	want := peerTicBatchOverhead + count*ticBatchSize
	if len(payload) != want {
		return 0, nil, fmt.Errorf("peer tic batch payload len=%d want=%d", len(payload), want)
	}
	tics = make([]demo.Tic, count)
	for i := range tics {
		tics[i] = unpackDemoTic(payload[peerTicBatchOverhead+i*ticBatchSize:])
	}
	return playerID, tics, nil
}

func marshalRosterUpdatePayload(roster RosterUpdate) []byte {
	n := len(roster.PlayerIDs)
	if n > 255 {
		n = 255
	}
	payload := make([]byte, 1+n)
	payload[0] = byte(n)
	copy(payload[1:], roster.PlayerIDs[:n])
	return payload
}

func unmarshalRosterUpdatePayload(payload []byte) (RosterUpdate, error) {
	if len(payload) < 1 {
		return RosterUpdate{}, fmt.Errorf("roster update payload too short")
	}
	n := int(payload[0])
	if len(payload) != 1+n {
		return RosterUpdate{}, fmt.Errorf("roster update payload len=%d want=%d", len(payload), 1+n)
	}
	ids := make([]byte, n)
	copy(ids, payload[1:])
	return RosterUpdate{PlayerIDs: ids}, nil
}

func marshalSessionConfig(session SessionConfig) ([]byte, error) {
	var buf bytes.Buffer
	putString := func(s string) error {
		if len(s) > 0xFFFF {
			return fmt.Errorf("session string too long")
		}
		var hdr [2]byte
		binary.LittleEndian.PutUint16(hdr[:], uint16(len(s)))
		if _, err := buf.Write(hdr[:]); err != nil {
			return err
		}
		_, err := buf.WriteString(s)
		return err
	}
	if err := putString(strings.TrimSpace(session.WADHash)); err != nil {
		return nil, err
	}
	if err := putString(strings.TrimSpace(session.MapName)); err != nil {
		return nil, err
	}
	if err := putString(strings.TrimSpace(session.GameMode)); err != nil {
		return nil, err
	}
	var maxPlayers [2]byte
	binary.LittleEndian.PutUint16(maxPlayers[:], uint16(clampUint16(session.MaxPlayers)))
	if _, err := buf.Write(maxPlayers[:]); err != nil {
		return nil, err
	}
	buf.WriteByte(byte(clampUint8(session.PlayerSlot)))
	buf.WriteByte(byte(clampUint8(session.SkillLevel)))
	buf.WriteByte(byte(clampUint8(session.CheatLevel)))
	buf.WriteByte(0)
	var flags uint16
	if session.ShowNoSkillItems {
		flags |= sessionFlagShowNoSkillItems
	}
	if session.ShowAllItems {
		flags |= sessionFlagShowAllItems
	}
	if session.FastMonsters {
		flags |= sessionFlagFastMonsters
	}
	if session.RespawnMonsters {
		flags |= sessionFlagRespawnMonsters
	}
	if session.NoMonsters {
		flags |= sessionFlagNoMonsters
	}
	if session.AutoWeaponSwitch {
		flags |= sessionFlagAutoWeaponSwitch
	}
	if session.Invulnerable {
		flags |= sessionFlagInvulnerable
	}
	if session.SourcePortMode {
		flags |= sessionFlagSourcePortMode
	}
	var flagBuf [2]byte
	binary.LittleEndian.PutUint16(flagBuf[:], flags)
	if _, err := buf.Write(flagBuf[:]); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func unmarshalSessionConfig(payload []byte) (SessionConfig, error) {
	var session SessionConfig
	readString := func(data []byte, off *int) (string, error) {
		if len(data)-*off < 2 {
			return "", fmt.Errorf("session payload truncated")
		}
		n := int(binary.LittleEndian.Uint16(data[*off : *off+2]))
		*off += 2
		if len(data)-*off < n {
			return "", fmt.Errorf("session payload truncated")
		}
		s := string(data[*off : *off+n])
		*off += n
		return s, nil
	}
	offset := 0
	var err error
	if session.WADHash, err = readString(payload, &offset); err != nil {
		return SessionConfig{}, err
	}
	if session.MapName, err = readString(payload, &offset); err != nil {
		return SessionConfig{}, err
	}
	if session.GameMode, err = readString(payload, &offset); err != nil {
		return SessionConfig{}, err
	}
	if len(payload)-offset < 8 {
		return SessionConfig{}, fmt.Errorf("session payload truncated")
	}
	session.MaxPlayers = int(binary.LittleEndian.Uint16(payload[offset : offset+2]))
	offset += 2
	session.PlayerSlot = int(payload[offset])
	session.SkillLevel = int(payload[offset+1])
	session.CheatLevel = int(payload[offset+2])
	offset += 4
	flags := binary.LittleEndian.Uint16(payload[offset : offset+2])
	session.ShowNoSkillItems = flags&sessionFlagShowNoSkillItems != 0
	session.ShowAllItems = flags&sessionFlagShowAllItems != 0
	session.FastMonsters = flags&sessionFlagFastMonsters != 0
	session.RespawnMonsters = flags&sessionFlagRespawnMonsters != 0
	session.NoMonsters = flags&sessionFlagNoMonsters != 0
	session.AutoWeaponSwitch = flags&sessionFlagAutoWeaponSwitch != 0
	session.Invulnerable = flags&sessionFlagInvulnerable != 0
	session.SourcePortMode = flags&sessionFlagSourcePortMode != 0
	return session, nil
}

func packDemoTic(tc demo.Tic) []byte {
	return []byte{
		byte(tc.Forward),
		byte(tc.Side),
		byte((uint16(tc.AngleTurn) + 128) >> 8),
		tc.Buttons,
	}
}

func unpackDemoTic(data []byte) demo.Tic {
	return demo.Tic{
		Forward:   int8(data[0]),
		Side:      int8(data[1]),
		AngleTurn: int16(uint16(data[2]) << 8),
		Buttons:   data[3],
	}
}

func clampUint8(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 0xFF:
		return 0xFF
	default:
		return v
	}
}

func clampUint16(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 0xFFFF:
		return 0xFFFF
	default:
		return v
	}
}
