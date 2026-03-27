//go:build !js || !wasm

package sessiontransition

import "gddoom/internal/platformcfg"

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}
