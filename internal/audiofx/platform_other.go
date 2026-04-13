//go:build !js || !wasm

package audiofx

import (
	"time"

	"gddoom/internal/platformcfg"
)

func maxSpatialVoices() int {
	if platformcfg.IsWASMBuild() {
		return 10
	}
	return 8
}

func maxMenuVoices() int {
	if platformcfg.IsWASMBuild() {
		return 8
	}
	return 8
}

func pcSpeakerPlayerBufferDuration() time.Duration {
	if platformcfg.IsWASMBuild() {
		return 60 * time.Millisecond
	}
	return 30 * time.Millisecond
}
