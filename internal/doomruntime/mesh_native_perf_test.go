package doomruntime

import (
	"strings"
	"testing"
	"time"
)

func TestNativeRenderCountersUseSharedFrameAndTicFormatting(t *testing.T) {
	c := nativeSaveFixture(t)
	g := c.Game.g
	g.fpsStamp = time.Now().Add(-2 * time.Second)
	g.worldTic, g.worldTicSample = 70, 0
	c.RecordRenderFrame(2 * time.Millisecond)
	if g.fpsDisplay < .4 || g.fpsDisplay > .6 || g.ticRateDisplay < 34 || g.ticRateDisplay > 36 || g.renderMSAvg < 1.9 || g.renderMSAvg > 3 {
		t.Fatalf("native counters: fps=%v tps=%v render=%vms", g.fpsDisplay, g.ticRateDisplay, g.renderMSAvg)
	}
	if g.fpsDisplayText != formatFPSDisplay(g.fpsDisplay, g.renderMSAvg) || g.ticDisplayText != formatTicDisplay(70, g.ticRateDisplay) || !strings.HasPrefix(g.ticDisplayText, "tic 70 | tps ") {
		t.Fatal("native counter text differs from main")
	}
}
