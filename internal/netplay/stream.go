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

	"github.com/klauspost/compress/zstd"
)

const (
	protocolVersion byte = 1
	protocolMagic        = "GDSF"

	helloRoleBroadcaster byte = 1
	helloRoleViewer      byte = 2
	helloRoleServer      byte = 3
	helloRoleAudioBroadcaster byte = 4
	helloRoleAudioViewer      byte = 5

	frameTypeKeyframe            byte = 1
	frameTypeTicBatch            byte = 4
	frameTypeIntermissionAdvance byte = 8
	frameTypeAudioConfig         byte = 16
	frameTypeAudioChunk          byte = 17
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
	frameHeaderSize  = 12
	ticBatchOverhead = 4
	ticBatchSize     = 4
	audioChunkOverhead = 8
)

const (
	keyframeFlagMandatoryApply byte = 1 << iota
	keyframeFlagZstdCompressed
)

const (
	audioChunkFlagSilence byte = 1 << iota
)

const (
	audioCodecOpus      byte = 1
	audioCodecPCM16Mono byte = 2
	audioViewerChunkQueue     = 24
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

type AudioConfig struct {
	Codec        byte
	SampleRate   int
	Channels     int
	FrameSamples int
	Bitrate      int
}

type AudioChunk struct {
	GameTic     uint32
	StartSample uint64
	Silence     bool
	Payload     []byte
}

type RelayBroadcaster struct {
	conn      net.Conn
	sessionID uint64
	meter     *bandwidthMeter

	mu           sync.Mutex
	closed       bool
	pendingTic   []demo.Tic
	ticBatchSize int
}

type AudioBroadcaster struct {
	conn      net.Conn
	sessionID uint64
	meter     *bandwidthMeter

	mu     sync.Mutex
	closed bool
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
	if err := writeHello(conn, helloRoleBroadcaster, 0, sessionID, session); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write relay hello: %w", err)
	}
	role, _, assignedID, _, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read relay hello ack: %w", err)
	}
	if role != helloRoleServer {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected relay hello ack role %d", role)
	}
	return &RelayBroadcaster{
		conn:         conn,
		sessionID:    assignedID,
		meter:        meter,
		pendingTic:   make([]demo.Tic, 0, ticBatchSize),
		ticBatchSize: ticBatchSize,
	}, nil
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
	if err := writeHello(conn, helloRoleAudioBroadcaster, 0, sessionID, SessionConfig{}); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write audio relay hello: %w", err)
	}
	role, _, assignedID, _, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read audio relay hello ack: %w", err)
	}
	if role != helloRoleServer {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected audio relay hello ack role %d", role)
	}
	return &AudioBroadcaster{
		conn:      conn,
		sessionID: assignedID,
		meter:     meter,
	}, nil
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

func (b *AudioBroadcaster) BroadcastAudioConfig(cfg AudioConfig) error {
	if b == nil {
		return nil
	}
	payload, err := marshalAudioConfig(cfg)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	return writeFrame(b.conn, frameHeader{
		Type:   frameTypeAudioConfig,
		Length: uint32(len(payload)),
	}, payload)
}

func (b *AudioBroadcaster) BroadcastAudioChunk(chunk AudioChunk) error {
	if b == nil {
		return nil
	}
	payloadLen := audioChunkOverhead + len(chunk.Payload)
	if chunk.Silence {
		payloadLen = audioChunkOverhead
	}
	payload := make([]byte, payloadLen)
	binary.LittleEndian.PutUint64(payload[:audioChunkOverhead], chunk.StartSample)
	if !chunk.Silence {
		copy(payload[audioChunkOverhead:], chunk.Payload)
	}
	var flags byte
	if chunk.Silence {
		flags |= audioChunkFlagSilence
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	return writeFrame(b.conn, frameHeader{
		Type:   frameTypeAudioChunk,
		Flags:  flags,
		Length: uint32(len(payload)),
		Tic:    chunk.GameTic,
	}, payload)
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
	meter     *bandwidthMeter

	mu     sync.Mutex
	err    error
	closed bool
	wg     sync.WaitGroup
}

type AudioViewer struct {
	conn    net.Conn
	session SessionConfig
	configs chan AudioConfig
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
	if err := writeHello(conn, helloRoleViewer, 0, sessionID, SessionConfig{}); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write relay hello: %w", err)
	}
	role, _, resolvedID, session, err := readHello(conn)
	if err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("read relay session hello: %w", err)
	}
	if role != helloRoleBroadcaster {
		_ = rawConn.Close()
		return nil, fmt.Errorf("unexpected relay session role %d", role)
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
	if err := writeHello(conn, helloRoleAudioViewer, 0, sessionID, SessionConfig{}); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("write audio relay hello: %w", err)
	}
	role, _, resolvedID, session, err := readHello(conn)
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
	if localWADHash != "" && session.WADHash != "" && session.WADHash != localWADHash {
		_ = rawConn.Close()
		return nil, fmt.Errorf("broadcast WAD hash mismatch: local=%s host=%s", localWADHash, session.WADHash)
	}
	v := &AudioViewer{
		conn:    conn,
		session: session,
		configs: make(chan AudioConfig, 2),
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

func (v *AudioViewer) PollAudioConfig() (AudioConfig, bool, error) {
	if v == nil {
		return AudioConfig{}, false, nil
	}
	select {
	case cfg, ok := <-v.configs:
		if ok {
			return cfg, true, nil
		}
		return AudioConfig{}, false, v.readErr()
	default:
		return AudioConfig{}, false, v.readErr()
	}
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
		default:
			v.setErr(fmt.Errorf("unexpected broadcast frame type %d", header.Type))
			return
		}
	}
}

func (v *AudioViewer) readLoop() {
	defer v.wg.Done()
	defer close(v.configs)
	defer close(v.chunks)
	for {
		header, payload, err := readFrame(v.conn)
		if err != nil {
			v.setErr(err)
			return
		}
		switch header.Type {
		case frameTypeAudioConfig:
			cfg, err := unmarshalAudioConfig(payload)
			if err != nil {
				v.setErr(err)
				return
			}
			v.configs <- cfg
		case frameTypeAudioChunk:
			if len(payload) < audioChunkOverhead {
				v.setErr(fmt.Errorf("audio chunk payload too short"))
				return
			}
			v.chunks <- AudioChunk{
				GameTic:     header.Tic,
				StartSample: binary.LittleEndian.Uint64(payload[:audioChunkOverhead]),
				Silence:     header.Flags&audioChunkFlagSilence != 0,
				Payload:     append([]byte(nil), payload[audioChunkOverhead:]...),
			}
		default:
			v.setErr(fmt.Errorf("unexpected audio frame type %d", header.Type))
			return
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

func marshalAudioConfig(cfg AudioConfig) ([]byte, error) {
	if cfg.Codec == 0 {
		cfg.Codec = audioCodecOpus
	}
	if cfg.SampleRate <= 0 {
		return nil, fmt.Errorf("audio sample rate must be > 0")
	}
	if cfg.Channels <= 0 {
		return nil, fmt.Errorf("audio channels must be > 0")
	}
	if cfg.FrameSamples <= 0 {
		return nil, fmt.Errorf("audio frame samples must be > 0")
	}
	if cfg.Bitrate < 0 {
		return nil, fmt.Errorf("audio bitrate must be >= 0")
	}
	payload := make([]byte, 16)
	payload[0] = cfg.Codec
	binary.LittleEndian.PutUint32(payload[4:8], uint32(cfg.SampleRate))
	binary.LittleEndian.PutUint16(payload[8:10], uint16(cfg.Channels))
	binary.LittleEndian.PutUint16(payload[10:12], uint16(cfg.FrameSamples))
	binary.LittleEndian.PutUint32(payload[12:16], uint32(cfg.Bitrate))
	return payload, nil
}

func unmarshalAudioConfig(payload []byte) (AudioConfig, error) {
	if len(payload) != 16 {
		return AudioConfig{}, fmt.Errorf("audio config payload len=%d want=16", len(payload))
	}
	cfg := AudioConfig{
		Codec:        payload[0],
		SampleRate:   int(binary.LittleEndian.Uint32(payload[4:8])),
		Channels:     int(binary.LittleEndian.Uint16(payload[8:10])),
		FrameSamples: int(binary.LittleEndian.Uint16(payload[10:12])),
		Bitrate:      int(binary.LittleEndian.Uint32(payload[12:16])),
	}
	if cfg.Codec == 0 {
		return AudioConfig{}, fmt.Errorf("audio codec is required")
	}
	if cfg.SampleRate <= 0 || cfg.Channels <= 0 || cfg.FrameSamples <= 0 {
		return AudioConfig{}, fmt.Errorf("audio config invalid")
	}
	return cfg, nil
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
	var buf [frameHeaderSize]byte
	buf[0] = header.Type
	buf[1] = header.Flags
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(payload)))
	binary.LittleEndian.PutUint32(buf[8:12], header.Tic)
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r io.Reader) (frameHeader, []byte, error) {
	var buf [frameHeaderSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return frameHeader{}, nil, err
	}
	header := frameHeader{
		Type:   buf[0],
		Flags:  buf[1],
		Length: binary.LittleEndian.Uint32(buf[4:8]),
		Tic:    binary.LittleEndian.Uint32(buf[8:12]),
	}
	payload := make([]byte, header.Length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return frameHeader{}, nil, err
	}
	return header, payload, nil
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
	if len(payload)-offset < 6 {
		return SessionConfig{}, fmt.Errorf("session payload truncated")
	}
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
