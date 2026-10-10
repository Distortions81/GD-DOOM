//go:build !js

package netgame

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	wt "github.com/quic-go/webtransport-go"
)

type WebTransportOptions struct{ OriginPatterns []string }
type WebTransportServer struct{ server *wt.Server }

func (s *Server) NewWebTransportServer(tlsConfig *tls.Config, options WebTransportOptions) (*WebTransportServer, error) {
	if tlsConfig == nil || len(tlsConfig.Certificates) == 0 {
		return nil, errors.New("WebTransport requires a TLS server certificate")
	}
	config := tlsConfig.Clone()
	config.MinVersion = tls.VersionTLS13
	mux := http.NewServeMux()
	origins := append([]string(nil), options.OriginPatterns...)
	server := &wt.Server{H3: &http3.Server{TLSConfig: http3.ConfigureTLSConfig(config), Handler: mux, QUICConfig: webTransportQUICConfig(), MaxHeaderBytes: 16 << 10}, Config: &wt.Config{MaxIncomingStreams: 1, MaxIncomingUniStreams: -1, MaxIncomingData: 16 << 20}, CheckOrigin: func(request *http.Request) bool { return allowedWebTransportOrigin(request, origins) }}
	wt.ConfigureHTTP3Server(server.H3)
	mux.HandleFunc("/netplay", func(writer http.ResponseWriter, request *http.Request) {
		ctx, admitted := s.reserveConnection()
		if !admitted {
			http.Error(writer, "multiplayer server unavailable", http.StatusServiceUnavailable)
			return
		}
		defer s.releaseConnection()
		// Unlike websocket.Accept, Upgrade returns validation errors without
		// writing an HTTP failure. A bare handler return would imply 200 OK.
		if !allowedWebTransportOrigin(request, origins) {
			http.Error(writer, "browser origin forbidden", http.StatusForbidden)
			return
		}
		session, err := server.Upgrade(writer, request)
		if err != nil {
			http.Error(writer, "invalid WebTransport request", http.StatusBadRequest)
			return
		}
		opening, cancel := context.WithTimeout(ctx, 5*time.Second)
		stream, err := session.AcceptStream(opening)
		cancel()
		if err != nil {
			_ = session.CloseWithError(1, "missing control stream")
			return
		}
		connection := newWebTransportTransport(ctx, &nativeWebTransportStream{Stream: stream, session: session}, nativeWebTransportSession{session}, true)
		defer connection.waitClosed()
		defer connection.Close()
		s.serveConnection(ctx, connection)
	})
	return &WebTransportServer{server: server}, nil
}

func (s *WebTransportServer) Serve(conn net.PacketConn) error { return s.server.Serve(conn) }
func (s *WebTransportServer) Close() error                    { return s.server.Close() }

func allowedWebTransportOrigin(request *http.Request, patterns []string) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	if parsed.Scheme == "https" && strings.EqualFold(parsed.Host, request.Host) {
		return true
	}
	for _, pattern := range patterns {
		if match, err := path.Match(strings.ToLower(pattern), strings.ToLower(origin)); err == nil && match {
			return true
		}
	}
	return false
}

func webTransportQUICConfig() *quic.Config {
	return &quic.Config{EnableDatagrams: true, EnableStreamResetPartialDelivery: true, HandshakeIdleTimeout: 5 * time.Second, MaxIdleTimeout: 15 * time.Second, MaxIncomingStreams: 8, MaxIncomingUniStreams: 8, InitialStreamReceiveWindow: 128 << 10, MaxStreamReceiveWindow: 2 << 20, InitialConnectionReceiveWindow: 256 << 10, MaxConnectionReceiveWindow: 16 << 20}
}

// OpenWebTransport authenticates the HTTPS peer with system trust roots. UDP
// availability/capability failures can be retried through the caller's WSS path.
func OpenWebTransport(ctx context.Context, address string) (MessageTransport, error) {
	return openNativeWebTransport(ctx, address, nil)
}

func openNativeWebTransport(ctx context.Context, address string, tlsConfig *tls.Config) (MessageTransport, error) {
	address, err := webTransportURL(address)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{MinVersion: tls.VersionTLS13}
	if tlsConfig != nil {
		config = tlsConfig.Clone()
		config.MinVersion = tls.VersionTLS13
	}
	if config.InsecureSkipVerify {
		return nil, errors.New("WebTransport requires certificate verification")
	}
	transport := &wt.Transport{TLSClientConfig: config, QUICConfig: webTransportQUICConfig(), Config: &wt.Config{MaxIncomingStreams: -1, MaxIncomingUniStreams: -1, MaxIncomingData: 16 << 20}}
	opening, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, session, err := transport.Dial(opening, address, nil)
	// This Transport is used for one establishment only; Close releases its
	// opening context without closing an already established QUIC session.
	_ = transport.Close()
	if err != nil {
		return nil, err
	}
	stream, err := session.OpenStreamSync(opening)
	if err != nil {
		_ = session.CloseWithError(1, "missing control stream")
		return nil, err
	}
	connection := newWebTransportTransport(ctx, &nativeWebTransportStream{Stream: stream, session: session}, nativeWebTransportSession{session}, false)
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	return connection, nil
}

type nativeWebTransportStream struct {
	*wt.Stream
	session *wt.Session
}

func (s *nativeWebTransportStream) LocalAddr() net.Addr  { return s.session.LocalAddr() }
func (s *nativeWebTransportStream) RemoteAddr() net.Addr { return s.session.RemoteAddr() }

type nativeWebTransportSession struct{ *wt.Session }

func (s nativeWebTransportSession) Close() error { return s.CloseWithError(0, "") }
func (s nativeWebTransportSession) SendDatagram(data []byte) error {
	err := s.Session.SendDatagram(data)
	var tooLarge *quic.DatagramTooLargeError
	if errors.As(err, &tooLarge) {
		return nil
	} // Treat a path-MTU reduction as packet loss.
	return err
}
