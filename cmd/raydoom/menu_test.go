//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/runtimecfg"
	"testing"
)

func TestNativeMenuNavigationSettingsAndNewGame(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{SkillLevel: 3}, []mapdata.MapName{"E1M1", "E1M2"}, "E1M1")
	s := nativeSettings{sfxVolume: .7, musicVolume: .5, mouseSensitivity: 1, textureFilter: raymesh.Anisotropic, lighting: raymesh.DoomLighting, fps: 144}
	input := func(in menuInput) menuCommand { in.mouseRow = -1; return m.update(in, &s) }
	m.open(menuMain)
	input(menuInput{up: true})
	if m.row != len(m.rows(s))-1 {
		t.Fatal("menu selection did not wrap")
	}
	input(menuInput{confirm: true})
	if m.page != menuQuit {
		t.Fatal("quit bypassed confirmation")
	}
	if input(menuInput{confirm: true}) == menuExit {
		t.Fatal("default quit choice should resume")
	}
	input(menuInput{quitNo: true})
	m.open(menuSound)
	for range 20 {
		input(menuInput{left: true})
	}
	if s.sfxVolume != 0 {
		t.Fatal("volume did not clamp at mute")
	}
	input(menuInput{down: true})
	for range 20 {
		input(menuInput{right: true})
	}
	if s.musicVolume != 1 {
		t.Fatal("music volume exceeded its range")
	}
	input(menuInput{back: true})
	if m.page != menuOptions {
		t.Fatal("sound back did not return to options")
	}
	m.open(menuGraphics)
	input(menuInput{right: true})
	if s.textureFilter != raymesh.Nearest {
		t.Fatal("filter cycle did not wrap")
	}
	input(menuInput{down: true})
	input(menuInput{right: true})
	if s.lighting != raymesh.SectorLighting {
		t.Fatal("lighting setting not applied")
	}
	m.open(menuMain)
	input(menuInput{confirm: true})
	if m.page != menuSkill || m.row != 2 {
		t.Fatal("shareware New Game must open the shared skill menu")
	}
	input(menuInput{down: true})
	if input(menuInput{confirm: true}) != menuStart || m.maps[m.mapIndex] != "E1M1" || m.skill != 4 || m.page != menuClosed {
		t.Fatal("new game ignored skill selection or did not start the episode's first map")
	}
}

func TestNativeMenuMouseCoordinatesMatchCenteredUI(t *testing.T) {
	m := &nativeMenu{page: menuMain}
	// At 1280x800: the shared menu uses scale=4 and origin=(0,0).
	if row := m.rowAt(300, 4*80, 1280, 800); row != 1 {
		t.Fatalf("mouse row=%d want=1", row)
	}
	if row := m.rowAt(0, 100, 1280, 800); row != -1 {
		t.Fatal("outside letterbox selected a row")
	}
}

func TestNativeSaveMenuSelectsActualSlotsAndScrolls(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{SkillLevel: 3}, []mapdata.MapName{"E1M1"}, "E1M1")
	s := nativeSettings{}
	m.open(menuMain)
	for i, row := range m.rows(s) {
		if row.label == "SAVE GAME" {
			m.row = i
			if command := m.update(menuInput{confirm: true, mouseRow: -1}, &s); command != menuNoCommand || m.page != menuSave {
				t.Fatal("Save Game did not open slots")
			}
			break
		}
	}
	for i := 0; i < 20; i++ {
		m.saveSlots = append(m.saveSlots, doomruntime.NativeSaveSlot{Slot: i * 3})
	}
	m.row = 15
	if cmd := m.update(menuInput{confirm: true, mouseRow: -1}, &s); cmd != menuSaveGame || m.saveSlot != 45 {
		t.Fatal("save selection confused row with slot")
	}
	first, step, offset, count := m.rowLayout(len(m.rows(s)))
	if offset == 0 || count != 7 {
		t.Fatal("large slot list did not scroll")
	}
	// Map the selected visible row through the same centered UI coordinates.
	y := (first + float64(m.row-offset)*step) * 4
	if row := m.rowAt(300, y, 1280, 800); row != m.row {
		t.Fatalf("scrolled mouse row=%d want=%d", row, m.row)
	}
	m.open(menuLoad)
	m.row = 15
	if cmd := m.update(menuInput{confirm: true, mouseRow: -1}, &s); cmd != menuLoadGame || m.saveSlot != 45 {
		t.Fatal("load selection confused row with slot")
	}
	m.update(menuInput{back: true, mouseRow: -1}, &s)
	if m.page != menuMain {
		t.Fatal("Escape from slots did not return to main")
	}
	m.frontend = true
	m.open(menuMain)
	foundLoad := false
	for _, row := range m.rows(s) {
		foundLoad = foundLoad || row.label == "LOAD GAME"
	}
	if !foundLoad {
		t.Fatal("frontend cannot load a save")
	}
}

func TestNativeBindingMenuCaptureCancelClearAndDefaults(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{bindings: runtimecfg.DefaultInputBindings(), mouseSensitivity: 1, keyboardSpeed: 1}
	input := func(in menuInput) { in.mouseRow = -1; m.update(in, &s) }
	m.open(menuControls)
	input(menuInput{confirm: true})
	if m.page != menuBindings {
		t.Fatal("controls did not open key bindings")
	}
	input(menuInput{right: true})
	if m.bindingSlot != 1 {
		t.Fatal("alternate slot cannot be selected")
	}
	input(menuInput{confirm: true})
	if !m.capture {
		t.Fatal("confirm did not start capture")
	}
	input(menuInput{captured: "MB2"})
	if m.capture || s.bindings.MoveForward[1] != "MB2" {
		t.Fatal("mouse binding capture failed")
	}
	input(menuInput{confirm: true})
	input(menuInput{back: true})
	if m.capture || m.page != menuBindings {
		t.Fatal("Escape did not cancel only capture")
	}
	input(menuInput{clear: true})
	if s.bindings.MoveForward[1] != "" {
		t.Fatal("alternate binding did not clear")
	}
	s.bindings.MoveForward = runtimecfg.KeyBinding{"I", "MB2"}
	input(menuInput{defaults: true})
	if s.bindings != runtimecfg.DefaultInputBindings() {
		t.Fatal("restore defaults failed")
	}
	for range 19 {
		input(menuInput{down: true})
	}
	first, step, offset, _ := m.rowLayout(len(m.rows(s)))
	if offset == 0 {
		t.Fatal("weapon rows did not scroll")
	}
	if row := m.rowAt(300, (first+float64(m.row-offset)*step)*4, 1280, 800); row != m.row {
		t.Fatal("scrolled binding mouse hit test diverged")
	}
	if m.bindingSlotAt(1000, 1280, 800) != 1 || m.bindingSlotAt(750, 1280, 800) != 0 {
		t.Fatal("mouse selected wrong binding column")
	}
	input(menuInput{back: true})
	if m.page != menuControls || m.row != 0 {
		t.Fatal("bindings back did not return to controls")
	}
}

func TestNativeMusicMenusBrowseAndPreserveGameSelection(t *testing.T) {
	catalog := []runtimecfg.MusicPlayerWAD{
		{Key: "doom.wad", Label: "DOOM", Episodes: []runtimecfg.MusicPlayerEpisode{
			{Label: "EPISODE 1", Tracks: []runtimecfg.MusicPlayerTrack{{LumpName: "D_E1M1", Label: "HANGAR", MusicName: "At Doom's Gate"}, {LumpName: "D_E1M2", Label: "PLANT"}}},
			{Label: "OTHER MUSIC", Tracks: []runtimecfg.MusicPlayerTrack{{LumpName: "D_INTRO", Label: "INTRO"}}},
		}},
		{Key: "doom2.wad", Label: "DOOM II", Episodes: []runtimecfg.MusicPlayerEpisode{{Label: "MAPS", Tracks: []runtimecfg.MusicPlayerTrack{{LumpName: "D_RUNNIN", Label: "ENTRYWAY"}}}}},
	}
	m := newNativeMenu(doomruntime.Options{MusicPlayerCatalog: catalog, MusicPlayerTrackLoader: func(string, string) ([]byte, error) { return []byte{1}, nil }, MusicSoundFontChoices: []string{"general-midi.sf2", "custom.sf2"}}, []mapdata.MapName{"E1M1", "E1M3"}, "E1M3")
	s := nativeSettings{musicBackend: music.BackendImpSynth, musicVolume: .5, speakerVolume: 1}
	input := func(in menuInput) menuCommand { in.mouseRow = -1; return m.update(in, &s) }
	m.open(menuSound)
	m.row = 2
	input(menuInput{right: true})
	if s.musicBackend != music.BackendPCSpeaker {
		t.Fatal("missing speaker backend")
	}
	m.row = 1
	input(menuInput{left: true})
	if s.speakerVolume != .9 || s.musicVolume != .5 {
		t.Fatal("speaker volume changed synth mixer volume")
	}
	m.row = 2
	input(menuInput{right: true})
	if s.musicBackend != music.BackendMeltySynth || s.soundFont != "general-midi.sf2" {
		t.Fatal("MeltySynth did not select available soundfont")
	}
	m.row = 3
	input(menuInput{right: true})
	if s.soundFont != "custom.sf2" {
		t.Fatal("soundfont selection did not advance")
	}
	m.row = 4
	input(menuInput{confirm: true})
	if m.page != menuMusicPlayer {
		t.Fatal("music player cannot open")
	}
	m.syncMusicSelection("doom.wad", "D_E1M2")
	if m.musicTrack != 1 {
		t.Fatal("current track not selected")
	}
	m.row = 2
	input(menuInput{right: true})
	if m.selectedMusicTrack().LumpName != "D_E1M1" {
		t.Fatal("track did not wrap")
	}
	if input(menuInput{confirm: true}) != menuPlayMusic || m.mapIndex != 1 {
		t.Fatal("track play disturbed selected game map")
	}
	m.row = 1
	input(menuInput{left: true})
	if m.selectedMusicTrack().LumpName != "D_INTRO" {
		t.Fatal("group selection failed")
	}
	m.row = 0
	input(menuInput{right: true})
	if m.selectedMusicTrack().LumpName != "D_RUNNIN" {
		t.Fatal("WAD selection did not reset group and track")
	}
	input(menuInput{back: true})
	if m.page != menuSound {
		t.Fatal("player back did not return to sound settings")
	}
}

func TestNativeSpeakerMenuControlsAreIndependent(t *testing.T) {
	m := &nativeMenu{page: menuSpeaker}
	s := nativeSettings{musicBackend: music.BackendImpSynth, sfxVolume: .7, musicVolume: .5, speakerVolume: .9, speakerVariant: "paper-speaker"}
	change := func(label string, in menuInput) {
		t.Helper()
		for i, row := range m.rows(s) {
			if row.label == label {
				m.row, in.mouseRow = i, -1
				m.update(in, &s)
				return
			}
		}
		t.Fatal("missing speaker row", label)
	}
	change("PC SPEAKER SFX", menuInput{confirm: true})
	change("SPEAKER VOLUME", menuInput{left: true})
	change("SPEAKER MODEL", menuInput{right: true})
	if !s.pcSpeaker || s.speakerVolume != .8 || s.speakerVariant != "small-buzzer" || s.musicVolume != .5 || s.sfxVolume != .7 || s.musicBackend != music.BackendImpSynth {
		t.Fatalf("speaker settings affected digital music: %+v", s)
	}
	change("BACK", menuInput{confirm: true})
	if m.page != menuGraphics || m.row != 6 {
		t.Fatal("speaker extension did not return to renderer options")
	}
}

func TestNativeMusicMuteDoesNotMuteSpeakerEffects(t *testing.T) {
	s := nativeSettings{musicBackend: music.BackendPCSpeaker, speakerVolume: .7, musicMuted: true}
	if s.musicPlaybackVolume() != 0 || s.speakerVolume != .7 {
		t.Fatal("music mute changed physical speaker volume")
	}
	m := &nativeMenu{page: menuSound, row: 1}
	m.update(menuInput{right: true, mouseRow: -1}, &s)
	if s.musicMuted || s.musicPlaybackVolume() != .1 {
		t.Fatal("music control cannot unmute playback")
	}
}
