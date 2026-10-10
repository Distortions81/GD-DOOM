//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"gddoom/internal/runtimecfg"
)

const authorityServersStorageKey = "gddoom.multiplayer.servers.v1"

func loadAuthorityServers(_ string) (entries []runtimecfg.AuthorityServerEntry) {
	// Privacy modes can deny storage; the default server remains usable.
	defer func() {
		if recover() != nil {
			entries = nil
		}
	}()
	value := js.Global().Get("localStorage").Call("getItem", authorityServersStorageKey)
	if value.IsNull() || value.IsUndefined() {
		return nil
	}
	data := value.String()
	if len(data) > 64<<10 || json.Unmarshal([]byte(data), &entries) != nil {
		return nil
	}
	return cleanAuthorityServers(entries)
}

func saveAuthorityServers(_ string, entries []runtimecfg.AuthorityServerEntry) (err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("browser storage unavailable; servers kept for this session")
		}
	}()
	data, err := json.Marshal(cleanAuthorityServers(entries))
	if err != nil {
		return err
	}
	js.Global().Get("localStorage").Call("setItem", authorityServersStorageKey, string(data))
	return nil
}
