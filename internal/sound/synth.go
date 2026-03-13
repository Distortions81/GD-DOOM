package sound

import (
	"fmt"
	"strings"
)

type Backend string

const (
	BackendAuto     Backend = "auto"
	BackendImpSynth Backend = "impsynth"
)

func (b Backend) String() string {
	if strings.TrimSpace(string(b)) == "" {
		return string(BackendAuto)
	}
	return string(b)
}

func ParseBackend(name string) (Backend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", string(BackendAuto):
		return BackendAuto, nil
	case string(BackendImpSynth):
		return BackendImpSynth, nil
	default:
		return "", fmt.Errorf("unknown backend %q (want auto|impsynth)", name)
	}
}

func DefaultBackend() Backend {
	return defaultBackend()
}

func ValidateBackend(backend Backend) error {
	return validateBackend(backend)
}

// Synth is the runtime synth interface used by the music driver.
type Synth interface {
	Reset()
	WriteReg(addr uint16, value uint8)
	GenerateStereoS16(frames int) []int16
	GenerateMonoU8(frames int) []byte
}

// NewSynth creates the default synth backend for the current build.
func NewSynth(sampleRate int) Synth {
	synth, err := NewSynthWithBackend(sampleRate, BackendAuto)
	if err == nil {
		return synth
	}
	return NewImpSynth(sampleRate)
}

func NewSynthWithBackend(sampleRate int, backend Backend) (Synth, error) {
	if strings.TrimSpace(string(backend)) == "" {
		backend = BackendAuto
	}
	if err := ValidateBackend(backend); err != nil {
		return nil, err
	}
	return newSynthWithBackend(sampleRate, backend)
}
