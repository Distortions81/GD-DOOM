package voicecodec

const (
	CodecIMA4To1         byte = 1
	CodecPCM16Mono       byte = 2
	CaptureSampleRate         = 48000
	SampleRate                = 32000
	Channels                  = 1
	FrameDurationMillis       = 10
	PacketFrames              = 5
	CaptureFrameSamples       = CaptureSampleRate * FrameDurationMillis / 1000
	FrameSamples              = SampleRate * FrameDurationMillis / 1000
	PacketSamples             = FrameSamples * PacketFrames
	PacketDurationMillis      = FrameDurationMillis * PacketFrames
	IMA41FrameBytes           = FrameSamples * Channels / 2
	IMA41PacketBytes          = PacketSamples * Channels / 2
)
