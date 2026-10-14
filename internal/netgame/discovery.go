package netgame

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"
)

var ErrDiscoveryUnavailable = errors.New("server does not publish match metadata")

func cloneCompatibilityManifest(m CompatibilityManifest) CompatibilityManifest {
	m.WADHashes = slices.Clone(m.WADHashes)
	return m
}

// ServerInfo belongs to the match owner, just like Join and Step. The result
// owns its content hash slice and never exposes mutable match configuration.
func (m *Match) ServerInfo() (ServerInfo, error) {
	if m.config.Manifest == nil {
		return ServerInfo{}, ErrDiscoveryUnavailable
	}
	return ServerInfo{Manifest: cloneCompatibilityManifest(*m.config.Manifest)}, nil
}

// ServerStatus runs on the same owner as Join, Suspend, Leave and Step. Counts
// therefore describe one consistent roster, including reserved reconnect bodies.
func (m *Match) ServerStatus() (ServerStatus, error) {
	info, err := m.ServerInfo()
	if err != nil {
		return ServerStatus{}, err
	}
	status := ServerStatus{Manifest: info.Manifest, PlayerLimit: m.config.PlayerLimit, SpectatorLimit: MaxSpectators}
	for _, player := range m.players {
		if player.spectator {
			status.Spectators++
		} else {
			status.Players++
			if player.suspended {
				status.ReservedPlayers++
			}
		}
	}
	return status, nil
}

// QueryServer consumes and closes a discovery transport without joining a slot.
// Content and rules remain untrusted metadata until the caller checks local
// resources, computes the full compatibility key, and performs a separate Hello.
func QueryServer(ctx context.Context, transport MessageTransport) (ServerInfo, error) {
	message, err := queryDiscoveryMessage(ctx, transport, Query{})
	if err != nil {
		return ServerInfo{}, err
	}
	info, ok := message.(ServerInfo)
	if !ok {
		return ServerInfo{}, ErrProtocol
	}
	info.Manifest = cloneCompatibilityManifest(info.Manifest)
	return info, nil
}

// QueryServerStatus closes its short-lived transport exactly like QueryServer.
// A caller falling back to legacy discovery must open a new transport, since an
// older peer is allowed to close a connection after an unsupported message kind.
func QueryServerStatus(ctx context.Context, transport MessageTransport) (ServerStatus, error) {
	message, err := queryDiscoveryMessage(ctx, transport, StatusQuery{})
	if err != nil {
		return ServerStatus{}, err
	}
	status, ok := message.(ServerStatus)
	if !ok {
		return ServerStatus{}, ErrProtocol
	}
	status.Manifest = cloneCompatibilityManifest(status.Manifest)
	return status, nil
}

func queryDiscoveryMessage(ctx context.Context, transport MessageTransport, request any) (any, error) {
	if transport == nil {
		return nil, errors.New("missing discovery transport")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	defer transport.Close()
	stop := context.AfterFunc(ctx, func() { _ = transport.Close() })
	defer stop()
	if err := transport.WriteMessage(request); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	message, err := transport.ReadMessage()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	if refusal, ok := message.(Disconnect); ok {
		return nil, fmt.Errorf("server refused discovery: %s", refusal.Reason)
	}
	if _, err := validateMessage(message); err != nil {
		return nil, err
	}
	return message, nil
}

type discoveryRequest struct {
	ctx    context.Context
	status bool
	reply  chan discoveryReply
}

type discoveryReply struct {
	message any
	err     error
}

func (s *Server) replyDiscovery(request discoveryRequest) {
	if request.ctx.Err() != nil {
		return
	}
	if request.status {
		status, err := s.match.ServerStatus()
		request.reply <- discoveryReply{message: status, err: err}
	} else {
		info, err := s.match.ServerInfo()
		request.reply <- discoveryReply{message: info, err: err}
	}
}

func (s *Server) serveDiscovery(ctx context.Context, conn net.Conn, status bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request := discoveryRequest{ctx: ctx, status: status, reply: make(chan discoveryReply, 1)}
	select {
	case s.queries <- request:
	case <-ctx.Done():
		return
	}
	select {
	case result := <-request.reply:
		if result.err != nil {
			_ = writeStreamMessage(conn, Disconnect{Reason: result.err.Error()})
		} else {
			_ = writeStreamMessage(conn, result.message)
		}
	case <-ctx.Done():
	}
}
