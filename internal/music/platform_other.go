//go:build !js || !wasm

package music

func streamChunkFrames() int {
	return 2048
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * 6
}

func streamChunkFramesForBackend(backend Backend) int {
	return streamChunkFrames()
}

func streamLookaheadFramesForBackend(backend Backend) int {
	return streamLookaheadFrames()
}

func chunkPlayerCommandQueueCap() int {
	return 16
}
