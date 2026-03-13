//go:build js && wasm

package music

func streamChunkFrames() int {
	return 256
}

func streamLookaheadFrames() int {
	return 512
}

func chunkPlayerCommandQueueCap() int {
	return 8
}
