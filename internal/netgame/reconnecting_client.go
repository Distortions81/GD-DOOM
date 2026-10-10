package netgame

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ServerDisconnectError is an explicit server decision, not a dropped network
// path. Retrying it automatically could create a new player after match end.
type ServerDisconnectError struct{ Reason string }

func (e *ServerDisconnectError) Error() string { return "multiplayer session ended: " + e.Reason }

type ConnectionState string

const (
	ConnectionConnected    ConnectionState = "connected"
	ConnectionReconnecting ConnectionState = "reconnecting"
	ConnectionDisconnected ConnectionState = "disconnected"
	ConnectionComplete     ConnectionState = "complete"
)

type ConnectionStatus struct {
	State   ConnectionState
	Attempt int
	Message string
}
type ClientSessionChange struct {
	Generation uint64
	Welcome    Welcome
	Manifest   CompatibilityManifest
}

// ResumeConnector must query current server metadata, verify local content and
// rules, then connect using this exact token. It must honor cancellation. The
// context has no deadline because a successful client retains it for its whole
// lifetime; this wrapper separately bounds each opening attempt to five seconds.
type ResumeConnector func(context.Context, [32]byte) (*Client, CompatibilityManifest, error)

var ErrClientOffline = errors.New("multiplayer client is not connected")

type resumedConnection struct {
	client   *Client
	manifest CompatibilityManifest
	cancel   context.CancelFunc
}

// ReconnectingClient keeps all retry IO off the game loop. A resumed socket is
// withheld until PollSessionChange, so the game can discard old input history
// even when the server preserves the same map epoch and player body.
type ReconnectingClient struct {
	ctx          context.Context
	cancel       context.CancelFunc
	dial         ResumeConnector
	mu           sync.Mutex
	active       *Client
	activeCancel context.CancelFunc
	pending      *resumedConnection
	welcome      Welcome
	manifest     CompatibilityManifest
	generation   uint64
	status       ConnectionStatus
	err          error
	closed       bool
}

func NewReconnectingClient(ctx context.Context, initial *Client, manifest CompatibilityManifest, dial ResumeConnector) (*ReconnectingClient, error) {
	if initial == nil {
		return nil, errors.New("missing initial multiplayer client")
	}
	if _, err := manifest.Key(); err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	c := &ReconnectingClient{ctx: lifetime, cancel: cancel, dial: dial, active: initial, welcome: initial.Welcome(), manifest: cloneCompatibilityManifest(manifest), generation: 1, status: ConnectionStatus{State: ConnectionConnected}}
	context.AfterFunc(lifetime, func() { c.Close() })
	return c, nil
}

func (c *ReconnectingClient) Welcome() Welcome { c.mu.Lock(); defer c.mu.Unlock(); return c.welcome }
func (c *ReconnectingClient) Status() ConnectionStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}
func (c *ReconnectingClient) SessionGeneration() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}
func (c *ReconnectingClient) Err() error { c.mu.Lock(); defer c.mu.Unlock(); return c.err }
func (c *ReconnectingClient) current() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.status.State != ConnectionConnected {
		return nil
	}
	return c.active
}

func (c *ReconnectingClient) PollSessionChange() (ClientSessionChange, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.pending == nil {
		return ClientSessionChange{}, false
	}
	next := c.pending
	c.pending = nil
	c.active, c.activeCancel = next.client, next.cancel
	c.welcome, c.manifest = next.client.Welcome(), next.manifest
	c.generation++
	c.status, c.err = ConnectionStatus{State: ConnectionConnected}, nil
	return ClientSessionChange{c.generation, c.welcome, cloneCompatibilityManifest(c.manifest)}, true
}

func (c *ReconnectingClient) PollTransition() (MapChange, bool, error) {
	active := c.current()
	if active == nil {
		return MapChange{}, false, nil
	}
	change, ok, err := active.PollTransition()
	if ok {
		c.mu.Lock()
		c.welcome = change.Welcome
		c.manifest.Map = change.Map
		c.mu.Unlock()
	}
	return change, ok, err
}
func (c *ReconnectingClient) PollSnapshot() (Snapshot, bool, error) {
	active := c.current()
	if active == nil {
		return Snapshot{}, false, nil
	}
	snapshot, ok, err := active.PollSnapshot()
	if err != nil {
		c.connectionLost(active, err)
		return Snapshot{}, false, nil
	}
	return snapshot, ok, nil
}
func (c *ReconnectingClient) SendInputs(batch InputBatch) error {
	active := c.current()
	if active == nil {
		return nil
	} // Offline intent is never queued for a new session.
	err := active.SendInputs(batch)
	if errors.Is(err, ErrProtocol) || errors.Is(err, ErrSessionEpoch) {
		return err
	}
	// PollSnapshot owns terminal detection so a final queued baseline is always
	// delivered before a completion or reconnect status replaces the connection.
	return nil
}
func (c *ReconnectingClient) RoundTripTime() time.Duration {
	active := c.current()
	if active == nil {
		return 0
	}
	return active.RoundTripTime()
}

func (c *ReconnectingClient) FollowPlayer(id byte) error {
	active := c.current()
	if active == nil {
		return ErrClientOffline
	}
	return active.FollowPlayer(id)
}
func (c *ReconnectingClient) SendChat(text string) error {
	active := c.current()
	if active == nil {
		return ErrClientOffline
	}
	return active.SendChat(text)
}
func (c *ReconnectingClient) PollChat() (ChatEvent, bool, error) {
	active := c.current()
	if active == nil {
		return ChatEvent{}, false, nil
	}
	event, ok, err := active.PollChat()
	if err != nil {
		return ChatEvent{}, false, nil
	}
	return event, ok, nil
}

func terminalConnectionStatus(err error) (ConnectionStatus, bool) {
	var server *ServerDisconnectError
	if errors.As(err, &server) {
		if strings.HasPrefix(strings.ToLower(server.Reason), "match complete:") {
			return ConnectionStatus{State: ConnectionComplete, Message: server.Reason}, true
		}
		return ConnectionStatus{State: ConnectionDisconnected, Message: server.Reason}, true
	}
	if errors.Is(err, ErrProtocol) {
		return ConnectionStatus{State: ConnectionDisconnected, Message: "Invalid server protocol"}, true
	}
	return ConnectionStatus{}, false
}

func (c *ReconnectingClient) connectionLost(active *Client, err error) {
	c.mu.Lock()
	if c.closed || c.active != active || c.status.State != ConnectionConnected {
		c.mu.Unlock()
		return
	}
	c.err = err
	c.active = nil
	if c.activeCancel != nil {
		c.activeCancel()
		c.activeCancel = nil
	}
	welcome, manifest := c.welcome, cloneCompatibilityManifest(c.manifest)
	terminal, stop := terminalConnectionStatus(err)
	if !stop && (c.dial == nil || welcome.ResumeToken == ([32]byte{}) || welcome.ResumeGraceTicks == 0) {
		terminal, stop = ConnectionStatus{State: ConnectionDisconnected, Message: "Connection lost"}, true
	}
	if stop {
		c.status = terminal
	} else {
		c.status = ConnectionStatus{State: ConnectionReconnecting, Message: "Connection lost; reconnecting"}
	}
	c.mu.Unlock()
	go active.Close()
	if !stop {
		go c.reconnect(welcome, manifest)
	}
}

func (c *ReconnectingClient) reconnect(previous Welcome, manifest CompatibilityManifest) {
	grace := time.Duration(previous.ResumeGraceTicks) * time.Second / TickRate
	if grace > 30*time.Second {
		grace = 30 * time.Second
	}
	deadline := time.Now().Add(grace)
	var lastError error
	for attempt := 1; attempt <= 6; attempt++ {
		remaining := time.Until(deadline)
		if remaining <= 0 || c.ctx.Err() != nil {
			break
		}
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.status.Attempt = attempt
		c.mu.Unlock()
		attemptCtx, stop := context.WithCancel(c.ctx)
		type reply struct {
			client   *Client
			manifest CompatibilityManifest
			err      error
		}
		ready := make(chan reply, 1)
		go func() {
			client, next, err := c.dial(attemptCtx, previous.ResumeToken)
			ready <- reply{client, next, err}
		}()
		wait := remaining
		if wait > 5*time.Second {
			wait = 5 * time.Second
		}
		timer := time.NewTimer(wait)
		var result reply
		select {
		case result = <-ready:
			timer.Stop()
		case <-timer.C:
			stop()
			result.err = context.DeadlineExceeded
			go func() {
				late := <-ready
				if late.client != nil {
					late.client.Close()
				}
			}()
		case <-c.ctx.Done():
			timer.Stop()
			stop()
			go func() {
				late := <-ready
				if late.client != nil {
					late.client.Close()
				}
			}()
			return
		}
		if result.err == nil && result.client == nil {
			result.err = errors.New("resume connector returned no client")
		}
		if result.err == nil {
			welcome := result.client.Welcome()
			_, keyErr := result.manifest.Key()
			if keyErr != nil || welcome.PlayerID != previous.PlayerID || welcome.Epoch < previous.Epoch || welcome.Epoch == previous.Epoch && result.manifest.Map != manifest.Map {
				result.err = ErrProtocol
			}
		}
		if result.err == nil {
			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				stop()
				result.client.Close()
				return
			}
			c.pending = &resumedConnection{result.client, cloneCompatibilityManifest(result.manifest), stop}
			c.status.Message = "Restoring session"
			c.mu.Unlock()
			return
		}
		stop()
		if result.client != nil {
			result.client.Close()
		}
		lastError = result.err
		if terminal, permanent := terminalConnectionStatus(result.err); permanent {
			c.finishReconnect(terminal, result.err)
			return
		}
		delay := time.Duration(1<<min(attempt-1, 3)) * 250 * time.Millisecond
		if delay > time.Until(deadline) {
			break
		}
		timer = time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-c.ctx.Done():
			timer.Stop()
			return
		}
	}
	c.finishReconnect(ConnectionStatus{State: ConnectionDisconnected, Message: "Reconnect window expired"}, lastError)
}

func (c *ReconnectingClient) finishReconnect(status ConnectionStatus, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	status.Attempt = c.status.Attempt
	c.status, c.err = status, err
}

func (c *ReconnectingClient) Close() error { return c.close(false) }
func (c *ReconnectingClient) Leave() error { return c.close(true) }
func (c *ReconnectingClient) close(leave bool) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	active, pending, stop := c.active, c.pending, c.activeCancel
	c.active, c.pending, c.activeCancel = nil, nil, nil
	c.mu.Unlock()
	// Explicit leave gets a bounded opportunity to release the reserved slot.
	if leave && active != nil {
		_ = active.Leave()
	}
	if leave && pending != nil {
		_ = pending.client.Leave()
	}
	c.cancel()
	if stop != nil {
		stop()
	}
	if active != nil {
		active.Close()
	}
	if pending != nil {
		pending.cancel()
		pending.client.Close()
	}
	return nil
}

func (s ConnectionStatus) String() string {
	if s.State == ConnectionReconnecting && s.Attempt > 0 {
		return fmt.Sprintf("RECONNECTING (%d/6)", s.Attempt)
	}
	if s.Message != "" {
		return s.Message
	}
	return string(s.State)
}
