package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gddoom/internal/lobby"
	"gddoom/internal/runtimecfg"
)

func TestAuthorityLobbyCreatesOnlyLoadedContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.wad")
	data := []byte("loaded WAD bytes")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	pack := lobby.Pack{ID: "custom", Name: "Custom", WADHashes: []string{hash}, Maps: []string{"E1M1"}}
	settings := lobby.Settings{PackID: pack.ID, Map: "E1M1", Mode: "coop", Skill: 3, PlayerLimit: 4}
	manifest, err := lobby.ValidateSettings(settings, pack)
	if err != nil {
		t.Fatal(err)
	}
	room := lobby.Room{ID: "room1", Name: "Our game", State: "ready", Address: "ws://127.0.0.1:1234/rooms/room1/netplay", Settings: settings, Manifest: manifest, PlayerLimit: 4, CreatedAt: time.Now().UTC()}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/lobby":
			_ = json.NewEncoder(w).Encode(lobby.State{Version: lobby.APIVersion, MaxRooms: 8, Packs: []lobby.Pack{pack}})
		case "/api/v1/rooms":
			posts++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(room)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var opts runtimecfg.Options
	if err := configureAuthorityLobby(&opts, []string{path}, server.URL); err != nil {
		t.Fatal(err)
	}
	request := lobby.CreateRequest{RequestID: strings.Repeat("a", 32), Name: room.Name, Settings: settings}
	got, err := opts.AuthorityCreateGame(context.Background(), server.URL, request)
	if err != nil || got.ID != room.ID || posts != 1 {
		t.Fatalf("create = %v, %v, posts=%d", got, err, posts)
	}
	// Neither mutable UI identity nor a changed catalog may authorize a process
	// with content different from the client's originally loaded world.
	opts.AuthorityWADHashes[0] = strings.Repeat("b", 64)
	pack.WADHashes = slices.Clone(opts.AuthorityWADHashes)
	_, err = opts.AuthorityCreateGame(context.Background(), server.URL, request)
	if err == nil || !strings.Contains(err.Error(), "matching WADs") || posts != 1 {
		t.Fatalf("mismatch = %v, posts=%d", err, posts)
	}
	pack.ID = "replacement"
	_, err = opts.AuthorityCreateGame(context.Background(), server.URL, request)
	if err == nil || !strings.Contains(err.Error(), "no longer") || posts != 1 {
		t.Fatalf("removed pack = %v, posts=%d", err, posts)
	}
}

func TestLoadedWADUploadPreservesOrderAndRejectsChangedContent(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "base.wad"), filepath.Join(dir, "overlay.wad")}
	contents := []string{"base data", "overlay data"}
	hashes := make([]string, len(paths))
	for i, path := range paths {
		if err := os.WriteFile(path, []byte(contents[i]), 0600); err != nil {
			t.Fatal(err)
		}
		hashes[i] = fmt.Sprintf("%x", sha256.Sum256([]byte(contents[i])))
	}
	request, err := loadedWADUpload(context.Background(), "Our content", paths, hashes)
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Files) != 2 || request.Name != "Our content" {
		t.Fatalf("upload = %+v", request)
	}
	for i, file := range request.Files {
		data, err := io.ReadAll(file.Reader)
		if err != nil || string(data) != contents[i] || file.SHA256 != hashes[i] || file.Name != filepath.Base(paths[i]) || file.Size != int64(len(data)) {
			t.Fatalf("file %d = %+v, %q, %v", i, file, data, err)
		}
	}
	if err := os.WriteFile(paths[1], []byte("edited overlay"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadedWADUpload(context.Background(), "Our content", paths, hashes); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed content = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loadedWADUpload(ctx, "Our content", paths, hashes); err != context.Canceled {
		t.Fatalf("canceled = %v", err)
	}
}

func TestAuthorityLobbyOptionalAndAddressValidation(t *testing.T) {
	var opts runtimecfg.Options
	if err := configureAuthorityLobby(&opts, nil, "  "); err != nil || opts.AuthorityLobby != nil {
		t.Fatalf("disabled = %v", err)
	}
	if err := configureAuthorityLobby(&opts, nil, "wss://example.test/netplay"); err == nil {
		t.Fatal("accepted a gameplay URL as lobby")
	}
}
