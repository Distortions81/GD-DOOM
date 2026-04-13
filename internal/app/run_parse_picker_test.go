package app

import "testing"

func TestPickerTextScaleForLayoutChoosesLargestFittingScale(t *testing.T) {
	game := &iwadPickerGame{
		stage: pickerStagePCSpeakerVariant,
		choices: []iwadChoice{
			{Path: "/tmp/doom2.wad", Label: "DOOM II"},
		},
	}
	screenW, screenH := 1280, 720
	got := game.pickerTextScaleForLayout(screenW, screenH)
	if got < 1 {
		t.Fatalf("pickerTextScaleForLayout(%d, %d) = %d want >= 1", screenW, screenH, got)
	}
	if !game.pickerTextFitsLayout(screenW, screenH, got) {
		t.Fatalf("chosen scale %d should fit", got)
	}
	if game.pickerTextFitsLayout(screenW, screenH, got+1) {
		t.Fatalf("chosen scale %d is not maximal", got)
	}
}

func TestPickerTextScaleForLayoutRespectsIWADListWidth(t *testing.T) {
	game := &iwadPickerGame{
		stage: pickerStageIWAD,
		choices: []iwadChoice{
			{Path: "/tmp/doom1.wad", Label: "THE ULTIMATE DOOM"},
			{Path: "/tmp/plutonia-experiment.wad", Label: "FINAL DOOM - THE PLUTONIA EXPERIMENT"},
		},
	}
	screenW, screenH := 960, 540
	got := game.pickerTextScaleForLayout(screenW, screenH)
	if got < 1 {
		t.Fatalf("pickerTextScaleForLayout(%d, %d) = %d want >= 1", screenW, screenH, got)
	}
	if !game.pickerTextFitsLayout(screenW, screenH, got) {
		t.Fatalf("chosen scale %d should fit", got)
	}
	if game.pickerTextFitsLayout(screenW, screenH, got+1) {
		t.Fatalf("chosen scale %d is not maximal for IWAD list", got)
	}
}
