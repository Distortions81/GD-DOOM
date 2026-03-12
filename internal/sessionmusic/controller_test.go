package sessionmusic

import (
	"testing"

	"gddoom/internal/sound"
)

func TestEffectiveOPLGainUsesPureGoRatio(t *testing.T) {
	want := 3.5 * pureGoOPLGainRatio
	if got := effectiveOPLGain(sound.BackendPureGo, 3.5); got != want {
		t.Fatalf("effectiveOPLGain(purego)=%.2f want %.2f", got, want)
	}
}

func TestEffectiveOPLGainUsesDefaultForAuto(t *testing.T) {
	want := 2.25
	if sound.DefaultBackend() == sound.BackendPureGo {
		want *= pureGoOPLGainRatio
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
