package sound

import "fmt"

func defaultBackend() Backend {
	return BackendImpSynth
}

func validateBackend(backend Backend) error {
	switch backend {
	case BackendAuto, BackendImpSynth:
		return nil
	default:
		return fmt.Errorf("unknown backend %q (want auto|impsynth)", backend)
	}
}

func newSynthWithBackend(sampleRate int, backend Backend) (Synth, error) {
	switch backend {
	case BackendAuto, BackendImpSynth:
		return NewImpSynth(sampleRate), nil
	default:
		return nil, fmt.Errorf("unknown backend %q (want auto|impsynth)", backend)
	}
}
