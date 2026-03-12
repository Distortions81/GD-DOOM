//go:build !cgo

package sound

import "fmt"

func defaultBackend() Backend {
	return BackendPureGo
}

func validateBackend(backend Backend) error {
	switch backend {
	case BackendAuto, BackendPureGo:
		return nil
	case BackendNuked:
		return fmt.Errorf("backend %q requires a cgo build", backend)
	default:
		return fmt.Errorf("unknown backend %q (want auto|purego|nuked)", backend)
	}
}

func newOPL3WithBackend(sampleRate int, backend Backend) (OPL3, error) {
	switch backend {
	case BackendAuto, BackendPureGo:
		return NewBasicOPL3(sampleRate), nil
	case BackendNuked:
		return nil, fmt.Errorf("backend %q requires a cgo build", backend)
	default:
		return nil, fmt.Errorf("unknown backend %q (want auto|purego|nuked)", backend)
	}
}
