package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// A proxy hosts another isolated match on the existing public HTTPS listener.
// Its destination is fixed by the operator and restricted to literal loopback
// addresses; browser input can never select an upstream.
type webProxyRoute struct {
	route    string
	upstream *url.URL
	prefix   bool
}

type webProxyFlags []webProxyRoute

// Native QUIC routes and room prefixes share overlap checks, but expose only
// fixed loopback destinations chosen by the operator.
type udpNativeProxyFlags struct{ routes *webProxyFlags }

func (f udpNativeProxyFlags) String() string { return f.routes.String() }
func (f udpNativeProxyFlags) Set(value string) error {
	route, address, found := strings.Cut(value, "=")
	if !found || route == "/" || route == "/netplay" || strings.HasPrefix(route, "/netplay/") || !cleanProxyPath(route, false) {
		return fmt.Errorf("UDP proxy requires an exact non-reserved route=tcp://loopback:port")
	}
	u, err := url.Parse(address)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(u.Port())
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "tcp" || ip == nil || !ip.IsLoopback() || err != nil || port < 1 || port > 65535 || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("UDP proxy upstream requires a literal loopback TCP endpoint without path or credentials")
	}
	entry := webProxyRoute{route: route, upstream: u}
	for _, existing := range *f.routes {
		if proxyRoutesOverlap(existing, entry) {
			return fmt.Errorf("overlapping UDP proxy route %q", route)
		}
	}
	*f.routes = append(*f.routes, entry)
	return nil
}

func (flags *webProxyFlags) String() string {
	var entries []string
	for _, proxy := range *flags {
		if !proxy.prefix {
			entries = append(entries, proxy.route+"="+proxy.upstream.String())
		}
	}
	return strings.Join(entries, ",")
}

func (flags *webProxyFlags) Set(value string) error {
	return flags.add(value, false)
}

// Prefix and exact flags share one registry so ambiguous routes are rejected
// regardless of which flag was supplied first.
type webProxyPrefixFlags struct{ routes *webProxyFlags }

func (flags webProxyPrefixFlags) String() string {
	var entries []string
	if flags.routes != nil {
		for _, proxy := range *flags.routes {
			if proxy.prefix {
				entries = append(entries, proxy.route+"="+proxy.upstream.String())
			}
		}
	}
	return strings.Join(entries, ",")
}

func (flags webProxyPrefixFlags) Set(value string) error { return flags.routes.add(value, true) }

func cleanProxyPath(value string, prefix bool) bool {
	if value == "" || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "{}%?# \t\r\n\\") {
		return false
	}
	if prefix {
		return value == "/" || (strings.HasSuffix(value, "/") && path.Clean(value)+"/" == value)
	}
	return !strings.HasSuffix(value, "/") && path.Clean(value) == value
}

func proxyRoutesOverlap(a, b webProxyRoute) bool {
	return strings.TrimSuffix(a.route, "/") == strings.TrimSuffix(b.route, "/") ||
		(a.prefix && strings.HasPrefix(b.route, a.route)) || (b.prefix && strings.HasPrefix(a.route, b.route))
}

func (flags *webProxyFlags) add(value string, prefix bool) error {
	route, address, found := strings.Cut(value, "=")
	if !found || route == "/" || route == "/netplay" || strings.HasPrefix(route, "/netplay/") || !cleanProxyPath(route, prefix) {
		return fmt.Errorf("web proxy requires a clean non-reserved route=upstream URL; prefix routes must end in /")
	}
	entry := webProxyRoute{route: route, prefix: prefix}
	for _, proxy := range *flags {
		if proxyRoutesOverlap(proxy, entry) {
			return fmt.Errorf("overlapping web proxy route %q", route)
		}
	}
	upstream, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("invalid web proxy upstream: %w", err)
	}
	port, err := strconv.Atoi(upstream.Port())
	cleanUpstream := cleanProxyPath(upstream.Path, prefix) || (!prefix && upstream.Path == "/")
	if upstream.Scheme != "http" || upstream.Opaque != "" || upstream.User != nil || upstream.RawQuery != "" || upstream.ForceQuery || upstream.Fragment != "" || upstream.RawPath != "" || !cleanUpstream || err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("web proxy upstream must be an HTTP loopback URL with an explicit port and clean path, without credentials, query, or fragment")
	}
	if ip := net.ParseIP(upstream.Hostname()); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("web proxy upstream must use a literal loopback IP address")
	}
	entry.upstream = upstream
	*flags = append(*flags, entry)
	return nil
}

func (route webProxyRoute) handler(lifetime context.Context, errOut io.Writer) http.Handler {
	// Do not inherit HTTP_PROXY: even loopback routing must stay local when the
	// launching environment configures a system proxy.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 5 * time.Second
	if route.prefix {
		transport.ResponseHeaderTimeout = 3 * time.Minute
	}
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		ErrorLog:  log.New(errOut, "gdserver web proxy: ", 0),
		Rewrite: func(request *httputil.ProxyRequest) {
			// Keep Origin and the WebSocket subprotocol intact. The backend's
			// normal WebSocket handler enforces its own browser origin policy.
			upstream := *route.upstream
			if route.prefix {
				upstream.Path += strings.TrimPrefix(request.In.URL.Path, route.route)
				// ReverseProxy removes inbound forwarding headers before Rewrite;
				// SetXForwarded uses the actual socket peer, never a claimed chain.
				request.SetXForwarded()
			}
			request.Out.URL = &upstream
			request.Out.Host = route.upstream.Host
		},
	}
	context.AfterFunc(lifetime, transport.CloseIdleConnections)
	capacity := 32
	if route.prefix {
		capacity = 256
	}
	slots := make(chan struct{}, capacity)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		valid := request.URL.Path == route.route
		if route.prefix {
			valid = strings.HasPrefix(request.URL.Path, route.route) &&
				(path.Clean(request.URL.Path) == request.URL.Path || request.URL.Path == route.route) &&
				!strings.ContainsAny(request.URL.Path, "{}%?# \t\r\n\\") && request.URL.EscapedPath() == request.URL.Path &&
				request.URL.RawQuery == "" && !request.URL.ForceQuery
		}
		if !valid || request.URL.RawPath != "" {
			http.NotFound(writer, request)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(writer, "multiplayer proxy unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithCancel(request.Context())
		stop := context.AfterFunc(lifetime, cancel)
		defer stop()
		defer cancel()
		if route.prefix {
			controller := http.NewResponseController(writer)
			deadline := time.Now().Add(3 * time.Minute)
			_ = controller.SetReadDeadline(deadline)
			_ = controller.SetWriteDeadline(deadline)
			stopped := make(chan struct{})
			stopIO := context.AfterFunc(ctx, func() {
				_ = controller.SetReadDeadline(time.Now())
				_ = controller.SetWriteDeadline(time.Now())
				close(stopped)
			})
			defer func() {
				if !stopIO() {
					<-stopped
				}
				_ = controller.SetReadDeadline(time.Time{})
				_ = controller.SetWriteDeadline(time.Time{})
			}()
			writer = prefixProxyWriter{writer}
		}
		// ReverseProxy closes both hijacked WebSocket sockets when this context
		// ends. http.Server.Close alone cannot close hijacked connections.
		proxy.ServeHTTP(writer, request.WithContext(ctx))
	})
}

// Only ordinary HTTP transfers have a three-minute deadline. A successful
// WebSocket upgrade transfers the connection to the match lifecycle instead.
type prefixProxyWriter struct{ http.ResponseWriter }

func (w prefixProxyWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w prefixProxyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	connection, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		err = connection.SetDeadline(time.Time{})
		if err != nil {
			_ = connection.Close()
		}
	}
	return connection, buffer, err
}
