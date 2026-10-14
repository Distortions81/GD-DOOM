package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gddoom/internal/netgame"
	"github.com/coder/websocket"
)

func TestWebProxyFlags(t *testing.T) {
	var flags webProxyFlags
	for _, value := range []string{"/deathmatch=http://127.0.0.1:6674/netplay", "/arena=http://[::1]:6675/netplay"} {
		if err := flags.Set(value); err != nil {
			t.Fatalf("%s: %v", value, err)
		}
	}
	if len(flags) != 2 || !strings.Contains(flags.String(), "/deathmatch=http://127.0.0.1:6674/netplay") {
		t.Fatalf("routes: %v", flags)
	}
	for _, value := range []string{
		"/deathmatch=http://127.0.0.1:6674/netplay",
		"/netplay=http://127.0.0.1:6674/netplay", "/=http://127.0.0.1:6674/netplay",
		"deathmatch=http://127.0.0.1:6674/netplay", "/deathmatch/=http://127.0.0.1:6674/netplay",
		"/dm/{id}=http://127.0.0.1:6674/netplay", "/dm/../other=http://127.0.0.1:6674/netplay",
		"/dm%2Fother=http://127.0.0.1:6674/netplay", "/dm\t=http://127.0.0.1:6674/netplay",
		"/dm=https://127.0.0.1:6674/netplay", "/dm=http://localhost:6674/netplay",
		"/dm=http://0.0.0.0:6674/netplay", "/dm=http://192.0.2.1:6674/netplay",
		"/dm=http://127.0.0.1/netplay", "/dm=http://127.0.0.1:0/netplay", "/dm=http://127.0.0.1:65536/netplay",
		"/dm=http://user@127.0.0.1:6674/netplay", "/dm=http://127.0.0.1:6674/netplay?target=x",
		"/dm=http://127.0.0.1:6674/netplay?", "/dm=http://127.0.0.1:6674/netplay#x",
		"/dm=http://127.0.0.1:6674", "/dm=http://127.0.0.1:6674/a/../netplay", "/dm=http://127.0.0.1:6674/%6Eetplay",
	} {
		t.Run(value, func(t *testing.T) {
			if err := flags.Set(value); err == nil {
				t.Fatalf("accepted invalid route: %s", value)
			}
		})
	}
	if len(flags) != 2 {
		t.Fatal("invalid configuration mutated routes")
	}
	if err := run(context.Background(), []string{"-web-proxy", "/dm=http://127.0.0.1:6674/netplay"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "requires -web-listen") {
		t.Fatalf("proxy without public listener: %v", err)
	}
}

func TestWebProxyUpgradeOriginAndLifecycle(t *testing.T) {
	for _, closer := range []string{"client", "backend", "shutdown"} {
		t.Run(closer, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			backendClosed := make(chan struct{})
			closeBackend := make(chan struct{})
			backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/netplay" || request.URL.RawQuery != "" || request.Header.Get("Origin") != "https://play.example" {
					t.Errorf("rewritten request path=%s query=%s origin=%s", request.URL.Path, request.URL.RawQuery, request.Header.Get("Origin"))
				}
				connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{Subprotocols: []string{netgame.WebSocketSubprotocol}, OriginPatterns: []string{"https://play.example"}})
				if err != nil {
					return
				}
				defer close(backendClosed)
				defer connection.CloseNow()
				stopClose := make(chan struct{})
				defer close(stopClose)
				go func() {
					select {
					case <-closeBackend:
						_ = connection.CloseNow()
					case <-stopClose:
					}
				}()
				if err := connection.Write(request.Context(), websocket.MessageBinary, []byte("authoritative snapshot")); err != nil {
					return
				}
				for {
					kind, data, err := connection.Read(request.Context())
					if err != nil {
						return
					}
					if err := connection.Write(request.Context(), kind, append([]byte("input received: "), data...)); err != nil {
						return
					}
				}
			}))
			defer backend.Close()
			var routes webProxyFlags
			if err := routes.Set("/deathmatch=" + backend.URL + "/netplay"); err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.Handle(routes[0].route, routes[0].handler(ctx, io.Discard))
			public := httptest.NewServer(mux)
			defer public.Close()
			connectionCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			url := "ws" + strings.TrimPrefix(public.URL, "http") + "/deathmatch?ignored=1"
			connection, _, err := websocket.Dial(connectionCtx, url, &websocket.DialOptions{Subprotocols: []string{netgame.WebSocketSubprotocol}, HTTPHeader: http.Header{"Origin": []string{"https://play.example"}}})
			if err != nil {
				t.Fatal(err)
			}
			defer connection.CloseNow()
			if connection.Subprotocol() != netgame.WebSocketSubprotocol {
				t.Fatalf("subprotocol lost: %q", connection.Subprotocol())
			}
			kind, data, err := connection.Read(connectionCtx)
			if err != nil || kind != websocket.MessageBinary || string(data) != "authoritative snapshot" {
				t.Fatalf("server-first message: %d %q %v", kind, data, err)
			}
			if err := connection.Write(connectionCtx, websocket.MessageBinary, []byte("move")); err != nil {
				t.Fatal(err)
			}
			kind, data, err = connection.Read(connectionCtx)
			if err != nil || kind != websocket.MessageBinary || string(data) != "input received: move" {
				t.Fatalf("bidirectional message: %d %q %v", kind, data, err)
			}
			switch closer {
			case "client":
				_ = connection.CloseNow()
			case "backend":
				close(closeBackend)
			case "shutdown":
				cancel()
			}
			if _, _, err := connection.Read(connectionCtx); err == nil {
				t.Fatal("client connection remained open")
			}
			select {
			case <-backendClosed:
			case <-connectionCtx.Done():
				t.Fatal("backend connection remained open")
			}
		})
	}
}

func TestWebProxyPreservesBackendOriginPolicyAndExactRoute(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{Subprotocols: []string{netgame.WebSocketSubprotocol}, OriginPatterns: []string{"https://play.example"}})
		if err == nil {
			_ = connection.CloseNow()
		}
	}))
	defer backend.Close()
	var routes webProxyFlags
	if err := routes.Set("/deathmatch=" + backend.URL + "/netplay"); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(routes[0].route, routes[0].handler(ctx, io.Discard))
	public := httptest.NewServer(mux)
	defer public.Close()
	for _, test := range []struct {
		path, origin string
		status       int
	}{
		{"/deathmatch", "https://evil.example", http.StatusForbidden},
		{"/deathmatch/extra", "https://play.example", http.StatusNotFound},
		{"/%64eathmatch", "https://play.example", http.StatusNotFound},
		{"/netplay", "https://play.example", http.StatusNotFound},
	} {
		connection, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(public.URL, "http")+test.path, &websocket.DialOptions{Subprotocols: []string{netgame.WebSocketSubprotocol}, HTTPHeader: http.Header{"Origin": []string{test.origin}}})
		if connection != nil {
			_ = connection.CloseNow()
		}
		if err == nil || response == nil || response.StatusCode != test.status {
			t.Fatalf("%s origin %s: response=%v err=%v", test.path, test.origin, response, err)
		}
	}
}
