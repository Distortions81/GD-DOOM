package doomruntime

import (
	"strings"

	"gddoom/internal/runtimecfg"
)

const (
	frontendOptionsRowMusic       = 6
	frontendMusicMenuRowVolume    = 0
	frontendMusicMenuRowSynth     = 1
	frontendMusicMenuRowSoundFont = 2
	frontendMusicMenuRowPlayer    = 3
	frontendMusicMenuRowCount     = 4
	frontendMusicPlayerRowWAD     = 0
	frontendMusicPlayerRowGroup   = 1
	frontendMusicPlayerRowTrack   = 2
	frontendMusicPlayerRowBack    = 3
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
	sg.musicPlayer = frontendMusicPlayerState{Row: frontendMusicPlayerRowTrack}
	sg.frontendMusicPlayerSyncToCurrentSource()
	sg.frontend.Mode = frontendModeMusicPlayer
	sg.frontend.MenuActive = false
	sg.frontendMusicPlayerClamp()
	return true
}

func (sg *sessionGame) frontendMusicPlayerSyncToCurrentSource() bool {
	if sg == nil || len(sg.opts.MusicPlayerCatalog) == 0 {
		return false
	}
	match := func(track runtimecfg.MusicPlayerTrack, wadKey string) bool {
		switch sg.currentMusicSource.kind {
		case musicPlaybackSourcePlayer:
			return strings.EqualFold(strings.TrimSpace(wadKey), strings.TrimSpace(sg.currentMusicSource.wadKey)) &&
				strings.EqualFold(strings.TrimSpace(track.LumpName), strings.TrimSpace(sg.currentMusicSource.lumpName))
		case musicPlaybackSourceMap:
			return track.MapName != "" && track.MapName == sg.currentMusicSource.mapName
		default:
			return false
		}
	}
	for wi := range sg.opts.MusicPlayerCatalog {
		wad := &sg.opts.MusicPlayerCatalog[wi]
		for ei := range wad.Episodes {
			ep := &wad.Episodes[ei]
			for ti := range ep.Tracks {
				if !match(ep.Tracks[ti], wad.Key) {
					continue
				}
				sg.musicPlayer.WADOn = wi
				sg.musicPlayer.EpisodeOn = ei
				sg.musicPlayer.TrackOn = ti
				sg.musicPlayer.Row = frontendMusicPlayerRowTrack
				return true
			}
		}
	}
	return false
}

func (sg *sessionGame) frontendMusicPlayerClose() {
	if sg == nil {
		return
	}
	sg.frontend.Mode = frontendModeSound
	sg.frontend.SoundOn = frontendMusicMenuRowPlayer
	sg.frontend.MenuActive = false
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
	case frontendMusicPlayerRowGroup:
		wad := sg.frontendMusicPlayerWAD()
		if wad == nil || len(wad.Episodes) <= 1 {
			return false
		}
		sg.musicPlayer.EpisodeOn = wrapMusicPlayerIndex(sg.musicPlayer.EpisodeOn, dir, len(wad.Episodes))
		sg.musicPlayer.TrackOn = 0
	case frontendMusicPlayerRowTrack:
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
	if sg == nil {
		return false
	}
	if sg.musicPlayer.Row == frontendMusicPlayerRowBack {
		sg.frontendMusicPlayerClose()
		return true
	}
	if sg.musicCtl == nil || sg.opts.MusicPlayerTrackLoader == nil {
		return false
	}
	wad := sg.frontendMusicPlayerWAD()
	track := sg.frontendMusicPlayerTrack()
	if wad == nil || track == nil {
		return false
	}
	data, err := sg.opts.MusicPlayerTrackLoader(wad.Key, track.LumpName)
	if err != nil {
		sg.frontendStatus("MUSIC LOAD FAILED", doomTicsPerSecond*2)
		return false
	}
	if len(data) == 0 {
		sg.frontendStatus("SONG NOT FOUND", doomTicsPerSecond*2)
		return false
	}
	sg.musicCtl.PlayData(data, clampVolume(sg.opts.MusicVolume))
	sg.currentMusicSource = musicPlaybackSource{
		kind:       musicPlaybackSourcePlayer,
		wadKey:     wad.Key,
		lumpName:   track.LumpName,
		levelLabel: track.Label,
		musicName:  track.MusicName,
	}
	sg.setNowPlayingLevel(track.Label, string(track.MapName))
	sg.setNowPlayingMusic(track.MusicName, track.LumpName)
	return true
}

func (sg *sessionGame) setNowPlayingMusic(candidates ...string) {
	if sg == nil {
		return
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		sg.nowPlayingMusic = candidate
		return
	}
	sg.nowPlayingMusic = ""
}

func (sg *sessionGame) setNowPlayingLevel(candidates ...string) {
	if sg == nil {
		return
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		sg.nowPlayingLevel = candidate
		return
	}
	sg.nowPlayingLevel = ""
}

func (sg *sessionGame) nowPlayingMusicLabel() string {
	if sg == nil {
		return "-"
	}
	level := strings.TrimSpace(sg.nowPlayingLevel)
	music := strings.TrimSpace(sg.nowPlayingMusic)
	switch {
	case level != "" && music != "":
		return strings.ToUpper(level) + "\nSONG: " + strings.ToUpper(music)
	case level != "":
		return strings.ToUpper(level)
	case music != "":
		return "SONG: " + strings.ToUpper(music)
	default:
		return "-"
	}
}
