//go:build js && wasm

package music

func streamChunkFrames() int {
	return 1260
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * 18
}

func streamChunkFramesForBackend(backend Backend) int {
	switch ResolveBackend(backend) {
	case BackendImpSynth:
		// Smaller render bursts reduce long single-threaded stalls on js/wasm.
		return 630
	default:
		return streamChunkFrames()
	}
}

func streamLookaheadFramesForBackend(backend Backend) int {
	switch ResolveBackend(backend) {
	case BackendImpSynth:
		// Preserve roughly the same total buffered time as the default wasm path.
		return streamChunkFramesForBackend(backend) * 36
	default:
		return streamLookaheadFrames()
	}
}

func chunkPlayerCommandQueueCap() int {
	return 32
}
