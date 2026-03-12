//go:build cgo

package sound

import "fmt"

func defaultBackend() Backend {
	return BackendNuked
}

func validateBackend(backend Backend) error {
	switch backend {
	case BackendAuto, BackendPureGo, BackendNuked:
		return nil
	default:
		return fmt.Errorf("unknown backend %q (want auto|purego|nuked)", backend)
	}
}

func newOPL3WithBackend(sampleRate int, backend Backend) (OPL3, error) {
	switch backend {
	case BackendAuto, BackendNuked:
		return NewNukedOPL3(sampleRate), nil
	case BackendPureGo:
		return NewBasicOPL3(sampleRate), nil
	default:
		return nil, fmt.Errorf("unknown backend %q (want auto|purego|nuked)", backend)
	}
}
