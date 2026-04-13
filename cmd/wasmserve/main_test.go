package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewHandlerOnlyServesKnownFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(name string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	for _, name := range []string{"index.html", "player.html", "launch.js", "wasm_exec.js", "gddoom.wasm"} {
		writeFile(name)
	}

	handler := newHandler(dir)

	for _, path := range []string{"/", "/index.html", "/player.html", "/launch.js", "/wasm_exec.js", "/gddoom.wasm"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status=%d want=%d", path, rec.Code, http.StatusOK)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/server.go", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /server.go: status=%d want=%d", rec.Code, http.StatusNotFound)
	}

	req = httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("GET /favicon.ico: status=%d want=%d", rec.Code, http.StatusNoContent)
	}
}

func TestNewHandlerServesWASMGzipVariantWhenAccepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gddoom.wasm"), []byte("plain"), 0o644); err != nil {
		t.Fatalf("write gddoom.wasm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gddoom.wasm.gz"), []byte("compressed"), 0o644); err != nil {
		t.Fatalf("write gddoom.wasm.gz: %v", err)
	}

	handler := newHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/gddoom.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /gddoom.wasm: status=%d want=%d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding=%q want gzip", got)
	}
	if got := rec.Header().Values("Vary"); len(got) == 0 || got[0] != "Accept-Encoding" {
		t.Fatalf("Vary=%v want [Accept-Encoding]", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
		t.Fatalf("Cache-Control=%q want no-cache, no-store, must-revalidate", got)
	}
	if body := rec.Body.String(); body != "compressed" {
		t.Fatalf("body=%q want compressed", body)
	}
}

func TestNewHandlerPrefersWASMBrotliVariantWhenAccepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gddoom.wasm"), []byte("plain"), 0o644); err != nil {
		t.Fatalf("write gddoom.wasm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gddoom.wasm.gz"), []byte("gzip"), 0o644); err != nil {
		t.Fatalf("write gddoom.wasm.gz: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gddoom.wasm.br"), []byte("brotli"), 0o644); err != nil {
		t.Fatalf("write gddoom.wasm.br: %v", err)
	}

	handler := newHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/gddoom.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /gddoom.wasm: status=%d want=%d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding=%q want br", got)
	}
	if body := rec.Body.String(); body != "brotli" {
		t.Fatalf("body=%q want brotli", body)
	}
}

func TestAcceptsGzip(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "br, deflate", want: false},
		{value: "gzip, br", want: true},
		{value: "br;q=1.0, gzip;q=0.8", want: true},
		{value: "gzip;q=0", want: false},
	} {
		if got := acceptsGzip(tc.value); got != tc.want {
			t.Fatalf("acceptsGzip(%q)=%v want=%v", tc.value, got, tc.want)
		}
	}
}

func TestPreferredWASMEncoding(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{
		{value: "", want: ""},
		{value: "deflate", want: ""},
		{value: "gzip", want: "gzip"},
		{value: "br", want: "br"},
		{value: "gzip, br", want: "br"},
		{value: "br;q=0, gzip;q=0.8", want: "gzip"},
		{value: "gzip;q=0, br;q=0", want: ""},
	} {
		if got := preferredWASMEncoding(tc.value); got != tc.want {
			t.Fatalf("preferredWASMEncoding(%q)=%q want=%q", tc.value, got, tc.want)
		}
	}
}

func TestHasAppFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"index.html", "player.html", "launch.js", "wasm_exec.js", "gddoom.wasm"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if !hasAppFiles(dir) {
		t.Fatalf("hasAppFiles(%q)=false want true", dir)
	}

	if err := os.Remove(filepath.Join(dir, "gddoom.wasm")); err != nil {
		t.Fatalf("remove gddoom.wasm: %v", err)
	}
	if hasAppFiles(dir) {
		t.Fatalf("hasAppFiles(%q)=true want false after removing gddoom.wasm", dir)
	}
}
