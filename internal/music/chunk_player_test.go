package music

import (
	"encoding/binary"
	"testing"
)

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
	if got := DefaultStreamChunkFrames(); got != streamChunkFrames() {
		t.Fatalf("DefaultStreamChunkFrames()=%d want=%d", got, streamChunkFrames())
	}
	if got := DefaultStreamLookahead(); got != streamLookaheadFrames() {
		t.Fatalf("DefaultStreamLookahead()=%d want=%d", got, streamLookaheadFrames())
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

func TestChunkPlayerResetPlaybackClearsQueuedBuffer(t *testing.T) {
	cp, err := NewChunkPlayer()
	if err != nil {
		t.Skipf("NewChunkPlayer unavailable: %v", err)
	}
	defer func() { _ = cp.Close() }()

	if err := cp.EnqueueBytesS16LE(make([]byte, 256)); err != nil {
		t.Fatalf("EnqueueBytesS16LE() error: %v", err)
	}
	if err := cp.ResetPlayback(); err != nil {
		t.Fatalf("ResetPlayback() error: %v", err)
	}
	if got := cp.BufferedBytes(); got != 0 {
		t.Fatalf("BufferedBytes() after reset=%d want=0", got)
	}
	if err := cp.EnqueueBytesS16LE(make([]byte, 128)); err != nil {
		t.Fatalf("enqueue after reset error: %v", err)
	}
}

func TestChunkPlayerEnqueueS16EncodesLittleEndian(t *testing.T) {
	cp := &ChunkPlayer{
		src:  newPCMChunkBuffer(),
		cmds: make(chan playerCmd, 1),
		done: make(chan struct{}),
	}
	samples := []int16{0x1234, -2}
	if err := cp.EnqueueS16(samples); err != nil {
		t.Fatalf("EnqueueS16() error: %v", err)
	}
	cmd := <-cp.cmds
	defer cp.src.releaseChunk(cmd.data)
	if len(cmd.data) != 4 {
		t.Fatalf("len=%d want=4", len(cmd.data))
	}
	if cmd.data[0] != 0x34 || cmd.data[1] != 0x12 {
		t.Fatalf("first sample bytes=%02x %02x want=34 12", cmd.data[0], cmd.data[1])
	}
	if cmd.data[2] != 0xFE || cmd.data[3] != 0xFF {
		t.Fatalf("second sample bytes=%02x %02x want=fe ff", cmd.data[2], cmd.data[3])
	}
}

func TestPCMChunkBufferAppliesStarvationFadeOutAndFadeIn(t *testing.T) {
	b := newPCMChunkBuffer()
	src := make([]byte, 8)
	putI16LE(src[0:], 1000)
	putI16LE(src[2:], -1000)
	putI16LE(src[4:], 2000)
	putI16LE(src[6:], -2000)
	b.Enqueue(src)

	readBuf := make([]byte, len(src))
	n, err := b.Read(readBuf)
	if err != nil || n != len(src) {
		t.Fatalf("initial Read n=%d err=%v", n, err)
	}

	starve := make([]byte, starvationFadeFrames()*4)
	n, err = b.Read(starve)
	if err != nil || n != len(starve) {
		t.Fatalf("starvation Read n=%d err=%v", n, err)
	}
	firstL := int16(binary.LittleEndian.Uint16(starve[0:]))
	lastL := int16(binary.LittleEndian.Uint16(starve[len(starve)-4:]))
	if firstL == 0 {
		t.Fatal("expected starvation fade-out to start above zero")
	}
	if lastL != 0 {
		t.Fatalf("expected starvation fade-out to end at zero, got %d", lastL)
	}

	recover := make([]byte, starvationFadeFrames()*4)
	for i := 0; i < starvationFadeFrames(); i++ {
		putI16LE(recover[i*4:], 4000)
		putI16LE(recover[i*4+2:], -4000)
	}
	b.Enqueue(recover)

	n, err = b.Read(recover)
	if err != nil || n != len(recover) {
		t.Fatalf("recovery Read n=%d err=%v", n, err)
	}
	firstRecovered := int16(binary.LittleEndian.Uint16(recover[0:]))
	lastRecovered := int16(binary.LittleEndian.Uint16(recover[len(recover)-4:]))
	if firstRecovered <= 0 || firstRecovered >= 4000 {
		t.Fatalf("expected fade-in on first recovery sample, got %d", firstRecovered)
	}
	if lastRecovered != 4000 {
		t.Fatalf("expected fade-in to reach full scale, got %d", lastRecovered)
	}
}

func putI16LE(dst []byte, v int16) {
	binary.LittleEndian.PutUint16(dst, uint16(v))
}
