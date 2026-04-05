package voicecodec

const (
	CodecIMA4To1        byte = 1
	CodecPCM16Mono      byte = 2
	SampleRate               = 48000
	Channels                 = 1
	FrameDurationMillis      = 20
	FrameSamples             = SampleRate * FrameDurationMillis / 1000
)
