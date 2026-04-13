//go:build js && wasm

package music

const (
	wasmStreamChunkFrames   = 512
	wasmStreamEnqueueFrames = wasmStreamChunkFrames * 4
	wasmStreamLookaheadMult = wasmStreamEnqueueFrames * 3
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
