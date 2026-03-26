package platformcfg

import (
	"runtime"
	"sync/atomic"
)

var forcedWASMMode atomic.Uint32

func SetForcedWASMMode(force bool) {
	if force {
		forcedWASMMode.Store(1)
		return
	}
	forcedWASMMode.Store(0)
}

func ForcedWASMMode() bool {
	return forcedWASMMode.Load() != 0
}

func IsWASMBuild() bool {
	return ForcedWASMMode() || runtime.GOOS == "js" || runtime.GOARCH == "wasm"
}
