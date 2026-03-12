package music

import "testing"

func TestPCMInt16ToBytesLE(t *testing.T) {
	in := []int16{0x1234, -2, 0, 32767, -32768}
	got := PCMInt16ToBytesLE(in)
	wantLen := len(in) * 2
	if len(got) != wantLen {
		t.Fatalf("len=%d want=%d", len(got), wantLen)
	}
	// 0x1234 little endian
	if got[0] != 0x34 || got[1] != 0x12 {
		t.Fatalf("first sample bytes=%02x %02x want=34 12", got[0], got[1])
	}
}

func TestPCMInt16ViewAsBytesLE(t *testing.T) {
	if !nativeLittleEndian() {
		t.Skip("unsafe LE byte view only applies on little-endian targets")
	}
	in := []int16{0x1234, -2}
	got := pcmInt16ViewAsBytesLE(in)
	if len(got) != len(in)*2 {
		t.Fatalf("len=%d want=%d", len(got), len(in)*2)
	}
	if got[0] != 0x34 || got[1] != 0x12 {
		t.Fatalf("first sample bytes=%02x %02x want=34 12", got[0], got[1])
	}
}

func TestDefaultStreamChunkSettings(t *testing.T) {
	if DefaultStreamChunkFrames != 1024 {
		t.Fatalf("DefaultStreamChunkFrames=%d want=1024", DefaultStreamChunkFrames)
	}
	if DefaultStreamLookahead != DefaultStreamChunkFrames*6 {
		t.Fatalf("DefaultStreamLookahead=%d want=%d", DefaultStreamLookahead, DefaultStreamChunkFrames*6)
	}
}

func TestPCMChunkBufferReadClear(t *testing.T) {
	b := newPCMChunkBuffer()
	b.Enqueue([]byte{1, 2, 3, 4})
	p := make([]byte, 2)
	n, err := b.Read(p)
	if err != nil || n != 2 || p[0] != 1 || p[1] != 2 {
		t.Fatalf("read1 n=%d err=%v p=%v", n, err, p)
	}
	b.Clear()
	b.Enqueue([]byte{9, 8})
	n, err = b.Read(p)
	if err != nil || n != 2 || p[0] != 9 || p[1] != 8 {
		t.Fatalf("read2 n=%d err=%v p=%v", n, err, p)
	}
}

func TestPCMChunkBufferBufferedBytes(t *testing.T) {
	b := newPCMChunkBuffer()
	b.Enqueue([]byte{1, 2, 3, 4, 5, 6})
	if got := b.BufferedBytes(); got != 6 {
		t.Fatalf("BufferedBytes()=%d want=6", got)
	}
	p := make([]byte, 4)
	n, err := b.Read(p)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != 4 {
		t.Fatalf("Read n=%d want=4", n)
	}
	if got := b.BufferedBytes(); got != 2 {
		t.Fatalf("BufferedBytes() after read=%d want=2", got)
	}
	b.Clear()
	if got := b.BufferedBytes(); got != 0 {
		t.Fatalf("BufferedBytes() after clear=%d want=0", got)
	}
}
