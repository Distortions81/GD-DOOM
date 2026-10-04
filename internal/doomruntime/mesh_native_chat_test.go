package doomruntime

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/runtimecfg"
)

func TestNativeChatComposeOwnsInputAndUsesSharedRules(t *testing.T) {
	c := nativeSaveFixture(t)
	endpoint := &testChatEndpoint{}
	c.session.opts.LiveTicSink, c.Game.g.opts.LiveTicSink = endpoint, endpoint
	n := c.Game
	if !n.ChatInput(NativeChatInput{Open: true, Runes: []rune("t")}) || !n.ChatOpen() || len(n.g.chatCompose) != 0 {
		t.Fatal("opening chat echoed the binding key")
	}
	n.ChatInput(NativeChatInput{Runes: []rune("  hello  世界")})
	n.ChatInput(NativeChatInput{Backspace: true})
	if string(n.g.chatCompose) != "  hello  世" {
		t.Fatal("backspace did not remove one rune")
	}
	data, err := c.CaptureKeyframe()
	if err != nil {
		t.Fatal(err)
	}
	menuRNG, playRNG := doomrand.State()
	n.Tick(NativeMeshInput{Forward: 1, Side: 1, Turn: 1, YawDelta: .5, Fire: true, Use: true, WeaponSlot: 2})
	want := captureLiveRuntimeRoundTripState(n.g)
	if err := c.LoadKeyframe(data); err != nil {
		t.Fatal(err)
	}
	doomrand.SetState(menuRNG, playRNG)
	c.Game.Tick(NativeMeshInput{})
	if !reflect.DeepEqual(want, captureLiveRuntimeRoundTripState(c.Game.g)) {
		t.Fatal("compose did not neutralize gameplay input")
	}
	// The original adapter still owns its compose state across the comparison.
	n.ChatInput(NativeChatInput{Send: true})
	if n.ChatOpen() || len(endpoint.sent) != 1 || endpoint.sent[0].Text != "hello 世" || len(n.g.chatHistory) != 0 {
		t.Fatal("sending did not normalize text or waited incorrectly for relay echo")
	}
	n.ChatInput(NativeChatInput{Open: true})
	n.ChatInput(NativeChatInput{Runes: []rune("hello 世"), Send: true})
	if len(endpoint.sent) != 1 {
		t.Fatal("duplicate message bypassed the main chat handler")
	}
	n.ChatInput(NativeChatInput{Open: true})
	n.ChatInput(NativeChatInput{Runes: []rune(strings.Repeat("界", 170))})
	if len(n.g.chatCompose) != chatComposeMaxRunes {
		t.Fatal("native compose exceeded the main rune limit")
	}
	n.ChatInput(NativeChatInput{Cancel: true})
	if n.ChatOpen() || len(endpoint.sent) != 1 {
		t.Fatal("Escape sent text")
	}
}

func TestNativeChatPollsWhileIdleAndDrawsPhysicalPixelLayout(t *testing.T) {
	c := nativeSaveFixture(t)
	endpoint := &testChatEndpoint{queue: []runtimecfg.ChatMessage{{Name: "P1", Text: "hello there"}}}
	c.session.opts.LiveTicSink, c.Game.g.opts.LiveTicSink = endpoint, endpoint
	before := c.Game.g.worldTic
	if err := c.PollNetwork(); err != nil {
		t.Fatal(err)
	}
	if c.Game.g.worldTic != before || len(c.Game.g.chatHistory) != 1 {
		t.Fatal("idle chat polling advanced gameplay or omitted the relay echo")
	}
	font := map[rune]WallTexture{}
	for ch := rune(33); ch <= 95; ch++ {
		font[ch] = WallTexture{Width: 5, Height: 7, OffsetX: 1, OffsetY: 2, RGBA: make([]byte, 5*7*4)}
	}
	c.Game.g.opts.MessageFontBank = font
	c.Game.ChatInput(NativeChatInput{Open: true})
	c.Game.ChatInput(NativeChatInput{Runes: []rune(strings.Repeat("hello ", 15))})
	for _, size := range [][2]int{{320, 168}, {640, 336}, {1280, 672}} {
		g := c.Game.g
		oldW, oldH := g.viewW, g.viewH
		actual := slices.Clone(c.ChatPatches(size[0], size[1]))
		if g.viewW != oldW || g.viewH != oldH || c.session.nativePatches != nil {
			t.Fatal("chat draw leaked borrowed viewport or patch state")
		}
		var expected []levelmesh.Patch
		g.viewW, g.viewH = size[0], size[1]
		g.drawChatOverlayText(func(text string, x, y, sx, sy float64) {
			px := x
			for _, ch := range strings.ToUpper(text) {
				p, ok := font[ch]
				if !ok || ch == ' ' {
					px += 4
					continue
				}
				expected = append(expected, levelmesh.Patch{Texture: nativeTexture(&p), X: px - 1, Y: y - 2, W: 5, H: 7, Alpha: 1})
				px += 5
			}
		})
		g.viewW, g.viewH = oldW, oldH
		if len(actual) == 0 || !reflect.DeepEqual(actual, expected) {
			t.Fatalf("size=%v: chat glyphs differ from main layout", size)
		}
	}
	for range chatHistoryTTL {
		c.Game.g.tickChatHistory()
	}
	if len(c.Game.g.chatHistory) != 0 {
		t.Fatal("chat history failed to expire")
	}
}
