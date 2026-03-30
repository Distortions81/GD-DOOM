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
	if !load.RequestLoadGame || load.RequestSaveGame {
		t.Fatalf("load request mismatch: %+v", load)
	}

	save := StepFrontend(
		Frontend{Active: true, InGame: true, Mode: FrontendModeTitle, MenuActive: true, ItemOn: 3},
		FrontendInput{Select: true},
		cfg,
	)
	if !save.RequestSaveGame || save.RequestLoadGame {
		t.Fatalf("save request mismatch: %+v", save)
	}
}

func TestStepFrontendOptionsSelectMusicOpensMusicSubmenu(t *testing.T) {
	cfg := FrontendConfig{
		OptionRows:     []int{0, 1, 2, 3, 4, 5, 6},
		MusicMenuCount: 4,
	}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeOptions, OptionsOn: 6},
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

func TestStepFrontendMusicSubmenuSelectPlayerRequestsOpen(t *testing.T) {
	cfg := FrontendConfig{MusicMenuCount: 4}
	got := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeSound, SoundOn: 3},
		FrontendInput{Select: true},
		cfg,
	)
	if !got.OpenMusicPlayer {
		t.Fatal("expected music player open request")
	}
}

func TestStepFrontendOptionsEscapeClosesAttractMenu(t *testing.T) {
	cfg := FrontendConfig{
		OptionRows: []int{0, 1, 2, 3, 4, 5, 6},
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
