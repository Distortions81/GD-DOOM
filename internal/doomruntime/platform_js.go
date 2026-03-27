//go:build js && wasm

package doomruntime

import "gddoom/internal/platformcfg"

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}
