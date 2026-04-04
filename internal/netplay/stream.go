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
)

const (
	protocolVersion byte = 1
	protocolMagic        = "GDSF"

	helloRoleBroadcaster byte = 1
	helloRoleViewer      byte = 2
	helloRoleServer      byte = 3

	frameTypeKeyframe            byte = 1
	frameTypeTicBatch            byte = 4
	frameTypeIntermissionAdvance byte = 8
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
)

const (
	keyframeFlagMandatoryApply byte = 1 << iota
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

type RelayBroadcaster struct {
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
	return &RelayBroadcaster{conn: conn, sessionID: assignedID, meter: meter}, nil
}

func (b *RelayBroadcaster) SessionID() uint64 {
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

func (b *RelayBroadcaster) BroadcastTic(tc demo.Tic) error {
	if b == nil {
		return nil
	}
	payload := make([]byte, ticBatchOverhead+4)
	binary.LittleEndian.PutUint16(payload[0:2], 1)
	copy(payload[ticBatchOverhead:], packDemoTic(tc))
	return writeFrame(b.conn, frameHeader{
		Type:   frameTypeTicBatch,
		Length: uint32(len(payload)),
	}, payload)
}

func (b *RelayBroadcaster) BroadcastKeyframe(tic uint32, blob []byte) error {
	return b.BroadcastKeyframeWithFlags(tic, blob, 0)
}

func (b *RelayBroadcaster) BroadcastKeyframeWithFlags(tic uint32, blob []byte, flags byte) error {
	if b == nil {
		return nil
	}
	return writeFrame(b.conn, frameHeader{
		Type:   frameTypeKeyframe,
		Flags:  flags,
		Length: uint32(len(blob)),
		Tic:    tic,
	}, blob)
}

func (b *RelayBroadcaster) BroadcastIntermissionAdvance() error {
	if b == nil {
		return nil
	}
	return writeFrame(b.conn, frameHeader{Type: frameTypeIntermissionAdvance}, nil)
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
	b.mu.Unlock()
	return b.conn.Close()
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
