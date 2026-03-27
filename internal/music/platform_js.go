//go:build js && wasm

package music

func streamChunkFrames() int {
	return 256
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * 2
}

func chunkPlayerCommandQueueCap() int {
	return 4
}
