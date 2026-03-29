package app

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gddoom/internal/music"
)

func TestDumpMusicRendererLabel(t *testing.T) {
	if got := dumpMusicRendererLabel("soundfonts/SC55.sf2"); got != "MIDI-SC55" {
		t.Fatalf("label=%q want %q", got, "MIDI-SC55")
	}
	if got := dumpMusicRendererLabel("soundfonts/general-midi.sf2"); got != "MIDI-GENERAL-MIDI" {
		t.Fatalf("label=%q want %q", got, "MIDI-GENERAL-MIDI")
	}
}

func TestDetectDumpMusicTargetsSkipsSharewareDuringAutoDetect(t *testing.T) {
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

	for _, name := range []string{"doom1.wad", "doomu.wad", "doom2.wad"} {
		if err := os.WriteFile(filepath.Join(td, name), []byte("wad"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	targets, err := detectDumpMusicTargets("", false, nil)
	if err != nil {
		t.Fatalf("detectDumpMusicTargets() error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("len(targets)=%d want 2", len(targets))
	}
	if got := filepath.Base(targets[0].path); got != "doomu.wad" {
		t.Fatalf("targets[0].path=%q want %q", got, "doomu.wad")
	}
	if got := filepath.Base(targets[1].path); got != "doom2.wad" {
		t.Fatalf("targets[1].path=%q want %q", got, "doom2.wad")
	}
}

func TestDetectDumpMusicTargetsKeepsExplicitSharewareWAD(t *testing.T) {
	td := t.TempDir()
	wadPath := filepath.Join(td, "doom1.wad")
	if err := os.WriteFile(wadPath, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}

	targets, err := detectDumpMusicTargets(wadPath, true, nil)
	if err != nil {
		t.Fatalf("detectDumpMusicTargets() error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(targets)=%d want 1", len(targets))
	}
	if targets[0].path != wadPath {
		t.Fatalf("target path=%q want %q", targets[0].path, wadPath)
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
	if got := tracks[0].fileBase; got != "MAP01-Running from Evil" {
		t.Fatalf("tracks[0].fileBase=%q want %q", got, "MAP01-Running from Evil")
	}
	if got := tracks[0].label; got != "MAP01 - Entryway | Running from Evil | D_RUNNIN" {
		t.Fatalf("tracks[0].label=%q", got)
	}
	if got := tracks[1].fileBase; got != "Doom II Intermission" {
		t.Fatalf("tracks[1].fileBase=%q want %q", got, "Doom II Intermission")
	}
	if got := tracks[1].label; got != "Other | Doom II Intermission | D_DM2INT" {
		t.Fatalf("tracks[1].label=%q", got)
	}
}

func TestMusicDumpFilenamePartKeepsReadableTitles(t *testing.T) {
	if got := musicDumpFilenamePart("At Doom's Gate"); got != "At Dooms Gate" {
		t.Fatalf("musicDumpFilenamePart=%q want %q", got, "At Dooms Gate")
	}
	if got := musicDumpFilenamePart("'O' of Destruction!"); got != "O of Destruction" {
		t.Fatalf("musicDumpFilenamePart=%q want %q", got, "O of Destruction")
	}
}

func TestNormalizeDumpMusicPCMTargetsThreeDBPad(t *testing.T) {
	in := []int16{1000, -2000, 4000, -8000}
	got := normalizeDumpMusicPCM(in, dumpMusicNormalizePadDB)
	if len(got) != len(in) {
		t.Fatalf("len(got)=%d want %d", len(got), len(in))
	}
	if &got[0] == &in[0] {
		t.Fatal("expected normalized PCM to allocate a new slice")
	}
	wantPeak := int(math.Round(32767.0 * math.Pow(10, -dumpMusicNormalizePadDB/20.0)))
	if peak := dumpMusicPeakAbsSample(got); peak != wantPeak {
		t.Fatalf("peak=%d want %d", peak, wantPeak)
	}
}

func TestNormalizeDumpMusicPCMSilenceIsUnchanged(t *testing.T) {
	in := []int16{0, 0, 0, 0}
	got := normalizeDumpMusicPCM(in, dumpMusicNormalizePadDB)
	if len(got) != len(in) {
		t.Fatalf("len(got)=%d want %d", len(got), len(in))
	}
	for i := range got {
		if got[i] != 0 {
			t.Fatalf("got[%d]=%d want 0", i, got[i])
		}
	}
}

func TestDumpMusicPeakAbsSampleHandlesMinInt16(t *testing.T) {
	if got := dumpMusicPeakAbsSample([]int16{-32768, 120, -10}); got != 32768 {
		t.Fatalf("peak=%d want 32768", got)
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

func TestDumpMusicPCMConcurrentSafeRendersSC55Doom2Intermission(t *testing.T) {
	const (
		lumpName = "D_DDTBLU"
	)
	wadPath := filepath.Join("..", "..", "DOOM2.WAD")
	soundFontPath := filepath.Join("..", "..", "soundfonts", "SC55.sf2")
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
		label:       "MIDI-SC55",
		displayName: "SC-55",
		backend:     music.BackendMeltySynth,
		fontPath:    soundFontPath,
		soundFont:   sf,
	}, musData, &meltySynthMu)
	if err != nil {
		t.Fatalf("render %s with SC55: %v", lumpName, err)
	}
	if len(pcm) == 0 {
		t.Fatalf("render %s produced no PCM", lumpName)
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
