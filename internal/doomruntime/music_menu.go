package doomruntime

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/sessionmusic"
)

var errNoMusicSoundFonts = errors.New("no soundfonts found")

type musicPlaybackSourceKind int

const (
	musicPlaybackSourceNone musicPlaybackSourceKind = iota
	musicPlaybackSourceTitle
	musicPlaybackSourceMap
	musicPlaybackSourceIntermission
	musicPlaybackSourcePlayer
)

type musicPlaybackSource struct {
	kind       musicPlaybackSourceKind
	mapName    mapdata.MapName
	commercial bool
	wadKey     string
	lumpName   string
	levelLabel string
	musicName  string
}

func musicBackendLabel(backend music.Backend) string {
	switch music.ResolveBackend(backend) {
	case music.BackendPCSpeaker:
		return "PC SPEAKER - SYNTH"
	case music.BackendMeltySynth:
		return "MIDI - MELTYSYNTH"
	default:
		return "OPL - IMPSYNTH"
	}
}

func effectiveMusicPlaybackVolume(opts Options) float64 {
	if music.ResolveBackend(opts.MusicBackend) == music.BackendPCSpeaker {
		return clampVolume(opts.PCSpeakerVolume)
	}
	return clampVolume(opts.MusicVolume)
}

func musicSoundFontLabel(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "N/A"
	}
	name := strings.TrimSpace(filepath.Base(path))
	if ext := filepath.Ext(name); strings.EqualFold(ext, ".sf2") {
		name = strings.TrimSpace(strings.TrimSuffix(name, ext))
	}
	return strings.ToUpper(name)
}

func (sg *sessionGame) musicSoundFontChoices() []string {
	if sg == nil {
		return nil
	}
	return sg.opts.MusicSoundFontChoices
}

func (sg *sessionGame) musicSelectedSoundFontPath() string {
	if sg == nil {
		return ""
	}
	if strings.TrimSpace(sg.opts.MusicSoundFontPath) != "" {
		return sg.opts.MusicSoundFontPath
	}
	choices := sg.musicSoundFontChoices()
	if len(choices) == 0 {
		return ""
	}
	return choices[0]
}

func (sg *sessionGame) musicSoundFontIndex() int {
	choices := sg.musicSoundFontChoices()
	if len(choices) == 0 {
		return -1
	}
	cur := strings.TrimSpace(sg.musicSelectedSoundFontPath())
	for i, path := range choices {
		if strings.EqualFold(strings.TrimSpace(path), cur) {
			return i
		}
	}
	return 0
}

func (sg *sessionGame) rebuildMusicPlayback() error {
	if sg == nil {
		return nil
	}
	sg.closeMusicPlayback()
	if sg.opts.MapMusicLoader == nil && sg.opts.TitleMusicLoader == nil {
		return nil
	}
	ctl, err := sessionmusic.NewPlayback(
		effectiveMusicPlaybackVolume(sg.opts),
		sg.opts.MUSPanMax,
		sg.opts.OPLVolume,
		sg.opts.AudioPreEmphasis,
		sg.opts.MusicBackend,
		sg.opts.MusicPatchBank,
		sg.opts.MusicSoundFont,
		sg.opts.SharedPCSpeaker,
		sg.opts.MapMusicLoader,
		sg.opts.TitleMusicLoader,
		sg.opts.IntermissionMusicLoader,
	)
	if err != nil {
		return err
	}
	sg.musicCtl = ctl
	return nil
}

func (sg *sessionGame) replayCurrentMusicSource() bool {
	if sg == nil || sg.musicCtl == nil || effectiveMusicPlaybackVolume(sg.opts) <= 0 {
		return false
	}
	switch sg.currentMusicSource.kind {
	case musicPlaybackSourceTitle:
		sg.musicCtl.PlayTitle(effectiveMusicPlaybackVolume(sg.opts))
		sg.setNowPlayingLevel("")
		sg.setNowPlayingMusic("Title Screen")
		return true
	case musicPlaybackSourceMap:
		sg.musicCtl.PlayMap(sg.currentMusicSource.mapName, effectiveMusicPlaybackVolume(sg.opts))
		sg.setNowPlayingLevel(sg.currentMusicSource.levelLabel, string(sg.currentMusicSource.mapName))
		sg.setNowPlayingMusic(sg.currentMusicSource.musicName, string(sg.currentMusicSource.mapName))
		return true
	case musicPlaybackSourceIntermission:
		sg.musicCtl.PlayIntermission(sg.currentMusicSource.commercial, effectiveMusicPlaybackVolume(sg.opts))
		sg.setNowPlayingLevel("")
		if sg.currentMusicSource.musicName != "" {
			sg.setNowPlayingMusic(sg.currentMusicSource.musicName)
		}
		return true
	case musicPlaybackSourcePlayer:
		if sg.opts.MusicPlayerTrackLoader == nil {
			return false
		}
		data, err := sg.opts.MusicPlayerTrackLoader(sg.currentMusicSource.wadKey, sg.currentMusicSource.lumpName)
		if err != nil || len(data) == 0 {
			return false
		}
		sg.musicCtl.PlayData(data, effectiveMusicPlaybackVolume(sg.opts))
		sg.setNowPlayingLevel(sg.currentMusicSource.levelLabel)
		sg.setNowPlayingMusic(sg.currentMusicSource.musicName, sg.currentMusicSource.lumpName)
		return true
	default:
		return false
	}
}

func (sg *sessionGame) restartCurrentMusicPlayback() {
	if sg == nil || sg.musicCtl == nil || effectiveMusicPlaybackVolume(sg.opts) <= 0 {
		return
	}
	if sg.replayCurrentMusicSource() {
		return
	}
	if sg.frontend.Active && !sg.frontend.InGame {
		sg.playTitleMusic()
		return
	}
	if sg.current != "" {
		sg.playMusicForMap(sg.current)
	}
}

func (sg *sessionGame) applyMusicConfig(backend music.Backend, soundFontPath string, soundFont *music.SoundFontBank) error {
	if sg == nil {
		return nil
	}
	sg.opts.MusicBackend = backend
	if strings.TrimSpace(soundFontPath) != "" {
		sg.opts.MusicSoundFontPath = soundFontPath
	}
	if soundFont != nil || music.ResolveBackend(backend) == music.BackendMeltySynth {
		sg.opts.MusicSoundFont = soundFont
	}
	if err := sg.rebuildMusicPlayback(); err != nil {
		return err
	}
	if sg.g != nil {
		sg.g.opts.MusicBackend = sg.opts.MusicBackend
		sg.g.opts.MusicSoundFontPath = sg.opts.MusicSoundFontPath
		sg.g.opts.MusicSoundFont = sg.opts.MusicSoundFont
	}
	sg.restartCurrentMusicPlayback()
	if sg.opts.OnRuntimeSettingsChanged != nil {
		sg.opts.OnRuntimeSettingsChanged(sg.runtimeSettingsSnapshot())
	}
	return nil
}

func frontendMusicSoundFontDownloadStatus(path string) string {
	label := musicSoundFontLabel(path)
	received, total := music.BrowserSoundFontLoadProgress(path)
	switch {
	case total > 0 && received > 0:
		pct := int((received * 100) / total)
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("DOWNLOADING %s %d%%", label, pct)
	case received > 0:
		return fmt.Sprintf("DOWNLOADING %s", label)
	default:
		return fmt.Sprintf("DOWNLOADING %s", label)
	}
}

func (sg *sessionGame) queueFrontendMusicConfigDownload(backend music.Backend, soundFontPath string) {
	if sg == nil {
		return
	}
	sg.frontendMusicConfig = frontendMusicConfigPending{
		active:        true,
		backend:       backend,
		soundFontPath: strings.TrimSpace(soundFontPath),
	}
	sg.frontend.Status = frontendMusicSoundFontDownloadStatus(soundFontPath)
	sg.frontend.StatusTic = 0
}

func (sg *sessionGame) tickPendingFrontendMusicConfig() (bool, error) {
	if sg == nil || !sg.frontendMusicConfig.active {
		return false, nil
	}
	path := strings.TrimSpace(sg.frontendMusicConfig.soundFontPath)
	if music.BrowserSoundFontLoadPending(path) {
		sg.frontend.Status = frontendMusicSoundFontDownloadStatus(path)
		sg.frontend.StatusTic = 0
		return true, nil
	}
	backend := sg.frontendMusicConfig.backend
	sg.frontendMusicConfig = frontendMusicConfigPending{}
	if err := music.BrowserSoundFontLoadError(path); err != nil {
		return true, err
	}
	bank, err := music.ParseSoundFontFile(path)
	if err != nil {
		return true, err
	}
	if err := sg.applyMusicConfig(backend, path, bank); err != nil {
		return true, err
	}
	sg.frontend.Status = ""
	sg.frontend.StatusTic = 0
	return true, nil
}

func (sg *sessionGame) frontendChangeMusicBackend(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	cur := music.ResolveBackend(sg.opts.MusicBackend)
	next := music.BackendImpSynth
	switch cur {
	case music.BackendImpSynth:
		next = music.BackendPCSpeaker
	case music.BackendPCSpeaker:
		next = music.BackendMeltySynth
	}
	if next == music.BackendMeltySynth {
		path := sg.musicSelectedSoundFontPath()
		if strings.TrimSpace(path) == "" {
			return errNoMusicSoundFonts
		}
		if music.StartBrowserSoundFontLoad(path) {
			sg.queueFrontendMusicConfigDownload(next, path)
			return nil
		}
		bank, err := music.ParseSoundFontFile(path)
		if err != nil {
			return err
		}
		return sg.applyMusicConfig(next, path, bank)
	}
	return sg.applyMusicConfig(next, sg.opts.MusicSoundFontPath, sg.opts.MusicSoundFont)
}

func (sg *sessionGame) frontendChangeMusicSoundFont(dir int) error {
	if sg == nil || dir == 0 {
		return nil
	}
	if music.ResolveBackend(sg.opts.MusicBackend) != music.BackendMeltySynth {
		return nil
	}
	choices := sg.musicSoundFontChoices()
	if len(choices) == 0 {
		return errNoMusicSoundFonts
	}
	idx := sg.musicSoundFontIndex()
	if idx < 0 {
		idx = 0
	}
	idx = wrapMusicPlayerIndex(idx, dir, len(choices))
	path := choices[idx]
	if music.StartBrowserSoundFontLoad(path) {
		sg.queueFrontendMusicConfigDownload(sg.opts.MusicBackend, path)
		return nil
	}
	bank, err := music.ParseSoundFontFile(path)
	if err != nil {
		return err
	}
	return sg.applyMusicConfig(sg.opts.MusicBackend, path, bank)
}
