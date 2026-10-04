//go:build raylib && cgo && !js

package main

import (
	"fmt"
	"gddoom/internal/music"
	"gddoom/internal/runtimecfg"
	"path/filepath"
	"strings"
)

func (s nativeSettings) musicConfig() nativeMusicConfig {
	backend := s.musicBackend
	if backend == "" {
		backend = music.DefaultBackend()
	}
	return nativeMusicConfig{backend: backend, soundFont: s.soundFont, panMax: s.musicPan, speakerVariant: s.speakerVariant, volumeCompression: music.NormalizeMUSVolumeCompression(s.musicCompression)}
}
func (s nativeSettings) musicPlaybackVolume() float64 {
	if s.musicMuted {
		return 0
	}
	if music.ResolveBackend(s.musicBackend) == music.BackendPCSpeaker {
		return s.speakerVolume
	}
	return s.musicVolume
}
func nativeSynthLabel(backend music.Backend) string {
	switch music.ResolveBackend(backend) {
	case music.BackendPCSpeaker:
		return "PC SPEAKER"
	case music.BackendMeltySynth:
		return "MELTYSYNTH"
	default:
		return "IMPSYNTH"
	}
}
func nativeFontLabel(path string) string {
	if path == "" {
		return "N/A"
	}
	return strings.ToUpper(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}
func (m *nativeMenu) selectedMusicWAD() *runtimecfg.MusicPlayerWAD {
	if len(m.opts.MusicPlayerCatalog) == 0 {
		return nil
	}
	m.musicWAD = (m.musicWAD%len(m.opts.MusicPlayerCatalog) + len(m.opts.MusicPlayerCatalog)) % len(m.opts.MusicPlayerCatalog)
	return &m.opts.MusicPlayerCatalog[m.musicWAD]
}
func (m *nativeMenu) selectedMusicGroup() *runtimecfg.MusicPlayerEpisode {
	w := m.selectedMusicWAD()
	if w == nil || len(w.Episodes) == 0 {
		return nil
	}
	m.musicGroup = (m.musicGroup%len(w.Episodes) + len(w.Episodes)) % len(w.Episodes)
	return &w.Episodes[m.musicGroup]
}
func (m *nativeMenu) selectedMusicTrack() *runtimecfg.MusicPlayerTrack {
	g := m.selectedMusicGroup()
	if g == nil || len(g.Tracks) == 0 {
		return nil
	}
	m.musicTrack = (m.musicTrack%len(g.Tracks) + len(g.Tracks)) % len(g.Tracks)
	return &g.Tracks[m.musicTrack]
}
func (m *nativeMenu) syncMusicSelection(key, lump string) {
	for wi, w := range m.opts.MusicPlayerCatalog {
		if key != "" && w.Key != key {
			continue
		}
		for gi, g := range w.Episodes {
			for ti, track := range g.Tracks {
				if track.LumpName == lump {
					m.musicWAD, m.musicGroup, m.musicTrack = wi, gi, ti
					return
				}
			}
		}
		if key == "" {
			break
		}
	}
}
func (m *nativeMenu) musicRows() []menuRow {
	rows := []menuRow{{label: "WAD", value: "N/A"}, {label: "GROUP", value: "N/A"}, {label: "TRACK", value: "N/A"}}
	if w := m.selectedMusicWAD(); w != nil {
		rows[0].value = w.Label
	}
	if g := m.selectedMusicGroup(); g != nil {
		rows[1].value = g.Label
	}
	if t := m.selectedMusicTrack(); t != nil {
		rows[2].value = t.Label
	}
	return rows
}
func (m *nativeMenu) adjustMusicSelection(dir int) {
	m.musicWAD, m.musicGroup, m.musicTrack = m.shared.AdjustMusicSelection(m.opts, m.row, m.musicWAD, m.musicGroup, m.musicTrack, dir)
}
func (m *nativeMenu) changeSoundFont(s *nativeSettings, dir int) {
	if music.ResolveBackend(s.musicBackend) != music.BackendMeltySynth {
		return
	}
	choices := m.opts.MusicSoundFontChoices
	if len(choices) == 0 {
		m.setStatus("NO SOUNDFONTS FOUND", 70)
		return
	}
	index := -1
	for i, path := range choices {
		if strings.EqualFold(path, s.soundFont) {
			index = i
			break
		}
	}
	if index < 0 {
		index = 0
		dir = 0
	}
	s.soundFont = choices[(index+dir+len(choices))%len(choices)]
}

func (s *nativeSettings) setMusicConfig(cfg nativeMusicConfig) {
	s.musicBackend, s.soundFont, s.musicPan, s.speakerVariant = cfg.backend, cfg.soundFont, cfg.panMax, cfg.speakerVariant
	s.musicCompression = cfg.volumeCompression
}
func nativeSoundFontDownloadStatus(path string) string {
	received, total := music.BrowserSoundFontLoadProgress(path)
	status := "DOWNLOADING " + nativeFontLabel(path)
	if total > 0 {
		status += fmt.Sprintf(" %d%%", min(100, max(0, int(received*100/total))))
	}
	return status
}
