package netplay

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
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
	lastKeyframe      []byte
	lastKeyframeFlags byte
	lastKeyframeTic   uint32
	backlog           []bufferedFrame
	lastAudioFormat   []byte
}

type relayViewer struct {
	conn net.Conn
	mu   sync.Mutex
}

type bufferedFrame struct {
	header  frameHeader
	payload []byte
}

const maxBufferedRelayFrames = 35 * 60 * 5

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

// forwardChat relays a chat frame to all session members except the sender.
func (s *Server) forwardChat(sess *relaySession, sender *relayViewer, header frameHeader, payload []byte) {
	s.mu.Lock()
	all := make([]*relayViewer, 0, 1+len(sess.viewers))
	if sess.srcViewer != sender {
		all = append(all, sess.srcViewer)
	}
	for v := range sess.viewers {
		if v != sender {
			all = append(all, v)
		}
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
