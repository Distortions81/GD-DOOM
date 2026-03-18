package app

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestMapMusicLumpNameEpisode4Aliases(t *testing.T) {
	cases := []struct {
		mapName string
		want    string
	}{
		{mapName: "E4M1", want: "D_E3M4"},
		{mapName: "E4M2", want: "D_E3M2"},
		{mapName: "E4M3", want: "D_E3M3"},
		{mapName: "E4M4", want: "D_E1M5"},
		{mapName: "E4M5", want: "D_E2M7"},
		{mapName: "E4M6", want: "D_E2M4"},
		{mapName: "E4M7", want: "D_E2M6"},
		{mapName: "E4M8", want: "D_E2M5"},
		{mapName: "E4M9", want: "D_E1M9"},
	}

	for _, tc := range cases {
		got, ok := mapMusicLumpName(mapdata.MapName(tc.mapName))
		if !ok {
			t.Fatalf("%s not mapped", tc.mapName)
		}
		if got != tc.want {
			t.Fatalf("%s mapped to %s want %s", tc.mapName, got, tc.want)
		}
	}
}

func TestMapMusicLumpNamePreservesEpisode1To3AndCommercialMappings(t *testing.T) {
	cases := []struct {
		mapName string
		want    string
	}{
		{mapName: "E1M1", want: "D_E1M1"},
		{mapName: "E3M9", want: "D_E3M9"},
		{mapName: "MAP01", want: "D_RUNNIN"},
		{mapName: "MAP31", want: "D_EVIL"},
		{mapName: "MAP32", want: "D_ULTIMA"},
	}

	for _, tc := range cases {
		got, ok := mapMusicLumpName(mapdata.MapName(tc.mapName))
		if !ok {
			t.Fatalf("%s not mapped", tc.mapName)
		}
		if got != tc.want {
			t.Fatalf("%s mapped to %s want %s", tc.mapName, got, tc.want)
		}
	}
}

func TestMapMusicInfoUsesReadableLevelAndMusicNames(t *testing.T) {
	levelLabel, musicName := mapMusicInfo("E1M1")
	if levelLabel != "E1M1 - Hangar" {
		t.Fatalf("levelLabel=%q want %q", levelLabel, "E1M1 - Hangar")
	}
	if musicName != "At Doom's Gate" {
		t.Fatalf("musicName=%q want %q", musicName, "At Doom's Gate")
	}

	levelLabel, musicName = mapMusicInfo("MAP01")
	if levelLabel != "MAP01 - Entryway" {
		t.Fatalf("levelLabel=%q want %q", levelLabel, "MAP01 - Entryway")
	}
	if musicName != "Running from Evil" {
		t.Fatalf("musicName=%q want %q", musicName, "Running from Evil")
	}
}

func TestMusicPlayerEpisodesForWADIncludesReadableMapLabelsAndOtherMusic(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "music.wad")
	lumps := append(appTestMapLumpSet("E1M1"),
		appTestLump{name: "D_E1M1", data: []byte{1}},
		appTestLump{name: "D_INTER", data: []byte{2}},
		appTestLump{name: "D_DM2INT", data: []byte{3}},
	)
	if err := os.WriteFile(path, buildAppTestWAD("IWAD", lumps), 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	wf, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}

	episodes := musicPlayerEpisodesForWAD(wf)
	if len(episodes) != 2 {
		t.Fatalf("len(episodes)=%d want 2", len(episodes))
	}
	if episodes[0].Label != "EPISODE 1" {
		t.Fatalf("episodes[0].Label=%q want EPISODE 1", episodes[0].Label)
	}
	if len(episodes[0].Tracks) != 1 {
		t.Fatalf("len(episodes[0].Tracks)=%d want 1", len(episodes[0].Tracks))
	}
	if got := episodes[0].Tracks[0]; got.Label != "E1M1 - Hangar" || got.MusicName != "At Doom's Gate" {
		t.Fatalf("map track=%+v want label/music for E1M1", got)
	}
	if episodes[1].Label != "OTHER MUSIC" {
		t.Fatalf("episodes[1].Label=%q want OTHER MUSIC", episodes[1].Label)
	}
	if len(episodes[1].Tracks) != 2 {
		t.Fatalf("len(episodes[1].Tracks)=%d want 2", len(episodes[1].Tracks))
	}
	if got := episodes[1].Tracks[0]; got.LumpName != "D_INTER" || got.Label != "Intermission from Doom" {
		t.Fatalf("episodes[1].Tracks[0]=%+v want D_INTER / Intermission from Doom", got)
	}
	if got := episodes[1].Tracks[1]; got.LumpName != "D_DM2INT" || got.Label != "Doom II Intermission" {
		t.Fatalf("episodes[1].Tracks[1]=%+v want D_DM2INT / Doom II Intermission", got)
	}
}
