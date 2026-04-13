//go:build js && wasm

package music

import "testing"

func TestWASMStreamEnqueueFramesReducedBatch(t *testing.T) {
	want := wasmStreamChunkFrames * 8
	if got := streamEnqueueFrames(); got != want {
		t.Fatalf("streamEnqueueFrames()=%d want=%d", got, want)
	}
}

func TestWASMStreamLookaheadMatchesTwoEnqueueBatches(t *testing.T) {
	want := streamEnqueueFrames() * 2
	if got := streamLookaheadFrames(); got != want {
		t.Fatalf("streamLookaheadFrames()=%d want=%d", got, want)
	}
}
