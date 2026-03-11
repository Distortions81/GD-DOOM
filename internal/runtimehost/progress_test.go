package runtimehost

import "testing"

func TestHandleProgressUsesExpectedPriority(t *testing.T) {
	called := ""
	handled, err := HandleProgress(
		ProgressSignals{
			HasNewGame:    true,
			HasQuitPrompt: true,
			HasReadThis:   true,
			HasRestart:    true,
		},
		ProgressHandlers{
			OnNewGame: func() error {
				called = "newgame"
				return nil
			},
			OnQuitPrompt: func() error {
				called = "quit"
				return nil
			},
			OnReadThis: func() error {
				called = "readthis"
				return nil
			},
			OnRestart: func() error {
				called = "restart"
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("HandleProgress() error = %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true")
	}
	if called != "newgame" {
		t.Fatalf("called=%q want newgame", called)
	}
}
