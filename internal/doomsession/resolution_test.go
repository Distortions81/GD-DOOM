package doomsession

import "testing"

func TestDefaultCLIWindowSize(t *testing.T) {
	w, h := DefaultCLIWindowSize()
	if w != 320*5 || h != 200*5 {
		t.Fatalf("DefaultCLIWindowSize()=%dx%d want %dx%d", w, h, 320*5, 200*5)
	}
}
