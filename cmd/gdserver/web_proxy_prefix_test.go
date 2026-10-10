package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gddoom/internal/netgame"
	"github.com/coder/websocket"
)

func TestWebProxyPrefixFlagsAndMixedOverlap(t *testing.T) {
	var flags webProxyFlags
	prefixes := webProxyPrefixFlags{&flags}
	for _, value := range []string{"/api/v1/=http://127.0.0.1:6675/api/v1/", "/rooms/=http://[::1]:6675/rooms/"} {
		if err := prefixes.Set(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := flags.Set("/deathmatch=http://127.0.0.1:6674/netplay"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prefixes.String(), "/api/v1/") || strings.Contains(flags.String(), "/api/v1/") || len(flags) != 3 {
		t.Fatal("exact and prefix flag views do not share routes correctly")
	}
	for _, value := range []string{
		"/=http://127.0.0.1:6675/", "/netplay/=http://127.0.0.1:6675/api/", "/netplay/child/=http://127.0.0.1:6675/api/",
		"/api/v1=http://127.0.0.1:6675/api/", "/a/../api/=http://127.0.0.1:6675/api/", "/a//api/=http://127.0.0.1:6675/api/",
		"/api%2fv1/=http://127.0.0.1:6675/api/", "/api/{id}/=http://127.0.0.1:6675/api/",
		"/new/=http://127.0.0.1:6675/api", "/new/=http://127.0.0.1:6675/%61pi/", "/new/=http://127.0.0.1:6675/api/../private/",
		"/new/=https://127.0.0.1:6675/api/", "/new/=http://localhost:6675/api/", "/new/=http://192.0.2.1:6675/api/",
		"/new/=http://127.0.0.1:0/api/", "/new/=http://user@127.0.0.1:6675/api/", "/new/=http://127.0.0.1:6675/api/?x=1",
		"/api/v1/=http://127.0.0.1:6675/api/v1/", "/api/=http://127.0.0.1:6675/api/", "/api/v1/content/=http://127.0.0.1:6675/content/",
		"/deathmatch/=http://127.0.0.1:6675/api/",
	} {
		if err := prefixes.Set(value); err == nil {
			t.Fatalf("accepted invalid/overlapping prefix %q", value)
		}
	}
	for _, value := range []string{"/api/v1=http://127.0.0.1:6675/api", "/api/v1/lobby=http://127.0.0.1:6675/lobby", "/rooms/game/netplay=http://127.0.0.1:6675/netplay"} {
		if err := flags.Set(value); err == nil {
			t.Fatalf("accepted exact route inside prefix: %s", value)
		}
	}
	var reverse webProxyFlags
	if err := reverse.Set("/api/v1/lobby=http://127.0.0.1:6675/lobby"); err != nil {
		t.Fatal(err)
	}
	if err := (webProxyPrefixFlags{&reverse}).Set("/api/v1/=http://127.0.0.1:6675/api/v1/"); err == nil {
		t.Fatal("prefix accepted overlap with earlier exact route")
	}
	if err := reverse.Set("/root=http://127.0.0.1:6675/"); err != nil {
		t.Fatalf("legacy exact proxy to backend root changed: %v", err)
	}
	if err := run(context.Background(), []string{"-web-proxy-prefix", "/api/v1/=http://127.0.0.1:6675/api/v1/"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "requires -web-listen") {
		t.Fatalf("prefix without public listener: %v", err)
	}
}

func TestWebProxyPrefixPreservesHTTPBodyAndReplacesForwardedIdentity(t *testing.T) {
	payload := strings.Repeat("uploaded-wad-", 4096)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != payload || r.URL.Path != "/backend/packs" || r.URL.RawQuery != "" || r.Method != http.MethodPost {
			t.Errorf("wrong rewritten request: %s %s body=%d err=%v", r.Method, r.URL.String(), len(body), err)
		}
		if r.Header.Get("X-Forwarded-For") != "127.0.0.1" || r.Header.Get("Forwarded") != "" || r.Header.Get("X-Forwarded-Proto") != "http" || r.Header.Get("X-Forwarded-Host") == "evil.example" {
			t.Errorf("spoofed identity survived: %v", r.Header)
		}
		if r.Header.Get("Origin") != "https://play.example" || r.Header.Get("Sec-WebSocket-Protocol") != netgame.WebSocketSubprotocol {
			t.Error("origin/subprotocol lost")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body)
	}))
	defer backend.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var routes webProxyFlags
	if err := (webProxyPrefixFlags{&routes}).Set("/api/v1/=" + backend.URL + "/backend/"); err != nil {
		t.Fatal(err)
	}
	public := httptest.NewServer(routes[0].handler(ctx, io.Discard))
	defer public.Close()
	request, _ := http.NewRequest(http.MethodPost, public.URL+"/api/v1/packs", strings.NewReader(payload))
	request.Header.Set("Origin", "https://play.example")
	request.Header.Set("Sec-WebSocket-Protocol", netgame.WebSocketSubprotocol)
	request.Header.Set("X-Forwarded-For", "203.0.113.1, 203.0.113.2")
	request.Header.Set("X-Forwarded-Host", "evil.example")
	request.Header.Set("X-Forwarded-Proto", "file")
	request.Header.Set("Forwarded", "for=203.0.113.3")
	response, err := public.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusCreated || string(body) != payload {
		t.Fatalf("HTTP response changed: %d %d %v", response.StatusCode, len(body), err)
	}
	for _, path := range []string{"/api/v1/../private", "/api/v1//packs", "/api/v1/%70acks", "/api/v1/%00", "/api/v1/packs?x=1", "/api/v1/packs?", "/api/v1/packs/extra/", "/netplay"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		writer := httptest.NewRecorder()
		routes[0].handler(ctx, io.Discard).ServeHTTP(writer, request)
		if writer.Code != http.StatusNotFound {
			t.Fatalf("noncanonical prefix request %q: %d", path, writer.Code)
		}
	}
}

func TestWebProxyPrefixWaitsForWorkerReadiness(t *testing.T) {
	if testing.Short() {
		t.Skip("real response timeout regression")
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5200 * time.Millisecond):
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "ready")
		case <-r.Context().Done():
		}
	}))
	defer backend.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var routes webProxyFlags
	if err := (webProxyPrefixFlags{&routes}).Set("/api/v1/=" + backend.URL + "/api/v1/"); err != nil {
		t.Fatal(err)
	}
	public := httptest.NewServer(routes[0].handler(ctx, io.Discard))
	defer public.Close()
	response, err := public.Client().Post(public.URL+"/api/v1/rooms", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("worker startup was cut off at the old five-second proxy timeout: %d", response.StatusCode)
	}
}

func TestWebProxyPrefixUpgradeAndShutdown(t *testing.T) {
	backendClosed := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rooms/abc/netplay" || r.Header.Get("X-Forwarded-For") != "127.0.0.1" {
			t.Errorf("wrong room route or identity: %s %s", r.URL.Path, r.Header.Get("X-Forwarded-For"))
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{netgame.WebSocketSubprotocol}, OriginPatterns: []string{"https://play.example"}})
		if err != nil {
			return
		}
		defer close(backendClosed)
		defer conn.CloseNow()
		for {
			kind, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			if err := conn.Write(context.Background(), kind, data); err != nil {
				return
			}
		}
	}))
	defer backend.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var routes webProxyFlags
	if err := (webProxyPrefixFlags{&routes}).Set("/rooms/=" + backend.URL + "/rooms/"); err != nil {
		t.Fatal(err)
	}
	public := httptest.NewServer(routes[0].handler(ctx, io.Discard))
	defer public.Close()
	dialCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	conn, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(public.URL, "http")+"/rooms/abc/netplay", &websocket.DialOptions{Subprotocols: []string{netgame.WebSocketSubprotocol}, HTTPHeader: http.Header{"Origin": []string{"https://play.example"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if conn.Subprotocol() != netgame.WebSocketSubprotocol {
		t.Fatal("room subprotocol lost")
	}
	if err := conn.Write(dialCtx, websocket.MessageBinary, []byte("move")); err != nil {
		t.Fatal(err)
	}
	_, data, err := conn.Read(dialCtx)
	if err != nil || string(data) != "move" {
		t.Fatalf("room did not echo input: %q %v", data, err)
	}
	cancel()
	if _, _, err := conn.Read(dialCtx); err == nil {
		t.Fatal("gateway shutdown did not close client")
	}
	select {
	case <-backendClosed:
	case <-dialCtx.Done():
		t.Fatal("gateway shutdown did not close backend")
	}
}

func TestWebProxyPrefixShutdownInterruptsStalledUpload(t *testing.T) {
	started, ended := make(chan struct{}), make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		_, _ = io.Copy(io.Discard, r.Body)
		close(ended)
	}))
	defer backend.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var routes webProxyFlags
	if err := (webProxyPrefixFlags{&routes}).Set("/api/v1/=" + backend.URL + "/api/v1/"); err != nil {
		t.Fatal(err)
	}
	public := httptest.NewServer(routes[0].handler(ctx, io.Discard))
	defer public.Close()
	conn, err := net.Dial("tcp", public.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = fmt.Fprintf(conn, "POST /api/v1/packs HTTP/1.1\r\nHost: localhost\r\nContent-Length: 1024\r\n\r\na")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("backend did not receive upload")
	}
	cancel()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("stalled upload survived gateway shutdown")
	}
}

type expiredHijackWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (w expiredHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	_ = w.conn.SetDeadline(time.Now().Add(-time.Second))
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

func TestWebProxyPrefixUpgradeClearsOrdinaryHTTPDeadline(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	conn, _, err := (prefixProxyWriter{expiredHijackWriter{httptest.NewRecorder(), local}}).Hijack()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := conn.Write([]byte{1}); done <- err }()
	_ = remote.SetReadDeadline(time.Now().Add(time.Second))
	var data [1]byte
	if _, err := io.ReadFull(remote, data[:]); err != nil || data[0] != 1 {
		t.Fatalf("HTTP deadline survived WebSocket upgrade: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
