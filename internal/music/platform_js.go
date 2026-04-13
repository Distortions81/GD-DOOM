//go:build js && wasm

package music

const (
	wasmStreamChunkFrames   = 512
	wasmStreamLookaheadMult = 28
)

func streamChunkFrames() int {
	return wasmStreamChunkFrames
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * wasmStreamLookaheadMult
}

func streamChunkFramesForBackend(backend Backend) int {
	return streamChunkFrames()
}

func streamLookaheadFramesForBackend(backend Backend) int {
	return streamLookaheadFrames()
}

func chunkPlayerCommandQueueCap() int {
	return 32
}
