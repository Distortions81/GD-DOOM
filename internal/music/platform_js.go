//go:build js && wasm

package music

func streamChunkFrames() int {
	return 512
}

func streamLookaheadFrames() int {
	return 2048
}

func chunkPlayerCommandQueueCap() int {
	return 16
}
