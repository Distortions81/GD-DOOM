package sessionflow

import (
	"gddoom/internal/mapdata"
	"testing"
)

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

func TestCommercialFinaleSelectionAndHeldButtonTiming(t *testing.T) {
	for _, tc := range []struct {
		name         mapdata.MapName
		secret, want bool
	}{
		{"MAP06", false, true}, {"MAP11", false, true}, {"MAP20", false, true},
		{"MAP30", false, true}, {"MAP15", false, false}, {"MAP15", true, true},
		{"MAP31", false, false}, {"MAP31", true, true}, {"MAP07", false, false},
	} {
		state, ok := StartCommercialFinale(tc.name, tc.secret)
		if ok != tc.want {
			t.Fatalf("%s secret=%t finale=%t want=%t", tc.name, tc.secret, ok, tc.want)
		}
		if !ok {
			continue
		}
		if !state.Commercial || state.Text == "" || state.Flat == "" {
			t.Fatalf("incomplete finale: %+v", state)
		}
		for i := 0; i < 51; i++ {
			var done bool
			state, done = TickFinale(state, true)
			if done || state.Stage != FinaleStageText {
				t.Fatalf("%s transitioned before finalecount>50", tc.name)
			}
		}
		next, done := TickFinale(state, true)
		if tc.name == "MAP30" {
			if done || next.Stage != FinaleStageCast {
				t.Fatal("MAP30 must enter cast without loading a level")
			}
		} else if !done {
			t.Fatalf("%s did not queue worlddone on the 52nd held-button tic", tc.name)
		}
		state.Tic = 100000
		next, done = TickFinale(state, false)
		if done || next.Stage != FinaleStageText {
			t.Fatal("commercial text must wait indefinitely without a button")
		}
	}
}
