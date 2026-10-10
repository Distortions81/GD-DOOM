package main

import (
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
}

type webProxyFlags []webProxyRoute

func (flags *webProxyFlags) String() string {
	var entries []string
	for _, proxy := range *flags {
		entries = append(entries, proxy.route+"="+proxy.upstream.String())
	}
	return strings.Join(entries, ",")
}

func (flags *webProxyFlags) Set(value string) error {
	route, address, found := strings.Cut(value, "=")
	if !found || route == "/" || route == "/netplay" || !strings.HasPrefix(route, "/") || strings.HasSuffix(route, "/") || path.Clean(route) != route || strings.ContainsAny(route, "{}%?# \t\r\n\\") {
		return fmt.Errorf("web proxy requires an exact non-reserved /route=upstream URL")
	}
	for _, proxy := range *flags {
		if proxy.route == route {
			return fmt.Errorf("duplicate web proxy route %q", route)
		}
	}
	upstream, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("invalid web proxy upstream: %w", err)
	}
	port, err := strconv.Atoi(upstream.Port())
	if upstream.Scheme != "http" || upstream.Opaque != "" || upstream.User != nil || upstream.RawQuery != "" || upstream.ForceQuery || upstream.Fragment != "" || upstream.RawPath != "" || upstream.Path == "" || path.Clean(upstream.Path) != upstream.Path || !strings.HasPrefix(upstream.Path, "/") || err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("web proxy upstream must be an HTTP loopback URL with an explicit port and clean path, without credentials, query, or fragment")
	}
	if ip := net.ParseIP(upstream.Hostname()); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("web proxy upstream must use a literal loopback IP address")
	}
	*flags = append(*flags, webProxyRoute{route: route, upstream: upstream})
	return nil
}

func (route webProxyRoute) handler(lifetime context.Context, errOut io.Writer) http.Handler {
	// Do not inherit HTTP_PROXY: even loopback routing must stay local when the
	// launching environment configures a system proxy.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 5 * time.Second
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		ErrorLog:  log.New(errOut, "gdserver web proxy: ", 0),
		Rewrite: func(request *httputil.ProxyRequest) {
			// Keep Origin and the WebSocket subprotocol intact. The backend's
			// normal WebSocket handler enforces its own browser origin policy.
			upstream := *route.upstream
			request.Out.URL = &upstream
			request.Out.Host = route.upstream.Host
		},
	}
	context.AfterFunc(lifetime, transport.CloseIdleConnections)
	slots := make(chan struct{}, 32)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != route.route || request.URL.RawPath != "" {
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
		// ReverseProxy closes both hijacked WebSocket sockets when this context
		// ends. http.Server.Close alone cannot close hijacked connections.
		proxy.ServeHTTP(writer, request.WithContext(ctx))
	})
}
