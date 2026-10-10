package lobby

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testDownloadFile(data []byte) PackFile {
	return PackFile{Name: "overlay.wad", Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Downloadable: true, License: "CC0-1.0", Source: "https://example.test/overlay"}
}

func testDownloadPack() Pack {
	pack := testPack()
	pack.Files = []PackFile{
		{Name: "base.wad", Size: 1024, SHA256: pack.WADHashes[0]},
		{Name: "overlay.wad", Size: 512, SHA256: pack.WADHashes[1], Downloadable: true},
	}
	return pack
}

func TestDownloadPackMetadataPreservesPrivateBaseAndOrderedOverlay(t *testing.T) {
	pack := testDownloadPack()
	if err := ValidateDownloadPack(pack); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Pack
	if err := decodeStrict(wire, &decoded); err != nil || !reflect.DeepEqual(decoded, pack) {
		t.Fatalf("metadata round trip changed ordered content: %+v, %v", decoded, err)
	}
	pack.Files[0], pack.Files[1] = pack.Files[1], pack.Files[0]
	if err := ValidatePack(pack); err == nil {
		t.Fatal("reordered metadata accepted")
	}
	pack = testPack()
	if err := ValidatePack(pack); err != nil {
		t.Fatal("legacy catalog rejected:", err)
	}
	if err := ValidateDownloadPack(pack); err == nil {
		t.Fatal("automatic loading accepted absent file metadata")
	}
}

func TestDownloadMetadataAndStackBounds(t *testing.T) {
	for name, change := range map[string]func(*Pack){
		"partial metadata":       func(p *Pack) { p.Files = p.Files[:1] },
		"path name":              func(p *Pack) { p.Files[0].Name = "../base.wad" },
		"control name":           func(p *Pack) { p.Files[0].Name = "base\nwad" },
		"zero size":              func(p *Pack) { p.Files[0].Size = 0 },
		"negative size":          func(p *Pack) { p.Files[0].Size = -1 },
		"oversize approved file": func(p *Pack) { p.Files[1].Size = MaxDownloadFileBytes + 1 },
		"uppercase hash":         func(p *Pack) { p.Files[0].SHA256 = strings.Repeat("A", 64) },
		"hash mismatch":          func(p *Pack) { p.Files[0].SHA256 = strings.Repeat("c", 64) },
		"license control":        func(p *Pack) { p.Files[0].License = "MIT\n" },
		"license long":           func(p *Pack) { p.Files[0].License = strings.Repeat("x", 257) },
		"source long":            func(p *Pack) { p.Files[0].Source = strings.Repeat("x", 513) },
	} {
		t.Run(name, func(t *testing.T) {
			pack := testDownloadPack()
			change(&pack)
			if err := ValidatePack(pack); err == nil {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
	pack := testDownloadPack()
	pack.Files[0].Size = math.MaxInt64
	if err := ValidatePack(pack); err != nil {
		t.Fatal("large installed private content should remain discoverable:", err)
	}
	if err := ValidateDownloadPack(pack); err == nil {
		t.Fatal("oversize private base accepted for automatic loading")
	}
	pack = testDownloadPack()
	for len(pack.Files) < 64 {
		pack.Files = append(pack.Files, pack.Files[0])
		pack.WADHashes = append(pack.WADHashes, pack.WADHashes[0])
	}
	if err := ValidatePack(pack); err != nil {
		t.Fatal("legacy 64-file installed stack rejected:", err)
	}
	if err := ValidateDownloadPack(pack); err == nil {
		t.Fatal("64-file stack accepted for automatic loading")
	}
	pack = testDownloadPack()
	pack.Files[0].Size, pack.Files[1].Size = MaxDownloadFileBytes, MaxDownloadFileBytes
	if err := ValidateDownloadPack(pack); err != nil {
		t.Fatal("exact stack limit rejected:", err)
	}
	pack.Files = append(pack.Files, PackFile{Name: "extra.wad", Size: 1, SHA256: pack.WADHashes[0]})
	pack.WADHashes = append(pack.WADHashes, pack.WADHashes[0])
	if err := ValidateDownloadPack(pack); err == nil {
		t.Fatal("aggregate stack limit ignored")
	}
}

func TestDownloadUsesLobbyHashEndpointAndVerifiesBytes(t *testing.T) {
	data := []byte("PWAD: exact downloadable overlay bytes")
	file := testDownloadFile(data)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/nested/api/v1/content/"+file.SHA256 || r.Method != http.MethodGet || r.Header.Get("Accept") != "application/octet-stream" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	// Even an informational external source never supplies the download URL.
	file.Source = "https://unrelated.invalid/file.wad"
	got, err := Download(context.Background(), server.URL+"/nested/", file)
	if err != nil || !bytes.Equal(got, data) || requests.Load() != 1 {
		t.Fatalf("download got %q, %v (%d requests)", got, err, requests.Load())
	}
	file.Downloadable = false
	if data, err := Download(context.Background(), server.URL, file); err == nil || data != nil || requests.Load() != 1 {
		t.Fatal("private WAD was requested")
	}
}

func TestDownloadRejectsMismatchesAndRefusals(t *testing.T) {
	data := []byte("PWAD content")
	for _, tc := range []struct {
		name, body, contentType string
		status                  int
		chunked                 bool
	}{
		{name: "wrong hash", body: "BAD! content", contentType: "application/octet-stream", status: 200},
		{name: "short declared body", body: "short", contentType: "application/octet-stream", status: 200},
		{name: "short streamed body", body: "short", contentType: "application/octet-stream", status: 200, chunked: true},
		{name: "long streamed body", body: "PWAD contentX", contentType: "application/octet-stream", status: 200, chunked: true},
		{name: "HTML", body: string(data), contentType: "text/html", status: 200},
		{name: "partial response", body: string(data), contentType: "application/octet-stream", status: 206},
		{name: "revoked approval", body: "private server details", contentType: "text/html", status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				if tc.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			got, err := Download(context.Background(), server.URL, testDownloadFile(data))
			if err == nil || got != nil || strings.Contains(err.Error(), "private server details") {
				t.Fatalf("invalid response leaked bytes: %q, %v", got, err)
			}
		})
	}
}

func TestDownloadRejectsRedirectsAndCanceledRequests(t *testing.T) {
	data := []byte("PWAD")
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetRequests.Add(1) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	if got, err := Download(context.Background(), redirect.URL, testDownloadFile(data)); err == nil || got != nil || targetRequests.Load() != 0 {
		t.Fatal("download followed redirect")
	}
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		got, err := Download(ctx, server.URL, testDownloadFile(data))
		if got != nil {
			result <- errors.New("canceled request returned bytes")
			return
		}
		result <- err
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation not propagated:", err)
		}
	case <-time.After(time.Second):
		t.Fatal("download did not cancel")
	}
	if got, err := Download(ctx, "bad address", PackFile{}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("pre-canceled request not rejected")
	}
}
