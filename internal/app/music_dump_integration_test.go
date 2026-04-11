//go:build integration

package app

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gddoom/internal/music"
)

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

	wavPath := filepath.Join(outDir, "MUSIC", "OPL", "OPL-MAP01-Running from Evil.wav")
	if _, err := os.Stat(wavPath); err != nil {
		t.Fatalf("stat wav: %v", err)
	}
	coverPath := filepath.Join(outDir, "MUSIC", "OPL", "OPL-MAP01-Running from Evil.png")
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

func TestRunParseDumpMusicSkipsExistingNonZeroWav(t *testing.T) {
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
		appTestLump{name: "D_RUNNIN", data: buildAppTestMUS([]byte{0x1F, 35, 0x60})},
	)
	if err := os.WriteFile(wadPath, buildAppTestWAD("IWAD", lumps), 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}

	wavPath := filepath.Join(outDir, "MUSIC", "OPL", "OPL-MAP01-Running from Evil.wav")
	if err := os.MkdirAll(filepath.Dir(wavPath), 0o755); err != nil {
		t.Fatalf("mkdir wav dir: %v", err)
	}
	want := []byte("keep-existing-wav")
	if err := os.WriteFile(wavPath, want, 0o644); err != nil {
		t.Fatalf("seed wav: %v", err)
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

	got, err := os.ReadFile(wavPath)
	if err != nil {
		t.Fatalf("read wav: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wav was rewritten, got=%q want=%q", got, want)
	}
	if strings.Contains(stdout.String(), "renderer=OPL track=D_RUNNIN") {
		t.Fatalf("stdout should not report skipped track, got %q", stdout.String())
	}
}

func TestRunParseDumpMusicRewritesZeroByteWav(t *testing.T) {
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

	wavPath := filepath.Join(outDir, "MUSIC", "OPL", "OPL-MAP01-Running from Evil.wav")
	if err := os.MkdirAll(filepath.Dir(wavPath), 0o755); err != nil {
		t.Fatalf("mkdir wav dir: %v", err)
	}
	if err := os.WriteFile(wavPath, nil, 0o644); err != nil {
		t.Fatalf("seed zero-byte wav: %v", err)
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

	info, err := os.Stat(wavPath)
	if err != nil {
		t.Fatalf("stat wav: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected zero-byte wav to be regenerated")
	}
	if !strings.Contains(stdout.String(), "renderer=OPL track=D_RUNNIN") {
		t.Fatalf("stdout should report regenerated track, got %q", stdout.String())
	}
}

func TestDumpMusicPCMConcurrentSafeRendersGeneralMIDIDoom1Intermission(t *testing.T) {
	const lumpName = "D_INTER"

	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	soundFontPath := filepath.Join("..", "..", "soundfonts", "general-midi.sf2")
	if _, err := os.Stat(wadPath); err != nil {
		t.Skipf("missing %s: %v", wadPath, err)
	}
	if _, err := os.Stat(soundFontPath); err != nil {
		t.Skipf("missing %s: %v", soundFontPath, err)
	}

	wf, _, err := openWADStack(wadPath, nil)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}
	lump, ok := wf.LumpByName(lumpName)
	if !ok {
		t.Fatalf("missing lump %s", lumpName)
	}
	musData, err := wf.LumpDataView(lump)
	if err != nil {
		t.Fatalf("read lump %s: %v", lumpName, err)
	}
	patchBank, err := resolveMusicPatchBank(wf, "", nil)
	if err != nil {
		t.Fatalf("resolve patch bank: %v", err)
	}
	sf, err := music.ParseSoundFontFile(soundFontPath)
	if err != nil {
		t.Fatalf("parse soundfont: %v", err)
	}

	var meltySynthMu sync.Mutex
	pcm, err := dumpMusicPCMConcurrentSafe(patchBank, dumpMusicRenderer{
		label:       "MIDI-GENERAL-MIDI",
		displayName: "General MIDI",
		backend:     music.BackendMeltySynth,
		fontPath:    soundFontPath,
		soundFont:   sf,
	}, musData, &meltySynthMu)
	if err != nil {
		t.Fatalf("render %s with general-midi: %v", lumpName, err)
	}
	if len(pcm) == 0 {
		t.Fatalf("render %s produced no PCM", lumpName)
	}
}
