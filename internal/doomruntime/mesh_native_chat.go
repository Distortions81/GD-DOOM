package doomruntime

import (
	"gddoom/internal/render/levelmesh"
	"github.com/hajimehoshi/ebiten/v2"
)

type NativeChatInput struct {
	Open, Cancel, Backspace, Send bool
	Runes                         []rune
}

func (n *NativeMeshGame) ChatOpen() bool      { return n.g.chatComposeOpen }
func (n *NativeMeshGame) ChatAvailable() bool { return n.g.chatAvailable() }

// Reuse the main compose handler, including rune limits, normalization and
// duplicate/burst rejection. Only the input snapshot comes from the host.
func (n *NativeMeshGame) ChatInput(in NativeChatInput) bool {
	g := n.g
	if !g.chatAvailable() {
		return false
	}
	if !g.chatComposeOpen {
		if !in.Open {
			return false
		}
		g.openChatCompose()
		return true
	}
	old := g.input
	defer func() { g.input = old }()
	g.input.inputChars = in.Runes
	g.input.justPressedKeys = make(map[ebiten.Key]struct{})
	if in.Cancel {
		g.input.justPressedKeys[ebiten.KeyEscape] = struct{}{}
	}
	if in.Backspace {
		g.input.justPressedKeys[ebiten.KeyBackspace] = struct{}{}
	}
	if in.Send {
		g.input.justPressedKeys[ebiten.KeyEnter] = struct{}{}
	}
	g.handleChatComposeInput()
	return true
}

func (c *NativeCampaign) ChatPatches(width, height int) []levelmesh.Patch {
	c.chatPatches = c.chatPatches[:0]
	g, sg := c.Game.g, c.session
	w, h := g.viewW, g.viewH
	g.viewW, g.viewH = width, height
	sg.nativePatches = &c.chatPatches
	defer func() { g.viewW, g.viewH = w, h; sg.nativePatches = nil }()
	g.drawChatOverlayText(func(text string, x, y, sx, sy float64) { sg.appendNativeMenuText(text, x, y, sx, sy, 1) })
	return c.chatPatches
}
