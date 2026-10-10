package netgame

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	WebSocketSubprotocol = "gd-doom.v3"
	// Bounded client controls include up to 160 UTF-8 chat runes.
	maxClientWebSocketMessage = 1024
	maxServerWebSocketMessage = MaxSnapshotBytes + 1024
)

// WebSocketOptions uses coder/websocket's origin policy. Same-host origins are
// accepted by default; explicit patterns authorize additional browser origins.
// Include the scheme for HTTPS-only allowlists, e.g. "https://play.example.com".
// Native clients without an Origin header are also permitted. This is origin
// verification, not user authentication; session admission validates content.
type WebSocketOptions struct {
	OriginPatterns []string
}

// WebSocketHandler attaches upgraded binary WebSockets to this Server's match
// owner. Start Serve before this handler; stopped/full owners respond 503.
// The caller owns the HTTP server, route, header timeouts, and TLS certificates.
// Serve cancellation closes hijacked sockets, which http.Shutdown alone does
// not close. HTTP request contexts are not used after upgrade.
func (s *Server) WebSocketHandler(options WebSocketOptions) http.Handler {
	origins := append([]string(nil), options.OriginPatterns...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, admitted := s.reserveConnection()
		if !admitted {
			http.Error(w, "multiplayer server unavailable", http.StatusServiceUnavailable)
			return
		}
		defer s.releaseConnection()
		connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{WebSocketSubprotocol}, OriginPatterns: origins,
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			return // Accept has already written the HTTP error response.
		}
		if connection.Subprotocol() != WebSocketSubprotocol {
			_ = connection.CloseNow()
			return
		}
		stream := newWebSocketStream(ctx, connection, maxClientWebSocketMessage)
		defer stream.Close()
		s.serveConnection(ctx, stream)
	})
}

// DialWebSocket connects native and GOOS=js/GOARCH=wasm clients to the same
// authoritative protocol. Use wss:// for Internet/browser deployments; the
// browser controls its Origin header and TLS trust. The caller's context bounds
// the session lifetime, while opening and game negotiation have separate
// five-second deadlines.
func DialWebSocket(ctx context.Context, url string, hello Hello) (*Client, error) {
	transport, err := OpenWebSocket(ctx, url)
	if err != nil {
		return nil, err
	}
	return Connect(ctx, transport, hello)
}

func OpenWebSocket(ctx context.Context, url string) (MessageTransport, error) {
	opening, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(opening, url, &websocket.DialOptions{
		Subprotocols: []string{WebSocketSubprotocol},
	})
	if err != nil {
		return nil, err
	}
	if connection.Subprotocol() != WebSocketSubprotocol {
		_ = connection.CloseNow()
		return nil, fmt.Errorf("%w: missing WebSocket subprotocol", ErrProtocol)
	}
	stream := newWebSocketStream(ctx, connection, maxServerWebSocketMessage)
	_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
	return &streamTransport{stream}, nil
}

type webSocketStream struct {
	net.Conn
	websocket *websocket.Conn
	cancel    context.CancelFunc
	once      sync.Once
}

func newWebSocketStream(ctx context.Context, connection *websocket.Conn, readLimit int64) *webSocketStream {
	lifetime, cancel := context.WithCancel(ctx)
	stream := websocket.NetConn(lifetime, connection, websocket.MessageBinary)
	// NetConn disables WebSocket message limits. Restore the directional limit
	// after wrapping; the game codec also bounds allocation for logical frames
	// even if a sender splits them across multiple WebSocket messages.
	connection.SetReadLimit(readLimit)
	return &webSocketStream{Conn: stream, websocket: connection, cancel: cancel}
}

func (stream *webSocketStream) Close() error {
	stream.once.Do(func() {
		stream.cancel()
		// On native servers CloseNow avoids performing a network close-handshake
		// write/wait in the match owner when it drops a slow or invalid sender.
		_ = stream.websocket.CloseNow()
		_ = stream.Conn.Close()
	})
	return nil
}
