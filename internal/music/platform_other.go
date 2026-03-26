package music

func streamChunkFrames() int {
	return 512
}

func streamLookaheadFrames() int {
	return streamChunkFrames() * 4
}

func chunkPlayerCommandQueueCap() int {
	return 16
}
