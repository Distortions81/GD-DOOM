//go:build js && wasm

package wad

import (
	"path/filepath"
	"sort"
	"strings"
	"syscall/js"
)

const browserLocalWADPrefix = "browser-upload/"

var browserLocalWADBytesCache = map[string][]byte{}

func BrowserLocalWADPaths() []string {
	store := browserLocalWADStore()
	if store.IsUndefined() || store.IsNull() {
		return nil
	}
	out := make([]string, 0, store.Length())
	seen := make(map[string]struct{}, store.Length())
	for i := 0; i < store.Length(); i++ {
		entry := store.Index(i)
		if entry.IsUndefined() || entry.IsNull() {
			continue
		}
		path := browserLocalWADPath(entry)
		if path == "" {
			continue
		}
		key := strings.ToUpper(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, path)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func browserLocalWADDataForPath(path string) ([]byte, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	cacheKey := strings.ToUpper(path)
	if data, ok := browserLocalWADBytesCache[cacheKey]; ok {
		return data, true
	}
	base := strings.ToUpper(filepath.Base(path))
	if data, ok := browserLocalWADBytesCache[base]; ok {
		return data, true
	}
	store := browserLocalWADStore()
	if store.IsUndefined() || store.IsNull() {
		return nil, false
	}
	for i := 0; i < store.Length(); i++ {
		entry := store.Index(i)
		if entry.IsUndefined() || entry.IsNull() {
			continue
		}
		entryPath := browserLocalWADPath(entry)
		if entryPath == "" {
			continue
		}
		if !strings.EqualFold(entryPath, path) && !strings.EqualFold(filepath.Base(entryPath), base) {
			continue
		}
		bytesVal := entry.Get("bytes")
		if bytesVal.IsUndefined() || bytesVal.IsNull() {
			return nil, false
		}
		n := bytesVal.Get("length").Int()
		if n <= 0 {
			return nil, false
		}
		data := make([]byte, n)
		js.CopyBytesToGo(data, bytesVal)
		browserLocalWADBytesCache[cacheKey] = data
		browserLocalWADBytesCache[base] = data
		entryCacheKey := strings.ToUpper(entryPath)
		if entryCacheKey != "" && entryCacheKey != cacheKey {
			browserLocalWADBytesCache[entryCacheKey] = data
		}
		return data, true
	}
	return nil, false
}

func browserLocalWADStore() js.Value {
	return js.Global().Get("__gddoomLocalWADs")
}

func browserLocalWADPath(entry js.Value) string {
	if entry.IsUndefined() || entry.IsNull() {
		return ""
	}
	path := strings.TrimSpace(entry.Get("path").String())
	if path != "" {
		return path
	}
	name := strings.TrimSpace(entry.Get("name").String())
	if name == "" {
		return ""
	}
	return browserLocalWADPrefix + filepath.Base(name)
}
