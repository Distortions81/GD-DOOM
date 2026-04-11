//go:build linux

package audiofx

import (
	"encoding/binary"
	"os"
	"path/filepath"
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

func TestLinuxPCSpeakerShouldSilenceOnCloseLocked(t *testing.T) {
	t.Parallel()

	t.Run("unused speaker skips silence", func(t *testing.T) {
		t.Parallel()

		p := &LinuxPCSpeakerPlayer{}
		if p.shouldSilenceOnCloseLocked() {
			t.Fatal("shouldSilenceOnCloseLocked() = true, want false")
		}
	})

	t.Run("used speaker emits silence", func(t *testing.T) {
		t.Parallel()

		p := &LinuxPCSpeakerPlayer{usedSpeaker: true}
		if !p.shouldSilenceOnCloseLocked() {
			t.Fatal("shouldSilenceOnCloseLocked() = false, want true")
		}
	})
}

func TestFindLinuxPCSpeakerDeviceFromPatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	devicePath := filepath.Join(dir, "platform-pcspkr-event-spkr")
	if err := os.WriteFile(devicePath, nil, 0o600); err != nil {
		t.Fatalf("write test device: %v", err)
	}

	got, err := findLinuxPCSpeakerDeviceFromPatterns([]string{
		filepath.Join(dir, "*pcspkr*-event-spkr"),
	})
	if err != nil {
		t.Fatalf("findLinuxPCSpeakerDeviceFromPatterns() error = %v", err)
	}
	if got != devicePath {
		t.Fatalf("findLinuxPCSpeakerDeviceFromPatterns() = %q want %q", got, devicePath)
	}
}

func TestWriteLinuxPCSpeakerTone(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "pcspkr-event-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	if err := writeLinuxPCSpeakerTone(f, 0); err != nil {
		t.Fatalf("writeLinuxPCSpeakerTone(silence): %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var tone linuxInputEvent
	if err := binary.Read(f, binary.LittleEndian, &tone); err != nil {
		t.Fatalf("read tone event: %v", err)
	}
	if tone.Type != linuxInputEventTypeSound || tone.Code != linuxInputSoundTone || tone.Value != 0 {
		t.Fatalf("tone event = %+v, want type=%d code=%d value=0", tone, linuxInputEventTypeSound, linuxInputSoundTone)
	}
	var sync linuxInputEvent
	if err := binary.Read(f, binary.LittleEndian, &sync); err != nil {
		t.Fatalf("read sync event: %v", err)
	}
	if sync.Type != linuxInputEventTypeSync || sync.Code != linuxInputSyncReport {
		t.Fatalf("sync event = %+v, want type=%d code=%d", sync, linuxInputEventTypeSync, linuxInputSyncReport)
	}
}
