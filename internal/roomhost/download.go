package roomhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gddoom/internal/lobby"
)

// RedistributionApproval is an operator assertion about one exact file, not a
// classification inferred from its filename, uploader, or WAD identification.
// Unknown files, and entries without Allow, remain private.
type RedistributionApproval struct {
	SHA256  string `json:"sha256"`
	Name    string `json:"name"`
	License string `json:"license,omitempty"`
	Source  string `json:"source,omitempty"`
	Allow   bool   `json:"allow"`
}

type downloadFile struct {
	path string
	file lobby.PackFile
}

func (m *Manager) initContent() error {
	if m.config.DownloadQuota == 0 {
		m.config.DownloadQuota = 512 << 20
	}
	if m.config.DownloadQuota < 1 {
		return errors.New("download quota must be positive")
	}
	if len(m.config.Redistribution) > 2048 {
		return errors.New("too many redistribution approvals")
	}
	m.approvals = make(map[string]RedistributionApproval)
	m.downloads = make(map[string]downloadFile)
	m.downloadSlots = make(chan struct{}, 4)
	for _, approval := range m.config.Redistribution {
		file := lobby.PackFile{Name: approval.Name, Size: 1, SHA256: approval.SHA256, Downloadable: approval.Allow, License: approval.License, Source: approval.Source}
		validation := lobby.Pack{ID: "policy", Name: "Policy", WADHashes: []string{approval.SHA256}, Maps: []string{"E1M1"}, Files: []lobby.PackFile{file}}
		if err := lobby.ValidatePack(validation); err != nil {
			return fmt.Errorf("invalid redistribution approval: %w", err)
		}
		if _, exists := m.approvals[approval.SHA256]; exists {
			return errors.New("duplicate redistribution approval hash")
		}
		m.approvals[approval.SHA256] = approval
	}
	if err := m.initUploads(); err != nil {
		return err
	}
	for i, pack := range m.config.Packs {
		prepared, err := m.prepareContent(pack)
		if err != nil {
			return fmt.Errorf("pack %q content: %w", pack.Pack.ID, err)
		}
		m.config.Packs[i], m.packs[pack.Pack.ID] = prepared, prepared
	}
	return nil
}

// prepareContent regenerates permissions from the current operator policy.
// Persisted metadata and upload declarations can never grant redistribution.
func (m *Manager) prepareContent(pack ContentPack) (ContentPack, error) {
	m.downloadMu.Lock()
	defer m.downloadMu.Unlock()
	pack.Paths = slices.Clone(pack.Paths)
	oldFiles := pack.Pack.Files
	pack.Pack.Files = make([]lobby.PackFile, len(pack.Paths))
	for i, source := range pack.Paths {
		info, err := os.Stat(source)
		if err != nil || !info.Mode().IsRegular() || info.Size() < 1 {
			return ContentPack{}, errors.New("WAD source is not a regular nonempty file")
		}
		hash := pack.Pack.WADHashes[i]
		name := filepath.Base(source)
		if len(oldFiles) == len(pack.Paths) {
			name = oldFiles[i].Name
		}
		if !downloadNameValid(name) {
			name = hash[:12] + ".wad"
		}
		file := lobby.PackFile{Name: name, Size: info.Size(), SHA256: hash}
		approval, approved := m.approvals[hash]
		if approved && approval.Allow {
			file.Name, file.License, file.Source = approval.Name, approval.License, approval.Source
			file.Downloadable = true
			if file.Size > lobby.MaxUploadFileBytes {
				return ContentPack{}, errors.New("approved WAD exceeds the 64 MiB download limit")
			}
			cached, found := m.downloads[hash]
			if !found {
				cached, err = m.snapshotDownload(source, file)
				if err != nil {
					return ContentPack{}, err
				}
				m.downloads[hash] = cached
			}
			if cached.file.Size != file.Size {
				return ContentPack{}, errors.New("approved WAD source size changed")
			}
			// Workers and download clients consume the same immutable bytes.
			pack.Paths[i] = cached.path
		}
		pack.Pack.Files[i] = file
	}
	if err := lobby.ValidatePack(pack.Pack); err != nil {
		return ContentPack{}, err
	}
	return pack, nil
}

func downloadNameValid(name string) bool {
	if name == "" || name == "." || name == ".." || strings.TrimSpace(name) != name || strings.ContainsAny(name, "/\\") || !utf8.ValidString(name) || utf8.RuneCountInString(name) > lobby.MaxNameLength {
		return false
	}
	for _, r := range name {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// snapshotDownload copies once, verifying both length and digest before the
// hash route becomes reachable. The caller serializes cache changes.
func (m *Manager) snapshotDownload(source string, file lobby.PackFile) (downloadFile, error) {
	if file.Size > m.config.DownloadQuota-m.downloadBytes {
		return downloadFile{}, reject(http.StatusInsufficientStorage, "approved WAD download cache is full")
	}
	if m.downloadDir == "" {
		directory, err := os.MkdirTemp(m.config.TempDir, "gdlobby-content-")
		if err != nil {
			return downloadFile{}, err
		}
		m.downloadDir = directory
	}
	input, err := os.Open(source)
	if err != nil {
		return downloadFile{}, err
	}
	defer input.Close()
	filename := filepath.Join(m.downloadDir, file.SHA256+".wad")
	output, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return downloadFile{}, err
	}
	digest := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(output, digest), io.LimitReader(input, file.Size+1))
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil || n != file.Size || hex.EncodeToString(digest.Sum(nil)) != file.SHA256 {
		_ = os.Remove(filename)
		return downloadFile{}, errors.New("approved WAD changed or does not match its SHA-256")
	}
	if err := os.Chmod(filename, 0400); err != nil {
		_ = os.Remove(filename)
		return downloadFile{}, err
	}
	m.downloadBytes += file.Size
	return downloadFile{path: filename, file: file}, nil
}

func (m *Manager) downloadHTTP(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, "/api/v1/content/")
	if !validPackDirectory(hash) {
		http.NotFound(w, r)
		return
	}
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil {
		m.mu.Unlock()
		writeError(w, reject(http.StatusServiceUnavailable, "lobby is stopping"))
		return
	}
	select {
	case m.downloadSlots <- struct{}{}:
	default:
		m.mu.Unlock()
		writeError(w, reject(http.StatusServiceUnavailable, "WAD downloads are busy; retry shortly"))
		return
	}
	m.wg.Add(1)
	m.mu.Unlock()
	defer m.wg.Done()
	defer func() { <-m.downloadSlots }()
	m.downloadMu.Lock()
	cached, allowed := m.downloads[hash]
	m.downloadMu.Unlock()
	if !allowed {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(cached.path)
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	stopManager := context.AfterFunc(m.ctx, cancel)
	defer stopManager()
	controller := http.NewResponseController(w)
	deadline, _ := ctx.Deadline()
	_ = controller.SetWriteDeadline(deadline)
	stopped := make(chan struct{})
	stopCopy := context.AfterFunc(ctx, func() {
		_ = controller.SetWriteDeadline(time.Now())
		_ = file.Close()
		close(stopped)
	})
	defer func() {
		if !stopCopy() {
			<-stopped
		}
		_ = controller.SetWriteDeadline(time.Time{})
	}()
	// Ranges are deliberately ignored: one request transfers at most one file.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprint(cached.file.Size))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": cached.file.Name}))
	w.Header().Set("ETag", `"`+hash+`"`)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = io.CopyN(w, file, cached.file.Size)
}
