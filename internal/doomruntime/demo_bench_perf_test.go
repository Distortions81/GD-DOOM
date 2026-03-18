package doomruntime

import "testing"

func TestDemoBenchLowFrameNS(t *testing.T) {
	frameNS := []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	if got := demoBenchLowFrameNS(frameNS, 0.99); got != 100 {
		t.Fatalf("99th percentile=%d want 100", got)
	}
	if got := demoBenchLowFrameNS(frameNS, 0.50); got != 50 {
		t.Fatalf("50th percentile=%d want 50", got)
	}
	if got := demoBenchLowFrameNS(nil, 0.99); got != 0 {
		t.Fatalf("empty percentile=%d want 0", got)
	}
}

func TestDemoBenchFPSFromFrameNS(t *testing.T) {
	if got := demoBenchFPSFromFrameNS(20_000_000); got != 50 {
		t.Fatalf("fps=%v want 50", got)
	}
	if got := demoBenchFPSFromFrameNS(0); got != 0 {
		t.Fatalf("zero frame fps=%v want 0", got)
	}
}
