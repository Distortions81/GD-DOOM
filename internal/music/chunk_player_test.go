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
