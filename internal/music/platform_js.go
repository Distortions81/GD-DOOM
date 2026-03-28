//go:build js && wasm

package music

func streamChunkFrames() int {
	return 1024
}

func streamLookaheadFrames() int {
	return 8192
}

func chunkPlayerCommandQueueCap() int {
	return 16
}
