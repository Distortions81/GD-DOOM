package music

func streamChunkFrames() int {
	return 1024
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * 6
}

func chunkPlayerCommandQueueCap() int {
	return 64
}
