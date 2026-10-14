package roomhost

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func roomProxy(ctx context.Context, upstream *url.URL, logOutput io.Writer) http.Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 5 * time.Second
	proxy := &httputil.ReverseProxy{
		Transport: transport, ErrorLog: log.New(logOutput, "room proxy: ", 0),
		Rewrite: func(request *httputil.ProxyRequest) {
			// Preserve Origin and subprotocol; the worker enforces its allowlist.
			u := *upstream
			request.Out.URL, request.Out.Host = &u, u.Host
		},
	}
	context.AfterFunc(ctx, transport.CloseIdleConnections)
	slots := make(chan struct{}, 32)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "room is busy", http.StatusServiceUnavailable)
			return
		}
		lifetime, cancel := context.WithCancel(r.Context())
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
		defer cancel()
		// HTTP shutdown alone does not close upgraded WebSocket connections.
		proxy.ServeHTTP(w, r.WithContext(lifetime))
	})
}
