package doomruntime

import "testing"

func TestPerfOverlayTimingDisplaysHiddenByDefault(t *testing.T) {
	tic, host := perfOverlayTimingDisplays(false, "tic 123 | tps 35.00", 120, 240)
	if tic != "" || host != "" {
		t.Fatalf("perfOverlayTimingDisplays(false)=(%q,%q) want empty strings", tic, host)
	}
}

func TestPerfOverlayTimingDisplaysShownWhenEnabled(t *testing.T) {
	tic, host := perfOverlayTimingDisplays(true, "tic 123 | tps 35.00", 120, 240)
	if tic != "tic 123 | tps 35.00" {
		t.Fatalf("tic display=%q want original tic display", tic)
	}
	if host != "ebi 120.00 tps | fps 240.00" {
		t.Fatalf("host display=%q want formatted host stats", host)
	}
}
