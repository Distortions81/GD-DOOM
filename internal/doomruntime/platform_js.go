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
	const frameBudget = 15 * time.Millisecond
	const minYield = 1 * time.Millisecond
	sleep := minYield
	if elapsed < frameBudget {
		sleep = frameBudget - elapsed
		if sleep < minYield {
			sleep = minYield
		}
	}
	if sleep < minYield {
		sleep = minYield
	}
	time.Sleep(sleep)
}
