//go:build cgo

package sound

import "fmt"

func defaultBackend() Backend {
	return BackendNuked
}

func validateBackend(backend Backend) error {
	switch backend {
	case BackendAuto, BackendImpSynth, BackendNuked:
		return nil
	default:
		return fmt.Errorf("unknown backend %q (want auto|impsynth|nuked)", backend)
	}
}

func newOPL3WithBackend(sampleRate int, backend Backend) (OPL3, error) {
	switch backend {
	case BackendAuto, BackendNuked:
		return NewNukedOPL3(sampleRate), nil
	case BackendImpSynth:
		return NewImpSynth(sampleRate), nil
	default:
		return nil, fmt.Errorf("unknown backend %q (want auto|impsynth|nuked)", backend)
	}
}
