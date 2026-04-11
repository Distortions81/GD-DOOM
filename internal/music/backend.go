package music

import (
	"fmt"
	"strings"
)

type Backend string

const (
	BackendAuto       Backend = "auto"
	BackendImpSynth   Backend = "impsynth"
	BackendPCSpeaker  Backend = "pcspeaker"
	BackendMeltySynth Backend = "meltysynth"
)

func (b Backend) String() string {
	if strings.TrimSpace(string(b)) == "" {
		return string(DefaultBackend())
	}
	return string(b)
}

func DefaultBackend() Backend {
	return BackendImpSynth
}

func ParseBackend(name string) (Backend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", string(BackendAuto):
		return BackendAuto, nil
	case string(BackendImpSynth):
		return BackendImpSynth, nil
	case string(BackendPCSpeaker):
		return BackendPCSpeaker, nil
	case string(BackendMeltySynth):
		return BackendMeltySynth, nil
	default:
		return "", fmt.Errorf("unknown backend %q (want auto|impsynth|pcspeaker|meltysynth)", name)
	}
}

func ValidateBackend(backend Backend) error {
	switch backend {
	case BackendAuto, BackendImpSynth, BackendPCSpeaker, BackendMeltySynth:
		return nil
	default:
		return fmt.Errorf("unknown backend %q (want auto|impsynth|pcspeaker|meltysynth)", backend)
	}
}

func ResolveBackend(backend Backend) Backend {
	if strings.TrimSpace(string(backend)) == "" || backend == BackendAuto {
		return DefaultBackend()
	}
	return backend
}
