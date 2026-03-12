package doomruntime

import "testing"

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
