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
		frontend: frontendState{Active: true, Mode: frontendModeOptions, OptionsOn: frontendOptionsRowMusic},
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

func TestFrontendMusicPlayerOpenStartsAtCurrentSong(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			MusicPlayerCatalog: []runtimecfg.MusicPlayerWAD{
				{
					Key:   "doom1.wad",
					Label: "The Ultimate DOOM",
					Episodes: []runtimecfg.MusicPlayerEpisode{
						{
							Label: "EPISODE 1",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{MapName: "E1M1", Label: "E1M1 - Hangar", LumpName: "D_E1M1", MusicName: "At Doom's Gate"},
								{MapName: "E1M2", Label: "E1M2 - Nuclear Plant", LumpName: "D_E1M2", MusicName: "The Imp's Song"},
							},
						},
					},
				},
				{
					Key:   "doom2.wad",
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
		currentMusicSource: musicPlaybackSource{
			kind:    musicPlaybackSourceMap,
			mapName: "MAP01",
		},
		frontend: frontendState{Active: true, Mode: frontendModeOptions, OptionsOn: frontendOptionsRowMusic},
	}

	if !sg.frontendMusicPlayerOpen() {
		t.Fatal("frontendMusicPlayerOpen should succeed with a catalog")
	}
	if sg.musicPlayer.WADOn != 1 || sg.musicPlayer.EpisodeOn != 0 || sg.musicPlayer.TrackOn != 0 {
		t.Fatalf("selection=(%d,%d,%d) want doom2/maps/map01", sg.musicPlayer.WADOn, sg.musicPlayer.EpisodeOn, sg.musicPlayer.TrackOn)
	}
	if sg.musicPlayer.Row != frontendMusicPlayerRowTrack {
		t.Fatalf("row=%d want track row", sg.musicPlayer.Row)
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

func TestPlayCheatMusic_CommercialMapTrack(t *testing.T) {
	var gotWAD string
	var gotLump string
	sg := &sessionGame{
		opts: Options{
			MusicVolume: 0.8,
			MusicPlayerCatalog: []runtimecfg.MusicPlayerWAD{
				{
					Key:   "doom2.wad",
					Label: "DOOM II",
					Episodes: []runtimecfg.MusicPlayerEpisode{
						{
							Label: "MAPS",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{MapName: "MAP15", Label: "MAP15 - Industrial Zone", LumpName: "D_RUNNI2", MusicName: "Waiting for Romero to Play"},
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
	}

	ok, err := sg.playCheatMusic("MAP01", "15")
	if err != nil {
		t.Fatalf("playCheatMusic err=%v", err)
	}
	if !ok {
		t.Fatal("playCheatMusic should succeed")
	}
	if gotWAD != "doom2.wad" || gotLump != "D_RUNNI2" {
		t.Fatalf("loader called with wad=%q lump=%q want doom2.wad/D_RUNNI2", gotWAD, gotLump)
	}
	if sg.currentMusicSource.kind != musicPlaybackSourcePlayer || sg.currentMusicSource.mapName != "MAP15" || sg.currentMusicSource.lumpName != "D_RUNNI2" {
		t.Fatalf("currentMusicSource=%+v", sg.currentMusicSource)
	}
	if sg.nowPlayingLevel != "MAP15 - Industrial Zone" {
		t.Fatalf("nowPlayingLevel=%q want MAP15 - Industrial Zone", sg.nowPlayingLevel)
	}
	if sg.nowPlayingMusic != "Waiting for Romero to Play" {
		t.Fatalf("nowPlayingMusic=%q want Waiting for Romero to Play", sg.nowPlayingMusic)
	}
}

func TestPlayCheatMusic_CommercialSpecialTrack(t *testing.T) {
	var gotWAD string
	var gotLump string
	sg := &sessionGame{
		opts: Options{
			MusicVolume: 0.8,
			MusicPlayerCatalog: []runtimecfg.MusicPlayerWAD{
				{
					Key:   "doom2.wad",
					Label: "DOOM II",
					Episodes: []runtimecfg.MusicPlayerEpisode{
						{
							Label: "OTHER MUSIC",
							Tracks: []runtimecfg.MusicPlayerTrack{
								{Label: "Read This", LumpName: "D_READ_M", MusicName: "Read This"},
							},
						},
					},
				},
			},
			MusicPlayerTrackLoader: func(wadKey string, lumpName string) ([]byte, error) {
				gotWAD = wadKey
				gotLump = lumpName
				return []byte{4, 5, 6}, nil
			},
		},
		musicCtl: &sessionmusic.Playback{},
	}

	ok, err := sg.playCheatMusic("MAP01", "33")
	if err != nil {
		t.Fatalf("playCheatMusic err=%v", err)
	}
	if !ok {
		t.Fatal("playCheatMusic should succeed")
	}
	if gotWAD != "doom2.wad" || gotLump != "D_READ_M" {
		t.Fatalf("loader called with wad=%q lump=%q want doom2.wad/D_READ_M", gotWAD, gotLump)
	}
	if sg.currentMusicSource.lumpName != "D_READ_M" {
		t.Fatalf("lumpName=%q want D_READ_M", sg.currentMusicSource.lumpName)
	}
}

func TestPlayCheatMusic_ZeroStopsPlayback(t *testing.T) {
	sg := &sessionGame{
		opts:     Options{MusicVolume: 0.8},
		musicCtl: &sessionmusic.Playback{},
		currentMusicSource: musicPlaybackSource{
			kind:       musicPlaybackSourcePlayer,
			lumpName:   "D_RUNNIN",
			levelLabel: "MAP01 - Entryway",
			musicName:  "Running from Evil",
		},
	}
	sg.setNowPlayingLevel("MAP01 - Entryway")
	sg.setNowPlayingMusic("Running from Evil")

	ok, err := sg.playCheatMusic("MAP01", "00")
	if err != nil {
		t.Fatalf("playCheatMusic err=%v", err)
	}
	if !ok {
		t.Fatal("playCheatMusic should succeed")
	}
	if sg.currentMusicSource.kind != musicPlaybackSourceNone {
		t.Fatalf("currentMusicSource.kind=%d want none", sg.currentMusicSource.kind)
	}
	if sg.nowPlayingLevel != "" || sg.nowPlayingMusic != "" {
		t.Fatalf("nowPlaying cleared = (%q,%q) want empty", sg.nowPlayingLevel, sg.nowPlayingMusic)
	}
}

func TestPlayCheatMusic_EpisodeRejectsZeroDigits(t *testing.T) {
	sg := &sessionGame{
		opts:     Options{MusicVolume: 0.8},
		musicCtl: &sessionmusic.Playback{},
	}

	ok, err := sg.playCheatMusic("E1M1", "00")
	if err != nil {
		t.Fatalf("playCheatMusic err=%v", err)
	}
	if ok {
		t.Fatal("playCheatMusic should reject E0M0")
	}
}

func TestFrontendMusicPlayerCloseReturnsToMusicSubmenuWhenOpenedInGame(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active: true,
			InGame: true,
			Mode:   frontendModeMusicPlayer,
		},
	}

	sg.frontendMusicPlayerClose()

	if !sg.frontend.Active {
		t.Fatal("frontend should remain active when leaving music player")
	}
	if sg.frontend.Mode != frontendModeSound {
		t.Fatalf("mode=%d want music submenu", sg.frontend.Mode)
	}
	if sg.frontend.SoundOn != frontendMusicMenuRowPlayer {
		t.Fatalf("soundOn=%d want player row", sg.frontend.SoundOn)
	}
}

func TestFrontendMusicPlayerMoveRowSkipsSongInfoRow(t *testing.T) {
	sg := &sessionGame{}
	sg.musicPlayer.Row = frontendMusicPlayerRowTrack

	if !sg.frontendMusicPlayerMoveRow(1) {
		t.Fatal("move row should succeed")
	}
	if sg.musicPlayer.Row != frontendMusicPlayerRowWAD {
		t.Fatalf("row=%d want wad row wrap", sg.musicPlayer.Row)
	}
}

func TestRestartCurrentMusicPlaybackUsesTrackedPlayerSong(t *testing.T) {
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
								{MapName: "E1M1", Label: "E1M1 - Hangar", LumpName: "D_E1M1", MusicName: "At Doom's Gate"},
								{MapName: "E1M2", Label: "E1M2 - Nuclear Plant", LumpName: "D_E1M2", MusicName: "The Imp's Song"},
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
		currentMusicSource: musicPlaybackSource{
			kind:       musicPlaybackSourcePlayer,
			wadKey:     "doom1",
			lumpName:   "D_E1M2",
			levelLabel: "E1M2 - Nuclear Plant",
			musicName:  "The Imp's Song",
		},
		frontend: frontendState{Active: true, Mode: frontendModeOptions},
	}
	sg.musicPlayer = frontendMusicPlayerState{Row: frontendMusicPlayerRowTrack, TrackOn: 0}

	sg.restartCurrentMusicPlayback()

	if gotWAD != "doom1" || gotLump != "D_E1M2" {
		t.Fatalf("restart used wad=%q lump=%q want doom1/D_E1M2", gotWAD, gotLump)
	}
}

func TestFrontendMusicPlayerPlaySelectedWithoutTrackLoaderFails(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true, Mode: frontendModeMusicPlayer},
	}

	if sg.frontendMusicPlayerPlaySelected() {
		t.Fatal("expected play selected to fail without music loader")
	}
	if sg.frontend.Mode != frontendModeMusicPlayer {
		t.Fatalf("mode=%d want music player to remain open", sg.frontend.Mode)
	}
}
