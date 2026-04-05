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
	audioSrc          net.Conn
	viewers           map[*relayViewer]struct{}
	audioViewers      map[*relayViewer]struct{}
	lastKeyframe      []byte
	lastKeyframeFlags byte
	lastKeyframeTic   uint32
	backlog           []bufferedFrame
	lastAudioConfig   []byte
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
	role, _, sessionID, sessionCfg, err := readHello(conn)
	if err != nil {
		_ = conn.Close()
		return
	}
	switch role {
	case helloRoleBroadcaster:
		s.handleBroadcaster(conn, sessionID, sessionCfg)
	case helloRoleAudioBroadcaster:
		if sessionID == 0 {
			_ = conn.Close()
			return
		}
		s.handleAudioBroadcaster(conn, sessionID)
	case helloRoleViewer:
		if sessionID == 0 {
			_ = conn.Close()
			return
		}
		s.handleViewer(conn, sessionID)
	case helloRoleAudioViewer:
		if sessionID == 0 {
			_ = conn.Close()
			return
		}
		s.handleAudioViewer(conn, sessionID)
	default:
		_ = conn.Close()
	}
}

func (s *Server) handleBroadcaster(conn net.Conn, sessionID uint64, cfg SessionConfig) {
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
		id:      sessionID,
		cfg:     cfg,
		server:  s,
		src:     conn,
		viewers: make(map[*relayViewer]struct{}),
		audioViewers: make(map[*relayViewer]struct{}),
	}
	s.sessions[sessionID] = sess
	s.mu.Unlock()
	defer s.closeSession(sess)
	if err := writeHello(conn, helloRoleServer, 0, sessionID, cfg); err != nil {
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
		s.forwardFrame(sess, header, payload)
	}
}

func (s *Server) handleAudioBroadcaster(conn net.Conn, sessionID uint64) {
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
	if err := writeHello(conn, helloRoleServer, 0, sessionID, cfg); err != nil {
		return
	}
	for {
		header, payload, err := readFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// ignore: audio source teardown handles disconnection
			}
			return
		}
		switch header.Type {
		case frameTypeAudioConfig, frameTypeAudioChunk:
			s.forwardAudioFrame(sess, header, payload)
		default:
			return
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

func (s *Server) handleViewer(conn net.Conn, sessionID uint64) {
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

	if err := writeHello(conn, helloRoleBroadcaster, 0, sessionID, cfg); err != nil {
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

	// Viewers are currently read-only and do not send steady-state frames.
	var one [1]byte
	for {
		_, err := conn.Read(one[:])
		if err != nil {
			s.removeViewer(sess, viewer)
			_ = conn.Close()
			return
		}
	}
}

func (s *Server) handleAudioViewer(conn net.Conn, sessionID uint64) {
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
		audioConfig []byte
	)
	viewer.mu.Lock()
	sess.audioViewers[viewer] = struct{}{}
	if len(sess.lastAudioConfig) > 0 {
		audioConfig = append([]byte(nil), sess.lastAudioConfig...)
	}
	s.mu.Unlock()

	if err := writeHello(conn, helloRoleAudioBroadcaster, 0, sessionID, cfg); err != nil {
		viewer.mu.Unlock()
		s.removeAudioViewer(sess, viewer)
		_ = conn.Close()
		return
	}
	if len(audioConfig) > 0 {
		if err := writeFrame(conn, frameHeader{
			Type:   frameTypeAudioConfig,
			Length: uint32(len(audioConfig)),
		}, audioConfig); err != nil {
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

func (s *Server) forwardAudioFrame(sess *relaySession, header frameHeader, payload []byte) {
	s.mu.Lock()
	switch header.Type {
	case frameTypeAudioConfig:
		sess.lastAudioConfig = append(sess.lastAudioConfig[:0], payload...)
	}
	viewers := make([]*relayViewer, 0, len(sess.audioViewers))
	for viewer := range sess.audioViewers {
		viewers = append(viewers, viewer)
	}
	s.mu.Unlock()

	for _, viewer := range viewers {
		if err := viewer.writeFrame(header, payload); err != nil {
			s.removeAudioViewer(sess, viewer)
			_ = viewer.conn.Close()
		}
	}
}

func (v *relayViewer) writeFrame(header frameHeader, payload []byte) error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return writeFrame(v.conn, header, payload)
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
	sess.lastAudioConfig = nil
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
