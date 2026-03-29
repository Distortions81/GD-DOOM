//go:build js && wasm

package music

func streamChunkFrames() int {
	return 1260
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * 12
}

func chunkPlayerCommandQueueCap() int {
	return 32
}
