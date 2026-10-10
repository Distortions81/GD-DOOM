package roomhost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gddoom/internal/lobby"
)

func approveFile(data []byte, name string) RedistributionApproval {
	return RedistributionApproval{SHA256: hashBytes(data), Name: name, Allow: true, License: "Operator-approved test distribution", Source: "Test fixture"}
}

func downloadRequest(t *testing.T, client *http.Client, method, address, origin string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, address, nil)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func TestDownloadPrivateByDefaultAndExactOriginRoutes(t *testing.T) {
	base := uploadBase(t)
	private, privateServer := testManager(t, Config{})
	file := private.State().Packs[0].Files[0]
	if file.Downloadable || file.Size != int64(len(base)) || file.SHA256 != hashBytes(base) {
		t.Fatalf("private metadata mismatch: %+v", file)
	}
	response := downloadRequest(t, privateServer.Client(), http.MethodGet, privateServer.URL+"/api/v1/content/"+file.SHA256, "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("guessed private hash was served: %d", response.StatusCode)
	}
	approved, server := testManager(t, Config{Redistribution: []RedistributionApproval{approveFile(base, "Approved.wad")}, WebOrigins: []string{"https://play.example"}})
	state, err := lobby.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	file = state.Packs[0].Files[0]
	if !file.Downloadable || file.Name != "Approved.wad" || file.License == "" || file.Source == "" {
		t.Fatalf("operator policy not advertised: %+v", file)
	}
	address := server.URL + "/api/v1/content/" + file.SHA256
	response = downloadRequest(t, server.Client(), http.MethodGet, address, "https://play.example")
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !bytes.Equal(data, base) || response.ContentLength != int64(len(base)) {
		t.Fatalf("approved bytes mismatch: status=%d size=%d err=%v", response.StatusCode, len(data), err)
	}
	if response.Header.Get("Access-Control-Allow-Origin") != "https://play.example" || response.Header.Get("Content-Type") != "application/octet-stream" || response.Header.Get("ETag") != `"`+file.SHA256+`"` {
		t.Fatal("missing download identity/CORS headers")
	}
	verified, err := lobby.Download(context.Background(), server.URL, file)
	if err != nil || !bytes.Equal(verified, base) {
		t.Fatalf("shared client failed to verify bytes: %v", err)
	}
	response = downloadRequest(t, server.Client(), http.MethodHead, address, "https://play.example")
	data, err = io.ReadAll(response.Body)
	if err != nil || len(data) != 0 || response.ContentLength != file.Size || response.StatusCode != http.StatusOK {
		t.Fatal("HEAD did not preserve length without body")
	}
	for _, test := range []struct {
		path, origin string
		status       int
	}{
		{"/api/v1/content/" + file.SHA256, "https://untrusted.example", http.StatusForbidden},
		{"/api/v1/content/" + strings.Repeat("f", 64), "", http.StatusNotFound},
		{"/api/v1/content/" + file.SHA256 + "/extra", "", http.StatusNotFound},
		{"/api/v1/content/" + file.SHA256 + "?path=DOOM1.WAD", "", http.StatusNotFound},
		{"/api/v1/content/%2f" + file.SHA256, "", http.StatusNotFound},
	} {
		response := downloadRequest(t, server.Client(), http.MethodGet, server.URL+test.path, test.origin)
		if response.StatusCode != test.status {
			t.Fatalf("%s returned %d, want %d", test.path, response.StatusCode, test.status)
		}
	}
	if approved.downloadBytes != int64(len(base)) {
		t.Fatal("repeated downloads changed disk allocation")
	}
}

func TestDownloadSnapshotsDeduplicateAndSurviveSourceChanges(t *testing.T) {
	base := uploadBase(t)
	source := filepath.Join(t.TempDir(), "source.wad")
	if err := os.WriteFile(source, base, 0600); err != nil {
		t.Fatal(err)
	}
	packs := testPacks(t)
	packs[0].Paths[0] = source
	second := packs[0]
	second.Pack = clonePack(second.Pack)
	second.Pack.ID = "second"
	packs = append(packs, second)
	cacheParent := t.TempDir()
	manager, server := testManager(t, Config{Packs: packs, TempDir: cacheParent, Redistribution: []RedistributionApproval{approveFile(base, "Approved.wad")}, DownloadQuota: int64(len(base))})
	if manager.downloadBytes != int64(len(base)) || len(manager.downloads) != 1 {
		t.Fatal("same approved file was copied more than once")
	}
	if manager.config.Packs[0].Paths[0] == source || manager.config.Packs[0].Paths[0] != manager.config.Packs[1].Paths[0] {
		t.Fatal("workers do not share the same immutable approved bytes")
	}
	if err := os.WriteFile(source, []byte("changed original"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := lobby.Download(context.Background(), server.URL, manager.State().Packs[0].Files[0])
	if err != nil || !bytes.Equal(data, base) {
		t.Fatal("changing source changed approved bytes")
	}
	cache := manager.downloadDir
	manager.Close()
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("manager did not remove its private download cache")
	}
}

func TestDownloadUploadedPolicyAppliedOnPublishAndRestart(t *testing.T) {
	base, addon := uploadBase(t), emptyAddon()
	directory := t.TempDir()
	config := Config{UploadDir: directory, Redistribution: []RedistributionApproval{approveFile(addon, "Addon.wad")}}
	manager, server := testManager(t, config)
	pack, err := lobby.Upload(context.Background(), server.URL, uploadRequest(base, addon))
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Files) != 2 || pack.Files[0].Downloadable || !pack.Files[1].Downloadable {
		t.Fatalf("per-file upload permissions lost: %+v", pack.Files)
	}
	data, err := lobby.Download(context.Background(), server.URL, pack.Files[1])
	if err != nil || !bytes.Equal(data, addon) {
		t.Fatalf("approved uploaded file unavailable: %v", err)
	}
	response := downloadRequest(t, server.Client(), http.MethodGet, server.URL+"/api/v1/content/"+pack.Files[0].SHA256, "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatal("private base became public because its overlay was approved")
	}
	manager.Close()
	// A persisted true flag is never an authority: removing the operator rule
	// revokes availability after restart, including a guessed content endpoint.
	config.Redistribution = nil
	reopened, next := testManager(t, config)
	for _, current := range reopened.State().Packs {
		for _, file := range current.Files {
			if file.Downloadable {
				t.Fatal("persisted upload granted redistribution after rule was removed")
			}
		}
	}
	response = downloadRequest(t, next.Client(), http.MethodGet, next.URL+"/api/v1/content/"+pack.Files[1].SHA256, "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatal("revoked uploaded hash remains available")
	}
	reopened.Close()
	config.Redistribution = []RedistributionApproval{approveFile(addon, "Addon.wad")}
	restored, last := testManager(t, config)
	var approved lobby.PackFile
	for _, current := range restored.State().Packs {
		if current.ID == pack.ID {
			approved = current.Files[1]
		}
	}
	data, err = lobby.Download(context.Background(), last.URL, approved)
	if err != nil || !bytes.Equal(data, addon) {
		t.Fatalf("new approval was not applied to existing private upload: %v", err)
	}
}

func TestDownloadCatalogPolicyAndQuotaRejectInvalidSources(t *testing.T) {
	base := uploadBase(t)
	packs := testPacks(t)
	approval := approveFile(base, "Approved.wad")
	catalog := Catalog{Packs: []CatalogPack{{ID: "doom", Name: "Doom", WADs: packs[0].Paths}}, Redistribution: []RedistributionApproval{approval}}
	data, _ := json.Marshal(catalog)
	filename := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, policy, err := LoadCatalogWithPolicy(filename)
	if err != nil || len(policy) != 1 || policy[0] != approval || len(loaded) != 1 {
		t.Fatalf("operator approval catalog failed: %v", err)
	}
	for _, kind := range []string{"quota", "changed-source", "invalid-name", "duplicate-hash"} {
		t.Run(kind, func(t *testing.T) {
			config := Config{WorkerPath: os.Args[0], PublicURL: "http://127.0.0.1", Packs: testPacks(t), Redistribution: []RedistributionApproval{approval}, TempDir: t.TempDir()}
			switch kind {
			case "quota":
				config.DownloadQuota = int64(len(base) - 1)
			case "changed-source":
				path := filepath.Join(t.TempDir(), "changed.wad")
				if err := os.WriteFile(path, []byte("different bytes"), 0600); err != nil {
					t.Fatal(err)
				}
				config.Packs[0].Paths[0] = path
			case "invalid-name":
				config.Redistribution[0].Name = "../private.wad"
			case "duplicate-hash":
				config.Redistribution = append(config.Redistribution, approval)
			}
			manager, err := New(context.Background(), config)
			if err == nil {
				manager.Close()
				t.Fatalf("accepted invalid %s", kind)
			}
			entries, err := os.ReadDir(config.TempDir)
			if err != nil || len(entries) != 0 {
				t.Fatal("failed initialization left download copies")
			}
		})
	}
}

func TestDownloadConcurrencyAndStalledTransferShutdown(t *testing.T) {
	base := uploadBase(t)
	data := append(bytes.Clone(base), make([]byte, (8<<20)-len(base))...)
	filename := filepath.Join(t.TempDir(), "large.wad")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		t.Fatal(err)
	}
	packs := testPacks(t)
	packs[0].Paths[0], packs[0].Pack.WADHashes[0] = filename, hashBytes(data)
	manager, server := testManager(t, Config{Packs: packs, Redistribution: []RedistributionApproval{approveFile(data, "Large.wad")}})
	var sockets []*net.TCPConn
	defer func() {
		for _, conn := range sockets {
			conn.Close()
		}
	}()
	for i := 0; i < 4; i++ {
		conn, err := net.Dial("tcp", server.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		tcp := conn.(*net.TCPConn)
		_ = tcp.SetReadBuffer(1024)
		sockets = append(sockets, tcp)
		if _, err := fmt.Fprintf(conn, "GET /api/v1/content/%s HTTP/1.1\r\nHost: localhost\r\n\r\n", hashBytes(data)); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, time.Second, func() bool { return len(manager.downloadSlots) == 4 })
	response := downloadRequest(t, server.Client(), http.MethodGet, server.URL+"/api/v1/content/"+hashBytes(data), "")
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("download concurrency limit not enforced")
	}
	done := make(chan struct{})
	go func() { manager.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on clients that stopped reading")
	}
	if len(manager.downloadSlots) != 0 {
		t.Fatal("shutdown retained download slots")
	}
}
