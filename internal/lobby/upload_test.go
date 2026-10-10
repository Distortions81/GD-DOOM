package lobby

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func testUploadFile(name string, data []byte) UploadFile {
	hash := sha256.Sum256(data)
	return UploadFile{Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:]), Reader: bytes.NewReader(data)}
}

func testUploadRequest() UploadRequest {
	return UploadRequest{Name: "Custom campaign", Files: []UploadFile{testUploadFile("base.wad", []byte("base WAD data")), testUploadFile("overlay.wad", []byte("overlay WAD data"))}}
}

func uploadMetadata(request UploadRequest) UploadMetadata {
	metadata := UploadMetadata{Name: request.Name}
	for _, file := range request.Files {
		metadata.Files = append(metadata.Files, UploadFileMetadata{Name: file.Name, Size: file.Size, SHA256: file.SHA256})
	}
	return metadata
}

func uploadedPack(request UploadRequest) Pack {
	pack := Pack{ID: "custom-pack", Name: request.Name, Maps: []string{"E1M1"}}
	for _, file := range request.Files {
		pack.WADHashes = append(pack.WADHashes, file.SHA256)
	}
	return pack
}

func TestUploadMetadataBoundsAndIdentity(t *testing.T) {
	for name, change := range map[string]func(*UploadMetadata){
		"empty name":        func(m *UploadMetadata) { m.Name = "" },
		"empty files":       func(m *UploadMetadata) { m.Files = nil },
		"many files":        func(m *UploadMetadata) { m.Files = make([]UploadFileMetadata, MaxUploadFiles+1) },
		"absolute filename": func(m *UploadMetadata) { m.Files[0].Name = "/tmp/map.wad" },
		"parent filename":   func(m *UploadMetadata) { m.Files[0].Name = "../map.wad" },
		"windows filename":  func(m *UploadMetadata) { m.Files[0].Name = `dir\map.wad` },
		"control filename":  func(m *UploadMetadata) { m.Files[0].Name = "map\n.wad" },
		"zero bytes":        func(m *UploadMetadata) { m.Files[0].Size = 0 },
		"negative bytes":    func(m *UploadMetadata) { m.Files[0].Size = -1 },
		"oversized file":    func(m *UploadMetadata) { m.Files[0].Size = MaxUploadFileBytes + 1 },
		"oversized stack": func(m *UploadMetadata) {
			m.Files[0].Size, m.Files[1].Size = MaxUploadFileBytes, MaxUploadFileBytes
			m.Files = append(m.Files, m.Files[0])
		},
		"invalid hash":   func(m *UploadMetadata) { m.Files[0].SHA256 = "not-a-hash" },
		"uppercase hash": func(m *UploadMetadata) { m.Files[0].SHA256 = strings.Repeat("A", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			metadata := uploadMetadata(testUploadRequest())
			change(&metadata)
			if err := ValidateUploadMetadata(metadata); err == nil {
				t.Fatal("invalid upload metadata accepted")
			}
		})
	}
	metadata := uploadMetadata(testUploadRequest())
	metadata.Files[0].Size, metadata.Files[1].Size = MaxUploadFileBytes, MaxUploadFileBytes
	if err := ValidateUploadMetadata(metadata); err != nil {
		t.Fatalf("exact size limits rejected: %v", err)
	}
}

func TestUploadStreamsMultipartInExactDeclaredOrder(t *testing.T) {
	request := testUploadRequest()
	wantMetadata := uploadMetadata(request)
	wantPack := uploadedPack(request)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/base/api/v1/packs" {
			t.Errorf("unexpected upload route: %s %s", r.Method, r.URL.Path)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "metadata" || part.FileName() != "" {
			t.Errorf("first part must be metadata: %v", err)
			return
		}
		var metadata UploadMetadata
		if err := json.NewDecoder(part).Decode(&metadata); err != nil || !reflect.DeepEqual(metadata, wantMetadata) {
			t.Errorf("upload metadata got %+v, err=%v", metadata, err)
		}
		for i, file := range request.Files {
			part, err := reader.NextPart()
			if err != nil || part.FormName() != fmt.Sprintf("file%d", i) || part.FileName() != file.Name {
				t.Errorf("file %d order/name: %v", i, err)
				return
			}
			data, err := io.ReadAll(part)
			hash := sha256.Sum256(data)
			if err != nil || int64(len(data)) != file.Size || hex.EncodeToString(hash[:]) != file.SHA256 {
				t.Errorf("file %d content changed: %v", i, err)
			}
		}
		if _, err := reader.NextPart(); err != io.EOF {
			t.Errorf("unexpected trailing multipart data: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(wantPack)
	}))
	defer server.Close()
	got, err := Upload(context.Background(), server.URL+"/base", request)
	if err != nil || !reflect.DeepEqual(got, wantPack) {
		t.Fatalf("uploaded pack: %+v, err=%v", got, err)
	}
}

func TestUploadRejectsWrongReturnedIdentity(t *testing.T) {
	for _, reordered := range []bool{false, true} {
		t.Run(fmt.Sprint(reordered), func(t *testing.T) {
			request := testUploadRequest()
			pack := uploadedPack(request)
			if reordered {
				pack.WADHashes[0], pack.WADHashes[1] = pack.WADHashes[1], pack.WADHashes[0]
			} else {
				pack.WADHashes[0] = strings.Repeat("c", 64)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(pack)
			}))
			defer server.Close()
			if _, err := Upload(context.Background(), server.URL, request); err == nil {
				t.Fatal("wrong uploaded identity accepted")
			}
		})
	}
}

func TestUploadBodyRejectsChangedOrMislabeledReaders(t *testing.T) {
	for _, test := range []struct {
		name    string
		change  func(*UploadFile)
		message string
	}{
		{"short", func(f *UploadFile) { f.Size++ }, "unexpected EOF"},
		{"long", func(f *UploadFile) { f.Size-- }, "declared file size"},
		{"wrong hash", func(f *UploadFile) { f.SHA256 = strings.Repeat("d", 64) }, "SHA-256"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := testUploadRequest()
			test.change(&request.Files[0])
			body, contentType, err := newUploadBody(context.Background(), uploadMetadata(request), request.Files)
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			_, parameters, _ := mime.ParseMediaType(contentType)
			reader := multipart.NewReader(body, parameters["boundary"])
			for {
				part, nextErr := reader.NextPart()
				if nextErr != nil {
					err = nextErr
					break
				}
				_, err = io.Copy(io.Discard, part)
				if err != nil {
					break
				}
			}
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("invalid reader err=%v want %s", err, test.message)
			}
		})
	}
}

type cancelUploadReader struct {
	started   chan struct{}
	closed    chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func (r *cancelUploadReader) Read([]byte) (int, error) {
	r.startOnce.Do(func() { close(r.started) })
	<-r.closed
	return 0, io.ErrClosedPipe
}

func (r *cancelUploadReader) Close() error {
	r.closeOnce.Do(func() { close(r.closed) })
	return nil
}

func TestUploadCancellationClosesSourceReader(t *testing.T) {
	reader := &cancelUploadReader{started: make(chan struct{}), closed: make(chan struct{})}
	request := testUploadRequest()
	request.Files[0].Reader = reader
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Upload(ctx, server.URL, request)
		done <- err
	}()
	select {
	case <-reader.started:
	case <-time.After(2 * time.Second):
		t.Fatal("upload did not read source")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("upload cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upload cancellation left reader blocked")
	}
	select {
	case <-reader.closed:
	default:
		t.Fatal("upload did not close reader")
	}
}
