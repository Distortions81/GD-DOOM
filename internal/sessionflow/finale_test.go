package sessionflow

import "testing"

func TestStartFinaleEpisodeOneStartsWithTextStage(t *testing.T) {
	state, ok := StartFinale("E1M8", false)
	if !ok {
		t.Fatal("StartFinale(E1M8)=false want true")
	}
	if state.Stage != FinaleStageText {
		t.Fatalf("stage=%d want %d", state.Stage, FinaleStageText)
	}
	if state.Flat != "FLOOR4_8" {
		t.Fatalf("flat=%q want FLOOR4_8", state.Flat)
	}
	if state.Screen != "CREDIT" {
		t.Fatalf("screen=%q want CREDIT", state.Screen)
	}
	if state.Text != e1Text {
		t.Fatal("unexpected E1 finale text")
	}
}

func TestTickFinaleTransitionsFromTextToPicture(t *testing.T) {
	state, ok := StartFinale("E1M8", false)
	if !ok {
		t.Fatal("StartFinale(E1M8)=false want true")
	}
	state.Tic = finaleTextTotalTics(state.Text)
	next, done := TickFinale(state, false)
	if done {
		t.Fatal("TickFinale() done=true want false on text->picture transition")
	}
	if next.Stage != FinaleStagePicture {
		t.Fatalf("stage=%d want %d", next.Stage, FinaleStagePicture)
	}
	if next.Tic != 0 {
		t.Fatalf("tic=%d want 0 after text->picture transition", next.Tic)
	}
}

func TestFinaleVisibleTextUsesDoomTiming(t *testing.T) {
	if got := FinaleVisibleText("ABC", finaleTextStartDelay-1); got != "" {
		t.Fatalf("visible text before delay=%q want empty", got)
	}
	if got := FinaleVisibleText("ABC", finaleTextStartDelay+FinaleTextSpeed); got != "A" {
		t.Fatalf("visible text after first interval=%q want A", got)
	}
	if got := FinaleVisibleText("ABC", finaleTextStartDelay+FinaleTextSpeed*4); got != "ABC" {
		t.Fatalf("visible text after full reveal=%q want ABC", got)
	}
}
