package sessionvoice

import (
	"encoding/binary"
	"fmt"
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

func TestStreamSourcePacketDurationAffectsThresholds(t *testing.T) {
	src := newStreamSource(testPlaybackRate)

	src.SetPacketDurationMillis(voicecodec.SilkPacketDurationMillis, testPlaybackRate)
	silkSamples := (testPlaybackRate*voicecodec.SilkPacketDurationMillis + 999) / 1000
	silkBytes := silkSamples * 4
	if src.startupBytes != audioStartupBufferFrames*silkBytes {
		t.Fatalf("silk startupBytes=%d want=%d", src.startupBytes, audioStartupBufferFrames*silkBytes)
	}
	if src.trimBufferedBytes != audioTrimBufferedFrames*silkBytes {
		t.Fatalf("silk trimBufferedBytes=%d want=%d", src.trimBufferedBytes, audioTrimBufferedFrames*silkBytes)
	}
	if src.resetBufferedBytes != audioResetBufferedFrames*silkBytes {
		t.Fatalf("silk resetBufferedBytes=%d want=%d", src.resetBufferedBytes, audioResetBufferedFrames*silkBytes)
	}

	src.SetPacketDurationMillis(voicecodec.PacketDurationMillis, testPlaybackRate)
	pcmSamples := (testPlaybackRate*voicecodec.PacketDurationMillis + 999) / 1000
	pcmBytes := pcmSamples * 4
	if src.startupBytes != audioStartupBufferFrames*pcmBytes {
		t.Fatalf("pcm startupBytes=%d want=%d", src.startupBytes, audioStartupBufferFrames*pcmBytes)
	}
	if src.trimBufferedBytes != audioTrimBufferedFrames*pcmBytes {
		t.Fatalf("pcm trimBufferedBytes=%d want=%d", src.trimBufferedBytes, audioTrimBufferedFrames*pcmBytes)
	}
	if src.resetBufferedBytes != audioResetBufferedFrames*pcmBytes {
		t.Fatalf("pcm resetBufferedBytes=%d want=%d", src.resetBufferedBytes, audioResetBufferedFrames*pcmBytes)
	}
}

func TestStreamSourceKeepsOnlyLatestStartupWindowBeforePlaybackStarts(t *testing.T) {
	src := newStreamSource(testPlaybackRate)
	frame := make([]byte, src.startupBytes/2)
	for i := 0; i < len(frame); i += 4 {
		binary.LittleEndian.PutUint16(frame[i:i+2], uint16(5000))
		binary.LittleEndian.PutUint16(frame[i+2:i+4], uint16(5000))
	}

	for range 8 {
		src.Write(frame)
	}

	if src.started {
		t.Fatal("stream should not be marked started before any read")
	}
	if got := len(src.buf); got != src.startupBytes {
		t.Fatalf("prestart buffered bytes=%d want=%d", got, src.startupBytes)
	}
	if len(src.fade) != 0 {
		t.Fatal("prestart trim should not queue fade data")
	}
	if src.totalDroppedFrames == 0 {
		t.Fatal("expected prestart trim to drop stale buffered audio")
	}
}

func TestStreamSourceDoesNotSkipUntilBufferedAudioHasActuallyPlayed(t *testing.T) {
	src := newStreamSource(testPlaybackRate)
	frame := make([]byte, src.startupBytes/2)
	for i := 0; i < len(frame); i += 4 {
		binary.LittleEndian.PutUint16(frame[i:i+2], uint16(4000))
		binary.LittleEndian.PutUint16(frame[i+2:i+4], uint16(4000))
	}

	out := make([]byte, 32)
	if _, err := src.Read(out); err != nil {
		t.Fatalf("prestart Read() error = %v", err)
	}
	for range 8 {
		src.Write(frame)
	}
	if len(src.fade) != 0 {
		t.Fatal("preplayback writes should not trigger skip fade")
	}
	if got := len(src.buf); got != src.startupBytes {
		t.Fatalf("preplayback buffered bytes=%d want=%d", got, src.startupBytes)
	}
}

func TestResolveBroadcasterFormatDefaults(t *testing.T) {
	got, err := resolveBroadcasterFormat(BroadcasterOptions{})
	if err != nil {
		t.Fatalf("resolveBroadcasterFormat() error = %v", err)
	}
	if got.Codec != voicecodec.CodecSilkV3 {
		t.Fatalf("codec=%d want %d", got.Codec, voicecodec.CodecSilkV3)
	}
	if got.BitsPerSample != 0 {
		t.Fatalf("bits/sample=%d want 0", got.BitsPerSample)
	}
	if got.SampleRate != 48000 {
		t.Fatalf("sample rate=%d want 48000", got.SampleRate)
	}
	if got.PacketSamples != 960 {
		t.Fatalf("packet samples=%d want 960", got.PacketSamples)
	}
}

func TestResolveBroadcasterFormatHonorsSilkAndSampleRate(t *testing.T) {
	got, err := resolveBroadcasterFormat(BroadcasterOptions{
		Codec:      "silk",
		SampleRate: 48000,
	})
	if err != nil {
		t.Fatalf("resolveBroadcasterFormat() error = %v", err)
	}
	if got.Codec != voicecodec.CodecSilkV3 {
		t.Fatalf("codec=%d want %d", got.Codec, voicecodec.CodecSilkV3)
	}
	if got.SampleRate != 48000 {
		t.Fatalf("sample rate=%d want 48000", got.SampleRate)
	}
	if got.PacketSamples != 960 {
		t.Fatalf("packet samples=%d want 960", got.PacketSamples)
	}
	if got.Bitrate != voicecodec.SilkDefaultBitrate {
		t.Fatalf("bitrate=%d want %d", got.Bitrate, voicecodec.SilkDefaultBitrate)
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

func TestResolveBroadcasterFormatHonorsG726AndSampleRate(t *testing.T) {
	got, err := resolveBroadcasterFormat(BroadcasterOptions{
		Codec:             "g726",
		G726BitsPerSample: 4,
		SampleRate:        16000,
	})
	if err != nil {
		t.Fatalf("resolveBroadcasterFormat() error = %v", err)
	}
	if got.Codec != voicecodec.CodecG72632 {
		t.Fatalf("codec=%d want %d", got.Codec, voicecodec.CodecG72632)
	}
	if got.SampleRate != 16000 {
		t.Fatalf("sample rate=%d want 16000", got.SampleRate)
	}
	if got.PacketSamples != 480 {
		t.Fatalf("packet samples=%d want 480", got.PacketSamples)
	}
	if got.Bitrate != 16000*4 {
		t.Fatalf("bitrate=%d want %d", got.Bitrate, 16000*4)
	}
}

func TestResolveBroadcasterFormatHonorsG726BitDepth(t *testing.T) {
	got, err := resolveBroadcasterFormat(BroadcasterOptions{
		Codec:             "g726",
		G726BitsPerSample: 2,
		SampleRate:        16000,
	})
	if err != nil {
		t.Fatalf("resolveBroadcasterFormat() error = %v", err)
	}
	if got.Bitrate != 16000*2 {
		t.Fatalf("bitrate=%d want %d", got.Bitrate, 16000*2)
	}
}

func TestResolveBroadcasterFormatRejectsBadCodec(t *testing.T) {
	if _, err := resolveBroadcasterFormat(BroadcasterOptions{Codec: "nope"}); err == nil {
		t.Fatal("expected codec error")
	}
}

func TestVoiceDecodePlaybackFrameAccountingStaysBalanced(t *testing.T) {
	testCases := []struct {
		name                string
		sampleRate          int
		packetDurationMilli int
		packets             int
	}{
		{name: "silk_48k_20ms", sampleRate: 48000, packetDurationMilli: voicecodec.SilkPacketDurationMillis, packets: 200},
		{name: "pcm_16k_30ms", sampleRate: 16000, packetDurationMilli: voicecodec.PacketDurationMillis, packets: 200},
		{name: "pcm_24k_30ms", sampleRate: 24000, packetDurationMilli: voicecodec.PacketDurationMillis, packets: 200},
		{name: "pcm_32k_30ms", sampleRate: 32000, packetDurationMilli: voicecodec.PacketDurationMillis, packets: 200},
		{name: "pcm_48k_30ms", sampleRate: 48000, packetDurationMilli: voicecodec.PacketDurationMillis, packets: 200},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			packetSamples, err := voicecodec.PacketSamplesFor(tc.sampleRate, tc.packetDurationMilli)
			if err != nil {
				t.Fatalf("PacketSamplesFor() error = %v", err)
			}

			src := newStreamSource(testPlaybackRate)
			playbackPacketSamples, err := voicecodec.PacketSamplesFor(testPlaybackRate, tc.packetDurationMilli)
			if err != nil {
				t.Fatalf("PacketSamplesFor(playback) error = %v", err)
			}
			packetBytes := playbackPacketSamples * 4
			readBuf := make([]byte, packetBytes)
			var totalQueuedFrames int
			var totalPlayedFrames int
			started := false
			for packet := range tc.packets {
				pcm := make([]int16, packetSamples)
				for i := range pcm {
					pcm[i] = int16(((packet+1)*(i+17))%20000 - 10000)
				}
				resampled := resampleMonoLinear(pcm, tc.sampleRate, testPlaybackRate)
				if len(resampled) != playbackPacketSamples {
					t.Fatalf("resampled len=%d want=%d", len(resampled), playbackPacketSamples)
				}
				totalQueuedFrames += len(resampled)
				src.Write(stereoBytesFromMonoPCM(resampled))
				if !started && len(src.buf) >= src.startupBytes {
					started = true
				}
				if !started {
					continue
				}
				n, err := src.Read(readBuf)
				if err != nil {
					t.Fatalf("Read() error = %v", err)
				}
				if n%4 != 0 {
					t.Fatalf("Read() bytes=%d must be divisible by 4", n)
				}
				totalPlayedFrames += n / 4
			}

			for len(src.buf) > 0 {
				n, err := src.Read(readBuf)
				if err != nil {
					t.Fatalf("Read() drain error = %v", err)
				}
				if n%4 != 0 {
					t.Fatalf("Read() drain bytes=%d must be divisible by 4", n)
				}
				totalPlayedFrames += n / 4
			}

			deltaFrames := totalQueuedFrames - totalPlayedFrames
			if deltaFrames != 0 {
				t.Fatalf("deltaFrames=%d queued=%d played=%d", deltaFrames, totalQueuedFrames, totalPlayedFrames)
			}
			if got := src.BufferedMillis(testPlaybackRate); got != 0 {
				t.Fatalf("BufferedMillis()=%d want 0", got)
			}
		})
	}
}

func TestResampleMonoLinearPacketLengthMatchesPlaybackDuration(t *testing.T) {
	testCases := []struct {
		sampleRate          int
		packetDurationMilli int
	}{
		{sampleRate: 16000, packetDurationMilli: voicecodec.PacketDurationMillis},
		{sampleRate: 24000, packetDurationMilli: voicecodec.PacketDurationMillis},
		{sampleRate: 32000, packetDurationMilli: voicecodec.PacketDurationMillis},
		{sampleRate: 48000, packetDurationMilli: voicecodec.PacketDurationMillis},
		{sampleRate: 48000, packetDurationMilli: voicecodec.SilkPacketDurationMillis},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("%dhz_%dms", tc.sampleRate, tc.packetDurationMilli)
		t.Run(name, func(t *testing.T) {
			packetSamples, err := voicecodec.PacketSamplesFor(tc.sampleRate, tc.packetDurationMilli)
			if err != nil {
				t.Fatalf("PacketSamplesFor() error = %v", err)
			}

			got := len(resampleMonoLinear(make([]int16, packetSamples), tc.sampleRate, testPlaybackRate))
			want, err := voicecodec.PacketSamplesFor(testPlaybackRate, tc.packetDurationMilli)
			if err != nil {
				t.Fatalf("PacketSamplesFor(playback) error = %v", err)
			}
			if got != want {
				t.Fatalf("resampled len=%d want=%d", got, want)
			}
		})
	}
}
