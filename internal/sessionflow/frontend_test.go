package sessionflow

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestStepFrontendMainMenuStartGameWithEpisodes(t *testing.T) {
	result := StepFrontend(
		Frontend{Active: true, Mode: FrontendModeTitle, MenuActive: true, ItemOn: 0},
		FrontendInput{Select: true},
		FrontendConfig{
			EpisodeChoices: []int{1, 2, 3},
			OptionRows:     []int{0, 1, 2, 5, 7},
			MainMenuCount:  6,
			SkillMenuCount: 5,
			StatusTics:     35,
		},
	)

	if result.State.Mode != FrontendModeEpisode {
		t.Fatalf("mode=%v want episode", result.State.Mode)
	}
	if result.State.SelectedEpisode != 1 {
		t.Fatalf("selectedEpisode=%d want 1", result.State.SelectedEpisode)
	}
	if result.Sound != FrontendSoundConfirm {
		t.Fatalf("sound=%v want confirm", result.Sound)
	}
}

func TestStepFrontendReadThisSkipClosesOnLastPage(t *testing.T) {
	result := StepFrontend(
		Frontend{
			Active:           true,
			Mode:             FrontendModeReadThis,
			ReadThisPage:     1,
			ReadThisFromGame: true,
		},
		FrontendInput{Skip: true},
		FrontendConfig{ReadThisPageCount: 2},
	)

	if result.State.Active {
		t.Fatal("expected close from game to leave frontend inactive")
	}
	if result.State.Mode != FrontendModeNone {
		t.Fatalf("mode=%v want none", result.State.Mode)
	}
	if result.Sound != FrontendSoundBack {
		t.Fatalf("sound=%v want back", result.Sound)
	}
}

func TestAdvanceFrontendFrameExpiresStatusAndRequestsAttractAdvance(t *testing.T) {
	state, advance := AdvanceFrontendFrame(
		Frontend{
			Status:         "TEST",
			StatusTic:      1,
			AttractPageTic: 1,
		},
		8,
	)

	if state.Status != "" || state.StatusTic != 0 {
		t.Fatalf("status=%q tic=%d want empty/0", state.Status, state.StatusTic)
	}
	if !advance {
		t.Fatal("expected attract advance when page tic expires")
	}
}

func TestAdvanceAttractReturnsTitlePageAction(t *testing.T) {
	state, action, ok := AdvanceAttract(
		Frontend{AttractSeq: -1},
		[]string{"TITLEPIC", "DEMO1"},
		true,
		385,
		170,
		200,
	)

	if !ok {
		t.Fatal("expected attract step")
	}
	if action.Kind != AttractActionPage || action.Name != "TITLEPIC" || !action.PlayTitle {
		t.Fatalf("unexpected action: %+v", action)
	}
	if state.AttractPage != "TITLEPIC" || state.AttractPageTic != 385 {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestNewGameStartMapUsesEpisodeSelection(t *testing.T) {
	got := NewGameStartMap(mapdata.MapName("E1M1"), []int{1, 4}, 4, true)
	if got != "E4M1" {
		t.Fatalf("startMap=%q want E4M1", got)
	}
}

func TestStepFrontendOptionsMessagesChangesOnLeftRightAndSelect(t *testing.T) {
	cfg := FrontendConfig{OptionRows: []int{0, 1, 2, 3, 4, 5, 6}}
	for _, tc := range []struct {
		name  string
		input FrontendInput
	}{
		{name: "left", input: FrontendInput{Left: true}},
		{name: "right", input: FrontendInput{Right: true}},
		{name: "select", input: FrontendInput{Select: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := StepFrontend(
				Frontend{Active: true, Mode: FrontendModeOptions, OptionsOn: 0},
				tc.input,
				cfg,
			)
			if !result.ChangeMessages {
				t.Fatal("expected messages toggle request")
			}
		})
	}
}
