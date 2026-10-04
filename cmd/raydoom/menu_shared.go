//go:build raylib && cgo && !js

package main

import (
	"fmt"
	"gddoom/internal/doomruntime"
	"gddoom/internal/music"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/sessionflow"
	"math"
)

func (m *nativeMenu) isSharedPage() bool {
	switch m.page {
	case menuMain, menuOptions, menuSound, menuNewGame, menuSkill, menuHelp, menuSave, menuLoad, menuQuit:
		return true
	}
	return false
}

// Shared actions retain their original IDs; native removes the voice row from
// drawing, keyboard navigation and mouse hit testing without changing main.
var nativeOptionActions = [...]int{0, 1, 2, 3, 4, 5, 7, 8}

func nativeOptionRow(action int) int {
	for row, id := range nativeOptionActions {
		if id == action {
			return row
		}
	}
	return 0
}

func (m *nativeMenu) sharedState() sessionflow.Frontend {
	f := m.flow
	f.Active, f.MenuActive, f.InGame = true, true, !m.frontend
	f.Status = m.status
	switch m.page {
	case menuMain:
		f.Mode, f.ItemOn = sessionflow.FrontendModeTitle, m.row
	case menuOptions:
		f.Mode, f.OptionsOn = sessionflow.FrontendModeOptions, nativeOptionActions[m.row]
	case menuSound:
		f.Mode, f.SoundOn = sessionflow.FrontendModeSound, m.row
	case menuNewGame:
		f.Mode, f.EpisodeOn = sessionflow.FrontendModeEpisode, m.row
	case menuSkill:
		f.Mode, f.SkillOn = sessionflow.FrontendModeSkill, m.row
	case menuHelp:
		f.Mode, f.MenuActive = sessionflow.FrontendModeReadThis, false
	case menuSave, menuLoad:
		f.Mode, f.SaveLoadOn, f.SaveLoadSaving = sessionflow.FrontendModeSaveLoad, m.row, m.page == menuSave
	}
	return f
}

func (m *nativeMenu) applySharedState(f sessionflow.Frontend) {
	if !f.Active {
		m.flow = f
		m.page = menuClosed
		return
	}
	m.flow = f
	m.status = f.Status
	switch f.Mode {
	case sessionflow.FrontendModeTitle:
		m.page, m.row = menuMain, f.ItemOn
		if !f.MenuActive {
			m.page = menuClosed
		}
	case sessionflow.FrontendModeOptions:
		m.page, m.row = menuOptions, nativeOptionRow(f.OptionsOn)
	case sessionflow.FrontendModeSound:
		m.page, m.row = menuSound, f.SoundOn
	case sessionflow.FrontendModeEpisode:
		m.page, m.row = menuNewGame, f.EpisodeOn
	case sessionflow.FrontendModeSkill:
		m.page, m.row = menuSkill, f.SkillOn
	case sessionflow.FrontendModeReadThis:
		m.page = menuHelp
	case sessionflow.FrontendModeSaveLoad:
		m.page, m.row = menuLoad, f.SaveLoadOn
		if f.SaveLoadSaving {
			m.page = menuSave
		}
	}
}

func (m *nativeMenu) updateShared(in menuInput, s *nativeSettings) menuCommand {
	if m.page == menuQuit {
		if m.quitPrompt.ExitDelayTic > 0 {
			return menuNoCommand
		}
		if in.quitYes {
			m.quitPrompt, _ = sessionflow.TickQuitPrompt(m.quitPrompt, true, false)
			return menuConfirmQuit
		}
		if in.back || in.quitNo {
			m.quitPrompt, _ = sessionflow.TickQuitPrompt(m.quitPrompt, false, true)
			m.page, m.row = m.quitFrom, m.quitFromRow
		}
		return menuNoCommand
	}
	if in.mouseRow >= 0 && in.mouseRow < len(m.rows(*s)) {
		m.row = in.mouseRow
	}
	if m.page == menuOptions && m.row == len(nativeOptionActions)-1 && in.confirm {
		m.open(menuGraphics)
		return menuNoCommand
	}
	old := m.sharedState()
	cfg := doomruntime.NativeFrontendConfig(m.opts.Episodes, len(m.saveSlots))
	if m.opts.LiveTicSource != nil && m.opts.LiveTicSink == nil {
		cfg.MainMenuRows = doomruntime.NativeWatchMenuRows()
		if m.page == menuMain && (m.row == 0 || m.row == 2 || m.row == 3) {
			m.row = 1
			in.confirm = false
			old.ItemOn = 1
		}
	}
	cfg.ReadThisPageCount = doomruntime.NativeReadThisPageCount(m.opts)
	cfg.OptionRows = nativeOptionActions[:]
	input := sessionflow.FrontendInput{Escape: in.back, Up: in.up, Down: in.down, Left: in.left, Right: in.right, Select: in.confirm, Skip: in.back || in.up || in.down || in.left || in.right || in.confirm || in.anyKey}
	result := sessionflow.StepFrontend(old, input, cfg)
	m.applySharedState(result.State)
	if result.ChangeMessages {
		s.messages = !s.messages
		message := "MESSAGES OFF"
		if s.messages {
			message = "MESSAGES ON"
		}
		m.setStatus(message, 35)
	}
	if result.ChangePerf {
		s.showFPS = !s.showFPS
		message := "FPS OFF"
		if s.showFPS {
			message = "FPS ON"
		}
		m.setStatus(message, 35)
	}
	if result.ChangeDetail {
		dir := 1
		if in.left {
			dir = -1
		}
		if old.OptionsOn == 1 {
			if in.confirm {
				s.screenBlocks = 1 + (s.screenBlocks % 2)
			} else {
				s.screenBlocks = min(2, max(1, s.screenBlocks+dir))
			}
		} else {
			if in.confirm {
				s.hudScale = (s.hudScale + 1) % 8
			} else {
				s.hudScale = min(7, max(0, s.hudScale+dir))
			}
		}
	}
	if result.StatusMessage != "" {
		m.setStatus(result.StatusMessage, result.StatusMessageTic)
	}
	if result.ChangeMouse != 0 {
		prev := s.mouseSensitivity
		s.mouseSensitivity = m.shared.NextMouseSensitivity(m.opts, prev, result.ChangeMouse, in.confirm)
		if s.mouseSensitivity != prev {
			m.setStatus(fmt.Sprintf("MOUSE SENSITIVITY %.2f", s.mouseSensitivity), 35)
		}
	}
	adjust := func(value *float64, dir int) {
		next := math.Max(0, math.Min(1, *value+float64(dir)*.1))
		if in.confirm && next == *value {
			next = 0
		}
		*value = next
	}
	if result.ChangeSFX != 0 {
		adjust(&s.sfxVolume, result.ChangeSFX)
	}
	if result.ChangeMusic != 0 {
		value := &s.musicVolume
		if music.ResolveBackend(s.musicBackend) == music.BackendPCSpeaker {
			value = &s.speakerVolume
		}
		if s.musicMuted {
			*value, s.musicMuted = 0, false
		}
		adjust(value, result.ChangeMusic)
	}
	if result.ChangeSynth != 0 {
		values := []music.Backend{music.BackendImpSynth, music.BackendPCSpeaker, music.BackendMeltySynth}
		for i, value := range values {
			if value == music.ResolveBackend(s.musicBackend) {
				s.musicBackend = values[(i+1)%len(values)]
				break
			}
		}
		if s.musicBackend == music.BackendMeltySynth && s.soundFont == "" && len(m.opts.MusicSoundFontChoices) > 0 {
			s.soundFont = m.opts.MusicSoundFontChoices[0]
		}
	}
	if result.ChangeSoundFont != 0 {
		m.changeSoundFont(s, result.ChangeSoundFont)
	}
	if result.OpenKeybinds {
		m.open(menuBindings)
	}
	if result.OpenMusicPlayer {
		m.open(menuMusicPlayer)
		m.row = 2
	}
	if result.RequestQuit {
		m.open(menuQuit)

	}
	if result.StartGameSkill > 0 && len(m.maps) > 0 {
		m.skill = result.StartGameSkill
		name := sessionflow.NewGameStartMap(m.maps[m.mapIndex], m.opts.Episodes, result.State.SelectedEpisode, true)
		for i, available := range m.maps {
			if string(available) == name {
				m.mapIndex = i
				m.frontend = false
				m.open(menuClosed)
				return menuStart
			}
		}
		m.setStatus("START MAP IS NOT AVAILABLE", 70)
	}
	if (result.SaveGameSlot >= 0 || result.LoadGameSlot >= 0) && old.Mode == sessionflow.FrontendModeSaveLoad && in.confirm && m.row < len(m.saveSlots) {
		m.saveSlot = m.saveSlots[m.row].Slot
		if old.SaveLoadSaving {
			return menuSaveGame
		}
		return menuLoadGame
	}
	return menuNoCommand
}

func (m *nativeMenu) drawShared(s nativeSettings, frame int) []levelmesh.Patch {
	if m.shared == nil {
		m.shared = doomruntime.NewNativeMenuRenderer()
	}
	opts := m.opts
	opts.SourcePortMode, opts.SFXVolume, opts.MusicVolume = true, s.sfxVolume, s.musicPlaybackVolume()
	opts.MusicBackend, opts.MusicSoundFontPath, opts.PCSpeakerVolume, opts.MouseLookSpeed, opts.InputBindings = s.musicBackend, s.soundFont, s.speakerVolume, s.mouseSensitivity, s.bindings
	v := doomruntime.NativeMenuView{State: m.sharedState(), Frame: frame, BindingRow: m.row, BindingSlot: m.bindingSlot, BindingCapture: m.capture, MusicRow: m.row, MusicWAD: m.musicWAD, MusicGroup: m.musicGroup, MusicTrack: m.musicTrack, NowPlaying: m.nowPlaying, Slots: m.saveSlots, Messages: s.messages, ShowFPS: s.showFPS, ScreenBlocks: s.screenBlocks, HUDScale: s.hudScale}
	v.HideVoiceOptions = true
	if m.page == menuBindings {
		v.State.Mode = doomruntime.NativeMenuBindings
		for _, action := range nativeBindingRows(s.bindings) {
			v.BindingActions = append(v.BindingActions, action.ID)
		}
	}
	if m.page == menuMusicPlayer {
		v.State.Mode = doomruntime.NativeMenuMusicPlayer
	}
	if m.page == menuQuit {
		v.QuitLines = m.quitPrompt.Lines
		if len(v.QuitLines) == 0 {
			v.QuitLines = doomruntime.NativeQuitPromptLines()
		}
	}
	m.patches = append(m.patches[:0], m.shared.Draw(opts, v)...)
	if m.page == menuOptions {
		m.textScaled("RAYLIB OPTIONS", 36, 151, 1.2)
		m.textScaled("OPEN", 251, 151, 1.2)
	}
	return m.patches
}
