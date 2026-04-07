package doomruntime

import (
	"testing"

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
	if len(g.chatHistory) != 1 {
		t.Fatalf("history count=%d want 1", len(g.chatHistory))
	}
	if got := g.chatHistory[0].Text; got != "P2: hello world" {
		t.Fatalf("history text=%q want %q", got, "P2: hello world")
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
