package doomruntime

import (
	"time"

	"gddoom/internal/render/hud"
	"gddoom/internal/render/levelmesh"
	"github.com/hajimehoshi/ebiten/v2"
)

// RecordRenderFrame keeps the shared frame/tic counters independent of the
// window backend. Render duration excludes the native host's frame limiter.
func (c *NativeCampaign) RecordRenderFrame(duration time.Duration) {
	g := c.Game.g
	if c.DemoStatus().Active {
		g.demoBenchDraws++
	}
	g.finishPerfCounter(time.Now().Add(-duration))
	c.rememberDetailSettings()
}

func (c *NativeCampaign) PerfPatches(width, height int) []levelmesh.Patch {
	c.perfPatches = c.perfPatches[:0]
	g, sg := c.Game.g, c.session
	sg.nativePatches = &c.perfPatches
	defer func() { sg.nativePatches = nil }()
	bench := ""
	if g.demoBenchmarkActive() {
		bench = formatBenchDisplay(g.benchLow1MS, g.benchLow01MS)
	}
	hud.DrawPerfOverlay(nil, hud.PerfInputs{
		ViewW: width, ViewH: height, SourcePort: g.hudUsesLogicalLayout(), HUDScale: g.hudScaleValue(),
		FPSDisplay: g.fpsDisplayText, TicDisplay: g.ticDisplayText, BenchLine: bench,
	}, g.huTextWidth, func(_ *ebiten.Image, text string, x, y, sx, sy float64) {
		sg.appendNativeMenuText(text, x, y, sx, sy, 1)
	})
	return c.perfPatches
}
