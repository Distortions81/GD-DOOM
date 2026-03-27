//go:build !js || !wasm

package music

func streamChunkFrames() int {
	return 2048
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * 6
}

func chunkPlayerCommandQueueCap() int {
	return 16
}
