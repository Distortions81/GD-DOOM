package roomhost

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTrustedProxyIdentityRequiresExplicitPeerAndOneIP(t *testing.T) {
	trusted, err := parseTrustedProxies([]string{"127.0.0.1", "::1"})
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{trustedProxies: trusted}
	for _, tc := range []struct {
		name, peer, want string
		forwarded        []string
	}{
		{name: "trusted IPv4", peer: "127.0.0.1:9000", forwarded: []string{"203.0.113.10"}, want: "203.0.113.10"},
		{name: "trusted IPv6", peer: "[::1]:9000", forwarded: []string{"2001:db8::1234"}, want: "2001:db8::1234"},
		{name: "mapped peer", peer: "[::ffff:127.0.0.1]:9000", forwarded: []string{"::ffff:203.0.113.10"}, want: "203.0.113.10"},
		{name: "canonical IPv6", peer: "[::1]:9000", forwarded: []string{"2001:0db8:0:0:0:0:0:1"}, want: "2001:db8::1"},
		{name: "untrusted spoof", peer: "198.51.100.1:9000", forwarded: []string{"203.0.113.10"}, want: "198.51.100.1"},
		{name: "unconfigured loopback", peer: "127.0.0.2:9000", forwarded: []string{"203.0.113.10"}, want: "127.0.0.2"},
		{name: "missing", peer: "127.0.0.1:9000", want: "127.0.0.1"},
		{name: "empty", peer: "127.0.0.1:9000", forwarded: []string{""}, want: "127.0.0.1"},
		{name: "malformed", peer: "127.0.0.1:9000", forwarded: []string{"not-an-ip"}, want: "127.0.0.1"},
		{name: "IP with port", peer: "127.0.0.1:9000", forwarded: []string{"203.0.113.10:44"}, want: "127.0.0.1"},
		{name: "zone", peer: "127.0.0.1:9000", forwarded: []string{"fe80::1%eth0"}, want: "127.0.0.1"},
		{name: "chain", peer: "127.0.0.1:9000", forwarded: []string{"203.0.113.10, 198.51.100.1"}, want: "127.0.0.1"},
		{name: "multiple values", peer: "127.0.0.1:9000", forwarded: []string{"203.0.113.10", "203.0.113.10"}, want: "127.0.0.1"},
		{name: "canonical direct", peer: "[2001:0db8:0:0:0:0:0:1]:9000", forwarded: []string{"203.0.113.10"}, want: "2001:db8::1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://lobby/api/v1/rooms", nil)
			r.RemoteAddr = tc.peer
			for _, value := range tc.forwarded {
				r.Header.Add("X-Forwarded-For", value)
			}
			r.Header.Set("Forwarded", "for=192.0.2.1")
			r.Header.Set("X-Real-IP", "192.0.2.2")
			if got := m.remoteAddress(r); got != tc.want {
				t.Fatalf("identity=%q, want %q", got, tc.want)
			}
		})
	}
	r := httptest.NewRequest(http.MethodPost, "http://lobby/", nil)
	r.RemoteAddr = "127.0.0.1:9000"
	r.Header.Set("X-Forwarded-For", "203.0.113.10")
	if got := (&Manager{}).remoteAddress(r); got != "127.0.0.1" {
		t.Fatal("default configuration trusted loopback forwarding")
	}
}

func TestTrustedProxyConfigurationRejectsNonliteralOrNonloopback(t *testing.T) {
	for _, value := range []string{"", "*", "localhost", "127.0.0.0/8", "127.0.0.1:443", "[::1]", "::1%lo", "192.0.2.1", "::", "0.0.0.0"} {
		if _, err := New(context.Background(), Config{TrustedProxies: []string{value}}); err == nil || !strings.Contains(err.Error(), "trusted proxy") {
			t.Fatalf("invalid trusted proxy %q did not fail configuration: %v", value, err)
		}
	}
	if _, err := parseTrustedProxies(make([]string, 17)); err == nil {
		t.Fatal("unbounded trusted proxy configuration accepted")
	}
	trusted, err := parseTrustedProxies([]string{"127.0.0.1", "::ffff:127.0.0.1", "::1"})
	if err != nil || len(trusted) != 2 {
		t.Fatal("literal IP normalization failed:", err)
	}
}

func TestTrustedProxySeparatesSharedGatewayUploadAndCreateRates(t *testing.T) {
	manager, _ := testManager(t, Config{TrustedProxies: []string{"127.0.0.1"}, UploadDir: t.TempDir()})
	handler := manager.Handler()
	upload := func(client string) int {
		r := httptest.NewRequest(http.MethodPost, "http://lobby/api/v1/packs", strings.NewReader("invalid multipart"))
		r.RemoteAddr = "127.0.0.1:48000"
		r.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	for i := 0; i < 2; i++ {
		if status := upload("203.0.113.10"); status != http.StatusBadRequest {
			t.Fatalf("first user's allowed attempt got %d", status)
		}
	}
	if status := upload("203.0.113.10"); status != http.StatusTooManyRequests {
		t.Fatalf("first user's upload rate was not bounded: %d", status)
	}
	if status := upload("2001:db8::20"); status != http.StatusBadRequest {
		t.Fatalf("different user inherited shared gateway rate: %d", status)
	}
	// Creation uses the same resolved identity and shared expensive-operation
	// budget, so an exhausted uploader cannot evade it via the rooms endpoint.
	body, _ := json.Marshal(testRequest(t, "Limited room", "E1M1", "coop"))
	r := httptest.NewRequest(http.MethodPost, "http://lobby/api/v1/rooms", bytes.NewReader(body))
	r.RemoteAddr = "127.0.0.1:49000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Forwarded-For", "203.0.113.10")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("creation ignored forwarded identity: %d %s", w.Code, w.Body.String())
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.rates) != 2 || manager.rates["2001:db8::20"].tokens < .99 || manager.rates["203.0.113.10"].tokens >= 1 {
		t.Fatal("requests did not use separate canonical client rate buckets")
	}
	// The second user retains one independent creation token.
	if !manager.allowCreateLocked("2001:db8::20", time.Now()) {
		t.Fatal("second client lost its independent remaining token")
	}
}
