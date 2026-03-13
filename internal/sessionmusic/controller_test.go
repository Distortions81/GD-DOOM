package sessionmusic

import (
	"testing"

	"gddoom/internal/sound"
)

func TestEffectiveSynthGainUsesImpSynthRatio(t *testing.T) {
	want := 3.5 * impSynthGainRatio
	if got := effectiveSynthGain(sound.BackendImpSynth, 3.5); got != want {
		t.Fatalf("effectiveSynthGain(impsynth)=%.2f want %.2f", got, want)
	}
}

func TestEffectiveSynthGainUsesDefaultForAuto(t *testing.T) {
	want := 2.25
	if sound.DefaultBackend() == sound.BackendImpSynth {
		want *= impSynthGainRatio
	}
	if got := effectiveSynthGain(sound.BackendAuto, 2.25); got != want {
		t.Fatalf("effectiveSynthGain(auto)=%.2f want %.2f", got, want)
	}
}
