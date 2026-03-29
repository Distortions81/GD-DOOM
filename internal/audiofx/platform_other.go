//go:build !js || !wasm

package audiofx

import "gddoom/internal/platformcfg"

func maxSpatialVoices() int {
	if platformcfg.IsWASMBuild() {
		return 8
	}
	return 8
}

func maxMenuVoices() int {
	if platformcfg.IsWASMBuild() {
		return 8
	}
	return 8
}
