//go:build js && wasm

package music

const (
	wasmStreamChunkFrames   = 256
	wasmStreamEnqueueFrames = wasmStreamChunkFrames * 8
	wasmStreamLookaheadMult = 48
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

func streamEnqueueFrames() int {
	return wasmStreamEnqueueFrames
}

func streamEnqueueFramesForBackend(backend Backend) int {
	return streamEnqueueFrames()
}

func streamLookaheadFramesForBackend(backend Backend) int {
	return streamLookaheadFrames()
}

func chunkPlayerCommandQueueCap() int {
	return 32
}
