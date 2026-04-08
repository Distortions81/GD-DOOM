package runtimecfg

import "testing"

func TestDefaultInputBindingsVoiceUsesCapsLock(t *testing.T) {
	got := DefaultInputBindings()
	if got.Voice[0] != "CAPSLOCK" || got.Voice[1] != "" {
		t.Fatalf("Voice=%v want [CAPSLOCK \"\"]", got.Voice)
	}
}

func TestNormalizeInputBindingsVoiceFallsBackToDefault(t *testing.T) {
	got := NormalizeInputBindings(InputBindings{})
	if got.Voice[0] != "CAPSLOCK" || got.Voice[1] != "" {
		t.Fatalf("Voice=%v want [CAPSLOCK \"\"]", got.Voice)
	}
}
