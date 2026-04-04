package sessionvoice

import (
	"encoding/binary"
	"testing"

	"gddoom/internal/voicecodec"
)

func TestStreamSourceWaitsForStartupBuffer(t *testing.T) {
	src := newStreamSource()
	out := make([]byte, 16)
	n, err := src.Read(out)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != len(out) {
		t.Fatalf("Read() bytes=%d want=%d", n, len(out))
	}
	for _, b := range out {
		if b != 0 {
			t.Fatal("startup read should be silence before jitter buffer fills")
		}
	}

	frame := make([]byte, voicecodec.FrameSamples*4)
	for range audioStartupBufferFrames {
		src.Write(frame)
	}
	n, err = src.Read(out)
	if err != nil {
		t.Fatalf("Read() after startup buffer error = %v", err)
	}
	if n != len(out) {
		t.Fatalf("Read() bytes=%d want=%d", n, len(out))
	}
}

func TestStreamSourceResetProducesFadeAndThenSilence(t *testing.T) {
	src := newStreamSource()
	frame := make([]byte, voicecodec.FrameSamples*4)
	for i := 0; i < len(frame); i += 4 {
		binary.LittleEndian.PutUint16(frame[i:i+2], uint16(12000))
		binary.LittleEndian.PutUint16(frame[i+2:i+4], uint16(12000))
	}
	for range audioStartupBufferFrames {
		src.Write(frame)
	}
	warm := make([]byte, len(frame))
	if _, err := src.Read(warm); err != nil {
		t.Fatalf("warm Read() error = %v", err)
	}

	src.Reset()
	fade := make([]byte, audioFadeSamples*4)
	if _, err := src.Read(fade); err != nil {
		t.Fatalf("fade Read() error = %v", err)
	}
	first := int16(binary.LittleEndian.Uint16(fade[0:2]))
	last := int16(binary.LittleEndian.Uint16(fade[len(fade)-4 : len(fade)-2]))
	if first == 0 {
		t.Fatal("fade should start above zero")
	}
	if last != 0 {
		t.Fatalf("fade should end at zero, got %d", last)
	}
}
