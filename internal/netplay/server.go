package netplay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type Server struct {
	listener net.Listener

	mu       sync.Mutex
	sessions map[uint64]*relaySession
	nextID   uint64

	closeOnce sync.Once
	closed    chan struct{}
	wg        sync.WaitGroup
}

type relaySession struct {
	id                uint64
	cfg               SessionConfig
	server            *Server
	src               net.Conn
	srcViewer         *relayViewer // broadcaster conn wrapped for chat writes
	audioSrc          net.Conn
	viewers           map[*relayViewer]struct{}
	audioViewers      map[*relayViewer]struct{}
	peers             map[*relayPeer]struct{}
	lastKeyframe      []byte
	lastKeyframeFlags byte
	lastKeyframeTic   uint32
	backlog           []bufferedFrame
	lastAudioFormat   []byte
}

type relayPeer struct {
	conn     net.Conn
	playerID byte
	mu       sync.Mutex
	chat     relayViewer // reuse chat throttle logic; conn field unused here
}

func (p *relayPeer) allowChatMessage(msg ChatMessage, now time.Time) bool {
	return p.chat.allowChatMessage(msg, now)
}

type relayViewer struct {
	conn net.Conn
	mu   sync.Mutex

	chatMu     sync.Mutex
	chatTimes  []time.Time
	chatRecent []string
}

type bufferedFrame struct {
	header  frameHeader
	payload []byte
}

const maxBufferedRelayFrames = 35 * 60 * 5

const (
	relayChatRecentRejectLines = 8
	relayChatThrottleWindow    = 8 * time.Second
	relayChatThrottleBurst     = 4
)

func ListenServer(addr string) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen server %s: %w", addr, err)
	}
	s := &Server{
		listener: ln,
		sessions: make(map[uint64]*relaySession),
		nextID:   1,
		closed:   make(chan struct{}),
	}
	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

func (s *Server) Addr() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		close(s.closed)
		if s.listener != nil {
			err = s.listener.Close()
			s.listener = nil
		}
		s.mu.Lock()
		sessions := make([]*relaySession, 0, len(s.sessions))
		for _, sess := range s.sessions {
			sessions = append(sessions, sess)
		}
		s.mu.Unlock()
		for _, sess := range sessions {
			s.closeSession(sess)
		}
		s.wg.Wait()
	})
	return err
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			select {
			case <-s.closed:
				return
			default:
				continue
			}
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	role, flags, sessionID, sessionCfg, err := readHello(conn)
	if err != nil {
		_ = conn.Close()
		return
	}
	switch role {
	case helloRoleBroadcaster:
		s.handleBroadcaster(conn, sessionID, sessionCfg, flags)
	case helloRoleAudioBroadcaster:
		if sessionID == 0 {
			_ = conn.Close()
			return
		}
		s.handleAudioBroadcaster(conn, sessionID, flags)
	case helloRoleViewer:
		if sessionID == 0 {
			_ = conn.Close()
			return
		}
		s.handleViewer(conn, sessionID, flags)
	case helloRoleAudioViewer:
		if sessionID == 0 {
			_ = conn.Close()
			return
		}
		s.handleAudioViewer(conn, sessionID, flags)
	case helloRolePlayerPeer:
		s.handlePlayerPeer(conn, sessionID, sessionCfg, flags)
	default:
		_ = conn.Close()
	}
}

func (s *Server) handleBroadcaster(conn net.Conn, sessionID uint64, cfg SessionConfig, flags uint16) {
	if flags&helloFlagGameplayCompactV1 == 0 {
		_ = conn.Close()
		return
	}
	s.mu.Lock()
	if sessionID == 0 {
		sessionID = s.nextSessionIDLocked()
	}
	if _, exists := s.sessions[sessionID]; exists {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	sess := &relaySession{
		id:           sessionID,
		cfg:          cfg,
		server:       s,
		src:          conn,
		srcViewer:    &relayViewer{conn: conn},
		viewers:      make(map[*relayViewer]struct{}),
		audioViewers: make(map[*relayViewer]struct{}),
		peers:        make(map[*relayPeer]struct{}),
	}
	s.sessions[sessionID] = sess
	s.mu.Unlock()
	defer s.closeSession(sess)
	if err := writeHello(conn, helloRoleServer, helloFlagGameplayCompactV1, sessionID, cfg); err != nil {
		return
	}

	for {
		header, payload, err := readFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// ignore: session teardown handles disconnection
			}
			return
		}
		if header.Type == frameTypeChat {
			s.forwardChat(sess, sess.srcViewer, header, payload)
		} else {
			s.forwardFrame(sess, header, payload)
		}
	}
}

func (s *Server) handlePlayerPeer(conn net.Conn, sessionID uint64, cfg SessionConfig, flags uint16) {
	if flags&helloFlagGameplayCompactV1 == 0 {
		_ = conn.Close()
		return
	}

	s.mu.Lock()
	// Create a new session if sessionID == 0, otherwise join existing.
	var sess *relaySession
	if sessionID == 0 {
		sessionID = s.nextSessionIDLocked()
		sess = &relaySession{
			id:           sessionID,
			cfg:          cfg,
			server:       s,
			viewers:      make(map[*relayViewer]struct{}),
			audioViewers: make(map[*relayViewer]struct{}),
			peers:        make(map[*relayPeer]struct{}),
		}
		s.sessions[sessionID] = sess
	} else {
		sess = s.sessions[sessionID]
		if sess == nil {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
	}

	// Assign a player slot: use cfg.PlayerSlot if available, else pick the
	// lowest free slot among 1-4.
	playerID := byte(cfg.PlayerSlot)
	if playerID < 1 || playerID > maxPeerPlayerID {
		playerID = s.nextPeerSlotLocked(sess)
	}
	if playerID == 0 {
		// No free slots.
		s.mu.Unlock()
		_ = conn.Close()
		return
	}

	peer := &relayPeer{conn: conn, playerID: playerID}
	sess.peers[peer] = struct{}{}
	serverCfg := sess.cfg
	serverCfg.PlayerSlot = int(playerID)

	// Snapshot join state for the new peer.
	var (
		joinKeyframe      []byte
		joinKeyframeFlags byte
		joinKeyframeTic   uint32
		joinBacklog       []bufferedFrame
	)
	if len(sess.lastKeyframe) > 0 {
		joinKeyframe = append([]byte(nil), sess.lastKeyframe...)
		joinKeyframeFlags = sess.lastKeyframeFlags
		joinKeyframeTic = sess.lastKeyframeTic
	}
	if len(sess.backlog) > 0 {
		joinBacklog = make([]bufferedFrame, len(sess.backlog))
		for i, f := range sess.backlog {
			joinBacklog[i] = bufferedFrame{
				header:  f.header,
				payload: append([]byte(nil), f.payload...),
			}
		}
	}
	roster := s.peerRosterLocked(sess)
	s.mu.Unlock()

	// Acknowledge with server hello; PlayerSlot carries the assigned ID.
	if err := writeHello(conn, helloRoleServer, helloFlagGameplayCompactV1, sessionID, serverCfg); err != nil {
		s.removeRelayPeer(sess, peer)
		_ = conn.Close()
		return
	}

	// Send the current keyframe + backlog so the joining peer can catch up.
	peer.mu.Lock()
	if len(joinKeyframe) > 0 {
		if err := writeFrame(conn, frameHeader{
			Type:   frameTypeKeyframe,
			Flags:  joinKeyframeFlags,
			Length: uint32(len(joinKeyframe)),
			Tic:    joinKeyframeTic,
		}, joinKeyframe); err != nil {
			peer.mu.Unlock()
			s.removeRelayPeer(sess, peer)
			_ = conn.Close()
			return
		}
	}
	for _, f := range joinBacklog {
		if err := writeFrame(conn, f.header, f.payload); err != nil {
			peer.mu.Unlock()
			s.removeRelayPeer(sess, peer)
			_ = conn.Close()
			return
		}
	}
	// Send current roster to the joining peer.
	if err := writePeerRosterFrame(conn, roster); err != nil {
		peer.mu.Unlock()
		s.removeRelayPeer(sess, peer)
		_ = conn.Close()
		return
	}
	peer.mu.Unlock()

	// Broadcast updated roster to all other peers.
	s.mu.Lock()
	updatedRoster := s.peerRosterLocked(sess)
	others := s.otherPeersLocked(sess, peer)
	s.mu.Unlock()
	for _, other := range others {
		if err := other.writeRoster(updatedRoster); err != nil {
			s.removeRelayPeer(sess, other)
			_ = other.conn.Close()
		}
	}

	// Read loop: fan out this peer's tics/chat to all other peers.
	for {
		header, payload, err := readFrame(conn)
		if err != nil {
			break
		}
		switch header.Type {
		case frameTypePeerTicBatch:
			s.forwardPeerTicBatch(sess, peer, payload)
		case frameTypeChat:
			s.forwardPeerChat(sess, peer, header, payload)
		case frameTypeKeyframe:
			// Peers may contribute keyframes for mid-join.
			s.mu.Lock()
			if header.Flags&keyframeFlagMandatoryApply == 0 {
				sess.lastKeyframeTic = header.Tic
				sess.lastKeyframeFlags = header.Flags
				sess.lastKeyframe = append(sess.lastKeyframe[:0], payload...)
				sess.backlog = sess.backlog[:0]
			}
			s.mu.Unlock()
		case frameTypeCheckpoint:
			// Relay the canonical peer's checkpoint to all other peers.
			s.forwardCheckpoint(sess, peer, payload)
		case frameTypeDesyncRequest:
			// Push the latest keyframe as mandatory to the requesting peer.
			s.sendKeyframeToPeer(sess, peer)
		}
	}

	s.removeRelayPeer(sess, peer)
	_ = conn.Close()

	// Broadcast updated roster after departure.
	s.mu.Lock()
	departedRoster := s.peerRosterLocked(sess)
	remaining := s.otherPeersLocked(sess, nil)
	s.mu.Unlock()
	for _, other := range remaining {
		if err := other.writeRoster(departedRoster); err != nil {
			s.removeRelayPeer(sess, other)
			_ = other.conn.Close()
		}
	}
}

func (s *Server) nextPeerSlotLocked(sess *relaySession) byte {
	used := [maxPeerPlayerID + 1]bool{}
	for p := range sess.peers {
		if p.playerID >= 1 && int(p.playerID) <= maxPeerPlayerID {
			used[p.playerID] = true
		}
	}
	for slot := byte(1); int(slot) <= maxPeerPlayerID; slot++ {
		if !used[slot] {
			return slot
		}
	}
	return 0
}

func (s *Server) peerRosterLocked(sess *relaySession) RosterUpdate {
	ids := make([]byte, 0, len(sess.peers))
	for p := range sess.peers {
		ids = append(ids, p.playerID)
	}
	return RosterUpdate{PlayerIDs: ids}
}

func (s *Server) otherPeersLocked(sess *relaySession, exclude *relayPeer) []*relayPeer {
	out := make([]*relayPeer, 0, len(sess.peers))
	for p := range sess.peers {
		if p != exclude {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) forwardPeerTicBatch(sess *relaySession, sender *relayPeer, payload []byte) {
	s.mu.Lock()
	others := s.otherPeersLocked(sess, sender)
	s.mu.Unlock()
	for _, other := range others {
		if err := other.writePeerTicBatch(payload); err != nil {
			s.removeRelayPeer(sess, other)
			_ = other.conn.Close()
		}
	}
}

func (s *Server) forwardPeerChat(sess *relaySession, sender *relayPeer, header frameHeader, payload []byte) {
	if sender != nil {
		msg, err := readChatFrame(bytes.NewReader(payload))
		if err != nil {
			return
		}
		if !sender.allowChatMessage(msg, time.Now()) {
			return
		}
	}
	s.mu.Lock()
	all := s.otherPeersLocked(sess, nil)
	s.mu.Unlock()
	for _, p := range all {
		if err := p.writeChat(header, payload); err != nil {
			s.removeRelayPeer(sess, p)
			_ = p.conn.Close()
		}
	}
}

// forwardCheckpoint relays a checkpoint frame from the sender to all other peers.
func (s *Server) forwardCheckpoint(sess *relaySession, sender *relayPeer, payload []byte) {
	s.mu.Lock()
	others := s.otherPeersLocked(sess, sender)
	s.mu.Unlock()
	for _, other := range others {
		other.mu.Lock()
		_ = writeFrame(other.conn, frameHeader{Type: frameTypeCheckpoint, Length: uint32(len(payload))}, payload)
		other.mu.Unlock()
	}
}

// sendKeyframeToPeer pushes the session's latest keyframe to the given peer as
// a mandatory apply, so the peer can resync after a detected desync.
func (s *Server) sendKeyframeToPeer(sess *relaySession, peer *relayPeer) {
	s.mu.Lock()
	if len(sess.lastKeyframe) == 0 {
		s.mu.Unlock()
		return
	}
	kf := append([]byte(nil), sess.lastKeyframe...)
	kfTic := sess.lastKeyframeTic
	s.mu.Unlock()

	peer.mu.Lock()
	_ = writeFrame(peer.conn, frameHeader{
		Type:   frameTypeKeyframe,
		Flags:  keyframeFlagMandatoryApply,
		Length: uint32(len(kf)),
		Tic:    kfTic,
	}, kf)
	peer.mu.Unlock()
}

func (s *Server) removeRelayPeer(sess *relaySession, peer *relayPeer) {
	if s == nil || sess == nil || peer == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur := s.sessions[sess.id]; cur == sess {
		delete(sess.peers, peer)
	}
}

func writePeerRosterFrame(w io.Writer, roster RosterUpdate) error {
	payload := marshalRosterUpdatePayload(roster)
	return writeFrame(w, frameHeader{Type: frameTypeRosterUpdate}, payload)
}

func (p *relayPeer) writePeerTicBatch(payload []byte) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return writeFrame(p.conn, frameHeader{Type: frameTypePeerTicBatch}, payload)
}

func (p *relayPeer) writeRoster(roster RosterUpdate) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return writePeerRosterFrame(p.conn, roster)
}

func (p *relayPeer) writeChat(header frameHeader, payload []byte) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return writeFrame(p.conn, header, payload)
}

func (s *Server) handleAudioBroadcaster(conn net.Conn, sessionID uint64, flags uint16) {
	if flags&helloFlagAudioCompactV1 == 0 {
		_ = conn.Close()
		return
	}
	s.mu.Lock()
	sess := s.sessions[sessionID]
	if sess == nil || sess.audioSrc != nil {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	sess.audioSrc = conn
	cfg := sess.cfg
	s.mu.Unlock()
	defer s.closeAudioSource(sess)
	if err := writeHello(conn, helloRoleServer, helloFlagAudioCompactV1, sessionID, cfg); err != nil {
		return
	}
	currentFormat := AudioFormat{}
	for {
		kind, nextFormat, chunk, err := readAudioRecord(conn, currentFormat)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// ignore: audio source teardown handles disconnection
			}
			return
		}
		switch kind {
		case audioRecordTypeFormat:
			currentFormat = nextFormat
			s.forwardAudioFormat(sess, nextFormat)
		default:
			s.forwardAudioChunk(sess, currentFormat, chunk)
		}
	}
}

func (s *Server) nextSessionIDLocked() uint64 {
	for {
		id := s.nextID
		s.nextID++
		if id == 0 {
			continue
		}
		if _, exists := s.sessions[id]; !exists {
			return id
		}
	}
}

func (s *Server) handleViewer(conn net.Conn, sessionID uint64, flags uint16) {
	if flags&helloFlagGameplayCompactV1 == 0 {
		_ = conn.Close()
		return
	}
	s.mu.Lock()
	sess := s.sessions[sessionID]
	if sess == nil {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	viewer := &relayViewer{conn: conn}
	cfg := sess.cfg
	var (
		keyframe []byte
		tic      uint32
		ok       bool
		backlog  []bufferedFrame
	)
	viewer.mu.Lock()
	sess.viewers[viewer] = struct{}{}
	if len(sess.lastKeyframe) > 0 {
		keyframe = append([]byte(nil), sess.lastKeyframe...)
		tic = sess.lastKeyframeTic
		ok = true
	}
	if len(sess.backlog) > 0 {
		backlog = make([]bufferedFrame, len(sess.backlog))
		for i, frame := range sess.backlog {
			backlog[i] = bufferedFrame{
				header:  frame.header,
				payload: append([]byte(nil), frame.payload...),
			}
		}
	}
	s.mu.Unlock()

	if err := writeHello(conn, helloRoleBroadcaster, helloFlagGameplayCompactV1, sessionID, cfg); err != nil {
		viewer.mu.Unlock()
		s.removeViewer(sess, viewer)
		_ = conn.Close()
		return
	}
	if ok {
		if err := writeFrame(conn, frameHeader{
			Type:   frameTypeKeyframe,
			Flags:  sess.lastKeyframeFlags,
			Length: uint32(len(keyframe)),
			Tic:    tic,
		}, keyframe); err != nil {
			viewer.mu.Unlock()
			s.removeViewer(sess, viewer)
			_ = conn.Close()
			return
		}
	}
	for _, frame := range backlog {
		if err := writeFrame(conn, frame.header, frame.payload); err != nil {
			viewer.mu.Unlock()
			s.removeViewer(sess, viewer)
			_ = conn.Close()
			return
		}
	}
	viewer.mu.Unlock()

	// Viewers can send chat messages.
	for {
		header, payload, err := readFrame(conn)
		if err != nil {
			s.removeViewer(sess, viewer)
			_ = conn.Close()
			return
		}
		if header.Type == frameTypeChat {
			s.forwardChat(sess, viewer, header, payload)
		}
	}
}

func (s *Server) handleAudioViewer(conn net.Conn, sessionID uint64, flags uint16) {
	if flags&helloFlagAudioCompactV1 == 0 {
		_ = conn.Close()
		return
	}
	s.mu.Lock()
	sess := s.sessions[sessionID]
	if sess == nil {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	viewer := &relayViewer{conn: conn}
	cfg := sess.cfg
	var (
		audioFormat []byte
	)
	viewer.mu.Lock()
	sess.audioViewers[viewer] = struct{}{}
	if len(sess.lastAudioFormat) > 0 {
		audioFormat = append([]byte(nil), sess.lastAudioFormat...)
	}
	s.mu.Unlock()

	if err := writeHello(conn, helloRoleAudioBroadcaster, helloFlagAudioCompactV1, sessionID, cfg); err != nil {
		viewer.mu.Unlock()
		s.removeAudioViewer(sess, viewer)
		_ = conn.Close()
		return
	}
	if len(audioFormat) > 0 {
		if err := writeAudioFormatRecord(conn, audioFormat); err != nil {
			viewer.mu.Unlock()
			s.removeAudioViewer(sess, viewer)
			_ = conn.Close()
			return
		}
	}
	viewer.mu.Unlock()

	var one [1]byte
	for {
		_, err := conn.Read(one[:])
		if err != nil {
			s.removeAudioViewer(sess, viewer)
			_ = conn.Close()
			return
		}
	}
}

func (s *Server) forwardFrame(sess *relaySession, header frameHeader, payload []byte) {
	s.mu.Lock()
	if header.Type == frameTypeKeyframe {
		sess.lastKeyframeTic = header.Tic
		sess.lastKeyframeFlags = header.Flags
		sess.lastKeyframe = append(sess.lastKeyframe[:0], payload...)
		sess.backlog = sess.backlog[:0]
		if header.Flags&keyframeFlagMandatoryApply == 0 {
			s.mu.Unlock()
			return
		}
	} else if header.Type == frameTypeTicBatch {
		sess.backlog = append(sess.backlog, bufferedFrame{
			header:  header,
			payload: append([]byte(nil), payload...),
		})
		if len(sess.backlog) > maxBufferedRelayFrames {
			copy(sess.backlog, sess.backlog[len(sess.backlog)-maxBufferedRelayFrames:])
			sess.backlog = sess.backlog[:maxBufferedRelayFrames]
		}
	}
	viewers := make([]*relayViewer, 0, len(sess.viewers))
	for viewer := range sess.viewers {
		viewers = append(viewers, viewer)
	}
	s.mu.Unlock()

	for _, viewer := range viewers {
		if err := viewer.writeFrame(header, payload); err != nil {
			s.removeViewer(sess, viewer)
			_ = viewer.conn.Close()
		}
	}
}

func (s *Server) forwardAudioFormat(sess *relaySession, format AudioFormat) {
	payload, err := marshalAudioFormat(format)
	if err != nil {
		return
	}
	s.mu.Lock()
	sess.lastAudioFormat = append(sess.lastAudioFormat[:0], payload...)
	viewers := make([]*relayViewer, 0, len(sess.audioViewers))
	for viewer := range sess.audioViewers {
		viewers = append(viewers, viewer)
	}
	s.mu.Unlock()

	for _, viewer := range viewers {
		if err := viewer.writeAudioFormat(payload); err != nil {
			s.removeAudioViewer(sess, viewer)
			_ = viewer.conn.Close()
		}
	}
}

func (s *Server) forwardAudioChunk(sess *relaySession, format AudioFormat, chunk AudioChunk) {
	s.mu.Lock()
	viewers := make([]*relayViewer, 0, len(sess.audioViewers))
	for viewer := range sess.audioViewers {
		viewers = append(viewers, viewer)
	}
	s.mu.Unlock()

	for _, viewer := range viewers {
		if err := viewer.writeAudioChunk(format, chunk); err != nil {
			s.removeAudioViewer(sess, viewer)
			_ = viewer.conn.Close()
		}
	}
}

// forwardChat relays a chat frame to all session members, including the sender.
func (s *Server) forwardChat(sess *relaySession, sender *relayViewer, header frameHeader, payload []byte) {
	if sender != nil {
		msg, err := readChatFrame(bytes.NewReader(payload))
		if err != nil || !sender.allowChatMessage(msg, time.Now()) {
			return
		}
	}
	s.mu.Lock()
	all := make([]*relayViewer, 0, 1+len(sess.viewers))
	all = append(all, sess.srcViewer)
	for v := range sess.viewers {
		all = append(all, v)
	}
	s.mu.Unlock()

	for _, v := range all {
		if err := v.writeChat(header, payload); err != nil {
			if v == sess.srcViewer {
				s.closeSession(sess)
			} else {
				s.removeViewer(sess, v)
				_ = v.conn.Close()
			}
		}
	}
}

func (v *relayViewer) allowChatMessage(msg ChatMessage, now time.Time) bool {
	if v == nil {
		return false
	}
	text := strings.TrimSpace(strings.Join(strings.Fields(msg.Text), " "))
	if text == "" {
		return false
	}
	v.chatMu.Lock()
	defer v.chatMu.Unlock()
	for _, recent := range v.chatRecent {
		if recent == text {
			return false
		}
	}
	cutoff := now.Add(-relayChatThrottleWindow)
	kept := v.chatTimes[:0]
	for _, ts := range v.chatTimes {
		if !ts.Before(cutoff) {
			kept = append(kept, ts)
		}
	}
	v.chatTimes = kept
	if len(v.chatTimes) >= relayChatThrottleBurst {
		return false
	}
	v.chatTimes = append(v.chatTimes, now)
	v.chatRecent = append(v.chatRecent, text)
	if len(v.chatRecent) > relayChatRecentRejectLines {
		v.chatRecent = append([]string(nil), v.chatRecent[len(v.chatRecent)-relayChatRecentRejectLines:]...)
	}
	return true
}

func (v *relayViewer) writeChat(header frameHeader, payload []byte) error {
	return v.writeFrame(header, payload)
}

func (v *relayViewer) writeFrame(header frameHeader, payload []byte) error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return writeFrame(v.conn, header, payload)
}

func (v *relayViewer) writeAudioFormat(payload []byte) error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return writeAudioFormatRecord(v.conn, payload)
}

func (v *relayViewer) writeAudioChunk(format AudioFormat, chunk AudioChunk) error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return writeAudioChunkRecord(v.conn, format, chunk)
}

func (s *Server) latestKeyframe(sess *relaySession) ([]byte, uint32, bool) {
	if s == nil || sess == nil {
		return nil, 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur := s.sessions[sess.id]; cur != sess || len(sess.lastKeyframe) == 0 {
		return nil, 0, false
	}
	return append([]byte(nil), sess.lastKeyframe...), sess.lastKeyframeTic, true
}

func (s *Server) backlogFrames(sess *relaySession) []bufferedFrame {
	if s == nil || sess == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur := s.sessions[sess.id]; cur != sess || len(sess.backlog) == 0 {
		return nil
	}
	out := make([]bufferedFrame, len(sess.backlog))
	for i, frame := range sess.backlog {
		out[i] = bufferedFrame{
			header:  frame.header,
			payload: append([]byte(nil), frame.payload...),
		}
	}
	return out
}

func (s *Server) removeViewer(sess *relaySession, viewer *relayViewer) {
	if s == nil || sess == nil || viewer == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur := s.sessions[sess.id]; cur == sess {
		delete(sess.viewers, viewer)
	}
}

func (s *Server) removeAudioViewer(sess *relaySession, viewer *relayViewer) {
	if s == nil || sess == nil || viewer == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur := s.sessions[sess.id]; cur == sess {
		delete(sess.audioViewers, viewer)
	}
}

func (s *Server) closeAudioSource(sess *relaySession) {
	if s == nil || sess == nil {
		return
	}
	s.mu.Lock()
	cur := s.sessions[sess.id]
	if cur != sess || sess.audioSrc == nil {
		s.mu.Unlock()
		return
	}
	src := sess.audioSrc
	sess.audioSrc = nil
	viewers := make([]*relayViewer, 0, len(sess.audioViewers))
	for viewer := range sess.audioViewers {
		viewers = append(viewers, viewer)
	}
	sess.audioViewers = make(map[*relayViewer]struct{})
	sess.lastAudioFormat = nil
	s.mu.Unlock()

	if src != nil {
		_ = src.Close()
	}
	for _, viewer := range viewers {
		_ = viewer.conn.Close()
	}
}

func (s *Server) closeSession(sess *relaySession) {
	if s == nil || sess == nil {
		return
	}
	s.mu.Lock()
	cur := s.sessions[sess.id]
	if cur != sess {
		s.mu.Unlock()
		return
	}
	delete(s.sessions, sess.id)
	viewers := make([]*relayViewer, 0, len(sess.viewers))
	for viewer := range sess.viewers {
		viewers = append(viewers, viewer)
	}
	audioViewers := make([]*relayViewer, 0, len(sess.audioViewers))
	for viewer := range sess.audioViewers {
		audioViewers = append(audioViewers, viewer)
	}
	src := sess.src
	audioSrc := sess.audioSrc
	s.mu.Unlock()

	if src != nil {
		_ = src.Close()
	}
	if audioSrc != nil {
		_ = audioSrc.Close()
	}
	for _, viewer := range viewers {
		_ = viewer.conn.Close()
	}
	for _, viewer := range audioViewers {
		_ = viewer.conn.Close()
	}
}
