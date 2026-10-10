//go:build !js

package netgame

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type bufferedGameConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedGameConn) Read(data []byte) (int, error) { return c.reader.Read(data) }

// OpenLocalGameTunnel connects an operator-selected loopback HTTP route to
// its worker's native binary stream. It is never an Internet transport.
func OpenLocalGameTunnel(ctx context.Context, address, origin string) (MessageTransport, error) {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || u.Path == "" {
		return nil, errors.New("game tunnel requires an exact loopback HTTP URL")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() || u.Port() == "" {
		return nil, errors.New("game tunnel requires a literal loopback address and port")
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	request, err := http.NewRequest(http.MethodConnect, u.String(), nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	request.Header.Set("Origin", origin)
	if err := request.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReaderSize(conn, 16<<10)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		conn.Close()
		return nil, errors.New("room tunnel unavailable")
	}
	return openStream(&bufferedGameConn{Conn: conn, reader: reader}), nil
}

// LocalGameTunnelHandler exposes a worker only to a literal loopback caller.
// remote is the supervisor's verified peer IP after trusted-proxy handling.
func LocalGameTunnelHandler(ctx context.Context, workerAddress, remote string, w http.ResponseWriter, request *http.Request) {
	ip := net.ParseIP(remote)
	if ip == nil || !ip.IsLoopback() {
		http.Error(w, "private game route", http.StatusForbidden)
		return
	}
	worker, err := OpenTCP(ctx, workerAddress)
	if err != nil {
		http.Error(w, "room unavailable", http.StatusServiceUnavailable)
		return
	}
	defer worker.Close()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "native tunnel unavailable", http.StatusBadRequest)
		return
	}
	conn, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	client := &boundedTunnelConn{Conn: &bufferedGameConn{Conn: conn, reader: buffered.Reader}}
	upstream := &boundedTunnelConn{Conn: worker.(net.Conn)}
	stop := context.AfterFunc(ctx, func() { conn.Close(); worker.Close() })
	defer stop()
	// The public QUIC adapter and the worker already validate framing. Copy
	// this private stream directly instead of parsing and allocating it again.
	done := make(chan struct{})
	go func() { defer close(done); defer worker.Close(); _, _ = io.Copy(upstream, client) }()
	_, _ = io.Copy(client, upstream)
	conn.Close()
	worker.Close()
	<-done
}

type boundedTunnelConn struct{ net.Conn }

func (c *boundedTunnelConn) Read(data []byte) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	return c.Conn.Read(data)
}
func (c *boundedTunnelConn) Write(data []byte) (int, error) {
	_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return c.Conn.Write(data)
}

// ProxyHandler routes datagram sessions to an existing room supervisor without
// an extra public listener or a WebSocket hop. The upstream is operator-owned.
func (s *WebTransportServer) ProxyHandler(ctx context.Context, upstream *url.URL, prefix string) http.Handler {
	slots := make(chan struct{}, 256)
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.RawPath != "" || request.URL.RawQuery != "" || !strings.HasPrefix(request.URL.Path, prefix) {
			http.NotFound(w, request)
			return
		}
		if !allowedWebTransportOrigin(request, s.origins) {
			http.Error(w, "browser origin forbidden", http.StatusForbidden)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "gateway is busy", http.StatusServiceUnavailable)
			return
		}
		target := *upstream
		target.Path += strings.TrimPrefix(request.URL.Path, prefix)
		var worker MessageTransport
		var err error
		if target.Scheme == "tcp" {
			ip := net.ParseIP(target.Hostname())
			if ip == nil || !ip.IsLoopback() || target.Path != "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" {
				http.Error(w, "invalid private upstream", http.StatusServiceUnavailable)
				return
			}
			worker, err = OpenTCP(ctx, target.Host)
		} else {
			worker, err = OpenLocalGameTunnel(ctx, target.String(), request.Header.Get("Origin"))
		}
		if err != nil {
			http.Error(w, "room unavailable", http.StatusServiceUnavailable)
			return
		}
		defer worker.Close()
		client, err := s.Accept(ctx, w, request)
		if err != nil {
			return
		}
		defer client.(*webTransportTransport).waitClosed()
		defer client.Close()
		RelayMessages(ctx, client, worker)
	})
}
