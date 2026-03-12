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

	for _, name := range []string{"index.html", "launch.js", "wasm_exec.js", "gddoom.wasm"} {
		writeFile(name)
	}

	handler := newHandler(dir)

	for _, path := range []string{"/", "/index.html", "/launch.js", "/wasm_exec.js", "/gddoom.wasm"} {
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

func TestHasAppFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"index.html", "launch.js", "wasm_exec.js", "gddoom.wasm"} {
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
