package runtimehost

import (
	"errors"
	"testing"
)

func TestRunUpdateRequestsQuitPromptBeforeOtherPhases(t *testing.T) {
	called := false
	requested := false

	err := RunUpdate(Update{
		QuitPromptTriggered: func() bool { return true },
		RequestQuitPrompt:   func() { requested = true },
		TickTransition:      func() { called = true },
	})

	if err != nil {
		t.Fatalf("RunUpdate() error = %v", err)
	}
	if !requested {
		t.Fatal("expected quit prompt request")
	}
	if called {
		t.Fatal("expected later phases to be skipped")
	}
}

func TestRunUpdateHandlesRuntimeTerminationCallback(t *testing.T) {
	called := false
	err := RunUpdate(Update{
		UpdateRuntime: func() error {
			return errors.New("stop")
		},
		HandleRuntimeTermination: func() (bool, error) {
			called = true
			return true, nil
		},
	})

	if err != nil {
		t.Fatalf("RunUpdate() error = %v", err)
	}
	if !called {
		t.Fatal("expected termination handler to run")
	}
}

func TestRunUpdateFinaleRequestsTermination(t *testing.T) {
	err := RunUpdate(Update{
		FinaleActive: func() bool { return true },
		TickFinale:   func() bool { return true },
	})

	if !errors.Is(err, ErrTerminate) {
		t.Fatalf("RunUpdate() error = %v, want ErrTerminate", err)
	}
}
