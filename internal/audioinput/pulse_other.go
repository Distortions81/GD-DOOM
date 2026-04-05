//go:build !linux

package audioinput

import (
	"context"
	"fmt"
)

func OpenPulseReader(_ context.Context, _ PulseConfig) (*PulseReader, error) {
	return nil, fmt.Errorf("pulse audio input is only supported on linux")
}

type PulseReader struct{}

func (r *PulseReader) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("pulse audio input is only supported on linux")
}

func (r *PulseReader) Close() error {
	return nil
}
