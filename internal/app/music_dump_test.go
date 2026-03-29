package app

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpMusicRendererLabel(t *testing.T) {
	if got := dumpMusicRendererLabel("soundfonts/SC55.sf2"); got != "MIDI-SC55" {
		t.Fatalf("label=%q want %q", got, "MIDI-SC55")
	}
	if got := dumpMusicRendererLabel("soundfonts/windows-gm.sf2"); got != "MIDI-WINDOWS-GM" {
		t.Fatalf("label=%q want %q", got, "MIDI-WINDOWS-GM")
	}
}

func TestDumpTrackCoverLines(t *testing.T) {
	got := dumpTrackCoverLines(dumpMusicTrack{
		version: "The Ultimate DOOM",
		level:   "E1M1 - Hangar",
		music:   "At Doom's Gate",
		synth:   "SC55",
	})
	want := []string{
		"THE ULTIMATE DOOM",
		"E1M1 - HANGAR",
		"AT DOOM'S GATE",
		"SC55",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines=%q want %q", got, want)
	}
}

func TestDumpMusicTracksForWADUsesMapAndOtherMusicNames(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "music.wad")
	lumps := append(appTestMapLumpSet("MAP01"),
		appTestLump{name: "D_RUNNIN", data: buildAppTestMUS([]byte{0x1F, 35, 0x60})},
		appTestLump{name: "D_DM2INT", data: buildAppTestMUS([]byte{0x1F, 35, 0x60})},
	)
	if err := os.WriteFile(path, buildAppTestWAD("IWAD", lumps), 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	wf, _, err := openWADStack(path, nil)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}

	tracks, err := dumpMusicTracksForWAD(wf)
	if err != nil {
		t.Fatalf("dumpMusicTracksForWAD() error: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("len(tracks)=%d want 2", len(tracks))
	}
	if got := tracks[0].fileBase; got != "MAP01-running-from-evil" {
		t.Fatalf("tracks[0].fileBase=%q want %q", got, "MAP01-running-from-evil")
	}
	if got := tracks[0].label; got != "MAP01 - Entryway | Running from Evil | D_RUNNIN" {
		t.Fatalf("tracks[0].label=%q", got)
	}
	if got := tracks[1].fileBase; got != "D_DM2INT-doom-ii-intermission" {
		t.Fatalf("tracks[1].fileBase=%q want %q", got, "D_DM2INT-doom-ii-intermission")
	}
	if got := tracks[1].label; got != "Other | Doom II Intermission | D_DM2INT" {
		t.Fatalf("tracks[1].label=%q", got)
	}
}

func TestRunParseDumpMusicWritesOPLWav(t *testing.T) {
	td := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(td); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(prevWD)
	}()

	wadPath := filepath.Join(td, "music.wad")
	outDir := filepath.Join(td, "out")
	lumps := append(appTestMapLumpSet("MAP01"),
		appTestLump{name: "TITLEPIC", data: make([]byte, 320*200)},
		appTestLump{name: "D_RUNNIN", data: buildAppTestMUS([]byte{
			0x90, 0xBC, 100,
			0x0A,
			0x00, 0x3C,
			0x60,
		})},
	)
	if err := os.WriteFile(wadPath, buildAppTestWAD("IWAD", lumps), 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := RunParse([]string{
		"-wad", wadPath,
		"-dump-music",
		"-dump-music-dir", outDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, stderr.String())
	}

	wavPath := filepath.Join(outDir, "MUSIC", "OPL", "MAP01-running-from-evil.wav")
	if _, err := os.Stat(wavPath); err != nil {
		t.Fatalf("stat wav: %v", err)
	}
	coverPath := filepath.Join(outDir, "MUSIC", "OPL", "MAP01-running-from-evil.png")
	cf, err := os.Open(coverPath)
	if err != nil {
		t.Fatalf("open cover: %v", err)
	}
	defer cf.Close()
	cover, err := png.Decode(cf)
	if err != nil {
		t.Fatalf("decode cover: %v", err)
	}
	if b := cover.Bounds(); b.Dx() != 1920 || b.Dy() != 1080 {
		t.Fatalf("cover size=%dx%d want 1920x1080", b.Dx(), b.Dy())
	}
	splashPath := filepath.Join(outDir, "MUSIC", "splash.png")
	f, err := os.Open(splashPath)
	if err != nil {
		t.Fatalf("open splash: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode splash: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 1920 || b.Dy() != 1080 {
		t.Fatalf("splash size=%dx%d want 1920x1080", b.Dx(), b.Dy())
	}
	tracksPath := filepath.Join(outDir, "MUSIC", "tracks.txt")
	data, err := os.ReadFile(tracksPath)
	if err != nil {
		t.Fatalf("read tracks.txt: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "MAP01 - Entryway | Running from Evil | D_RUNNIN" {
		t.Fatalf("tracks.txt=%q", got)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected dump-music progress on stdout")
	}
}

func buildAppTestMUS(score []byte) []byte {
	var b bytes.Buffer
	b.WriteString("MUS\x1a")
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(score)))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	b.Write(score)
	return b.Bytes()
}
