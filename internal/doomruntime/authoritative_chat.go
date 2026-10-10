package doomruntime

import (
	"fmt"
	"time"

	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
)

type authorityChatEndpoint struct {
	client runtimecfg.AuthorityChatClient
}

func (e authorityChatEndpoint) SendRuntimeChat(message runtimecfg.ChatMessage) error {
	return e.client.SendChat(message.Text)
}

func (e authorityChatEndpoint) PollRuntimeChat() (runtimecfg.ChatMessage, bool, error) {
	event, ok, err := e.client.PollChat()
	if !ok || err != nil {
		return runtimecfg.ChatMessage{}, ok, err
	}
	name := fmt.Sprintf("%s (P%d)", event.Name, event.PlayerID)
	if event.PlayerID == 0 {
		name = event.Name + " (spectator)"
	}
	return runtimecfg.ChatMessage{Name: name, Text: event.Text}, true, nil
}

func (g *game) updateAuthoritativeChatAt(now time.Time) error {
	// Menu/background updates keep chat active even while player input is
	// neutral. Expiry follows real presentation time, not prediction replay.
	clock := &g.clientUpdate
	if !clock.chatStamp.IsZero() {
		elapsed := now.Sub(clock.chatStamp)
		if elapsed < 0 {
			elapsed = 0
		}
		clock.chatAccum += min(elapsed, 11*time.Second)
		const tic = time.Second / netgame.TickRate
		steps := min(int(clock.chatAccum/tic), chatHistoryTTL+1)
		clock.chatAccum %= tic
		for range steps {
			g.tickChatHistory()
		}
	}
	clock.chatStamp = now
	return g.pollChatMessages()
}
