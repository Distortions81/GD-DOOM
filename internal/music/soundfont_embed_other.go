//go:build !js || !wasm

package music

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const browserSGMHQSoundFontPath = "soundfonts/SGM-HQ.sf2"
const browserSGMHQSoundFontURL = "https://m45sci.xyz/u/dist/GD-DOOM/SGM-HQ.sf2"

type nativeSoundFontLoad struct {
	pending  bool
	err      error
	received int64
	total    int64
}

var (
	nativeSoundFontLoadsMu sync.Mutex
	nativeSoundFontLoads   = map[string]*nativeSoundFontLoad{}
)

func embeddedSoundFontDataForPath(path string) ([]byte, bool) {
	return nil, false
}

func EmbeddedSoundFontChoices() []string {
	return nil
}

func BrowserSoundFontChoices() []string {
	return []string{browserSGMHQSoundFontPath}
}

func BrowserSGMHQSoundFontPath() string {
	return browserSGMHQSoundFontPath
}

func StartBrowserSoundFontLoad(path string) bool {
	path = resolveNativeSoundFontPath(path)
	if !isNativeDownloadableSoundFont(path) {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return false
	}
	key := nativeSoundFontKey(path)
	nativeSoundFontLoadsMu.Lock()
	if load, ok := nativeSoundFontLoads[key]; ok && load.pending {
		nativeSoundFontLoadsMu.Unlock()
		return true
	}
	load := &nativeSoundFontLoad{pending: true}
	nativeSoundFontLoads[key] = load
	nativeSoundFontLoadsMu.Unlock()
	go downloadNativeSoundFont(path, key)
	return true
}

func BrowserSoundFontLoadPending(path string) bool {
	key := nativeSoundFontKey(resolveNativeSoundFontPath(path))
	nativeSoundFontLoadsMu.Lock()
	defer nativeSoundFontLoadsMu.Unlock()
	if load, ok := nativeSoundFontLoads[key]; ok {
		return load.pending
	}
	return false
}

func BrowserSoundFontLoadError(path string) error {
	key := nativeSoundFontKey(resolveNativeSoundFontPath(path))
	nativeSoundFontLoadsMu.Lock()
	defer nativeSoundFontLoadsMu.Unlock()
	if load, ok := nativeSoundFontLoads[key]; ok && !load.pending {
		return load.err
	}
	return nil
}

func BrowserSoundFontLoadProgress(path string) (int64, int64) {
	path = resolveNativeSoundFontPath(path)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		n := info.Size()
		return n, n
	}
	key := nativeSoundFontKey(path)
	nativeSoundFontLoadsMu.Lock()
	defer nativeSoundFontLoadsMu.Unlock()
	if load, ok := nativeSoundFontLoads[key]; ok {
		return load.received, load.total
	}
	return 0, 0
}

func DefaultEmbeddedSoundFontPath() string {
	return ""
}

func ensureBrowserSoundFontLoaded(path string) error {
	path = resolveNativeSoundFontPath(path)
	if !isNativeDownloadableSoundFont(path) {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if !StartBrowserSoundFontLoad(path) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	for BrowserSoundFontLoadPending(path) {
		time.Sleep(20 * time.Millisecond)
	}
	if err := BrowserSoundFontLoadError(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

func EnsureSoundFontAvailable(path string) error {
	if err := ensureBrowserSoundFontLoaded(path); err != nil {
		return fmt.Errorf("music: ensure soundfont %s: %w", path, err)
	}
	return nil
}

func SoundFontDownloadURL(path string) string {
	if isNativeDownloadableSoundFont(resolveNativeSoundFontPath(path)) {
		return browserSGMHQSoundFontURL
	}
	return ""
}

func resolveNativeSoundFontPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return browserSGMHQSoundFontPath
	}
	if strings.EqualFold(filepath.Base(path), "sgm-hq.sf2") {
		if filepath.Base(path) == path {
			return browserSGMHQSoundFontPath
		}
	}
	return path
}

func isNativeDownloadableSoundFont(path string) bool {
	return strings.EqualFold(filepath.Base(strings.TrimSpace(path)), "sgm-hq.sf2")
}

func nativeSoundFontKey(path string) string {
	return strings.ToUpper(filepath.Clean(path))
}

func downloadNativeSoundFont(path string, key string) {
	err := func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create soundfont dir: %w", err)
		}
		req, err := http.NewRequest(http.MethodGet, browserSGMHQSoundFontURL, nil)
		if err != nil {
			return fmt.Errorf("build soundfont request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("download soundfont: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("download soundfont: status %d", resp.StatusCode)
		}
		nativeSoundFontLoadsMu.Lock()
		if load, ok := nativeSoundFontLoads[key]; ok {
			load.total = resp.ContentLength
		}
		nativeSoundFontLoadsMu.Unlock()
		tmpPath := path + ".part"
		f, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("create soundfont file: %w", err)
		}
		defer func() {
			f.Close()
			if err != nil {
				_ = os.Remove(tmpPath)
			}
		}()
		buf := make([]byte, 32*1024)
		var written int64
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := f.Write(buf[:n]); werr != nil {
					return fmt.Errorf("write soundfont file: %w", werr)
				}
				written += int64(n)
				nativeSoundFontLoadsMu.Lock()
				if load, ok := nativeSoundFontLoads[key]; ok {
					load.received = written
				}
				nativeSoundFontLoadsMu.Unlock()
			}
			if rerr == nil {
				continue
			}
			if rerr == io.EOF {
				break
			}
			return fmt.Errorf("read soundfont response: %w", rerr)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close soundfont file: %w", err)
		}
		if err := os.Rename(tmpPath, path); err != nil {
			return fmt.Errorf("install soundfont file: %w", err)
		}
		nativeSoundFontLoadsMu.Lock()
		if load, ok := nativeSoundFontLoads[key]; ok {
			load.received = written
			if load.total <= 0 {
				load.total = written
			}
		}
		nativeSoundFontLoadsMu.Unlock()
		return nil
	}()
	nativeSoundFontLoadsMu.Lock()
	if load, ok := nativeSoundFontLoads[key]; ok {
		load.pending = false
		load.err = err
	}
	nativeSoundFontLoadsMu.Unlock()
}
