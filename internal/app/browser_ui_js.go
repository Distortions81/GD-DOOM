//go:build js && wasm

package app

import "syscall/js"

func notifyBrowserSessionStarted() {
	root := js.Global()
	parent := root.Get("parent")
	if parent.IsUndefined() || parent.IsNull() || parent.Equal(root) {
		return
	}
	parent.Call("postMessage", map[string]any{"type": "gddoom-session-started"}, root.Get("location").Get("origin"))
}
