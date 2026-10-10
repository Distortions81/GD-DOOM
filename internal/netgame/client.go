package netgame

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// MessageTransport provides the same game messages over native streams,
// WebSockets, or datagrams with their reliable control channel. Close must
// interrupt blocked reads and writes. Implementations serialize each direction.
type MessageTransport interface {
	ReadMessage() (any, error)
	WriteMessage(any) error
	Close() error
}

type Client struct {
	transport    MessageTransport
	welcome      Welcome
	wireWelcome  Welcome
	transitions  chan MapChange
	snapshots    chan Snapshot
	outgoing     chan InputBatch
	chatOutgoing chan ChatSay
	chats        chan ChatEvent
	leaving      chan clientLeaveRequest
	lastChatID   uint64
	following    chan FollowPlayer
	done         chan struct{}
	once         sync.Once
	wg           sync.WaitGroup
	mu           sync.Mutex
	err          error
	stopContext  func() bool
	latency      latencyProbe
	unreliable   bool
}

// Connect performs the reliable handshake, then moves all network IO off the
// caller's game loop. Queues hold only the latest full baseline/input bundle.
func Connect(ctx context.Context, transport MessageTransport, hello Hello) (*Client, error) {
	if transport == nil {
		return nil, errors.New("missing multiplayer transport")
	}
	stop := context.AfterFunc(ctx, func() { _ = transport.Close() })
	fail := func(err error) (*Client, error) { stop(); transport.Close(); return nil, err }
	handshakeStarted := time.Now()
	if err := transport.WriteMessage(hello); err != nil {
		return fail(err)
	}
	message, err := transport.ReadMessage()
	if err != nil {
		return fail(err)
	}
	if refusal, ok := message.(Disconnect); ok {
		return fail(fmt.Errorf("server refused multiplayer connection: %w", &ServerDisconnectError{Reason: refusal.Reason}))
	}
	welcome, ok := message.(Welcome)
	if !ok {
		return fail(ErrProtocol)
	}
	if _, err := validateMessage(welcome); err != nil {
		return fail(err)
	}
	if deadlines, ok := transport.(interface{ SetDeadline(time.Time) error }); ok {
		_ = deadlines.SetDeadline(time.Time{})
	}
	c := &Client{transport: transport, welcome: welcome, wireWelcome: welcome, transitions: make(chan MapChange, 4), snapshots: make(chan Snapshot, 1), outgoing: make(chan InputBatch, 1), done: make(chan struct{}), stopContext: stop, latency: latencyProbe{roundTrip: clampRoundTrip(time.Since(handshakeStarted))}}
	c.chatOutgoing, c.chats, c.leaving = make(chan ChatSay, 4), make(chan ChatEvent, 64), make(chan clientLeaveRequest, 1)
	c.following = make(chan FollowPlayer, 4)
	if datagrams, ok := transport.(DatagramTransport); ok {
		c.unreliable = datagrams.UnreliableSnapshots()
	}
	c.wg.Add(2)
	go c.readLoop()
	go c.writeLoop()
	return c, nil
}

// DialTCP is the native stream path. Production Internet deployments should
// use a TLS-protected adapter; plain TCP is useful for LAN and initial testing.
func DialTCP(ctx context.Context, address string, hello Hello) (*Client, error) {
	transport, err := OpenTCP(ctx, address)
	if err != nil {
		return nil, err
	}
	return Connect(ctx, transport, hello)
}

// DialTLS authenticates the server using the system certificate roots. The
// hostname in address is verified; certificate verification is never skipped.
func DialTLS(ctx context.Context, address string, hello Hello) (*Client, error) {
	transport, err := OpenTLS(ctx, address)
	if err != nil {
		return nil, err
	}
	return Connect(ctx, transport, hello)
}

func OpenTCP(ctx context.Context, address string) (MessageTransport, error) {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	return openStream(conn), nil
}

func OpenTLS(ctx context.Context, address string) (MessageTransport, error) {
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second}, Config: &tls.Config{MinVersion: tls.VersionTLS12}}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	return openStream(conn), nil
}

func openStream(conn net.Conn) MessageTransport {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	return &streamTransport{conn}
}

type streamTransport struct{ net.Conn }

func (s *streamTransport) ReadMessage() (any, error) { return ReadMessage(s.Conn) }
func (s *streamTransport) WriteMessage(message any) error {
	_ = s.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return writeStreamMessage(s.Conn, message)
}

func (c *Client) Welcome() Welcome { c.mu.Lock(); defer c.mu.Unlock(); return c.welcome }

// SendInputs never waits on a socket. Callers provide their recent command
// history in each bundle so replacing an unsent bundle preserves redundancy.
func (c *Client) SendInputs(batch InputBatch) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if batch.Epoch != c.welcome.Epoch {
		return ErrSessionEpoch
	}
	if _, err := validateMessage(batch); err != nil {
		return err
	}
	select {
	case <-c.done:
		return c.err
	default:
	}
	// A reliable map change may have arrived between game-loop polls. Never
	// transmit old-map intent while the caller is still loading its new map.
	if batch.Epoch != c.wireWelcome.Epoch {
		return nil
	}
	batch.Inputs = append([]Input(nil), batch.Inputs...)
	select {
	case c.outgoing <- batch:
		return nil
	default:
	}
	select {
	case <-c.outgoing:
	default:
	}
	select {
	case c.outgoing <- batch:
		return nil
	case <-c.done:
		return c.err
	default:
		return nil
	}
}

func (c *Client) PollSnapshot() (Snapshot, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.transitions) != 0 {
		return Snapshot{}, false, nil
	}
	select {
	case snapshot := <-c.snapshots:
		return snapshot, true, nil
	default:
	}
	select {
	case <-c.done:
		return Snapshot{}, false, c.err
	default:
		return Snapshot{}, false, nil
	}
}

// PollTransition publishes the next epoch to the game loop. Callers must load
// that map before polling snapshots or predicting input. Keeping this separate
// from the network's current epoch prevents an asynchronous map change from
// applying a new-map baseline to an old-map renderer.
func (c *Client) PollTransition() (MapChange, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case change := <-c.transitions:
		c.welcome = change.Welcome
		return change, true, nil
	default:
	}
	// Let PollSnapshot deliver the final baseline before the terminal error.
	return MapChange{}, false, nil
}

func (c *Client) Err() error   { c.mu.Lock(); defer c.mu.Unlock(); return c.err }
func (c *Client) Close() error { c.fail(net.ErrClosed); c.stopContext(); c.wg.Wait(); return nil }

func (c *Client) fail(err error) {
	c.once.Do(func() { c.mu.Lock(); c.err = err; c.mu.Unlock(); close(c.done); _ = c.transport.Close() })
}

// Reliable control keeps even an empty spectator lobby alive. A blackholed
// socket must fail while the server's resume reservation still exists.
const serverSilenceTimeout = 5 * time.Second

func (c *Client) readLoop() {
	defer c.wg.Done()
	decoder, err := NewSnapshotDecoder()
	if err != nil {
		c.fail(err)
		return
	}
	defer decoder.Close()
	var latest uint32
	for {
		if deadlines, ok := c.transport.(interface{ SetReadDeadline(time.Time) error }); ok {
			if err := deadlines.SetReadDeadline(time.Now().Add(serverSilenceTimeout)); err != nil {
				c.fail(err)
				return
			}
		}
		message, err := c.transport.ReadMessage()
		if err != nil {
			c.fail(err)
			return
		}
		if pong, ok := message.(Pong); ok {
			if _, err := validateMessage(pong); err != nil {
				c.fail(err)
				return
			}
			c.mu.Lock()
			c.latency.receive(pong, time.Now())
			c.mu.Unlock()
			continue
		}
		if chat, ok := message.(ChatEvent); ok {
			if err := c.receiveChat(chat); err != nil {
				c.fail(err)
				return
			}
			continue
		}
		if end, ok := message.(Disconnect); ok {
			c.fail(&ServerDisconnectError{Reason: end.Reason})
			return
		}
		if change, ok := message.(MapChange); ok {
			if _, err := validateMessage(change); err != nil {
				c.fail(err)
				return
			}
			c.mu.Lock()
			valid := change.PreviousEpoch == c.wireWelcome.Epoch && change.Welcome.PlayerID == c.wireWelcome.PlayerID && change.Welcome.Epoch > c.wireWelcome.Epoch
			if !valid || len(c.transitions) == cap(c.transitions) {
				c.mu.Unlock()
				c.fail(ErrProtocol)
				return
			}
			c.wireWelcome = change.Welcome
			select {
			case <-c.snapshots:
			default:
			}
			select {
			case <-c.outgoing:
			default:
			}
			c.transitions <- change
			latest = 0
			c.mu.Unlock()
			continue
		}
		snapshot, ok := message.(Snapshot)
		c.mu.Lock()
		wireEpoch := c.wireWelcome.Epoch
		c.mu.Unlock()
		if ok && c.unreliable && snapshot.Epoch != wireEpoch {
			continue
		}
		if !ok || snapshot.Epoch != wireEpoch {
			c.fail(ErrProtocol)
			return
		}
		if _, err := validateMessage(snapshot); err != nil {
			c.fail(err)
			return
		}
		if snapshot.ID <= latest {
			continue
		}
		// Decode and retain every received baseline before coalescing frames for
		// the game loop. A subsequent delta may reference a successfully decoded
		// frame even when rendering has already moved on to a newer one.
		snapshot, err = decoder.Decode(snapshot)
		if err != nil {
			if c.unreliable && errors.Is(err, ErrSnapshotBaseline) {
				// An empty acknowledgment requests an independent full frame.
				// The server sends it reliably without pausing the simulation.
				_ = c.SendInputs(InputBatch{Epoch: c.Welcome().Epoch})
				continue
			}
			c.fail(err)
			return
		}
		latest = snapshot.ID
		c.mu.Lock()
		select {
		case c.snapshots <- snapshot:
			c.mu.Unlock()
			continue
		default:
		}
		select {
		case <-c.snapshots:
		default:
		}
		select {
		case c.snapshots <- snapshot:
		case <-c.done:
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}
}

func (c *Client) writeLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case request := <-c.leaving:
			c.writeLeave(request)
			return
		default:
		}
		select {
		case <-c.done:
			return
		case request := <-c.leaving:
			c.writeLeave(request)
			return
		case say := <-c.chatOutgoing:
			if err := c.transport.WriteMessage(say); err != nil {
				c.fail(err)
				return
			}
		case follow := <-c.following:
			if err := c.transport.WriteMessage(follow); err != nil {
				c.fail(err)
				return
			}
		case <-ticker.C:
			c.mu.Lock()
			ping, send := c.latency.begin(time.Now())
			c.mu.Unlock()
			if send {
				if err := c.transport.WriteMessage(ping); err != nil {
					c.fail(err)
					return
				}
			}
		case batch := <-c.outgoing:
			c.mu.Lock()
			current := batch.Epoch == c.wireWelcome.Epoch
			c.mu.Unlock()
			if !current {
				continue
			}
			if err := c.transport.WriteMessage(batch); err != nil {
				c.fail(err)
				return
			}
		}
	}
}
