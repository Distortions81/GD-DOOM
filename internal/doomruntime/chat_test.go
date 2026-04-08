package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
)

type testChatEndpoint struct {
	sent  []runtimecfg.ChatMessage
	queue []runtimecfg.ChatMessage
}

func (*testChatEndpoint) BroadcastTic(demo.Tic) error {
	return nil
}

func (*testChatEndpoint) PollTic() (demo.Tic, bool, error) {
	return demo.Tic{}, false, nil
}

func (e *testChatEndpoint) SendRuntimeChat(msg runtimecfg.ChatMessage) error {
	e.sent = append(e.sent, msg)
	return nil
}

func (e *testChatEndpoint) PollRuntimeChat() (runtimecfg.ChatMessage, bool, error) {
	if len(e.queue) == 0 {
		return runtimecfg.ChatMessage{}, false, nil
	}
	msg := e.queue[0]
	e.queue = e.queue[1:]
	return msg, true, nil
}

func TestHandleChatComposeInputSendsAndEchoes(t *testing.T) {
	endpoint := &testChatEndpoint{}
	g := &game{
		opts:      Options{LiveTicSink: endpoint},
		localSlot: 2,
		input: gameInputSnapshot{
			inputChars:      []rune("  hello world  "),
			justPressedKeys: map[ebiten.Key]struct{}{ebiten.KeyEnter: {}},
		},
	}
	g.chatComposeOpen = true

	g.handleChatComposeInput()

	if g.chatComposeOpen {
		t.Fatal("chat compose should close after send")
	}
	if len(endpoint.sent) != 1 {
		t.Fatalf("sent count=%d want 1", len(endpoint.sent))
	}
	if got := endpoint.sent[0].Name; got != "P2" {
		t.Fatalf("sent name=%q want P2", got)
	}
	if got := endpoint.sent[0].Text; got != "hello world" {
		t.Fatalf("sent text=%q want hello world", got)
	}
	if len(g.chatHistory) != 0 {
		t.Fatalf("history count=%d want 0", len(g.chatHistory))
	}
}

func TestHandleChatInputUsesConfiguredBinding(t *testing.T) {
	endpoint := &testChatEndpoint{}
	g := &game{
		opts: Options{
			LiveTicSink: endpoint,
			InputBindings: runtimecfg.NormalizeInputBindings(runtimecfg.InputBindings{
				Chat: runtimecfg.KeyBinding{"Y", ""},
			}),
		},
		input: gameInputSnapshot{
			justPressedKeys: map[ebiten.Key]struct{}{
				ebiten.KeyY: {},
			},
		},
	}
	if !g.handleChatInput() {
		t.Fatal("handleChatInput() = false, want true for configured chat binding")
	}
	if !g.chatComposeOpen {
		t.Fatal("expected chat compose to open")
	}
}

func TestPollChatMessagesAppendsHistory(t *testing.T) {
	endpoint := &testChatEndpoint{
		queue: []runtimecfg.ChatMessage{
			{Name: "P1", Text: "  first  "},
			{Name: "P3", Text: "second"},
		},
	}
	g := &game{
		opts: Options{LiveTicSource: endpoint},
	}

	if err := g.pollChatMessages(); err != nil {
		t.Fatalf("poll chat: %v", err)
	}

	if len(g.chatHistory) != 2 {
		t.Fatalf("history count=%d want 2", len(g.chatHistory))
	}
	if got := g.chatHistory[0].Text; got != "P1: first" {
		t.Fatalf("history[0]=%q want %q", got, "P1: first")
	}
	if got := g.chatHistory[1].Text; got != "P3: second" {
		t.Fatalf("history[1]=%q want %q", got, "P3: second")
	}
}

func TestAllowOutgoingChatRejectsDuplicate(t *testing.T) {
	g := &game{
		chatRecentSent: []string{"hello"},
	}
	if ok, reason := g.allowOutgoingChat("hello", time.Now()); ok || reason != "CHAT DUPLICATE" {
		t.Fatalf("allowOutgoingChat duplicate = (%t, %q) want (false, %q)", ok, reason, "CHAT DUPLICATE")
	}
}

func TestAllowOutgoingChatRejectsBurst(t *testing.T) {
	now := time.Now()
	g := &game{
		chatSentTimes: []time.Time{
			now.Add(-1 * time.Second),
			now.Add(-2 * time.Second),
			now.Add(-3 * time.Second),
			now.Add(-4 * time.Second),
		},
	}
	if ok, reason := g.allowOutgoingChat("fresh", now); ok || reason != "CHAT THROTTLED" {
		t.Fatalf("allowOutgoingChat burst = (%t, %q) want (false, %q)", ok, reason, "CHAT THROTTLED")
	}
}
