package wad

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

const (
	memoryWADPrefix    = "memory-wad/"
	maxMemoryFiles     = 16
	maxMemoryFileBytes = 64 << 20
	maxMemoryBytes     = 128 << 20
)

var memoryWADScopes atomic.Uint64

var memoryWADRegistry = struct {
	sync.RWMutex
	files map[string][]byte
}{files: make(map[string][]byte)}

// RegisterMemoryFiles registers an ordered stack without copying its bytes.
// The caller transfers ownership and must not mutate data after registration.
// Bytes returned by EmbeddedDataForPath or File.LumpDataView are also immutable.
// Cleanup is safe to call repeatedly or concurrently. It removes path lookup;
// already opened File objects continue to own references to their WAD bytes.
func RegisterMemoryFiles(names []string, data [][]byte) (paths []string, cleanup func(), err error) {
	if len(names) == 0 || len(names) != len(data) || len(names) > maxMemoryFiles {
		return nil, nil, errors.New("memory WAD stack requires 1–16 matching names and files")
	}
	total := 0
	for i, name := range names {
		if !utf8.ValidString(name) || strings.TrimSpace(name) != name || name == "" || name == "." || name == ".." || utf8.RuneCountInString(name) > 64 || strings.ContainsAny(name, "/\\") || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return nil, nil, errors.New("memory WAD file requires a plain name of 1–64 characters")
		}
		size := len(data[i])
		if size == 0 || size > maxMemoryFileBytes || size > maxMemoryBytes-total {
			return nil, nil, errors.New("memory WAD stack exceeds the 64 MiB file or 128 MiB stack limit")
		}
		total += size
	}
	scope := memoryWADScopes.Add(1)
	if scope == 0 {
		return nil, nil, errors.New("memory WAD scope identifiers exhausted")
	}
	registered := make([]string, len(names))
	for i, name := range names {
		registered[i] = fmt.Sprintf("%s%d/%d/%s", memoryWADPrefix, scope, i, name)
	}
	memoryWADRegistry.Lock()
	for i, key := range registered {
		memoryWADRegistry.files[key] = data[i]
	}
	memoryWADRegistry.Unlock()
	var once sync.Once
	cleanup = func() {
		once.Do(func() {
			memoryWADRegistry.Lock()
			defer memoryWADRegistry.Unlock()
			for _, key := range registered {
				delete(memoryWADRegistry.files, key)
			}
		})
	}
	// Cleanup keeps its own path slice even if a caller replaces returned paths.
	return slices.Clone(registered), cleanup, nil
}

func memoryDataForPath(path string) ([]byte, bool) {
	memoryWADRegistry.RLock()
	data, ok := memoryWADRegistry.files[path]
	memoryWADRegistry.RUnlock()
	return data, ok
}

func isMemoryWADPath(value string) bool {
	// Only exact registered paths resolve. Alternate spellings of the reserved
	// namespace must not fall through to a disk file or a DOOM1.WAD alias.
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if strings.HasPrefix(strings.TrimPrefix(value, "./"), memoryWADPrefix) {
		return true
	}
	value = path.Clean(value)
	return value == strings.TrimSuffix(memoryWADPrefix, "/") || strings.HasPrefix(value, memoryWADPrefix)
}
