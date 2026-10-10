package netgame

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

var ErrServerOverloaded = errors.New("authoritative server cannot keep up with the simulation clock")

const maxQueuedPeerInputs = 8

// Server owns a Match. Its transport goroutines may read/write only connection
// queues; all simulation, roster and input-buffer operations run in Serve.
type Server struct {
	match           *Match
	joins           chan joinRequest
	queries         chan discoveryRequest
	chats           chan chatRequest
	follows         chan followRequest
	incoming        chan clientMessage
	leaves          chan clientLeave
	peers           map[ConnectionID]*streamPeer
	wg              sync.WaitGroup
	started         atomic.Bool
	transition      TransitionHandler
	lifecycleMu     sync.Mutex
	serveContext    context.Context
	accepting       bool
	connectionSlots chan struct{}
}

type joinRequest struct {
	hello Hello
	peer  *streamPeer
	reply chan joinReply
}
type joinReply struct {
	id      ConnectionID
	welcome Welcome
	err     error
}
type clientMessage struct {
	id    ConnectionID
	batch InputBatch
	peer  *streamPeer
}

type clientLeave struct {
	id        ConnectionID
	resumable bool
}
type streamPeer struct {
	conn        net.Conn
	snapshots   chan Snapshot
	controls    chan Pong
	final       chan finalMessage
	transitions chan transitionMessage
	inputSlots  chan struct{}
	chatSlots   chan struct{}
	chats       chan ChatEvent
	done        chan struct{}
	once        sync.Once
	finishing   atomic.Bool
	requestFull atomic.Bool
	snapshotAck atomic.Pointer[snapshotAcknowledgment]
}

type snapshotAcknowledgment struct {
	epoch uint64
	id    uint32
}

type finalMessage struct {
	snapshot Snapshot
	reason   string
}

func NewServer(match *Match) (*Server, error) {
	if match == nil || match.snapshots == nil {
		return nil, errors.New("server requires a match with authoritative snapshots")
	}
	return &Server{match: match, joins: make(chan joinRequest, 16), queries: make(chan discoveryRequest, 16), chats: make(chan chatRequest, 64), follows: make(chan followRequest, 32), incoming: make(chan clientMessage, 128), leaves: make(chan clientLeave, 32), peers: make(map[ConnectionID]*streamPeer), connectionSlots: make(chan struct{}, 32)}, nil
}

// Serve takes ownership of listener and runs until cancellation or a fatal
// simulation failure. Call it once. TCP here is the initial reliable transport;
// the game codec and match semantics are shared by future transport adapters.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if listener == nil {
		return errors.New("server requires a listener")
	}
	if !s.started.CompareAndSwap(false, true) {
		return errors.New("server Serve may only be called once")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer listener.Close()
	acceptErr := make(chan error, 1)
	s.wg.Add(1)
	s.startConnections(ctx)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case acceptErr <- err:
				default:
				}
				return
			}
			connectionContext, admitted := s.reserveConnection()
			if !admitted {
				conn.Close()
				continue
			}
			go func() { defer s.releaseConnection(); s.serveConnection(connectionContext, conn) }()
		}
	}()
	defer func() {
		s.stopConnections()
		cancel()
		listener.Close()
		for id, p := range s.peers {
			p.close()
			s.match.Leave(id)
		}
		for _, id := range s.match.orderedPlayers() {
			s.match.Leave(id)
		}
		s.wg.Wait()
	}()
	period := time.Second / TickRate
	next := time.Now().Add(period)
	timer := time.NewTimer(period)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-acceptErr:
			if ctx.Err() != nil {
				return nil
			}
			return err
		case request := <-s.queries:
			s.replyDiscovery(request)
		case request := <-s.chats:
			s.acceptChat(request, time.Now())
		case request := <-s.follows:
			s.acceptFollow(request)
		case request := <-s.joins:
			select {
			case <-request.peer.done:
				continue
			default:
			}
			id, welcome, err := s.match.Join(request.hello)
			if err == nil {
				// Resume replaces a prior transport identity before either its
				// queued inputs or its later leave notification can be observed.
				for old, peer := range s.peers {
					if s.match.players[old] == nil {
						delete(s.peers, old)
						peer.close()
					}
				}
				s.peers[id] = request.peer
			}
			request.reply <- joinReply{id: id, welcome: welcome, err: err}
		case message := <-s.incoming:
			s.acceptClientInput(message)
		case leave := <-s.leaves:
			if leave.resumable {
				s.suspend(leave.id)
			} else {
				s.drop(leave.id)
			}
		case <-timer.C:
			// The channel value is the scheduled firing time. Real dispatch can
			// be later after expensive work; debt must use the actual clock.
			now := time.Now()
			if s.match.PlayerCount() == 0 {
				next = now.Add(period)
			} else {
				// Execute every owed tic, but bound work per dispatch. Large debt
				// is a visible server failure, never silently skipped physics.
				if now.Sub(next) > time.Second {
					return ErrServerOverloaded
				}
				for count := 0; !now.Before(next) && count < 4; count++ {
					result, err := s.match.Step()
					if err != nil {
						return err
					}
					for _, id := range result.Dropped {
						s.suspend(id)
					}
					if result.Completed {
						advanced, err := s.advanceLevel()
						if err != nil {
							return err
						}
						if !advanced {
							return s.finish(ctx, result)
						}
						// Loading a map is a session transition, not simulation
						// debt to execute against the new level in one burst.
						next = time.Now().Add(period)
						break
					}
					for id, snapshot := range result.Snapshots {
						if p := s.peers[id]; p != nil {
							p.offer(snapshot)
						}
					}
					next = next.Add(period)
				}
			}
			delay := time.Until(next)
			if delay < 0 {
				delay = 0
			}
			timer.Reset(delay)
		}
	}
}

// finish gives each independent writer a terminal full snapshot and a reliable
// completion message. Closing sockets in Serve's cleanup before writers flush
// would lose the final score/map-exit state. A slow peer has a bounded drain.
func (s *Server) finish(ctx context.Context, result TickResult) error {
	drain, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for id, peer := range s.peers {
		peer.finishing.Store(true)
		snapshot, ok := result.Snapshots[id]
		if !ok && (s.match.players[id] == nil || !s.match.players[id].spectator) {
			peer.close()
			continue
		}
		select {
		case peer.final <- finalMessage{snapshot: snapshot, reason: result.CompletionReason}:
		case <-peer.done:
		case <-drain.Done():
			return nil
		}
	}
	for _, peer := range s.peers {
		select {
		case <-peer.done:
		case <-drain.Done():
			return nil
		}
	}
	return nil
}

func (s *Server) drop(id ConnectionID) {
	if p := s.peers[id]; p != nil {
		delete(s.peers, id)
		p.close()
	}
	s.match.Leave(id)
}

func (s *Server) acceptClientInput(message clientMessage) {
	if message.peer != nil {
		<-message.peer.inputSlots
	}
	if err := s.match.Submit(message.id, message.batch); err != nil {
		// A socket may have queued one last input before its leave or
		// replacement reached this owner. It cannot revoke a suspended
		// reservation or affect the new authenticated connection.
		if !errors.Is(err, ErrUnknownConnection) && !(errors.Is(err, ErrSessionEpoch) && message.batch.Epoch < s.match.Epoch()) {
			s.drop(message.id)
		}
	} else if peer := s.peers[message.id]; peer != nil {
		if message.batch.SnapshotAck == 0 && len(message.batch.Inputs) == 0 {
			peer.requestFull.Store(true)
		}
		// Publish one atomic epoch/ID pair only after the whole batch
		// passed owner validation. Writers still require a sent baseline.
		peer.snapshotAck.Store(&snapshotAcknowledgment{epoch: s.match.Epoch(), id: s.match.players[message.id].ackedSnapshot})
	}
}

func (s *Server) suspend(id ConnectionID) {
	if p := s.peers[id]; p != nil {
		delete(s.peers, id)
		p.close()
	}
	s.match.Suspend(id)
}

func (p *streamPeer) close() { p.once.Do(func() { close(p.done); _ = p.conn.Close() }) }

// offer coalesces unsent full snapshots only. Reliable setup is written before
// this queue is consumed. A delta transport must preserve acknowledged baselines.
func (p *streamPeer) offer(snapshot Snapshot) {
	select {
	case p.snapshots <- snapshot:
		return
	default:
	}
	select {
	case <-p.snapshots:
	default:
	}
	select {
	case p.snapshots <- snapshot:
	default:
	}
}

func (s *Server) serveConnection(ctx context.Context, conn net.Conn) {
	p := &streamPeer{conn: conn, snapshots: make(chan Snapshot, 1), controls: make(chan Pong, 4), chats: make(chan ChatEvent, 32), chatSlots: make(chan struct{}, 4), final: make(chan finalMessage, 1), transitions: make(chan transitionMessage, 4), inputSlots: make(chan struct{}, maxQueuedPeerInputs), done: make(chan struct{})}
	defer p.close()
	// Cancellation must interrupt reads, writes and pending handshakes.
	stop := context.AfterFunc(ctx, p.close)
	defer stop()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	first, err := readPeerClientMessage(conn)
	if err != nil {
		return
	}
	if _, ok := first.(Query); ok {
		s.serveDiscovery(ctx, conn, false)
		return
	}
	if _, ok := first.(StatusQuery); ok {
		s.serveDiscovery(ctx, conn, true)
		return
	}
	hello, ok := first.(Hello)
	if !ok {
		return
	}
	request := joinRequest{hello: hello, peer: p, reply: make(chan joinReply, 1)}
	select {
	case s.joins <- request:
	case <-ctx.Done():
		return
	case <-p.done:
		return
	}
	var reply joinReply
	select {
	case reply = <-request.reply:
	case <-ctx.Done():
		return
	}
	if reply.err != nil {
		_ = writeStreamMessage(conn, Disconnect{Reason: reply.err.Error()})
		return
	}
	resumable := true
	defer func() {
		select {
		case s.leaves <- clientLeave{id: reply.id, resumable: resumable}:
		case <-ctx.Done():
		}
	}()
	if err := writeStreamMessage(conn, reply.welcome); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer p.close()
		encoder, err := NewSnapshotEncoder()
		if err != nil {
			return
		}
		defer encoder.Close()
		writerEpoch := reply.welcome.Epoch
		writeTransition := func(change transitionMessage) bool {
			if change.control.PreviousEpoch != writerEpoch {
				return false
			}
			if err := p.writeTransition(change, encoder); err != nil {
				return false
			}
			writerEpoch = change.control.Welcome.Epoch
			return true
		}
		writeFinal := func(final finalMessage) {
			// A very short new round can complete before this writer drains
			// its map change. Reliable controls must still precede that final.
			for writerEpoch != final.snapshot.Epoch {
				if writerEpoch > final.snapshot.Epoch {
					return
				}
				select {
				case change := <-p.transitions:
					if !writeTransition(change) {
						return
					}
				default:
					return
				}
			}
			p.writeFinal(final, encoder)
		}
		for {
			// Prioritize completion over replaceable snapshots already queued.
			select {
			case final := <-p.final:
				writeFinal(final)
				return
			case change := <-p.transitions:
				if !writeTransition(change) {
					return
				}
				continue
			default:
			}
			select {
			case <-p.done:
				return
			case final := <-p.final:
				writeFinal(final)
				return
			case change := <-p.transitions:
				if !writeTransition(change) {
					return
				}
			case pong := <-p.controls:
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := writeStreamMessage(conn, pong); err != nil {
					return
				}
			case chat := <-p.chats:
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := writeStreamMessage(conn, chat); err != nil {
					return
				}
			case snapshot := <-p.snapshots:
				if snapshot.Epoch != writerEpoch {
					// The reliable transition includes its own baseline. Never
					// publish a new-epoch replaceable snapshot ahead of control.
					continue
				}
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := p.writeSnapshot(encoder, snapshot); err != nil {
					return
				}
			}
		}
	}()
	defer func() { p.close(); <-writerDone }()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		message, err := readPeerClientMessage(conn)
		if err != nil {
			if errors.Is(err, ErrProtocol) {
				resumable = false
			}
			return
		}
		if _, ok := message.(Disconnect); ok {
			resumable = false
			return
		}
		if say, ok := message.(ChatSay); ok {
			select {
			case p.chatSlots <- struct{}{}:
			default:
				resumable = false
				return
			}
			select {
			case s.chats <- chatRequest{id: reply.id, say: say, peer: p}:
			case <-ctx.Done():
				<-p.chatSlots
				return
			case <-p.done:
				<-p.chatSlots
				return
			}
			continue
		}
		if follow, ok := message.(FollowPlayer); ok {
			select {
			case p.inputSlots <- struct{}{}:
			default:
				resumable = false
				return
			}
			select {
			case s.follows <- followRequest{id: reply.id, target: follow.PlayerID, peer: p}:
			case <-ctx.Done():
				<-p.inputSlots
				return
			case <-p.done:
				<-p.inputSlots
				return
			}
			continue
		}
		if ping, ok := message.(Ping); ok {
			// Latency probes neither enter the simulation owner nor refresh
			// gameplay activity. A peer cannot grow the reliable writer queue.
			select {
			case p.controls <- Pong{Nonce: ping.Nonce}:
			default:
				resumable = false
				return
			}
			continue
		}
		batch, ok := message.(InputBatch)
		if !ok {
			resumable = false
			return
		}
		if p.finishing.Load() {
			// Gameplay has ended. Drain outstanding client sends while the
			// independent writer delivers its final state and closes the socket.
			continue
		}
		// Bound each sender before entering the shared queue. A flood cannot
		// fill it and cause an unrelated, well-behaved player to be dropped.
		select {
		case p.inputSlots <- struct{}{}:
		default:
			resumable = false
			return
		}
		select {
		case s.incoming <- clientMessage{id: reply.id, batch: batch, peer: p}:
		case <-ctx.Done():
			<-p.inputSlots
			return
		case <-p.done:
			<-p.inputSlots
			return
		}
	}
}

func (p *streamPeer) writeFinal(final finalMessage, encoder *SnapshotEncoder) {
	// The owner has stopped accepting chat. Flush its already accepted lines
	// before the final baseline and termination control.
	for range len(p.chats) {
		chat := <-p.chats
		_ = p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := writeStreamMessage(p.conn, chat); err != nil {
			return
		}
	}
	_ = p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if final.snapshot.ID != 0 {
		if err := p.writeSnapshot(encoder, final.snapshot, true); err != nil {
			return
		}
	}
	reason := "Match complete"
	if final.reason != "" && utf8.ValidString(final.reason) && len(reason)+2+len(final.reason) <= 256 {
		reason += ": " + final.reason
	}
	_ = writeStreamMessage(p.conn, Disconnect{Reason: reason})
}

// writeSnapshot runs only on the connection writer; coalescing happens before
// encoding, and a failed write never enters the acknowledgeable history.
func (p *streamPeer) writeSnapshot(encoder *SnapshotEncoder, full Snapshot, reliable ...bool) error {
	var acknowledgedID uint32
	if ack := p.snapshotAck.Load(); ack != nil && ack.epoch == full.Epoch {
		acknowledgedID = ack.id
	}
	if p.requestFull.Swap(false) || (len(reliable) > 0 && reliable[0]) {
		acknowledgedID = 0
	}
	wire, err := encoder.Encode(full, acknowledgedID)
	if err != nil {
		return err
	}
	if err := writeStreamMessage(p.conn, wire); err != nil {
		return err
	}
	return encoder.Commit(full)
}

func writeStreamMessage(w io.Writer, message any) error {
	if transport, ok := w.(interface{ WriteMessage(any) error }); ok {
		return transport.WriteMessage(message)
	}
	data, err := MarshalMessage(message)
	if err != nil {
		return err
	}
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return fmt.Errorf("write multiplayer message: %w", io.ErrShortWrite)
		}
		data = data[n:]
	}
	return nil
}
