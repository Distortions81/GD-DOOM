package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gddoom/internal/sound"
)

func TestRunCaptureSFXWritesCapture(t *testing.T) {
	oldOut := stdOut
	oldErr := stdErr
	stdOut = &bytes.Buffer{}
	stdErr = &bytes.Buffer{}
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	td := t.TempDir()
	wadPath := filepath.Join(td, "pcsfx.wad")
	if err := os.WriteFile(wadPath, minimalWAD(t, "PWAD", []wadLump{
		{
			name: "DPPISTOL",
			data: []byte{
				0, 0,
				4, 0,
				10, 20, 0, 30,
			},
		},
	}), 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}
	outPath := filepath.Join(td, "pistol.gdpc")
	code := run([]string{
		"capture-sfx",
		"-wad", wadPath,
		"-sound", "DPPISTOL",
		"-out", outPath,
	})
	if code != 0 {
		t.Fatalf("run() code=%d want=0", code)
	}
	capture, err := readCaptureFile(outPath)
	if err != nil {
		t.Fatalf("readCaptureFile() error: %v", err)
	}
	if capture.TickRate != 140 {
		t.Fatalf("TickRate=%d want=140", capture.TickRate)
	}
	if len(capture.Tones) == 0 {
		t.Fatal("expected non-empty tone capture")
	}
}

func TestRunCaptureSFXRepeatWritesLongerCapture(t *testing.T) {
	oldOut := stdOut
	oldErr := stdErr
	stdOut = &bytes.Buffer{}
	stdErr = &bytes.Buffer{}
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	td := t.TempDir()
	wadPath := filepath.Join(td, "pcsfx.wad")
	if err := os.WriteFile(wadPath, minimalWAD(t, "PWAD", []wadLump{
		{
			name: "DPPISTOL",
			data: []byte{
				0, 0,
				2, 0,
				10, 20,
			},
		},
	}), 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}
	outPath := filepath.Join(td, "pistol-repeat.gdpc")
	code := run([]string{
		"capture-sfx",
		"-wad", wadPath,
		"-sound", "DPPISTOL",
		"-out", outPath,
		"-repeat", "3",
		"-gap", "1",
	})
	if code != 0 {
		t.Fatalf("run() code=%d want=0", code)
	}
	capture, err := readCaptureFile(outPath)
	if err != nil {
		t.Fatalf("readCaptureFile() error: %v", err)
	}
	if len(capture.Tones) != 8 {
		t.Fatalf("len(Tones)=%d want=8", len(capture.Tones))
	}
}

func TestRunWAVWritesWAV(t *testing.T) {
	oldOut := stdOut
	oldErr := stdErr
	stdOut = &bytes.Buffer{}
	stdErr = &bytes.Buffer{}
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	td := t.TempDir()
	capturePath := filepath.Join(td, "tone.gdpc")
	if err := writeCaptureFile(capturePath, sound.PCSpeakerCapture{
		TickRate: 140,
		Tones: []sound.PCSpeakerTone{
			{Active: true, ToneValue: 20},
			{Active: true, ToneValue: 20},
			{Active: false},
			{Active: true, ToneValue: 30},
		},
	}); err != nil {
		t.Fatalf("writeCaptureFile() error: %v", err)
	}
	outPath := filepath.Join(td, "tone.wav")
	code := run([]string{
		"wav",
		"-in", capturePath,
		"-out", outPath,
		"-variant", "passthrough",
	})
	if code != 0 {
		t.Fatalf("run() code=%d want=0", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) < 44 {
		t.Fatalf("wav too small: %d", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("invalid wav header: %q %q", data[0:4], data[8:12])
	}
}

func TestRunWAVWritesInterleavedWAV(t *testing.T) {
	oldOut := stdOut
	oldErr := stdErr
	stdOut = &bytes.Buffer{}
	stdErr = &bytes.Buffer{}
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	td := t.TempDir()
	leftPath := filepath.Join(td, "left.gdpc")
	rightPath := filepath.Join(td, "right.gdpc")
	if err := writeCaptureFile(leftPath, sound.PCSpeakerCapture{
		TickRate: 140,
		Tones: []sound.PCSpeakerTone{
			{Active: true, ToneValue: 20},
			{Active: true, ToneValue: 20},
			{Active: true, ToneValue: 20},
			{Active: true, ToneValue: 20},
		},
	}); err != nil {
		t.Fatalf("writeCaptureFile(left) error: %v", err)
	}
	if err := writeCaptureFile(rightPath, sound.PCSpeakerCapture{
		TickRate: 140,
		Tones: []sound.PCSpeakerTone{
			{Active: true, ToneValue: 40},
			{Active: true, ToneValue: 40},
			{Active: true, ToneValue: 40},
			{Active: true, ToneValue: 40},
		},
	}); err != nil {
		t.Fatalf("writeCaptureFile(right) error: %v", err)
	}
	outPath := filepath.Join(td, "mixed.wav")
	code := run([]string{
		"wav",
		"-in", leftPath,
		"-mix", rightPath,
		"-out", outPath,
		"-variant", "passthrough",
	})
	if code != 0 {
		t.Fatalf("run() code=%d want=0", code)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) < 44 {
		t.Fatalf("wav too small: %d", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("invalid wav header: %q %q", data[0:4], data[8:12])
	}
}

func TestRunInterleaveWritesCaptureAt560Hz(t *testing.T) {
	oldOut := stdOut
	oldErr := stdErr
	stdOut = &bytes.Buffer{}
	stdErr = &bytes.Buffer{}
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	td := t.TempDir()
	leftPath := filepath.Join(td, "left.gdpc")
	rightPath := filepath.Join(td, "right.gdpc")
	outPath := filepath.Join(td, "merged.gdpc")
	if err := writeCaptureFile(leftPath, sound.PCSpeakerCapture{
		TickRate: 140,
		Tones: []sound.PCSpeakerTone{
			{Active: true, ToneValue: 20},
			{Active: true, ToneValue: 20},
		},
	}); err != nil {
		t.Fatalf("writeCaptureFile(left) error: %v", err)
	}
	if err := writeCaptureFile(rightPath, sound.PCSpeakerCapture{
		TickRate: 140,
		Tones: []sound.PCSpeakerTone{
			{Active: true, ToneValue: 40},
			{Active: true, ToneValue: 40},
		},
	}); err != nil {
		t.Fatalf("writeCaptureFile(right) error: %v", err)
	}
	code := run([]string{
		"interleave",
		"-in", leftPath,
		"-mix", rightPath,
		"-out", outPath,
	})
	if code != 0 {
		t.Fatalf("run() code=%d want=0", code)
	}
	got, err := readCaptureFile(outPath)
	if err != nil {
		t.Fatalf("readCaptureFile() error: %v", err)
	}
	if got.TickRate != 560 {
		t.Fatalf("TickRate=%d want=560", got.TickRate)
	}
	if len(got.Tones) == 0 {
		t.Fatal("expected interleaved tones")
	}
}

func TestRunDumpCSVWritesOneLinePerTone(t *testing.T) {
	oldOut := stdOut
	oldErr := stdErr
	var stdout bytes.Buffer
	stdOut = &stdout
	stdErr = &bytes.Buffer{}
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	td := t.TempDir()
	inPath := filepath.Join(td, "tones.gdpc")
	if err := writeCaptureFile(inPath, sound.PCSpeakerCapture{
		TickRate: 140,
		Tones: []sound.PCSpeakerTone{
			{Active: false},
			{Active: true, ToneValue: 20},
			{Active: true, Divisor: 1234},
		},
	}); err != nil {
		t.Fatalf("writeCaptureFile() error: %v", err)
	}
	code := run([]string{
		"dump-csv",
		"-in", inPath,
	})
	if code != 0 {
		t.Fatalf("run() code=%d want=0", code)
	}
	got := stdout.String()
	want := "0,0,0,0\n1,1,20,3950\n2,1,0,1234\n"
	if got != want {
		t.Fatalf("stdout=%q want=%q", got, want)
	}
}

func TestRunPlayLinuxRequiresInput(t *testing.T) {
	var stderr bytes.Buffer
	oldOut := stdOut
	oldErr := stdErr
	stdOut = &bytes.Buffer{}
	stdErr = &stderr
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	code := run([]string{"play-linux"})
	if code != 2 {
		t.Fatalf("run() code=%d want=2", code)
	}
	if !strings.Contains(stderr.String(), "play-linux requires -in") {
		t.Fatalf("stderr=%q want missing -in", stderr.String())
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	oldOut := stdOut
	oldErr := stdErr
	stdOut = &bytes.Buffer{}
	stdErr = &stderr
	defer func() {
		stdOut = oldOut
		stdErr = oldErr
	}()
	code := run([]string{"wat"})
	if code != 2 {
		t.Fatalf("run() code=%d want=2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr=%q want unknown subcommand", stderr.String())
	}
}

type wadLump struct {
	name string
	data []byte
}

func minimalWAD(t *testing.T, ident string, lumps []wadLump) []byte {
	t.Helper()
	if len(lumps) == 0 {
		t.Fatal("minimalWAD requires at least one lump")
	}
	const (
		headerSize    = 12
		directorySize = 16
	)
	totalData := 0
	for _, lump := range lumps {
		if len(lump.name) > 8 {
			t.Fatalf("lump name too long: %q", lump.name)
		}
		totalData += len(lump.data)
	}
	dirPos := headerSize + totalData
	buf := make([]byte, headerSize+totalData+len(lumps)*directorySize)
	copy(buf[0:4], []byte(ident))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(lumps)))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(dirPos))
	writePos := headerSize
	for i, lump := range lumps {
		copy(buf[writePos:], lump.data)
		dir := buf[dirPos+i*directorySize : dirPos+(i+1)*directorySize]
		binary.LittleEndian.PutUint32(dir[0:4], uint32(writePos))
		binary.LittleEndian.PutUint32(dir[4:8], uint32(len(lump.data)))
		copy(dir[8:16], []byte(lump.name))
		writePos += len(lump.data)
	}
	return buf
}
