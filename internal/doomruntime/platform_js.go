//go:build js && wasm

package doomruntime

import (
	"time"

	"gddoom/internal/platformcfg"
)

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}

func yieldWASMRenderTime(time.Time) {
	const minYield = 1 * time.Millisecond
	time.Sleep(minYield)
}
