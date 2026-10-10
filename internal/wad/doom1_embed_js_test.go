//go:build js && wasm

package wad

import (
	"bytes"
	"syscall/js"
	"testing"
)

func TestExplicitBrowserUploadDoesNotResolveBundledOrBasenameAlias(t *testing.T) {
	previousStore := js.Global().Get("__gddoomLocalWADs")
	previousCache := browserLocalWADBytesCache
	browserLocalWADBytesCache = make(map[string][]byte)
	t.Cleanup(func() {
		js.Global().Set("__gddoomLocalWADs", previousStore)
		browserLocalWADBytesCache = previousCache
	})
	custom := []byte("IWAD custom DOOM1 bytes")
	upload := js.Global().Get("Uint8Array").New(len(custom))
	js.CopyBytesToJS(upload, custom)
	js.Global().Set("__gddoomLocalWADs", js.ValueOf([]any{map[string]any{"path": "browser-upload/DOOM1.WAD", "name": "DOOM1.WAD", "bytes": upload}}))
	for _, path := range []string{"DOOM1.WAD", "doom1.wad"} {
		data, ok := embeddedDataForPath(path)
		if !ok || !bytes.Equal(data, embeddedDOOM1WAD) {
			t.Fatalf("bare built-in alias changed: %s", path)
		}
	}
	for _, path := range []string{"browser-upload/DOOM1.WAD", "BROWSER-UPLOAD/doom1.wad"} {
		data, ok := embeddedDataForPath(path)
		if !ok || !bytes.Equal(data, custom) {
			t.Fatalf("explicit custom file was replaced by another identity: %s", path)
		}
	}
	// A matching basename and even a warmed basename cache must not authorize a
	// different explicit path that the launcher never supplied.
	browserLocalWADBytesCache["DOOM1.WAD"] = []byte("unrelated basename cache")
	for _, path := range []string{"browser-upload/missing/DOOM1.WAD", "browser-upload/other.wad"} {
		if _, ok := embeddedDataForPath(path); ok {
			t.Fatalf("missing explicit upload fell back to an alias: %s", path)
		}
	}
	browserLocalWADBytesCache = make(map[string][]byte)
	js.Global().Set("__gddoomLocalWADs", js.ValueOf([]any{}))
	if _, ok := embeddedDataForPath("browser-upload/DOOM1.WAD"); ok {
		t.Fatal("absent explicit DOOM1 upload fell back to the bundled alias")
	}
}
