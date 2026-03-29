//go:build js && wasm

package doomruntime

import (
	"time"

	"gddoom/internal/platformcfg"
)

var lastWASMFrameStart time.Time

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}

func yieldWASMRenderTime(frameStart time.Time) {
	const frameBudget = 15 * time.Millisecond
	const minYield = 1 * time.Millisecond
	sleep := minYield
	if !lastWASMFrameStart.IsZero() {
		elapsed := frameStart.Sub(lastWASMFrameStart)
		if elapsed < frameBudget {
			sleep = frameBudget - elapsed
			if sleep < minYield {
				sleep = minYield
			}
		}
	}
	if sleep < minYield {
		sleep = minYield
	}
	lastWASMFrameStart = frameStart
	time.Sleep(sleep)
}
