package doomruntime

import "testing"

func TestMouseLookTurnRawWithWidthIgnoresResolution(t *testing.T) {
	base := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW)
	if base >= 0 {
		t.Fatalf("base turn=%d want negative for +dx", base)
	}
	doubleW := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW*2)
	if doubleW >= 0 {
		t.Fatalf("double-width turn=%d want negative for +dx", doubleW)
	}
	halfW := mouseLookTurnRawWithWidth(10, 1.0, doomLogicalW/2)
	if halfW >= 0 {
		t.Fatalf("half-width turn=%d want negative for +dx", halfW)
	}
	if doubleW != base {
		t.Fatalf("double-width turn=%d want=%d", doubleW, base)
	}
	if halfW != base {
		t.Fatalf("half-width turn=%d want=%d", halfW, base)
	}
}

func TestMouseLookTurnRawWithWidthPreservesDirectionAndMinimumStep(t *testing.T) {
	if got := mouseLookTurnRawWithWidth(0, 1.0, doomLogicalW); got != 0 {
		t.Fatalf("dx=0 got=%d want=0", got)
	}
	if got := mouseLookTurnRawWithWidth(1, 0.0000001, doomLogicalW); got != -1 {
		t.Fatalf("+tiny dx got=%d want=-1", got)
	}
	if got := mouseLookTurnRawWithWidth(-1, 0.0000001, doomLogicalW); got != 1 {
		t.Fatalf("-tiny dx got=%d want=+1", got)
	}
}
