//go:build js && wasm

package audiofx

import "time"

func maxSpatialVoices() int {
	return 8
}

func maxMenuVoices() int {
	return 8
}

func pcSpeakerPlayerBufferDuration() time.Duration {
	return 60 * time.Millisecond
}
