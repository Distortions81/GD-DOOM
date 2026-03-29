//go:build js && wasm

package music

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"syscall/js"

	"gddoom/soundfonts"
)

const browserSC55HQSoundFontPath = "soundfonts/SC55-HQ.sf2"
const browserSC55HQSoundFontURL = "https://m45sci.xyz/u/dist/GD-DOOM/SC55-HQ.sf2"
const browserSGMHQSoundFontPath = "soundfonts/SGM-HQ.sf2"
const browserSGMHQSoundFontURL = "https://m45sci.xyz/u/dist/GD-DOOM/SGM-HQ.sf2"

func embeddedSoundFontDataForPath(path string) ([]byte, bool) {
	path = strings.TrimSpace(path)
	if data, ok := soundfonts.EmbeddedDataForPath(path); ok {
		return data, true
	}
	base := strings.ToLower(filepath.Base(path))
	if base == "sgm-hq.sf2" {
		return browserCachedSoundFontBytes(base)
	}
	return nil, false
}

func EmbeddedSoundFontChoices() []string {
	return soundfonts.EmbeddedChoices()
}

func BrowserSoundFontChoices() []string {
	return []string{browserSC55HQSoundFontPath, browserSGMHQSoundFontPath}
}

func BrowserSGMHQSoundFontPath() string {
	return browserSGMHQSoundFontPath
}

type browserSoundFontLoad struct {
	pending     bool
	err         error
	received    int64
	total       int64
	progress    js.Func
	progressSet bool
	load        js.Func
	loadSet     bool
	fail        js.Func
	failSet     bool
	xhr         js.Value
}

var browserSoundFontLoads = map[string]*browserSoundFontLoad{}

func DefaultEmbeddedSoundFontPath() string {
	if choices := soundfonts.EmbeddedChoices(); len(choices) > 0 {
		return choices[0]
	}
	return ""
}

func ensureBrowserSoundFontLoaded(path string) error {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	url, ok := browserDownloadableSoundFontURL(base)
	if !ok {
		return nil
	}
	if _, ok := browserCachedSoundFontBytes(base); ok {
		return nil
	}
	return browserFetchAndCacheSoundFont(base, url)
}

func StartBrowserSoundFontLoad(path string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	url, ok := browserDownloadableSoundFontURL(base)
	if !ok {
		return false
	}
	if _, ok := browserCachedSoundFontBytes(base); ok {
		return false
	}
	if load, ok := browserSoundFontLoads[base]; ok && load.pending {
		return true
	}
	load := &browserSoundFontLoad{pending: true}
	browserSoundFontLoads[base] = load
	xhr := js.Global().Get("XMLHttpRequest").New()
	load.xhr = xhr
	xhr.Set("responseType", "arraybuffer")
	load.progress = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 || args[0].IsUndefined() || args[0].IsNull() {
			return nil
		}
		event := args[0]
		load.received = int64(event.Get("loaded").Int())
		if event.Get("lengthComputable").Bool() {
			load.total = int64(event.Get("total").Int())
		}
		return nil
	})
	load.progressSet = true
	load.load = js.FuncOf(func(this js.Value, args []js.Value) any {
		status := xhr.Get("status").Int()
		if status < 200 || status >= 300 {
			finishBrowserSoundFontLoad(base, jsErrorString("fetch soundfont", url, status))
			return nil
		}
		bytesVal := js.Global().Get("Uint8Array").New(xhr.Get("response"))
		if bytesVal.Get("length").Int() <= 0 {
			finishBrowserSoundFontLoad(base, jsErrorString("fetch soundfont", url, 0))
			return nil
		}
		load.received = int64(bytesVal.Get("length").Int())
		load.total = load.received
		browserSoundFontStore().Set(base, bytesVal)
		finishBrowserSoundFontLoad(base, nil)
		return nil
	})
	load.loadSet = true
	load.fail = js.FuncOf(func(this js.Value, args []js.Value) any {
		finishBrowserSoundFontLoad(base, jsErrorString("fetch soundfont", url, xhr.Get("status").Int()))
		return nil
	})
	load.failSet = true
	xhr.Set("onprogress", load.progress)
	xhr.Set("onload", load.load)
	xhr.Set("onerror", load.fail)
	xhr.Call("open", "GET", url, true)
	xhr.Call("send")
	return true
}

func BrowserSoundFontLoadPending(path string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	if load, ok := browserSoundFontLoads[base]; ok {
		return load.pending
	}
	return false
}

func BrowserSoundFontLoadError(path string) error {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	if load, ok := browserSoundFontLoads[base]; ok && !load.pending {
		return load.err
	}
	return nil
}

func BrowserSoundFontLoadProgress(path string) (int64, int64) {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	if data, ok := browserCachedSoundFontBytes(base); ok {
		n := int64(len(data))
		return n, n
	}
	if load, ok := browserSoundFontLoads[base]; ok {
		return load.received, load.total
	}
	return 0, 0
}

func finishBrowserSoundFontLoad(base string, err error) {
	load, ok := browserSoundFontLoads[base]
	if !ok {
		return
	}
	load.pending = false
	load.err = err
	if load.progressSet {
		load.progress.Release()
		load.progressSet = false
	}
	if load.loadSet {
		load.load.Release()
		load.loadSet = false
	}
	if load.failSet {
		load.fail.Release()
		load.failSet = false
	}
	if !load.xhr.IsUndefined() && !load.xhr.IsNull() {
		load.xhr.Set("onprogress", js.Null())
		load.xhr.Set("onload", js.Null())
		load.xhr.Set("onerror", js.Null())
	}
}

func browserSoundFontStore() js.Value {
	root := js.Global()
	store := root.Get("__gddoomSoundFonts")
	if store.IsUndefined() || store.IsNull() {
		store = root.Get("Object").New()
		root.Set("__gddoomSoundFonts", store)
	}
	return store
}

func browserCachedSoundFontBytes(base string) ([]byte, bool) {
	entry := browserSoundFontStore().Get(base)
	if entry.IsUndefined() || entry.IsNull() {
		return nil, false
	}
	n := entry.Get("length").Int()
	if n <= 0 {
		return nil, false
	}
	data := make([]byte, n)
	js.CopyBytesToGo(data, entry)
	return data, true
}

func browserFetchAndCacheSoundFont(base string, url string) error {
	xhr := js.Global().Get("XMLHttpRequest").New()
	xhr.Set("responseType", "arraybuffer")
	xhr.Call("open", "GET", url, false)
	xhr.Call("send")
	status := xhr.Get("status").Int()
	if status < 200 || status >= 300 {
		return jsErrorString("fetch soundfont", url, status)
	}
	bytesVal := js.Global().Get("Uint8Array").New(xhr.Get("response"))
	if bytesVal.Get("length").Int() <= 0 {
		return jsErrorString("fetch soundfont", url, 0)
	}
	browserSoundFontStore().Set(base, bytesVal)
	return nil
}

func jsError(prefix string, args []js.Value) error {
	if len(args) == 0 || args[0].IsUndefined() || args[0].IsNull() {
		return jsErrorString(prefix, "", 0)
	}
	msg := strings.TrimSpace(args[0].String())
	if msg == "" || msg == "<object>" {
		msg = "unknown error"
	}
	return jsErrorText(prefix + ": " + msg)
}

func jsErrorString(prefix string, target string, status int) error {
	if status > 0 && target != "" {
		return jsErrorText(prefix + " " + target + ": status " + strconv.Itoa(status))
	}
	if target != "" {
		return jsErrorText(prefix + " " + target)
	}
	return jsErrorText(prefix)
}

type jsErrorText string

func (e jsErrorText) Error() string {
	return string(e)
}

func SoundFontDownloadURL(path string) string {
	if url, ok := browserDownloadableSoundFontURL(strings.ToLower(strings.TrimSpace(filepath.Base(path)))); ok {
		return url
	}
	return ""
}

func browserDownloadableSoundFontURL(base string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(base)) {
	case "sc55-hq.sf2":
		return browserSC55HQSoundFontURL, true
	case "sgm-hq.sf2":
		return browserSGMHQSoundFontURL, true
	default:
		return "", false
	}
}

func EnsureSoundFontAvailable(path string) error {
	if err := ensureBrowserSoundFontLoaded(path); err != nil {
		return fmt.Errorf("music: ensure soundfont %s: %w", path, err)
	}
	return nil
}
