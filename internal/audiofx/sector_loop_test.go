package audiofx

import (
	"bytes"
	"gddoom/internal/media"
	"testing"
)

func TestSectorLoopPhaseExpiryAndCancellation(t *testing.T) {
	sample := media.PCMSample{SampleRate: 35, Data: []byte{1, 2, 3, 4, 5}}
	var seq LoopSequencer
	first, pitch, start := seq.Chunk("sector:0", 8, 1, sample, 1)
	if !start || pitch != 1 || !bytes.Equal(first.Data, []byte{1, 2, 3, 4, 5, 1, 2, 3}) {
		t.Fatalf("first window: %+v", first)
	}
	if _, _, start = seq.Chunk("sector:0", 8, 1, sample, 1); start {
		t.Fatal("same-tic refresh restarted channel")
	}
	next, _, _ := seq.Chunk("sector:0", 8, 9, sample, 1)
	if !bytes.Equal(next.Data, []byte{4, 5, 1, 2, 3, 4, 5, 1}) {
		t.Fatalf("phase reset at refresh: %v", next.Data)
	}
	independent, _, _ := seq.Chunk("sector:1", 8, 9, sample, 1)
	if !bytes.Equal(independent.Data, first.Data) {
		t.Fatal("independent origin inherited phase")
	}
	expired, _, _ := seq.Chunk("sector:0", 8, 25, sample, 1)
	if !bytes.Equal(expired.Data, first.Data) {
		t.Fatal("expired loop did not restart")
	}
	seq.Chunk("sector:0", 0, 26, sample, 1)
	restarted, _, _ := seq.Chunk("sector:0", 8, 33, sample, 1)
	if !bytes.Equal(restarted.Data, first.Data) {
		t.Fatal("one-shot did not cancel loop phase")
	}
	if !bytes.Equal(first.Data, []byte{1, 2, 3, 4, 5, 1, 2, 3}) {
		t.Fatal("refresh mutated a playing PCM window")
	}
	seq.Clear()
	if len(seq.states) != 0 {
		t.Fatal("stop retained phases")
	}
}
func TestSectorLoopDurationAndHeldPitch(t *testing.T) {
	sample := media.PCMSample{SampleRate: 11025, Data: []byte{1, 2, 3}}
	var seq LoopSequencer
	chunk, pitch, _ := seq.Chunk("sector:0", 8, 1, sample, 1.25)
	if len(chunk.Data) != 3150 || pitch != 1.25 {
		t.Fatalf("window duration at pitch: %d %f", len(chunk.Data), pitch)
	}
	chunk, pitch, _ = seq.Chunk("sector:0", 8, 9, sample, .75)
	if len(chunk.Data) != 3150 || pitch != 1.25 {
		t.Fatal("refresh changed active-channel pitch")
	}
}
