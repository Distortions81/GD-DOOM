//go:build js && wasm

package doomruntime

import (
	"time"

	"gddoom/internal/platformcfg"
)

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}

func yieldWASMRenderTime(elapsed time.Duration) {
	const frameBudget = 16 * time.Millisecond
	const minYield = 2 * time.Millisecond
	if elapsed >= frameBudget {
		return
	}
	sleep := frameBudget - elapsed
	if sleep < minYield {
		sleep = minYield
	}
	time.Sleep(sleep)
}
