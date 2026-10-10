package wad

import (
	"bytes"
	"errors"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
)

func TestMemoryFilesExactIdentityAndLifetime(t *testing.T) {
	base := minimalWAD(t, "IWAD", "TEST", []byte{1, 2, 3})
	patch := minimalWAD(t, "PWAD", "TEST", []byte{9, 8, 7})
	paths, cleanup, err := RegisterMemoryFiles([]string{"DOOM1.WAD", "DOOM1.WAD"}, [][]byte{base, patch})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if paths[0] == paths[1] {
		t.Fatal("duplicate basenames collided")
	}
	if path.Base(paths[0]) != "DOOM1.WAD" || path.Base(paths[1]) != "DOOM1.WAD" {
		t.Fatal("registration changed the original file names")
	}
	for i, want := range [][]byte{base, patch} {
		got, ok := EmbeddedDataForPath(paths[i])
		if !ok || !bytes.Equal(got, want) || &got[0] != &want[0] {
			t.Fatal("registered bytes were copied or replaced")
		}
	}
	for _, missing := range []string{strings.ToUpper(paths[0]), "memory-wad/missing/DOOM1.WAD", "./memory-wad/missing/DOOM1.WAD", "memory-wad/missing\\DOOM1.WAD", "memory-wad/missing/../../DOOM1.WAD"} {
		if got, ok := EmbeddedDataForPath(missing); ok || got != nil {
			t.Fatalf("missing identity %q resolved", missing)
		}
		if _, err := Open(missing); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing identity %q fell back: %v", missing, err)
		}
	}
	merged, err := OpenFiles(paths...)
	if err != nil {
		t.Fatal(err)
	}
	lump, ok := merged.LumpByName("TEST")
	if !ok {
		t.Fatal("merged overlay missing")
	}
	lookup := append([]string(nil), paths...)
	paths[0] = "caller replaced returned path"
	cleanup()
	cleanup()
	for _, key := range lookup {
		if _, ok := EmbeddedDataForPath(key); ok {
			t.Fatal("cleanup retained a path")
		}
		if _, err := Open(key); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("cleanup allowed fallback:", err)
		}
	}
	got, err := merged.LumpDataView(lump)
	if err != nil || !bytes.Equal(got, []byte{9, 8, 7}) {
		t.Fatalf("cleanup invalidated open WAD bytes: %v %v", got, err)
	}
}

func TestMemoryFilesUniqueScopesAndConcurrentCleanup(t *testing.T) {
	data := minimalWAD(t, "IWAD", "TEST", []byte{1})
	const count = 32
	paths := make(chan string, count)
	errorsFound := make(chan error, count)
	var workers sync.WaitGroup
	for i := 0; i < count; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			registered, cleanup, err := RegisterMemoryFiles([]string{"same.wad"}, [][]byte{data})
			if err != nil {
				errorsFound <- err
				return
			}
			paths <- registered[0]
			file, err := Open(registered[0])
			if err != nil {
				cleanup()
				errorsFound <- err
				return
			}
			var cleaners sync.WaitGroup
			for j := 0; j < 4; j++ {
				cleaners.Add(1)
				go func() { defer cleaners.Done(); cleanup() }()
			}
			for j := 0; j < 8; j++ {
				_, _ = EmbeddedDataForPath(registered[0])
			}
			cleaners.Wait()
			if got, err := file.LumpDataView(file.Lumps[0]); err != nil || len(got) != 1 || got[0] != 1 {
				errorsFound <- errors.New("opened bytes changed after cleanup")
			}
		}()
	}
	workers.Wait()
	close(paths)
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	seen := make(map[string]bool)
	for path := range paths {
		if seen[path] {
			t.Fatal("scope reused")
		}
		seen[path] = true
		if _, ok := EmbeddedDataForPath(path); ok {
			t.Fatal("concurrent cleanup retained path")
		}
	}
	if len(seen) != count {
		t.Fatal("registrations missing")
	}
}

func TestMemoryFilesRejectInvalidNamesAndSizes(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../test.wad", "a/b.wad", "a\\b.wad", " name.wad", "name.wad ", "bad\nname", string([]byte{255}), strings.Repeat("x", 65)} {
		paths, cleanup, err := RegisterMemoryFiles([]string{name}, [][]byte{{1}})
		if err == nil || paths != nil || cleanup != nil {
			t.Fatalf("invalid name %q registered", name)
		}
	}
	for _, tc := range []struct {
		names []string
		data  [][]byte
	}{
		{},
		{names: []string{"a"}},
		{names: []string{"a"}, data: [][]byte{nil}},
		{names: make([]string, 17), data: make([][]byte, 17)},
	} {
		if _, cleanup, err := RegisterMemoryFiles(tc.names, tc.data); err == nil || cleanup != nil {
			t.Fatal("invalid stack registered")
		}
	}
	// Share one allocation across immutable entries; bounds apply to stack
	// content size even when callers reuse backing storage.
	large := make([]byte, maxMemoryFileBytes+1)
	if _, cleanup, err := RegisterMemoryFiles([]string{"a"}, [][]byte{large}); err == nil || cleanup != nil {
		t.Fatal("oversize file registered")
	}
	if _, cleanup, err := RegisterMemoryFiles([]string{"a", "b", "c"}, [][]byte{large[:maxMemoryFileBytes], large[:maxMemoryFileBytes], {1}}); err == nil || cleanup != nil {
		t.Fatal("oversize stack registered")
	}
	paths, cleanup, err := RegisterMemoryFiles([]string{"a", "b"}, [][]byte{large[:maxMemoryFileBytes], large[:maxMemoryFileBytes]})
	if err != nil {
		t.Fatal("exact stack bound rejected:", err)
	}
	cleanup()
	if len(paths) != 2 {
		t.Fatal("stack order missing")
	}
}
