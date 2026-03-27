//go:build js && wasm

package runtimecfg

import "gddoom/internal/platformcfg"

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}
