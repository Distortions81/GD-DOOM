package roomhost

import (
	"crypto/tls"
	"errors"
	"net/http"
	"strings"

	"gddoom/internal/netgame"
)

// NewWebTransportServer multiplexes isolated workers on one public UDP port.
// The short local TCP hop carries the existing binary protocol; Internet input
// and small snapshot deltas travel as authenticated QUIC datagrams.
func (m *Manager) NewWebTransportServer(config *tls.Config) (*netgame.WebTransportServer, error) {
	if !m.config.WebTransport {
		return nil, errors.New("WebTransport is not enabled for this lobby")
	}
	server, err := netgame.NewWebTransportServer(config, netgame.WebTransportOptions{OriginPatterns: m.config.WebOrigins})
	if err != nil {
		return nil, err
	}
	server.Handle("/rooms/", http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !m.allowOrigin(request.Header.Get("Origin")) {
			http.Error(w, "browser origin forbidden", http.StatusForbidden)
			return
		}
		parts := strings.Split(request.URL.Path, "/")
		if len(parts) != 4 || parts[3] != "netplay" || request.URL.RawPath != "" || request.URL.RawQuery != "" {
			http.NotFound(w, request)
			return
		}
		m.mu.Lock()
		room := m.rooms[parts[2]]
		if room == nil || room.room.State != "ready" || m.closed {
			m.mu.Unlock()
			http.NotFound(w, request)
			return
		}
		ctx, address := room.ctx, room.tcpAddress
		select {
		case room.connections <- struct{}{}:
			m.wg.Add(1)
		default:
			m.mu.Unlock()
			http.Error(w, "room is busy", http.StatusServiceUnavailable)
			return
		}
		m.mu.Unlock()
		defer m.wg.Done()
		defer func() { <-room.connections }()
		// The destination comes only from validated worker readiness, never
		// from a client URL, header or request body.
		upstream, err := netgame.OpenTCP(ctx, address)
		if err != nil {
			http.Error(w, "room is unavailable", http.StatusServiceUnavailable)
			return
		}
		defer upstream.Close()
		client, err := server.Accept(ctx, w, request)
		if err != nil {
			return
		}
		defer client.Close()
		netgame.RelayMessages(ctx, client, upstream)
	}))
	return server, nil
}
