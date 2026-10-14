package netgame

import "context"

// Connection admission is shared by all transports, including incomplete
// handshakes. The lifecycle lock prevents HTTP handler WaitGroup additions from
// racing Serve's final Wait after the TCP accept goroutine has stopped.
func (s *Server) startConnections(ctx context.Context) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.serveContext = ctx
	s.accepting = true
}

func (s *Server) stopConnections() {
	s.lifecycleMu.Lock()
	s.accepting = false
	s.lifecycleMu.Unlock()
}

func (s *Server) reserveConnection() (context.Context, bool) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if !s.accepting || s.serveContext.Err() != nil {
		return nil, false
	}
	select {
	case s.connectionSlots <- struct{}{}:
		s.wg.Add(1)
		return s.serveContext, true
	default:
		return nil, false
	}
}

func (s *Server) releaseConnection() {
	<-s.connectionSlots
	s.wg.Done()
}
