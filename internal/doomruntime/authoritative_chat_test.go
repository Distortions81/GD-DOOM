package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
)

type authorityChatTestClient struct {
	*fakeAuthorityClient
	chat     []netgame.ChatEvent
	sentChat []string
}

func (c *authorityChatTestClient) SendChat(text string) error {
	c.sentChat = append(c.sentChat, text)
	return nil
}
func (c *authorityChatTestClient) PollChat() (netgame.ChatEvent, bool, error) {
	if len(c.chat) == 0 {
		return netgame.ChatEvent{}, false, nil
	}
	event := c.chat[0]
	c.chat = c.chat[1:]
	return event, true, nil
}

func TestAuthorityChatUsesExistingUIAndBackgroundPump(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 3)
	c := &authorityChatTestClient{fakeAuthorityClient: connection, chat: []netgame.ChatEvent{{ID: 1, PlayerID: 2, Name: "Marine", Text: "hello"}}}
	g.opts.AuthorityClient = c
	if !g.chatAvailable() || !g.chatSendAvailable() {
		t.Fatal("authoritative chat was unavailable to existing UI")
	}
	if err := g.chatSink().SendRuntimeChat(runtimecfg.ChatMessage{Name: "forged local name", Text: "ready"}); err != nil {
		t.Fatal(err)
	}
	if len(c.sentChat) != 1 || c.sentChat[0] != "ready" {
		t.Fatal("chat adapter did not send text-only intent")
	}
	now := time.Unix(100, 0)
	if err := g.updateAuthoritativeClientAt(now, nil); err != nil {
		t.Fatal(err)
	}
	if len(g.chatHistory) != 1 || g.chatHistory[0].Text != "Marine (P2): hello" || connection.snapshotPolls == 0 {
		t.Fatal("menu/background update omitted chat or stopped snapshots")
	}
	if err := g.updateAuthoritativeClientAt(now.Add(11*time.Second), nil); err != nil {
		t.Fatal(err)
	}
	if len(g.chatHistory) != 0 {
		t.Fatal("chat history did not expire while awaiting baseline")
	}
}
