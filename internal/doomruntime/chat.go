package doomruntime

import (
	"fmt"
	"strings"

	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	chatComposeMaxRunes   = 160
	chatHistoryMaxEntries = 6
	chatHistoryTTL        = 35 * 10
	chatPollPerUpdateMax  = 8
	chatMarginX           = 6
	chatMarginY           = 6
	chatLineAdvance       = 10
	chatWrapPadding       = 8
)

type chatHistoryEntry struct {
	Text string
	Tics int
}

func (g *game) chatSource() runtimecfg.LiveChatSource {
	if g == nil {
		return nil
	}
	if src, ok := g.opts.LiveTicSource.(runtimecfg.LiveChatSource); ok && src != nil {
		return src
	}
	if src, ok := g.opts.LiveTicSink.(runtimecfg.LiveChatSource); ok && src != nil {
		return src
	}
	return nil
}

func (g *game) chatSink() runtimecfg.LiveChatSink {
	if g == nil {
		return nil
	}
	if sink, ok := g.opts.LiveTicSink.(runtimecfg.LiveChatSink); ok && sink != nil {
		return sink
	}
	if sink, ok := g.opts.LiveTicSource.(runtimecfg.LiveChatSink); ok && sink != nil {
		return sink
	}
	return nil
}

func (g *game) chatAvailable() bool {
	return g != nil && (g.chatSource() != nil || g.chatSink() != nil)
}

func (g *game) chatSendAvailable() bool {
	return g != nil && g.chatSink() != nil
}

func (g *game) chatLocalName() string {
	if g == nil {
		return "PLAYER"
	}
	if g.localSlot > 0 {
		return fmt.Sprintf("P%d", g.localSlot)
	}
	return "PLAYER"
}

func normalizeChatText(text string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

func (g *game) openChatCompose() {
	if g == nil || !g.chatSendAvailable() {
		return
	}
	g.chatComposeOpen = true
	g.chatCompose = g.chatCompose[:0]
	g.typedCheatBuffer = ""
	g.pendingUse = false
	g.input.mouseTurnRawAccum = 0
}

func (g *game) closeChatCompose() {
	if g == nil {
		return
	}
	g.chatComposeOpen = false
	g.chatCompose = g.chatCompose[:0]
	g.typedCheatBuffer = ""
	g.input.mouseTurnRawAccum = 0
}

func (g *game) handleChatInput() bool {
	if g == nil || !g.chatAvailable() {
		return false
	}
	if g.chatComposeOpen {
		g.handleChatComposeInput()
		return true
	}
	if g.chatSendAvailable() && g.keyJustPressed(ebiten.KeyT) {
		g.openChatCompose()
		return true
	}
	return false
}

func (g *game) handleChatComposeInput() {
	if g == nil {
		return
	}
	if g.keyJustPressed(ebiten.KeyEscape) {
		g.closeChatCompose()
		return
	}
	if g.keyJustPressed(ebiten.KeyBackspace) && len(g.chatCompose) > 0 {
		g.chatCompose = g.chatCompose[:len(g.chatCompose)-1]
	}
	for _, ch := range g.chatTypedRunes() {
		if len(g.chatCompose) >= chatComposeMaxRunes {
			break
		}
		g.chatCompose = append(g.chatCompose, ch)
	}
	if !g.keyJustPressed(ebiten.KeyEnter) && !g.keyJustPressed(ebiten.KeyKPEnter) {
		return
	}
	text := normalizeChatText(string(g.chatCompose))
	g.closeChatCompose()
	if text == "" {
		return
	}
	msg := runtimecfg.ChatMessage{
		Name: g.chatLocalName(),
		Text: text,
	}
	if err := g.chatSink().SendRuntimeChat(msg); err != nil {
		g.setHUDMessage(strings.ToUpper(err.Error()), 70)
		return
	}
	g.appendChatHistory(msg.Name, msg.Text)
}

func (g *game) pollChatMessages() error {
	src := g.chatSource()
	if g == nil || src == nil {
		return nil
	}
	for i := 0; i < chatPollPerUpdateMax; i++ {
		msg, ok, err := src.PollRuntimeChat()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		g.appendChatHistory(msg.Name, msg.Text)
	}
	return nil
}

func (g *game) appendChatHistory(name, text string) {
	if g == nil {
		return
	}
	text = normalizeChatText(text)
	if text == "" {
		return
	}
	label := text
	if strings.TrimSpace(name) != "" {
		label = fmt.Sprintf("%s: %s", name, text)
	}
	g.chatHistory = append(g.chatHistory, chatHistoryEntry{
		Text: label,
		Tics: chatHistoryTTL,
	})
	if len(g.chatHistory) > chatHistoryMaxEntries {
		g.chatHistory = append([]chatHistoryEntry(nil), g.chatHistory[len(g.chatHistory)-chatHistoryMaxEntries:]...)
	}
}

func (g *game) tickChatHistory() {
	if g == nil || len(g.chatHistory) == 0 {
		return
	}
	keep := g.chatHistory[:0]
	for _, entry := range g.chatHistory {
		entry.Tics--
		if entry.Tics > 0 {
			keep = append(keep, entry)
		}
	}
	g.chatHistory = keep
}

func (g *game) drawChatOverlay(screen *ebiten.Image) {
	if g == nil || screen == nil || (!g.chatComposeOpen && len(g.chatHistory) == 0) {
		return
	}
	maxWidth := max(80, g.viewW-chatMarginX*2-chatWrapPadding)
	lines := make([]string, 0, len(g.chatHistory))
	for _, entry := range g.chatHistory {
		lines = append(lines, g.wrapChatText(entry.Text, maxWidth, "", "  ")...)
	}
	y := float64(chatMarginY)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		w := g.huTextWidth(line)
		x := float64(g.viewW - chatMarginX - w)
		if x < 0 {
			x = 0
		}
		g.drawHUTextAt(screen, line, x, y, 1, 1)
		y += chatLineAdvance
	}
	if g.chatComposeOpen {
		promptWidth := max(80, min(g.viewW-chatMarginX*2-chatWrapPadding, g.viewW/2))
		promptLines := g.wrapChatText("SAY: "+string(g.chatCompose)+"_", promptWidth, "", "     ")
		y := float64(g.viewH - chatMarginY - chatLineAdvance*(len(promptLines)))
		if y < 0 {
			y = 0
		}
		for _, line := range promptLines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			g.drawHUTextAt(screen, line, float64(chatMarginX), y, 1, 1)
			y += chatLineAdvance
		}
	}
}

func (g *game) wrapChatText(text string, maxWidth int, firstPrefix, nextPrefix string) []string {
	if g == nil {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	prefix := firstPrefix
	line := prefix
	usedPrefix := prefix
	lines := make([]string, 0, 4)
	flush := func(force bool) {
		trimmed := strings.TrimRight(line, " ")
		if !force && strings.TrimSpace(trimmed) == strings.TrimSpace(usedPrefix) {
			return
		}
		lines = append(lines, trimmed)
		prefix = nextPrefix
		usedPrefix = prefix
		line = prefix
	}
	for _, ch := range text {
		if ch == '\n' || ch == '\r' {
			flush(true)
			continue
		}
		next := line + string(ch)
		if g.huTextWidth(next) > maxWidth && strings.TrimSpace(line) != strings.TrimSpace(usedPrefix) {
			flush(true)
			if ch == ' ' {
				continue
			}
			next = line + string(ch)
		}
		line = next
	}
	flush(false)
	if len(lines) == 0 {
		return []string{strings.TrimSpace(text)}
	}
	return lines
}

func (g *game) chatTypedRunes() []rune {
	if g == nil {
		return nil
	}
	if len(g.input.inputChars) > 0 {
		out := make([]rune, 0, len(g.input.inputChars))
		for _, ch := range g.input.inputChars {
			if ch >= 0x20 && ch != 0x7f {
				out = append(out, ch)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	keys := []ebiten.Key{
		ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE, ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
		ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO, ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
		ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY, ebiten.KeyZ,
		ebiten.Key0, ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4, ebiten.Key5, ebiten.Key6, ebiten.Key7, ebiten.Key8, ebiten.Key9,
		ebiten.KeySpace, ebiten.KeyMinus, ebiten.KeyEqual, ebiten.KeyComma, ebiten.KeyPeriod, ebiten.KeySlash, ebiten.KeySemicolon,
		ebiten.KeyQuote, ebiten.KeyBracketLeft, ebiten.KeyBracketRight, ebiten.KeyBackslash, ebiten.KeyBackquote,
	}
	shifted := g.keyHeld(ebiten.KeyShiftLeft) || g.keyHeld(ebiten.KeyShiftRight)
	out := make([]rune, 0, 4)
	for _, key := range keys {
		if !g.chatKeyShouldEmit(key) {
			continue
		}
		if ch, ok := chatKeyRune(key, shifted); ok {
			out = append(out, ch)
		}
	}
	return out
}

func (g *game) chatKeyShouldEmit(key ebiten.Key) bool {
	return g != nil && g.keyJustPressed(key)
}

func chatKeyRune(key ebiten.Key, shifted bool) (rune, bool) {
	switch key {
	case ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE, ebiten.KeyF, ebiten.KeyG, ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ,
		ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN, ebiten.KeyO, ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT,
		ebiten.KeyU, ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY, ebiten.KeyZ:
		base := rune('a' + (key - ebiten.KeyA))
		if shifted {
			base = rune('A' + (key - ebiten.KeyA))
		}
		return base, true
	case ebiten.Key0:
		if shifted {
			return ')', true
		}
		return '0', true
	case ebiten.Key1:
		if shifted {
			return '!', true
		}
		return '1', true
	case ebiten.Key2:
		if shifted {
			return '@', true
		}
		return '2', true
	case ebiten.Key3:
		if shifted {
			return '#', true
		}
		return '3', true
	case ebiten.Key4:
		if shifted {
			return '$', true
		}
		return '4', true
	case ebiten.Key5:
		if shifted {
			return '%', true
		}
		return '5', true
	case ebiten.Key6:
		if shifted {
			return '^', true
		}
		return '6', true
	case ebiten.Key7:
		if shifted {
			return '&', true
		}
		return '7', true
	case ebiten.Key8:
		if shifted {
			return '*', true
		}
		return '8', true
	case ebiten.Key9:
		if shifted {
			return '(', true
		}
		return '9', true
	case ebiten.KeySpace:
		return ' ', true
	case ebiten.KeyMinus:
		if shifted {
			return '_', true
		}
		return '-', true
	case ebiten.KeyEqual:
		if shifted {
			return '+', true
		}
		return '=', true
	case ebiten.KeyComma:
		if shifted {
			return '<', true
		}
		return ',', true
	case ebiten.KeyPeriod:
		if shifted {
			return '>', true
		}
		return '.', true
	case ebiten.KeySlash:
		if shifted {
			return '?', true
		}
		return '/', true
	case ebiten.KeySemicolon:
		if shifted {
			return ':', true
		}
		return ';', true
	case ebiten.KeyQuote:
		if shifted {
			return '"', true
		}
		return '\'', true
	case ebiten.KeyBracketLeft:
		if shifted {
			return '{', true
		}
		return '[', true
	case ebiten.KeyBracketRight:
		if shifted {
			return '}', true
		}
		return ']', true
	case ebiten.KeyBackslash:
		if shifted {
			return '|', true
		}
		return '\\', true
	case ebiten.KeyBackquote:
		if shifted {
			return '~', true
		}
		return '`', true
	default:
		return 0, false
	}
}
