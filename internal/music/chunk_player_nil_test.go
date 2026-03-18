package music

import "testing"

func TestChunkPlayerEnqueueBytesS16LENilReceiver(t *testing.T) {
	var cp *ChunkPlayer
	if err := cp.EnqueueBytesS16LE([]byte{1, 2, 3, 4}); err == nil {
		t.Fatal("EnqueueBytesS16LE() error = nil, want error for nil receiver")
	}
}
