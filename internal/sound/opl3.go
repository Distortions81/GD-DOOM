package sound

import (
	"fmt"
	"strings"
)

type Backend string

const (
	BackendAuto     Backend = "auto"
	BackendImpSynth Backend = "impsynth"
	BackendNuked    Backend = "nuked"
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
	case string(BackendNuked):
		return BackendNuked, nil
	default:
		return "", fmt.Errorf("unknown backend %q (want auto|impsynth|nuked)", name)
	}
}

func DefaultBackend() Backend {
	return defaultBackend()
}

func ValidateBackend(backend Backend) error {
	return validateBackend(backend)
}

// OPL3 is the runtime synth interface used by the music driver.
type OPL3 interface {
	Reset()
	WriteReg(addr uint16, value uint8)
	GenerateStereoS16(frames int) []int16
	GenerateMonoU8(frames int) []byte
}

// NewOPL3 creates the default OPL3 backend for the current build.
func NewOPL3(sampleRate int) OPL3 {
	opl, err := NewOPL3WithBackend(sampleRate, BackendAuto)
	if err == nil {
		return opl
	}
	return NewImpSynth(sampleRate)
}

func NewOPL3WithBackend(sampleRate int, backend Backend) (OPL3, error) {
	if strings.TrimSpace(string(backend)) == "" {
		backend = BackendAuto
	}
	if err := ValidateBackend(backend); err != nil {
		return nil, err
	}
	return newOPL3WithBackend(sampleRate, backend)
}
