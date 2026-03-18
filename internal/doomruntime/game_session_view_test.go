package doomruntime

import "testing"

func TestSessionAcknowledgeLevelRestartClearsSignal(t *testing.T) {
	g := &game{levelRestartRequested: true}
	g.sessionAcknowledgeLevelRestart()
	if g.levelRestartRequested {
		t.Fatal("levelRestartRequested should be cleared")
	}
}
