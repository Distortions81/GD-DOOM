package sessionmusic

import (
	"testing"

	"gddoom/internal/sound"
)

func TestEffectiveOPLGainBypassesPureGo(t *testing.T) {
	if got := effectiveOPLGain(sound.BackendPureGo, 3.5); got != 1.0 {
		t.Fatalf("effectiveOPLGain(purego)=%.2f want 1.0", got)
	}
}

func TestEffectiveOPLGainUsesDefaultForAuto(t *testing.T) {
	want := 2.25
	if sound.DefaultBackend() == sound.BackendPureGo {
		want = 1.0
	}
	if got := effectiveOPLGain(sound.BackendAuto, 2.25); got != want {
		t.Fatalf("effectiveOPLGain(auto)=%.2f want %.2f", got, want)
	}
}

func TestEffectiveOPLGainKeepsNukedGain(t *testing.T) {
	if got := effectiveOPLGain(sound.BackendNuked, 3.5); got != 3.5 {
		t.Fatalf("effectiveOPLGain(nuked)=%.2f want 3.5", got)
	}
}
