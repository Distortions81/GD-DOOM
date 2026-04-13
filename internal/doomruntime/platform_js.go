//go:build js && wasm

package doomruntime

import (
	"time"

	"gddoom/internal/platformcfg"
)

const wasmRenderTargetFPS = 75

var wasmLastRenderYield time.Time

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}

func yieldWASMRenderTime() {
	const minFrame = time.Second / wasmRenderTargetFPS
	now := time.Now()
	if !wasmLastRenderYield.IsZero() {
		if sleep := minFrame - now.Sub(wasmLastRenderYield); sleep > 0 {
			time.Sleep(sleep)
		}
	}
	wasmLastRenderYield = time.Now()
}
