//go:build js && wasm

package app

import "gddoom/internal/platformcfg"

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}
