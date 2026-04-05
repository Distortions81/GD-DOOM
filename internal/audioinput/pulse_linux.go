//go:build linux

package audioinput

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
)

type PulseReader struct {
	stdout io.ReadCloser
	cmd    *exec.Cmd
	stderr bytes.Buffer

	closeOnce sync.Once
	closeErr  error
}

func OpenPulseReader(ctx context.Context, cfg PulseConfig) (*PulseReader, error) {
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "parec", pulseArgs(cfg)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("parec stdout pipe: %w", err)
	}
	reader := &PulseReader{
		stdout: stdout,
		cmd:    cmd,
	}
	cmd.Stderr = &reader.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start parec: %w", err)
	}
	return reader, nil
}

func pulseArgs(cfg PulseConfig) []string {
	args := []string{
		"--raw",
		"--rate=" + strconv.Itoa(cfg.SampleRate),
		"--channels=" + strconv.Itoa(cfg.Channels),
		"--format=" + cfg.Format,
		"--latency-msec=" + strconv.Itoa(cfg.LatencyMillis),
	}
	if cfg.Device != "" {
		args = append(args, "--device="+cfg.Device)
	}
	return args
}

func (r *PulseReader) Read(p []byte) (int, error) {
	if r == nil || r.stdout == nil {
		return 0, io.EOF
	}
	return r.stdout.Read(p)
}

func (r *PulseReader) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.stdout != nil {
			_ = r.stdout.Close()
		}
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
			if err := r.cmd.Wait(); err != nil {
				if _, ok := err.(*exec.ExitError); ok {
					return
				}
				if r.stderr.Len() > 0 {
					r.closeErr = fmt.Errorf("stop parec: %w: %s", err, r.stderr.String())
					return
				}
				r.closeErr = fmt.Errorf("stop parec: %w", err)
			}
		}
	})
	return r.closeErr
}
