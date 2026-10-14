package roomhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"gddoom/internal/lobby"
	"gddoom/internal/netgame"
)

// Called with the manager mutex held so room shutdown cannot race admission.
func (m *Manager) roomTunnel(room *managedRoom) http.Handler {
	ctx, address := room.ctx, room.tcpAddress
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		// Public HTTP proxy requests always carry forwarding headers. The
		// native UDP gateway opens this private hop itself, with no such header.
		if len(request.Header.Values("X-Forwarded-For")) != 0 {
			http.Error(w, "private game route", http.StatusForbidden)
			return
		}
		if !m.allowOrigin(request.Header.Get("Origin")) {
			http.Error(w, "browser origin forbidden", http.StatusForbidden)
			return
		}
		m.mu.Lock()
		if m.closed || room.room.State != "ready" {
			m.mu.Unlock()
			http.NotFound(w, request)
			return
		}
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
		netgame.LocalGameTunnelHandler(ctx, address, m.remoteAddress(request), w, request)
	})
}

func (m *Manager) Handler() http.Handler { return http.HandlerFunc(m.serveHTTP) }

func (m *Manager) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.RawPath != "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/rooms/") {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 4 || parts[3] != "netplay" || (r.Method != http.MethodGet && r.Method != http.MethodConnect) {
			http.NotFound(w, r)
			return
		}
		m.mu.Lock()
		room := m.rooms[parts[2]]
		var proxy http.Handler
		var native http.Handler
		if room != nil && room.room.State == "ready" && !m.closed {
			proxy = room.proxy
			if r.Method == http.MethodConnect {
				native = m.roomTunnel(room)
			}
		}
		m.mu.Unlock()
		if proxy == nil {
			http.NotFound(w, r)
			return
		}
		if native != nil {
			native.ServeHTTP(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
		return
	}
	content := strings.HasPrefix(r.URL.Path, "/api/v1/content/")
	if r.URL.RawQuery != "" || (r.URL.Path != "/api/v1/lobby" && r.URL.Path != "/api/v1/rooms" && r.URL.Path != "/api/v1/packs" && !content) {
		http.NotFound(w, r)
		return
	}
	w.Header().Add("Vary", "Origin")
	if !m.allowOrigin(r.Header.Get("Origin")) {
		writeError(w, reject(http.StatusForbidden, "browser origin is not allowed"))
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "300")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch {
	case content && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		m.downloadHTTP(w, r)
	case r.URL.Path == "/api/v1/lobby" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, m.State())
	case r.URL.Path == "/api/v1/rooms" && r.Method == http.MethodPost:
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			writeError(w, reject(http.StatusUnsupportedMediaType, "room creation requires application/json"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, lobby.MaxRequestBytes)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var request lobby.CreateRequest
		if err := decoder.Decode(&request); err != nil {
			writeError(w, reject(http.StatusBadRequest, "invalid room creation JSON"))
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			writeError(w, reject(http.StatusBadRequest, "room creation requires one JSON object"))
			return
		}
		room, err := m.Create(r.Context(), m.remoteAddress(r), request)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, room)
	case r.URL.Path == "/api/v1/packs" && r.Method == http.MethodPost:
		m.uploadHTTP(w, r)
	default:
		writeError(w, reject(http.StatusMethodNotAllowed, "method is not allowed"))
	}
}

func (m *Manager) allowOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	for _, pattern := range m.config.WebOrigins {
		if match, err := path.Match(strings.ToLower(pattern), strings.ToLower(origin)); err == nil && match {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(value); err != nil || buffer.Len() > lobby.MaxResponseBytes {
		http.Error(w, "lobby response unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buffer.Bytes())
}

func writeError(w http.ResponseWriter, err error) {
	status, message := http.StatusInternalServerError, "lobby request failed"
	var request *requestError
	if errors.As(err, &request) {
		status, message = request.status, request.message
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		status, message = http.StatusRequestTimeout, "request canceled; refresh the lobby before retrying"
	}
	writeJSON(w, status, map[string]string{"error": message})
}
