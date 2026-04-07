package voicecodec

import "fmt"

type SampleRateChoice byte

const (
	SampleRateChoiceCustom SampleRateChoice = iota
	SampleRateChoice16000
	SampleRateChoice24000
	SampleRateChoice32000
	SampleRateChoice48000
)

const (
	CodecIMA4To1         byte = 1
	CodecPCM16Mono       byte = 2
	CodecG72632          byte = 3
	CaptureSampleRate         = 48000
	SampleRate                = 48000
	Channels                  = 1
	FrameDurationMillis       = 30
	PacketFrames              = 1
	CaptureFrameSamples       = CaptureSampleRate * FrameDurationMillis / 1000
	FrameSamples              = SampleRate * FrameDurationMillis / 1000
	PacketSamples             = FrameSamples * PacketFrames
	PacketDurationMillis      = FrameDurationMillis * PacketFrames
	IMA41FrameBytes           = FrameSamples * Channels / 2
	IMA41PacketBytes          = PacketSamples * Channels / 2
	G72632FrameBytes          = FrameSamples * Channels / 2
	G72632PacketBytes         = PacketSamples * Channels / 2
)

func NormalizeG726BitsPerSample(bits int) int {
	switch bits {
	case 2, 3, 4, 5:
		return bits
	default:
		return 4
	}
}

func G726Bitrate(sampleRate, channels, bits int) int {
	if sampleRate <= 0 || channels <= 0 {
		return 0
	}
	return sampleRate * channels * NormalizeG726BitsPerSample(bits)
}

func G726BitsPerSampleFromBitrate(sampleRate, channels, bitrate int) (int, error) {
	if sampleRate <= 0 {
		return 0, fmt.Errorf("sample rate must be > 0")
	}
	if channels <= 0 {
		return 0, fmt.Errorf("channels must be > 0")
	}
	if bitrate <= 0 {
		return 0, fmt.Errorf("bitrate must be > 0")
	}
	denom := sampleRate * channels
	if bitrate%denom != 0 {
		return 0, fmt.Errorf("g726 bitrate=%d must divide evenly by sampleRate*channels=%d", bitrate, denom)
	}
	bits := bitrate / denom
	if bits < 2 || bits > 5 {
		return 0, fmt.Errorf("g726 bits per sample=%d want 2..5", bits)
	}
	return bits, nil
}

func G726PacketBytes(packetSamples, channels, bits int) (int, error) {
	if packetSamples <= 0 {
		return 0, fmt.Errorf("packet samples must be > 0")
	}
	if channels <= 0 {
		return 0, fmt.Errorf("channels must be > 0")
	}
	bits = NormalizeG726BitsPerSample(bits)
	totalBits := packetSamples * channels * bits
	if totalBits%8 != 0 {
		return 0, fmt.Errorf("g726 packet bits=%d must be byte-aligned", totalBits)
	}
	return totalBits / 8, nil
}

func (c SampleRateChoice) SampleRate() int {
	switch c {
	case SampleRateChoice16000:
		return 16000
	case SampleRateChoice24000:
		return 24000
	case SampleRateChoice32000:
		return 32000
	case SampleRateChoice48000:
		return 48000
	default:
		return 0
	}
}

func SampleRateChoiceFromRate(sampleRate int) SampleRateChoice {
	switch sampleRate {
	case 16000:
		return SampleRateChoice16000
	case 24000:
		return SampleRateChoice24000
	case 32000:
		return SampleRateChoice32000
	case 48000:
		return SampleRateChoice48000
	default:
		return SampleRateChoiceCustom
	}
}

func DefaultSampleRateChoice() SampleRateChoice {
	return SampleRateChoiceFromRate(SampleRate)
}

func ResolveSampleRate(choice SampleRateChoice, sampleRate int) (int, error) {
	if rate := choice.SampleRate(); rate > 0 {
		if sampleRate > 0 && sampleRate != rate {
			return 0, fmt.Errorf("sample rate %d does not match choice %d", sampleRate, choice)
		}
		return rate, nil
	}
	if sampleRate <= 0 {
		return 0, fmt.Errorf("sample rate must be > 0")
	}
	return sampleRate, nil
}

func PacketSamplesFor(sampleRate, packetDurationMillis int) (int, error) {
	if sampleRate <= 0 {
		return 0, fmt.Errorf("sample rate must be > 0")
	}
	if packetDurationMillis <= 0 {
		return 0, fmt.Errorf("packet duration must be > 0")
	}
	if (sampleRate*packetDurationMillis)%1000 != 0 {
		return 0, fmt.Errorf("sample rate %d must divide evenly into %d ms packets", sampleRate, packetDurationMillis)
	}
	return sampleRate * packetDurationMillis / 1000, nil
}
