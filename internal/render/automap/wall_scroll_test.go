package automap

import "testing"

func TestWallSpecialScrollXOffset(t *testing.T) {
	if got := wallSpecialScrollXOffset(0, 123); got != 0 {
		t.Fatalf("special 0 scroll=%v want=0", got)
	}
	if got := wallSpecialScrollXOffset(48, 0); got != 0 {
		t.Fatalf("special 48 at tic 0 scroll=%v want=0", got)
	}
	if got := wallSpecialScrollXOffset(48, 35); got != 35 {
		t.Fatalf("special 48 at tic 35 scroll=%v want=35", got)
	}
}
