package audiofx

import "gddoom/internal/sound"

type PCSpeaker interface {
	Play(seq []sound.PCSpeakerTone)
	SetMusic(seq []sound.PCSpeakerTone, tickRate int, loop bool)
	ClearMusic()
	Stop()
	SetVolume(v float64)
	Close() error
}

type PCSpeakerOutput string

const (
	PCSpeakerOutputEmulated PCSpeakerOutput = "emulated"
	PCSpeakerOutputLinux    PCSpeakerOutput = "linux"
)

func ParsePCSpeakerOutput(s string) PCSpeakerOutput {
	switch s {
	case string(PCSpeakerOutputLinux):
		return PCSpeakerOutputLinux
	default:
		return PCSpeakerOutputEmulated
	}
}
