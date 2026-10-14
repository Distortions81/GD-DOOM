package lobby

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	MaxUploadFiles     = 16
	MaxUploadFileBytes = 64 << 20
	MaxUploadBytes     = 128 << 20
)

type UploadFileMetadata struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type UploadMetadata struct {
	Name  string               `json:"name"`
	Files []UploadFileMetadata `json:"files"`
}

type UploadFile struct {
	Name   string
	Size   int64
	SHA256 string
	Reader io.Reader
}

type UploadRequest struct {
	Name  string
	Files []UploadFile
}

func ValidateUploadMetadata(metadata UploadMetadata) error {
	if !validName(metadata.Name) || len(metadata.Files) == 0 || len(metadata.Files) > MaxUploadFiles {
		return errors.New("upload requires a name and an ordered stack of 1–16 WAD files")
	}
	var total int64
	for _, file := range metadata.Files {
		digest, err := hex.DecodeString(file.SHA256)
		if !validName(file.Name) || file.Name == "." || file.Name == ".." || strings.ContainsAny(file.Name, "/\\") || file.Size <= 0 || file.Size > MaxUploadFileBytes || err != nil || len(digest) != sha256.Size || file.SHA256 != strings.ToLower(file.SHA256) {
			return errors.New("upload file requires a plain name, size of 1–64 MiB, and lowercase SHA-256 hash")
		}
		total += file.Size
		if total > MaxUploadBytes {
			return errors.New("uploaded WAD stack exceeds 128 MiB")
		}
	}
	return nil
}

// Upload sends one complete ordered stack (base WAD first, then overlays).
// Readers are consumed once and any io.Closers are closed. Readers must return
// promptly or support cancellation through Close. Native HTTP streams each
// bounded file; Go's browser Fetch transport buffers the complete multipart body.
// Exact content hashes let the service deduplicate retries without request IDs.
func Upload(ctx context.Context, address string, request UploadRequest) (Pack, error) {
	if err := ctx.Err(); err != nil {
		return Pack{}, err
	}
	metadata := UploadMetadata{Name: request.Name, Files: make([]UploadFileMetadata, len(request.Files))}
	for i, file := range request.Files {
		if file.Reader == nil {
			return Pack{}, errors.New("upload file has no reader")
		}
		metadata.Files[i] = UploadFileMetadata{Name: file.Name, Size: file.Size, SHA256: file.SHA256}
	}
	if err := ValidateUploadMetadata(metadata); err != nil {
		return Pack{}, err
	}
	if _, err := NormalizeAddress(address); err != nil {
		return Pack{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	body, contentType, err := newUploadBody(ctx, metadata, request.Files)
	if err != nil {
		return Pack{}, err
	}
	defer body.Close()
	stop := context.AfterFunc(ctx, func() { _ = body.Close() })
	defer stop()
	var pack Pack
	if err := requestWithBody(ctx, address, "/api/v1/packs", http.MethodPost, body, contentType, &pack, 2*time.Minute); err != nil {
		return Pack{}, err
	}
	if err := ValidatePack(pack); err != nil {
		return Pack{}, err
	}
	wanted := make([]string, len(metadata.Files))
	for i, file := range metadata.Files {
		wanted[i] = file.SHA256
	}
	if !slices.Equal(pack.WADHashes, wanted) {
		return Pack{}, errors.New("uploaded pack does not match the ordered WAD hashes")
	}
	return pack, nil
}

type uploadBody struct {
	ctx     context.Context
	reader  io.Reader
	closers []io.Closer
	once    sync.Once
}

func (body *uploadBody) Read(data []byte) (int, error) {
	if err := body.ctx.Err(); err != nil {
		return 0, err
	}
	return body.reader.Read(data)
}

func (body *uploadBody) Close() error {
	body.once.Do(func() {
		for _, closer := range body.closers {
			_ = closer.Close()
		}
	})
	return nil
}

func newUploadBody(ctx context.Context, metadata UploadMetadata, files []UploadFile) (*uploadBody, string, error) {
	var headers bytes.Buffer
	writer := multipart.NewWriter(&headers)
	part, err := writer.CreateFormField("metadata")
	if err != nil {
		return nil, "", err
	}
	data, err := json.Marshal(metadata)
	if err != nil || len(data) > MaxRequestBytes {
		return nil, "", errors.New("upload metadata exceeds size limit")
	}
	if _, err := part.Write(data); err != nil {
		return nil, "", err
	}
	// Multipart boundaries/headers are small owned buffers; WAD bytes remain
	// separate readers and are never copied into an aggregate native buffer.
	readers := make([]io.Reader, 0, len(files)*2+1)
	body := &uploadBody{ctx: ctx}
	for i, file := range files {
		if _, err := writer.CreateFormFile(fmt.Sprintf("file%d", i), file.Name); err != nil {
			return nil, "", err
		}
		readers = append(readers, bytes.NewReader(bytes.Clone(headers.Bytes())), &uploadFileReader{source: file.Reader, remaining: file.Size, expected: file.SHA256, hash: sha256.New()})
		headers.Reset()
		if closer, ok := file.Reader.(io.Closer); ok {
			body.closers = append(body.closers, closer)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	readers = append(readers, bytes.NewReader(bytes.Clone(headers.Bytes())))
	body.reader = io.MultiReader(readers...)
	return body, writer.FormDataContentType(), nil
}

type uploadFileReader struct {
	source    io.Reader
	remaining int64
	expected  string
	hash      hash.Hash
	done      bool
}

func (file *uploadFileReader) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if file.done {
		return 0, io.EOF
	}
	if file.remaining == 0 {
		var probe [1]byte
		n, err := file.source.Read(probe[:])
		if n != 0 {
			return 0, errors.New("upload reader exceeds its declared file size")
		}
		if err == io.EOF {
			return 0, file.finish()
		}
		return 0, err
	}
	if int64(len(data)) > file.remaining {
		data = data[:file.remaining]
	}
	n, err := file.source.Read(data)
	_, _ = file.hash.Write(data[:n])
	file.remaining -= int64(n)
	if err == io.EOF {
		if file.remaining != 0 {
			return n, io.ErrUnexpectedEOF
		}
		return n, file.finish()
	}
	return n, err
}

func (file *uploadFileReader) finish() error {
	file.done = true
	if hex.EncodeToString(file.hash.Sum(nil)) != file.expected {
		return errors.New("upload reader does not match its declared SHA-256 hash")
	}
	return io.EOF
}
