//go:build js && wasm

package wad

import (
	_ "embed"
	"path/filepath"
	"strings"
	"syscall/js"
)

//go:embed DOOM1.WAD
var embeddedDOOM1WAD []byte

func embeddedDataForPath(path string) ([]byte, bool) {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(strings.ToLower(path), browserLocalWADPrefix) {
		// Explicit uploaded paths must resolve exactly. In particular, a custom
		// file named DOOM1.WAD must not resolve to the bundled shareware alias,
		// and a missing upload must not fall back to another matching basename.
		key := strings.ToUpper(path)
		if data, ok := browserLocalWADBytesCache[key]; ok {
			return data, true
		}
		store := browserLocalWADStore()
		if store.IsUndefined() || store.IsNull() {
			return nil, false
		}
		for i := 0; i < store.Length(); i++ {
			entry := store.Index(i)
			if entry.IsUndefined() || entry.IsNull() || !strings.EqualFold(browserLocalWADPath(entry), path) {
				continue
			}
			bytes := entry.Get("bytes")
			if bytes.IsUndefined() || bytes.IsNull() || bytes.Get("length").Int() <= 0 {
				return nil, false
			}
			data := make([]byte, bytes.Get("length").Int())
			js.CopyBytesToGo(data, bytes)
			browserLocalWADBytesCache[key] = data
			return data, true
		}
		return nil, false
	}
	base := strings.ToUpper(filepath.Base(path))
	if base == "DOOM1.WAD" && len(embeddedDOOM1WAD) > 0 {
		return embeddedDOOM1WAD, true
	}
	if data, ok := browserLocalWADDataForPath(path); ok {
		return data, true
	}
	return nil, false
}
