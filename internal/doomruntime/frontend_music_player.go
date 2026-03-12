package doomruntime

import (
	"fmt"
	"strings"

	"gddoom/internal/runtimecfg"
)

const (
	frontendOptionsRowMusicPlayer = 7
	frontendMusicPlayerRowWAD     = 0
	frontendMusicPlayerRowEpisode = 1
	frontendMusicPlayerRowLevel   = 2
	frontendMusicPlayerRowSong    = 3
	frontendMusicPlayerRowCount   = 4
)

type frontendMusicPlayerState struct {
	Row       int
	WADOn     int
	EpisodeOn int
	TrackOn   int
}

func wrapMusicPlayerIndex(cur, dir, n int) int {
	if n <= 0 {
		return 0
	}
	cur = (cur + dir) % n
	if cur < 0 {
		cur += n
	}
	return cur
}

func (sg *sessionGame) frontendMusicPlayerAvailable() bool {
	return sg != nil && len(sg.opts.MusicPlayerCatalog) > 0 && sg.opts.MusicPlayerTrackLoader != nil
}

func (sg *sessionGame) frontendMusicPlayerClamp() {
	if sg == nil {
		return
	}
	if sg.musicPlayer.Row < 0 || sg.musicPlayer.Row >= frontendMusicPlayerRowCount {
		sg.musicPlayer.Row = frontendMusicPlayerRowWAD
	}
	if len(sg.opts.MusicPlayerCatalog) == 0 {
		sg.musicPlayer.WADOn = 0
		sg.musicPlayer.EpisodeOn = 0
		sg.musicPlayer.TrackOn = 0
		return
	}
	if sg.musicPlayer.WADOn < 0 || sg.musicPlayer.WADOn >= len(sg.opts.MusicPlayerCatalog) {
		sg.musicPlayer.WADOn = 0
	}
	wad := &sg.opts.MusicPlayerCatalog[sg.musicPlayer.WADOn]
	if len(wad.Episodes) == 0 {
		sg.musicPlayer.EpisodeOn = 0
		sg.musicPlayer.TrackOn = 0
		return
	}
	if sg.musicPlayer.EpisodeOn < 0 || sg.musicPlayer.EpisodeOn >= len(wad.Episodes) {
		sg.musicPlayer.EpisodeOn = 0
	}
	ep := &wad.Episodes[sg.musicPlayer.EpisodeOn]
	if len(ep.Tracks) == 0 {
		sg.musicPlayer.TrackOn = 0
		return
	}
	if sg.musicPlayer.TrackOn < 0 || sg.musicPlayer.TrackOn >= len(ep.Tracks) {
		sg.musicPlayer.TrackOn = 0
	}
}

func (sg *sessionGame) frontendMusicPlayerOpen() bool {
	if !sg.frontendMusicPlayerAvailable() {
		return false
	}
	sg.musicPlayer = frontendMusicPlayerState{}
	sg.frontend.Mode = frontendModeMusicPlayer
	sg.frontend.MenuActive = false
	sg.frontendMusicPlayerClamp()
	return true
}

func (sg *sessionGame) frontendMusicPlayerClose() {
	if sg == nil {
		return
	}
	sg.frontend.Mode = frontendModeOptions
	sg.frontend.OptionsOn = frontendOptionsRowMusicPlayer
}

func (sg *sessionGame) frontendMusicPlayerWAD() *runtimecfg.MusicPlayerWAD {
	if sg == nil || len(sg.opts.MusicPlayerCatalog) == 0 {
		return nil
	}
	sg.frontendMusicPlayerClamp()
	return &sg.opts.MusicPlayerCatalog[sg.musicPlayer.WADOn]
}

func (sg *sessionGame) frontendMusicPlayerEpisode() *runtimecfg.MusicPlayerEpisode {
	wad := sg.frontendMusicPlayerWAD()
	if wad == nil || len(wad.Episodes) == 0 {
		return nil
	}
	sg.frontendMusicPlayerClamp()
	return &wad.Episodes[sg.musicPlayer.EpisodeOn]
}

func (sg *sessionGame) frontendMusicPlayerTrack() *runtimecfg.MusicPlayerTrack {
	ep := sg.frontendMusicPlayerEpisode()
	if ep == nil || len(ep.Tracks) == 0 {
		return nil
	}
	sg.frontendMusicPlayerClamp()
	return &ep.Tracks[sg.musicPlayer.TrackOn]
}

func (sg *sessionGame) frontendMusicPlayerMoveRow(dir int) bool {
	if sg == nil || dir == 0 {
		return false
	}
	sg.musicPlayer.Row = wrapMusicPlayerIndex(sg.musicPlayer.Row, dir, frontendMusicPlayerRowCount)
	return true
}

func (sg *sessionGame) frontendMusicPlayerAdjust(dir int) bool {
	if sg == nil || dir == 0 || !sg.frontendMusicPlayerAvailable() {
		return false
	}
	sg.frontendMusicPlayerClamp()
	switch sg.musicPlayer.Row {
	case frontendMusicPlayerRowWAD:
		if len(sg.opts.MusicPlayerCatalog) <= 1 {
			return false
		}
		sg.musicPlayer.WADOn = wrapMusicPlayerIndex(sg.musicPlayer.WADOn, dir, len(sg.opts.MusicPlayerCatalog))
		sg.musicPlayer.EpisodeOn = 0
		sg.musicPlayer.TrackOn = 0
	case frontendMusicPlayerRowEpisode:
		wad := sg.frontendMusicPlayerWAD()
		if wad == nil || len(wad.Episodes) <= 1 {
			return false
		}
		sg.musicPlayer.EpisodeOn = wrapMusicPlayerIndex(sg.musicPlayer.EpisodeOn, dir, len(wad.Episodes))
		sg.musicPlayer.TrackOn = 0
	case frontendMusicPlayerRowLevel:
		ep := sg.frontendMusicPlayerEpisode()
		if ep == nil || len(ep.Tracks) <= 1 {
			return false
		}
		sg.musicPlayer.TrackOn = wrapMusicPlayerIndex(sg.musicPlayer.TrackOn, dir, len(ep.Tracks))
	default:
		return false
	}
	sg.frontendMusicPlayerClamp()
	return true
}

func (sg *sessionGame) frontendMusicPlayerPlaySelected() bool {
	if sg == nil || sg.musicCtl == nil || sg.opts.MusicPlayerTrackLoader == nil {
		return false
	}
	wad := sg.frontendMusicPlayerWAD()
	track := sg.frontendMusicPlayerTrack()
	if wad == nil || track == nil {
		return false
	}
	data, err := sg.opts.MusicPlayerTrackLoader(wad.Key, string(track.MapName))
	if err != nil {
		sg.frontendStatus("MUSIC LOAD FAILED", doomTicsPerSecond*2)
		return false
	}
	if len(data) == 0 {
		sg.frontendStatus("SONG NOT FOUND", doomTicsPerSecond*2)
		return false
	}
	sg.musicCtl.PlayData(data, clampVolume(sg.opts.MusicVolume))
	sg.frontendStatus(fmt.Sprintf("PLAYING %s", strings.ToUpper(strings.TrimSpace(track.Label))), doomTicsPerSecond*2)
	return true
}
