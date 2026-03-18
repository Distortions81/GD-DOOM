package sessionflow

import "testing"

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
