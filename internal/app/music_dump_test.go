package app

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"gddoom/internal/wad"
)

func TestDumpMusicRendererLabel(t *testing.T) {
	if got := dumpMusicRendererLabel("soundfonts/SC55-HQ.sf2"); got != "MIDI-SC55-HQ" {
		t.Fatalf("label=%q want %q", got, "MIDI-SC55-HQ")
	}
	if got := dumpMusicRendererLabel("soundfonts/general-midi.sf2"); got != "MIDI-GENERAL-MIDI" {
		t.Fatalf("label=%q want %q", got, "MIDI-GENERAL-MIDI")
	}
}

func TestDetectDumpMusicTargetsKeepsExplicitSharewareWAD(t *testing.T) {
	wadPath := "/tmp/doom1.wad"

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

func TestDumpMusicShouldSkipIWADChoiceSkipsSharewareOnly(t *testing.T) {
	if !dumpMusicShouldSkipIWADChoice(iwadChoice{Label: "DOOM Shareware"}) {
		t.Fatal("shareware choice should be skipped")
	}
	if dumpMusicShouldSkipIWADChoice(iwadChoice{Label: "The Ultimate DOOM"}) {
		t.Fatal("non-shareware choice should not be skipped")
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
	lumps := append(appTestMapLumpSet("MAP01"),
		appTestLump{name: "D_RUNNIN", data: buildAppTestMUS([]byte{0x1F, 35, 0x60})},
		appTestLump{name: "D_DM2INT", data: buildAppTestMUS([]byte{0x1F, 35, 0x60})},
	)
	wf, err := wad.OpenData("music.wad", buildAppTestWAD("IWAD", lumps))
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

func TestDumpMusicOutputBaseUsesTitleStyleNames(t *testing.T) {
	got := dumpMusicOutputBase(
		dumpMusicTarget{label: "DOOMU", displayName: "The Ultimate DOOM"},
		dumpMusicRenderer{label: "MIDI-SGM-ULTRA-HQ", displayName: "SGM-Ultra-HQ"},
		dumpMusicTrack{fileBase: "E1M1-At Dooms Gate", lumpName: "D_E1M1"},
	)
	want := "DOOM 1993 - MIDI SGM HQ E1M1 At Dooms Gate"
	if got != want {
		t.Fatalf("dumpMusicOutputBase()=%q want %q", got, want)
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
