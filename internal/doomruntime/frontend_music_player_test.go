package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionmusic"
)

func TestFrontendMusicPlayerOpenAndAdjustSelection(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			MusicPlayerCatalog: []runtimecfg.MusicPlayerWAD{
				{
					Key:   "doom1",
					Label: "The Ultimate DOOM",
					Episodes: []runtimecfg.MusicPlayerEpisode{
						{
							Label: "EPISODE 1",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{MapName: "E1M1", Label: "E1M1 - Hangar", LumpName: "D_E1M1", MusicName: "At Doom's Gate"},
								{MapName: "E1M2", Label: "E1M2 - Nuclear Plant", LumpName: "D_E1M2", MusicName: "The Imp's Song"},
							},
						},
						{
							Label: "EPISODE 2",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{MapName: "E2M1", Label: "E2M1 - Deimos Anomaly", LumpName: "D_E2M1", MusicName: "I Sawed the Demons"},
							},
						},
					},
				},
				{
					Key:   "doom2",
					Label: "DOOM II",
					Episodes: []runtimecfg.MusicPlayerEpisode{
						{
							Label: "MAPS",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{MapName: "MAP01", Label: "MAP01 - Entryway", LumpName: "D_RUNNIN", MusicName: "Running from Evil"},
							},
						},
					},
				},
			},
			MusicPlayerTrackLoader: func(wadKey string, lumpName string) ([]byte, error) { return nil, nil },
		},
		frontend: frontendState{Active: true, Mode: frontendModeOptions, OptionsOn: frontendOptionsRowMusicPlayer},
	}

	if !sg.frontendMusicPlayerOpen() {
		t.Fatal("frontendMusicPlayerOpen should succeed with a catalog")
	}
	if sg.frontend.Mode != frontendModeMusicPlayer {
		t.Fatalf("mode=%d want=%d", sg.frontend.Mode, frontendModeMusicPlayer)
	}
	if got := sg.frontendMusicPlayerTrack(); got == nil || got.MapName != mapdata.MapName("E1M1") {
		t.Fatalf("initial track=%v want E1M1", got)
	}

	sg.musicPlayer.Row = frontendMusicPlayerRowTrack
	if !sg.frontendMusicPlayerAdjust(1) {
		t.Fatal("level adjust should succeed")
	}
	if got := sg.frontendMusicPlayerTrack(); got == nil || got.MapName != mapdata.MapName("E1M2") {
		t.Fatalf("track after level adjust=%v want E1M2", got)
	}

	sg.musicPlayer.Row = frontendMusicPlayerRowGroup
	if !sg.frontendMusicPlayerAdjust(1) {
		t.Fatal("episode adjust should succeed")
	}
	if got := sg.frontendMusicPlayerTrack(); got == nil || got.MapName != mapdata.MapName("E2M1") {
		t.Fatalf("track after episode adjust=%v want E2M1", got)
	}

	sg.musicPlayer.Row = frontendMusicPlayerRowWAD
	if !sg.frontendMusicPlayerAdjust(1) {
		t.Fatal("wad adjust should succeed")
	}
	if got := sg.frontendMusicPlayerTrack(); got == nil || got.MapName != mapdata.MapName("MAP01") {
		t.Fatalf("track after wad adjust=%v want MAP01", got)
	}
}

func TestFrontendMusicPlayerPlaySelectedLoadsTrack(t *testing.T) {
	var gotWAD string
	var gotLump string
	sg := &sessionGame{
		opts: Options{
			MusicVolume: 0.8,
			MusicPlayerCatalog: []runtimecfg.MusicPlayerWAD{
				{
					Key:   "doom1",
					Label: "The Ultimate DOOM",
					Episodes: []runtimecfg.MusicPlayerEpisode{
						{
							Label: "EPISODE 1",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{MapName: "E1M3", Label: "E1M3 - Toxin Refinery", LumpName: "D_E1M3", MusicName: "Dark Halls"},
							},
						},
					},
				},
			},
			MusicPlayerTrackLoader: func(wadKey string, lumpName string) ([]byte, error) {
				gotWAD = wadKey
				gotLump = lumpName
				return []byte{1, 2, 3}, nil
			},
		},
		musicCtl: &sessionmusic.Playback{},
		frontend: frontendState{Active: true, Mode: frontendModeMusicPlayer},
	}

	if !sg.frontendMusicPlayerPlaySelected() {
		t.Fatal("frontendMusicPlayerPlaySelected should succeed")
	}
	if gotWAD != "doom1" || gotLump != "D_E1M3" {
		t.Fatalf("loader called with wad=%q lump=%q want doom1/D_E1M3", gotWAD, gotLump)
	}
	if sg.frontend.Status != "" {
		t.Fatalf("status=%q want empty", sg.frontend.Status)
	}
	if sg.nowPlayingMusic != "Dark Halls" {
		t.Fatalf("nowPlayingMusic=%q want Dark Halls", sg.nowPlayingMusic)
	}
	if sg.nowPlayingLevel != "E1M3 - Toxin Refinery" {
		t.Fatalf("nowPlayingLevel=%q want E1M3 - Toxin Refinery", sg.nowPlayingLevel)
	}
	if got := sg.nowPlayingMusicLabel(); got != "E1M3 - TOXIN REFINERY\nSONG: DARK HALLS" {
		t.Fatalf("nowPlayingMusicLabel=%q want multiline level/song", got)
	}
}

func TestFrontendMusicPlayerCloseReturnsToGameWhenOpenedInGame(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active: true,
			InGame: true,
			Mode:   frontendModeMusicPlayer,
		},
	}

	sg.frontendMusicPlayerClose()

	if sg.frontend.Active {
		t.Fatal("frontend should close when leaving in-game music player")
	}
}
