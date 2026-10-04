package doomruntime

import (
	"image/color"
	"math"
	"strings"

	"gddoom/internal/render/hud"
	"gddoom/internal/render/levelmesh"
	"github.com/hajimehoshi/ebiten/v2"
)

// MessagePatches uses the main HUD's physical-pixel layout and WAD glyphs.
// Its slice remains valid until the next MessagePatches call.
func (c *NativeCampaign) MessagePatches(width, height int) []levelmesh.Patch {
	c.messagePatches = c.messagePatches[:0]
	g, sg := c.Game.g, c.session
	if g.useFlash <= 0 {
		return c.messagePatches
	}
	sg.nativePatches = &c.messagePatches
	defer func() { sg.nativePatches = nil }()
	hud.DrawHUDMessage(nil, hud.MessageInputs{
		ViewW: width, ViewH: height, SourcePort: g.hudUsesLogicalLayout(), HUDScale: g.hudScaleValue(),
		Message: g.useText, X: huMsgX, Y: huMsgY,
	}, func(_ *ebiten.Image, text string, x, y, sx, sy float64) {
		sg.appendNativeMenuText(text, x, y, sx, sy, 1)
	})
	return c.messagePatches
}

type NativeDeathOverlay struct {
	Tint    color.RGBA
	Patches []levelmesh.Patch
}

// DeathOverlay shares the main death prompts and dimming, including during
// demo playback. The host draws this after the HUD and before powerup flashes.
func (c *NativeCampaign) DeathOverlay(width, height int) NativeDeathOverlay {
	c.deathPatches = c.deathPatches[:0]
	var overlay NativeDeathOverlay
	g, sg := c.Game.g, c.session
	if !g.isDead {
		return overlay
	}
	sg.nativePatches = &c.deathPatches
	defer func() { sg.nativePatches = nil }()
	hud.DrawDeathOverlayWithRect(nil, hud.DeathOverlayInputs{ViewW: width, ViewH: height}, g.huTextWidth,
		func(_ *ebiten.Image, text string, x, y, sx, sy float64) {
			sg.appendNativeMenuText(text, x, y, sx, sy, 1)
		},
		func(_ *ebiten.Image, clr color.RGBA) { overlay.Tint = clr })
	overlay.Patches = c.deathPatches
	return overlay
}

type NativeRecordingOverlay struct {
	X, Y, Radius float32
	Color        color.RGBA
	Patches      []levelmesh.Patch
}

func (c *NativeCampaign) RecordingOverlay(width, height int) NativeRecordingOverlay {
	c.recordingPatches = c.recordingPatches[:0]
	var overlay NativeRecordingOverlay
	g, sg := c.Game.g, c.session
	if strings.TrimSpace(g.opts.RecordDemoPath) == "" {
		return overlay
	}
	sg.nativePatches = &c.recordingPatches
	defer func() { sg.nativePatches = nil }()
	hud.DrawRecordingIndicatorWithCircle(nil, width, height, g.huTextWidth,
		func(_ *ebiten.Image, text string, x, y, sx, sy float64) {
			sg.appendNativeMenuText(text, x, y, sx, sy, 1)
		},
		func(_ *ebiten.Image, x, y, radius float32, clr color.RGBA) {
			overlay.X, overlay.Y, overlay.Radius, overlay.Color = x, y, radius, clr
		})
	overlay.Patches = c.recordingPatches
	return overlay
}

// The native standalone P/Pause control uses the same M_PAUSE artwork as
// the main pause menu, fitted to its centered logical screen.
func (c *NativeCampaign) PausePatches(width, height int) []levelmesh.Patch {
	c.pausePatches = c.pausePatches[:0]
	g, sg := c.Game.g, c.session
	scale := math.Min(float64(width)/320, float64(height)/200)
	x, y := (float64(width)-320*scale)/2, (float64(height)-200*scale)/2
	if tex, ok := g.opts.MenuPatchBank["M_PAUSE"]; ok {
		c.pausePatches = append(c.pausePatches, levelmesh.Patch{
			Texture: nativeTexture(&tex), X: x + (126-float64(tex.OffsetX))*scale, Y: y + (4-float64(tex.OffsetY))*scale,
			W: float64(tex.Width) * scale, H: float64(tex.Height) * scale, Alpha: 1,
		})
	} else {
		sg.nativePatches = &c.pausePatches
		defer func() { sg.nativePatches = nil }()
		sg.appendNativeMenuText("PAUSED", (float64(width)-float64(g.huTextWidth("PAUSED"))*scale)/2, y+4*scale, scale, scale, 1)
	}
	return c.pausePatches
}
