package sessionflow

import "testing"

func TestStartFrontendDefaultsToMediumSkill(t *testing.T) {
	got := StartFrontend()
	if got.SkillOn != 2 {
		t.Fatalf("SkillOn=%d want 2", got.SkillOn)
	}
}

func TestStepFrontendMainMenuLoadSaveBindings(t *testing.T) {
	cfg := FrontendConfig{
		MainMenuCount: 6,
		StatusTics:    35,
	}

	load := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeTitle, MenuActive: true, ItemOn: 2},
		FrontendInput{Select: true},
		cfg,
	)
	if !load.OpenLoadGameMenu || load.OpenSaveGameMenu {
		t.Fatalf("load request mismatch: %+v", load)
	}
	if load.State.Mode != FrontendModeSaveLoad || load.State.SaveLoadSaving {
		t.Fatalf("load state mismatch: %+v", load.State)
	}

	save := StepFrontend(
		Frontend{Active: true, InGame: true, Mode: FrontendModeTitle, MenuActive: true, ItemOn: 3},
		FrontendInput{Select: true},
		cfg,
	)
	if !save.OpenSaveGameMenu || save.OpenLoadGameMenu {
		t.Fatalf("save request mismatch: %+v", save)
	}
	if save.State.Mode != FrontendModeSaveLoad || !save.State.SaveLoadSaving {
		t.Fatalf("save state mismatch: %+v", save.State)
	}
}

func TestStepFrontendSaveLoadMenuSelectsNumberedSlot(t *testing.T) {
	load := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeSaveLoad, SaveLoadOn: 2},
		FrontendInput{Select: true},
		FrontendConfig{},
	)
	if got := load.LoadGameSlot; got != 2 {
		t.Fatalf("LoadGameSlot=%d want 2", got)
	}

	save := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeSaveLoad, SaveLoadSaving: true, SaveLoadOn: 4},
		FrontendInput{Select: true},
		FrontendConfig{},
	)
	if got := save.SaveGameSlot; got != 4 {
		t.Fatalf("SaveGameSlot=%d want 4", got)
	}
}

func TestStepFrontendSaveLoadMenuSelectsQuicksaveSlotZero(t *testing.T) {
	load := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeSaveLoad, SaveLoadOn: 0},
		FrontendInput{Select: true},
		FrontendConfig{},
	)
	if got := load.LoadGameSlot; got != 0 {
		t.Fatalf("LoadGameSlot=%d want 0", got)
	}

	save := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeSaveLoad, SaveLoadSaving: true, SaveLoadOn: 0},
		FrontendInput{Select: true},
		FrontendConfig{},
	)
	if got := save.SaveGameSlot; got != 0 {
		t.Fatalf("SaveGameSlot=%d want 0", got)
	}
}

func TestStepFrontendMainMenuSelectableRowsSkipsDisabledItems(t *testing.T) {
	cfg := FrontendConfig{
		MainMenuCount: 6,
		MainMenuRows:  []int{1, 4, 5},
	}

	up := StepFrontend(
		Frontend{Active: true, InGame: true, Mode: FrontendModeTitle, MenuActive: true, ItemOn: 1},
		FrontendInput{Up: true},
		cfg,
	)
	if got := up.State.ItemOn; got != 5 {
		t.Fatalf("up itemOn=%d want 5", got)
	}

	down := StepFrontend(
		Frontend{Active: true, InGame: true, Mode: FrontendModeTitle, MenuActive: true, ItemOn: 1},
		FrontendInput{Down: true},
		cfg,
	)
	if got := down.State.ItemOn; got != 4 {
		t.Fatalf("down itemOn=%d want 4", got)
	}
}

func TestNewGameStartMapUsesEpisodeOneForSingleEpisodeCustomLoader(t *testing.T) {
	got := NewGameStartMap("E1M3", []int{1}, 0, true)
	if got != "E1M1" {
		t.Fatalf("NewGameStartMap()=%q want %q", got, "E1M1")
	}
}

func TestStepFrontendOptionsSelectMusicOpensMusicSubmenu(t *testing.T) {
	cfg := FrontendConfig{
		OptionRows:     []int{0, 1, 2, 3, 4, 5, 6, 7},
		SoundMenuCount: 5,
	}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeOptions, OptionsOn: 5},
		FrontendInput{Select: true},
		cfg,
	)
	if got.State.Mode != FrontendModeSound {
		t.Fatalf("mode=%v want sound submenu", got.State.Mode)
	}
	if got.State.SoundOn != 0 {
		t.Fatalf("soundOn=%d want 0", got.State.SoundOn)
	}
}

func TestStepFrontendOptionsSelectVoiceOpensVoiceMenu(t *testing.T) {
	cfg := FrontendConfig{
		OptionRows:     []int{0, 1, 2, 3, 4, 5, 6, 7},
		VoiceMenuCount: 7,
	}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeOptions, OptionsOn: 6},
		FrontendInput{Select: true},
		cfg,
	)
	if got.State.Mode != FrontendModeVoice {
		t.Fatalf("mode=%v want voice submenu", got.State.Mode)
	}
	if got.State.VoiceOn != 0 {
		t.Fatalf("voiceOn=%d want 0", got.State.VoiceOn)
	}
}

func TestStepFrontendVoiceMenuNavigatesAllRows(t *testing.T) {
	cfg := FrontendConfig{VoiceMenuCount: 7}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeVoice, VoiceOn: 0},
		FrontendInput{Down: true},
		cfg,
	)
	if got.State.VoiceOn != 1 {
		t.Fatalf("voiceOn=%d want 1", got.State.VoiceOn)
	}
	got = StepFrontend(
		got.State,
		FrontendInput{Up: true},
		cfg,
	)
	if got.State.VoiceOn != 0 {
		t.Fatalf("voiceOn=%d want 0 after moving back up", got.State.VoiceOn)
	}
}

func TestStepFrontendVoiceMenuSelectPushToTalk(t *testing.T) {
	cfg := FrontendConfig{VoiceMenuCount: 7}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeVoice, VoiceOn: 6},
		FrontendInput{Select: true},
		cfg,
	)
	if !got.ChangeVoicePushToTalk {
		t.Fatal("expected push-to-talk toggle request")
	}
}

func TestStepFrontendOptionsSelectKeybindsOpensKeybindMenu(t *testing.T) {
	cfg := FrontendConfig{
		OptionRows: []int{0, 1, 2, 3, 4, 5, 6, 7},
	}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeOptions, OptionsOn: 7},
		FrontendInput{Select: true},
		cfg,
	)
	if !got.OpenKeybinds {
		t.Fatal("expected keybind menu open request")
	}
}

func TestStepFrontendMusicSubmenuSelectPlayerRequestsOpen(t *testing.T) {
	cfg := FrontendConfig{SoundMenuCount: 5}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeSound, SoundOn: 4},
		FrontendInput{Select: true},
		cfg,
	)
	if !got.OpenMusicPlayer {
		t.Fatal("expected music player open request")
	}
}

func TestStepFrontendOptionsEscapeClosesAttractMenu(t *testing.T) {
	cfg := FrontendConfig{
		OptionRows: []int{0, 1, 2, 3, 4, 5, 6, 7},
	}
	got := StepFrontend(
		Frontend{Active: true, Attract: true, Mode: FrontendModeOptions, MenuActive: true, OptionsOn: 2},
		FrontendInput{Escape: true},
		cfg,
	)
	if got.State.Mode != FrontendModeTitle {
		t.Fatalf("mode=%v want title", got.State.Mode)
	}
	if got.State.MenuActive {
		t.Fatal("expected attract menu to close on escape")
	}
}

func TestShowAttractBeginPrompt(t *testing.T) {
	if !ShowAttractBeginPrompt(Frontend{Active: true, Mode: FrontendModeTitle}) {
		t.Fatal("expected prompt during frontend attract when menu is closed")
	}
	if ShowAttractBeginPrompt(Frontend{Active: true, Mode: FrontendModeTitle, MenuActive: true}) {
		t.Fatal("did not expect prompt while menu is open")
	}
	if ShowAttractBeginPrompt(Frontend{Active: true, InGame: true, Mode: FrontendModeTitle}) {
		t.Fatal("did not expect prompt for in-game frontend")
	}
	if ShowAttractBeginPrompt(Frontend{Active: true, Mode: FrontendModeOptions}) {
		t.Fatal("did not expect prompt outside title frontend mode")
	}
}
