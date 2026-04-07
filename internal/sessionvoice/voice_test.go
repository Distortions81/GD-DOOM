package sessionvoice

import (
	"encoding/binary"
	"testing"

	"gddoom/internal/voicecodec"
)

const testPlaybackRate = 44100

func TestStreamSourceWaitsForStartupBuffer(t *testing.T) {
	src := newStreamSource(testPlaybackRate)
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

	frame := make([]byte, src.startupBytes)
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
	src := newStreamSource(testPlaybackRate)
	frame := make([]byte, src.startupBytes)
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
	fade := make([]byte, audioCatchupFadeSamples*4)
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
	silence := make([]byte, 32)
	if _, err := src.Read(silence); err != nil {
		t.Fatalf("silence Read() error = %v", err)
	}
	for i, b := range silence {
		if b != 0 {
			t.Fatalf("silence[%d]=%d want 0 after one fade-out", i, b)
		}
	}
}

func TestStreamSourceResetsLargeBacklogToNewestTail(t *testing.T) {
	src := newStreamSource(testPlaybackRate)
	frame := make([]byte, src.startupBytes)
	for i := 0; i < len(frame); i += 4 {
		binary.LittleEndian.PutUint16(frame[i:i+2], uint16(4000))
		binary.LittleEndian.PutUint16(frame[i+2:i+4], uint16(4000))
	}
	for range audioStartupBufferFrames {
		src.Write(frame)
	}
	warm := make([]byte, src.startupBytes)
	if _, err := src.Read(warm); err != nil {
		t.Fatalf("warm Read() error = %v", err)
	}
	for range audioResetBufferedFrames/audioStartupBufferFrames + 2 {
		src.Write(frame)
		if len(src.fade) > 0 {
			break
		}
	}
	if got, wantMax := len(src.buf), src.trimBufferedBytes; got > wantMax {
		t.Fatalf("buffered bytes=%d want <= %d", got, wantMax)
	}
	if len(src.fade) == 0 {
		t.Fatal("expected backlog skip to queue fade-out transition")
	}
	first := int16(binary.LittleEndian.Uint16(src.buf[0:2]))
	if first == 4000 {
		t.Fatal("expected kept tail to be faded in after backlog skip")
	}
}

func TestResolveBroadcasterFormatDefaults(t *testing.T) {
	got, err := resolveBroadcasterFormat(BroadcasterOptions{})
	if err != nil {
		t.Fatalf("resolveBroadcasterFormat() error = %v", err)
	}
	if got.Codec != voicecodec.CodecIMA4To1 {
		t.Fatalf("codec=%d want %d", got.Codec, voicecodec.CodecIMA4To1)
	}
	if got.SampleRate != voicecodec.SampleRate {
		t.Fatalf("sample rate=%d want %d", got.SampleRate, voicecodec.SampleRate)
	}
}

func TestResolveBroadcasterFormatHonorsPCMAndSampleRate(t *testing.T) {
	got, err := resolveBroadcasterFormat(BroadcasterOptions{
		Codec:      "pcm",
		SampleRate: 16000,
	})
	if err != nil {
		t.Fatalf("resolveBroadcasterFormat() error = %v", err)
	}
	if got.Codec != voicecodec.CodecPCM16Mono {
		t.Fatalf("codec=%d want %d", got.Codec, voicecodec.CodecPCM16Mono)
	}
	if got.SampleRate != 16000 {
		t.Fatalf("sample rate=%d want 16000", got.SampleRate)
	}
	if got.PacketSamples != 480 {
		t.Fatalf("packet samples=%d want 480", got.PacketSamples)
	}
}

func TestResolveBroadcasterFormatRejectsBadCodec(t *testing.T) {
	if _, err := resolveBroadcasterFormat(BroadcasterOptions{Codec: "nope"}); err == nil {
		t.Fatal("expected codec error")
	}
}
