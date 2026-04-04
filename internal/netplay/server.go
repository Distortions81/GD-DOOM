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
	id      uint64
	cfg     SessionConfig
	server  *Server
	src     net.Conn
	viewers map[*relayViewer]struct{}
}

type relayViewer struct {
	conn net.Conn
}

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
	case helloRoleViewer:
		if sessionID == 0 {
			_ = conn.Close()
			return
		}
		s.handleViewer(conn, sessionID)
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
	sess.viewers[viewer] = struct{}{}
	cfg := sess.cfg
	s.mu.Unlock()

	if err := writeHello(conn, helloRoleBroadcaster, 0, sessionID, cfg); err != nil {
		s.removeViewer(sess, viewer)
		_ = conn.Close()
		return
	}

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

func (s *Server) forwardFrame(sess *relaySession, header frameHeader, payload []byte) {
	s.mu.Lock()
	viewers := make([]*relayViewer, 0, len(sess.viewers))
	for viewer := range sess.viewers {
		viewers = append(viewers, viewer)
	}
	s.mu.Unlock()

	for _, viewer := range viewers {
		if err := writeFrame(viewer.conn, header, payload); err != nil {
			s.removeViewer(sess, viewer)
			_ = viewer.conn.Close()
		}
	}
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
	src := sess.src
	s.mu.Unlock()

	if src != nil {
		_ = src.Close()
	}
	for _, viewer := range viewers {
		_ = viewer.conn.Close()
	}
}
