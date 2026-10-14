package roomhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gddoom/internal/lobby"
	"gddoom/internal/wad"
)

func uploadBase(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "DOOM1.WAD"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func hashBytes(data []byte) string { hash := sha256.Sum256(data); return hex.EncodeToString(hash[:]) }

func emptyAddon() []byte {
	data := make([]byte, 28)
	copy(data, "PWAD")
	binary.LittleEndian.PutUint32(data[4:], 1)
	binary.LittleEndian.PutUint32(data[8:], 12)
	copy(data[20:], "CUSTOM")
	return data
}

func replacementMap(t *testing.T, base []byte) []byte {
	t.Helper()
	file, err := wad.OpenData("base.wad", base)
	if err != nil {
		t.Fatal(err)
	}
	start := -1
	for i, lump := range file.Lumps {
		if lump.Name == "E1M1" {
			start = i
			break
		}
	}
	if start < 0 || start+11 > len(file.Lumps) {
		t.Fatal("missing original map")
	}
	data := make([]byte, 12)
	copy(data, "PWAD")
	directory := make([]byte, 11*16)
	for i, lump := range file.Lumps[start : start+11] {
		entry := directory[i*16:]
		binary.LittleEndian.PutUint32(entry, uint32(len(data)))
		binary.LittleEndian.PutUint32(entry[4:], uint32(lump.Size))
		copy(entry[8:], lump.Name)
		data = append(data, base[lump.FilePos:lump.FilePos+lump.Size]...)
	}
	binary.LittleEndian.PutUint32(data[4:], 11)
	binary.LittleEndian.PutUint32(data[8:], uint32(len(data)))
	return append(data, directory...)
}

func uploadEntries(t *testing.T, directory string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var packs []os.DirEntry
	for _, entry := range entries {
		if entry.Name() != ".lock" {
			packs = append(packs, entry)
		}
	}
	return packs
}

func uploadRequest(data ...[]byte) lobby.UploadRequest {
	upload := lobby.UploadRequest{Name: "Custom stack"}
	for i, data := range data {
		upload.Files = append(upload.Files, lobby.UploadFile{Name: fmt.Sprintf("resource%d.wad", i), Size: int64(len(data)), SHA256: hashBytes(data), Reader: bytes.NewReader(data)})
	}
	return upload
}

func TestUploadCompleteStackDeduplicationPersistenceAndQuota(t *testing.T) {
	base, addon := uploadBase(t), emptyAddon()
	directory := t.TempDir()
	config := Config{UploadDir: directory, UploadQuota: int64(len(base) + len(addon)), Packs: testPacks(t)}
	manager, server := testManager(t, config)
	pack, err := lobby.Upload(context.Background(), server.URL, uploadRequest(base, addon))
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.WADHashes) != 2 || pack.WADHashes[0] != hashBytes(base) || pack.WADHashes[1] != hashBytes(addon) || len(pack.Maps) == 0 {
		t.Fatalf("ordered upload identity: %+v", pack)
	}
	duplicate, err := lobby.Upload(context.Background(), server.URL, uploadRequest(base, addon))
	if err != nil || duplicate.ID != pack.ID {
		t.Fatalf("exact stack retry did not deduplicate: %+v %v", duplicate, err)
	}
	if manager.uploadBytes != int64(len(base)+len(addon)) {
		t.Fatal("duplicate consumed disk quota")
	}
	files := uploadEntries(t, directory)
	if len(files) != 1 || files[0].Name() != pack.ID {
		t.Fatalf("unexpected upload paths: %v", files)
	}
	manager.mu.Lock()
	stored := manager.packs[pack.ID]
	manager.mu.Unlock()
	for i, path := range stored.Paths {
		if filepath.Base(path) != fmt.Sprintf("%d.wad", i) {
			t.Fatal("client filename leaked into storage path")
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0400 {
			t.Fatal("committed upload is not immutable to ordinary writes")
		}
	}
	manager.Close()
	reopened, _ := testManager(t, config)
	if len(reopened.State().Packs) != 2 || reopened.uploadBytes != manager.uploadBytes {
		t.Fatal("committed catalog was not restored after restart")
	}
	// Changing bytes changes identity and must require new quota.
	addon[len(addon)-1] = 'X'
	reopened.mu.Lock()
	reopened.rates = make(map[string]creationRate)
	reopened.mu.Unlock()
	request, body := rawUpload(t, uploadRequest(base, addon))
	response := httptestUpload(reopened, request, body)
	if response.status != http.StatusInsufficientStorage {
		t.Fatalf("quota overflow accepted: %d %s", response.status, response.body)
	}
	files = uploadEntries(t, directory)
	if len(files) != 1 {
		t.Fatal("quota rejection left unpublished content")
	}
}

type uploadResponse struct {
	status int
	body   string
}

func rawUpload(t *testing.T, request lobby.UploadRequest) (*http.Request, []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	metadata := lobby.UploadMetadata{Name: request.Name}
	for _, file := range request.Files {
		metadata.Files = append(metadata.Files, lobby.UploadFileMetadata{Name: file.Name, Size: file.Size, SHA256: file.SHA256})
	}
	part, _ := writer.CreateFormField("metadata")
	if err := json.NewEncoder(part).Encode(metadata); err != nil {
		t.Fatal(err)
	}
	for i, file := range request.Files {
		part, _ = writer.CreateFormFile(fmt.Sprintf("file%d", i), file.Name)
		if _, err := io.Copy(part, file.Reader); err != nil {
			t.Fatal(err)
		}
	}
	writer.Close()
	r, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/packs", bytes.NewReader(body.Bytes()))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return r, body.Bytes()
}

func httptestUpload(m *Manager, r *http.Request, body []byte) uploadResponse {
	r.Body = io.NopCloser(bytes.NewReader(body))
	w := &responseCapture{header: make(http.Header)}
	m.Handler().ServeHTTP(w, r)
	return uploadResponse{w.status, w.body.String()}
}

type responseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *responseCapture) Header() http.Header    { return w.header }
func (w *responseCapture) WriteHeader(status int) { w.status = status }
func (w *responseCapture) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(b)
}

func TestUploadRejectsCorruptionAndAllocationBombsWithoutPublishing(t *testing.T) {
	base, addon := uploadBase(t), emptyAddon()
	for _, kind := range []string{"digest", "size", "directory", "geometry", "no-maps", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			manager, _ := testManager(t, Config{UploadDir: directory})
			request := uploadRequest(base, addon)
			switch kind {
			case "digest":
				request.Files[1].SHA256 = strings.Repeat("0", 64)
			case "size":
				request.Files[1].Size++
			case "directory":
				bad := emptyAddon()
				binary.LittleEndian.PutUint32(bad[4:], 100001)
				request = uploadRequest(base, bad)
			case "geometry":
				// A valid map replacement with too many sectors and empty REJECT
				// must be rejected by the upload path before LoadMap synthesizes it.
				bad := replacementMap(t, base)
				directoryAt := int(binary.LittleEndian.Uint32(bad[8:]))
				directory := bytes.Clone(bad[directoryAt:])
				sectors := make([]byte, (maxUploadSectors+1)*26)
				binary.LittleEndian.PutUint32(directory[8*16:], uint32(directoryAt))
				binary.LittleEndian.PutUint32(directory[8*16+4:], uint32(len(sectors)))
				binary.LittleEndian.PutUint32(directory[9*16+4:], 0)
				bad = append(bad[:directoryAt], sectors...)
				binary.LittleEndian.PutUint32(bad[8:], uint32(len(bad)))
				request = uploadRequest(base, append(bad, directory...))
			case "no-maps":
				request = uploadRequest(addon)
			case "oversize":
				request.Files[1].Size = lobby.MaxUploadFileBytes + 1
			}
			r, body := rawUpload(t, request)
			response := httptestUpload(manager, r, body)
			if response.status != http.StatusBadRequest {
				t.Fatalf("bad upload accepted: %d %s", response.status, response.body)
			}
			if len(manager.State().Packs) != 1 || manager.uploadBytes != 0 {
				t.Fatal("rejected upload changed catalog/quota")
			}
			files := uploadEntries(t, directory)
			if len(files) != 0 {
				t.Fatalf("rejected upload left files: %v", files)
			}
		})
	}
}

func TestUploadReplacementMapAndExclusiveStorage(t *testing.T) {
	base := uploadBase(t)
	addon := replacementMap(t, base)
	directory := t.TempDir()
	manager, server := testManager(t, Config{UploadDir: directory})
	pack, err := lobby.Upload(context.Background(), server.URL, uploadRequest(base, addon))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, name := range pack.Maps {
		if name == "E1M1" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("overridden map should be listed once, got %d in %+v", count, pack.Maps)
	}
	config := manager.config
	config.Packs = testPacks(t)
	if second, err := New(context.Background(), config); err == nil {
		second.Close()
		t.Fatal("two supervisors acquired the same upload storage")
	}
	// The installed catalog must accept the same normal PWAD replacement.
	basePath := filepath.Join(t.TempDir(), "base.wad")
	addonPath := filepath.Join(filepath.Dir(basePath), "custom.wad")
	if err := os.WriteFile(basePath, base, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(addonPath, addon, 0600); err != nil {
		t.Fatal(err)
	}
	catalogData, _ := json.Marshal(Catalog{Packs: []CatalogPack{{ID: "replacement", Name: "Custom", WADs: []string{"base.wad", "custom.wad"}}}})
	path := filepath.Join(filepath.Dir(basePath), "catalog.json")
	if err := os.WriteFile(path, catalogData, 0600); err != nil {
		t.Fatal(err)
	}
	packs, err := LoadCatalog(path)
	if err != nil || len(packs) != 1 || len(packs[0].Pack.Maps) != len(pack.Maps) {
		t.Fatalf("replacement map catalog: %+v %v", packs, err)
	}
}

func TestUploadShutdownInterruptsStalledHTTPBody(t *testing.T) {
	manager, server := testManager(t, Config{UploadDir: t.TempDir()})
	connection, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	// Declare a long multipart body and stop mid-metadata. Closing the body
	// alone cannot interrupt net/http's blocked socket read.
	_, err = fmt.Fprintf(connection, "POST /api/v1/packs HTTP/1.1\r\nHost: %s\r\nContent-Type: multipart/form-data; boundary=stalled\r\nContent-Length: 1000000\r\n\r\n--stalled\r\nContent-Disposition: form-data; name=\"metadata\"\r\n\r\n{", server.Listener.Addr())
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return len(manager.uploadSlot) == 1 })
	done := make(chan struct{})
	go func() { manager.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on stalled upload")
	}
	if len(manager.uploadSlot) != 0 {
		t.Fatal("canceled upload did not release admission")
	}
}
