package doomruntime

import (
	"testing"
	"time"
)

func TestConsumeSimTicks_SupportsSlowAndFastRates(t *testing.T) {
	g := &game{simTickScale: 0.5}

	if got := g.consumeSimTicks(); got != 0 {
		t.Fatalf("first half-rate frame: got ticks=%d want=0", got)
	}
	if got := g.consumeSimTicks(); got != 1 {
		t.Fatalf("second half-rate frame: got ticks=%d want=1", got)
	}

	g.simTickScale = 2.0
	g.simTickAccum = 0
	if got := g.consumeSimTicks(); got != 2 {
		t.Fatalf("double-rate frame: got ticks=%d want=2", got)
	}
}

func TestSetSimTickScale_Clamps(t *testing.T) {
	g := &game{hudMessagesEnabled: false}

	g.setSimTickScale(0.1)
	if g.simTickScale != 0.1 {
		t.Fatalf("min clamp: got=%0.2f want=0.10", g.simTickScale)
	}

	g.setSimTickScale(4.9)
	if g.simTickScale != 4.9 {
		t.Fatalf("in-range preserve: got=%0.2f want=4.90", g.simTickScale)
	}

	g.setSimTickScale(9.0)
	if g.simTickScale != 8.0 {
		t.Fatalf("max clamp: got=%0.2f want=8.00", g.simTickScale)
	}
}

func TestInterpAlpha_UsesMeasuredSimInterval(t *testing.T) {
	g := &game{
		lastUpdate:      time.Now().Add(-25 * time.Millisecond),
		lastSimInterval: 50 * time.Millisecond,
		simTickScale:    1.0,
	}

	if got := g.interpAlpha(); got < 0.45 || got > 0.55 {
		t.Fatalf("measured interval alpha: got=%0.3f want about 0.5", got)
	}
}

func TestInterpAlpha_FallsBackToConfiguredTickRate(t *testing.T) {
	g := &game{
		lastUpdate:   time.Now().Add(-14 * time.Millisecond),
		simTickScale: 2.0,
	}

	if got := g.interpAlpha(); got < 0.9 || got > 1.0 {
		t.Fatalf("fallback alpha: got=%0.3f want close to 1", got)
	}
}
