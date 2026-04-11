//go:build linux

package audiofx

import (
	"os"
	"testing"
)

func TestLinuxPCSpeakerPrepareDivisorChangeLocked(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "pcspkr-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	p := &LinuxPCSpeakerPlayer{f: f}

	gotFile, changed, err := p.prepareDivisorChangeLocked(1234)
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	if !changed {
		t.Fatal("expected first divisor change to be emitted")
	}
	if gotFile != f {
		t.Fatal("expected original file handle")
	}

	p.lastDivisor = 1234
	gotFile, changed, err = p.prepareDivisorChangeLocked(1234)
	if err != nil {
		t.Fatalf("repeat prepare: %v", err)
	}
	if changed {
		t.Fatal("expected repeated divisor to be suppressed")
	}
	if gotFile != nil {
		t.Fatal("expected no file when no change is needed")
	}

	p.lastDivisor = 1234
	gotFile, changed, err = p.prepareDivisorChangeLocked(0)
	if err != nil {
		t.Fatalf("silence prepare: %v", err)
	}
	if !changed {
		t.Fatal("expected silence to be emitted when stopping an active tone")
	}
	if gotFile != f {
		t.Fatal("expected original file handle for silence write")
	}
}

func TestLinuxPCSpeakerPrepareDivisorChangeLockedClosed(t *testing.T) {
	t.Parallel()

	p := &LinuxPCSpeakerPlayer{}
	if _, _, err := p.prepareDivisorChangeLocked(42); err == nil {
		t.Fatal("expected closed player error")
	}
}
