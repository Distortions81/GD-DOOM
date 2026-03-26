package platformcfg

import "testing"

func TestForcedWASMModeOverride(t *testing.T) {
	prev := ForcedWASMMode()
	SetForcedWASMMode(false)
	defer SetForcedWASMMode(prev)

	SetForcedWASMMode(true)
	if !ForcedWASMMode() {
		t.Fatal("ForcedWASMMode()=false want true")
	}
	if !IsWASMBuild() {
		t.Fatal("IsWASMBuild()=false want true when forced")
	}

	SetForcedWASMMode(false)
	if ForcedWASMMode() {
		t.Fatal("ForcedWASMMode()=true want false")
	}
}
