//go:build !linux

package audiofx

import (
	"fmt"

	"gddoom/internal/sound"
)

type LinuxPCSpeakerPlayer struct{}

func NewLinuxPCSpeakerPlayer() (*LinuxPCSpeakerPlayer, error) {
	return nil, fmt.Errorf("linux pc speaker playback is only supported on linux")
}

func (p *LinuxPCSpeakerPlayer) Close() error { return nil }

func (p *LinuxPCSpeakerPlayer) Stop() {}

func (p *LinuxPCSpeakerPlayer) PlaySequence(seq []sound.PCSpeakerTone, tickRate int) error {
	return fmt.Errorf("linux pc speaker playback is only supported on linux")
}

func (p *LinuxPCSpeakerPlayer) Play(seq []sound.PCSpeakerTone) {}
func (p *LinuxPCSpeakerPlayer) SetMusic(seq []sound.PCSpeakerTone, tickRate int, loop bool) {
}
func (p *LinuxPCSpeakerPlayer) ClearMusic()         {}
func (p *LinuxPCSpeakerPlayer) SetVolume(v float64) {}
