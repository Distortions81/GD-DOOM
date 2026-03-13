package sessionmusic

import (
	"testing"

	"gddoom/internal/sound"
)

func TestEffectiveOPLGainUsesImpSynthRatio(t *testing.T) {
	want := 3.5 * impSynthOPLGainRatio
	if got := effectiveOPLGain(sound.BackendImpSynth, 3.5); got != want {
		t.Fatalf("effectiveOPLGain(impsynth)=%.2f want %.2f", got, want)
	}
}

func TestEffectiveOPLGainUsesDefaultForAuto(t *testing.T) {
	want := 2.25
	if sound.DefaultBackend() == sound.BackendImpSynth {
		want *= impSynthOPLGainRatio
	} else if sound.DefaultBackend() == sound.BackendNuked {
		want *= nukedOPLGainRatio
	}
	if got := effectiveOPLGain(sound.BackendAuto, 2.25); got != want {
		t.Fatalf("effectiveOPLGain(auto)=%.2f want %.2f", got, want)
	}
}

func TestEffectiveOPLGainUsesNukedRatio(t *testing.T) {
	want := 3.5 * nukedOPLGainRatio
	if got := effectiveOPLGain(sound.BackendNuked, 3.5); got != want {
		t.Fatalf("effectiveOPLGain(nuked)=%.2f want %.2f", got, want)
	}
}
