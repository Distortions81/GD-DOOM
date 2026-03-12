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
								{MapName: "E1M1", Label: "E1M1", LumpName: "D_E1M1"},
								{MapName: "E1M2", Label: "E1M2", LumpName: "D_E1M2"},
							},
						},
						{
							Label: "EPISODE 2",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{MapName: "E2M1", Label: "E2M1", LumpName: "D_E2M1"},
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
								{MapName: "MAP01", Label: "MAP01", LumpName: "D_RUNNIN"},
							},
						},
					},
				},
			},
			MusicPlayerTrackLoader: func(wadKey string, mapName string) ([]byte, error) { return nil, nil },
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

	sg.musicPlayer.Row = frontendMusicPlayerRowLevel
	if !sg.frontendMusicPlayerAdjust(1) {
		t.Fatal("level adjust should succeed")
	}
	if got := sg.frontendMusicPlayerTrack(); got == nil || got.MapName != mapdata.MapName("E1M2") {
		t.Fatalf("track after level adjust=%v want E1M2", got)
	}

	sg.musicPlayer.Row = frontendMusicPlayerRowEpisode
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
	var gotMap string
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
								{MapName: "E1M3", Label: "E1M3", LumpName: "D_E1M3"},
							},
						},
					},
				},
			},
			MusicPlayerTrackLoader: func(wadKey string, mapName string) ([]byte, error) {
				gotWAD = wadKey
				gotMap = mapName
				return []byte{1, 2, 3}, nil
			},
		},
		musicCtl: &sessionmusic.Playback{},
		frontend: frontendState{Active: true, Mode: frontendModeMusicPlayer},
	}

	if !sg.frontendMusicPlayerPlaySelected() {
		t.Fatal("frontendMusicPlayerPlaySelected should succeed")
	}
	if gotWAD != "doom1" || gotMap != "E1M3" {
		t.Fatalf("loader called with wad=%q map=%q want doom1/E1M3", gotWAD, gotMap)
	}
	if sg.frontend.Status != "PLAYING E1M3" {
		t.Fatalf("status=%q want PLAYING E1M3", sg.frontend.Status)
	}
}
