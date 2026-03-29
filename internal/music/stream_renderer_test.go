package music

import "testing"

func TestStreamRendererChunksUntilDone(t *testing.T) {
	d := NewOutputDriver(nil)
	score := []byte{
		0x40, 0, 0, // program 0
		0x10, 60, // note on
		0x80, 60, // note off (with delay follows)
		0x08, // delay 8 tics
		0x60, // end
	}
	mus := buildMUSTestLump(score)
	sr, err := NewMUSStreamRenderer(d, mus)
	if err != nil {
		t.Fatalf("NewMUSStreamRenderer() error: %v", err)
	}
	got := 0
	done := false
	for i := 0; i < 64; i++ {
		chunk, d, err := sr.NextChunkS16LE(256)
		if err != nil {
			t.Fatalf("NextChunkS16LE() error: %v", err)
		}
		got += len(chunk)
		if d {
			done = true
			break
		}
	}
	if !done {
		t.Fatal("stream did not report done")
	}
	if got == 0 {
		t.Fatal("expected non-empty chunked PCM")
	}
}

func TestParsedMUSStreamRendererMatchesMUSStreamRenderer(t *testing.T) {
	direct := NewOutputDriver(nil)
	parsedDriver := NewOutputDriver(nil)
	score := []byte{
		0x40, 0, 0,
		0x10, 60,
		0x80, 60,
		0x08,
		0x60,
	}
	mus := buildMUSTestLump(score)
	parsed, err := ParseMUSData(mus)
	if err != nil {
		t.Fatalf("ParseMUSData() error: %v", err)
	}
	a, err := NewMUSStreamRenderer(direct, mus)
	if err != nil {
		t.Fatalf("NewMUSStreamRenderer() error: %v", err)
	}
	b, err := NewParsedMUSStreamRenderer(parsedDriver, parsed)
	if err != nil {
		t.Fatalf("NewParsedMUSStreamRenderer() error: %v", err)
	}
	for i := 0; i < 64; i++ {
		chunkA, doneA, err := a.NextChunkS16LE(256)
		if err != nil {
			t.Fatalf("direct NextChunkS16LE() error: %v", err)
		}
		chunkB, doneB, err := b.NextChunkS16LE(256)
		if err != nil {
			t.Fatalf("parsed NextChunkS16LE() error: %v", err)
		}
		if string(chunkA) != string(chunkB) || doneA != doneB {
			t.Fatalf("chunk mismatch iter=%d doneA=%v doneB=%v lenA=%d lenB=%d", i, doneA, doneB, len(chunkA), len(chunkB))
		}
		if doneA {
			return
		}
	}
	t.Fatal("stream did not finish")
}
