package runtimecfg

import "testing"

func TestClampSourcePortWindowSizeForWASM(t *testing.T) {
	w, h := clampSourcePortWindowSizeForPlatform(2560, 1440, true)
	if w != 1280 || h != 720 {
		t.Fatalf("clamped window=%dx%d want 1280x720", w, h)
	}
}

func TestClampSourcePortWindowSizeForNativeLeavesSizeUnchanged(t *testing.T) {
	w, h := clampSourcePortWindowSizeForPlatform(2560, 1440, false)
	if w != 2560 || h != 1440 {
		t.Fatalf("native window=%dx%d want 2560x1440", w, h)
	}
}
