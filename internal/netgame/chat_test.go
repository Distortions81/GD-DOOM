package netgame

import (
	"bytes"
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestChatProtocolBoundsAndRejectsForgedSender(t *testing.T) {
	for _, message := range []any{ChatSay{Text: strings.Repeat("😀", MaxChatRunes)}, ChatEvent{ID: 8, PlayerID: 4, Name: "Marine", Text: "hello 世界"}} {
		frame := marshalProtocol(t, message)
		got, err := UnmarshalMessage(frame)
		if err != nil || !reflect.DeepEqual(got, message) {
			t.Fatalf("chat roundtrip: %T %v", got, err)
		}
	}
	for _, message := range []any{ChatSay{}, ChatSay{Text: strings.Repeat("界", MaxChatRunes+1)}, ChatSay{Text: "\xff"}, ChatSay{Text: "first\nsecond"}, ChatSay{Text: "   "}, ChatEvent{ID: 1, PlayerID: 5, Name: "a", Text: "b"}} {
		if _, err := MarshalMessage(message); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted invalid %T", message)
		}
	}
	frame := marshalProtocol(t, ChatEvent{ID: 1, PlayerID: 1, Name: "forged", Text: "forged"})
	if _, err := ReadClientMessage(bytes.NewReader(frame[:messageHeaderBytes])); !errors.Is(err, ErrProtocol) {
		t.Fatal("client could supply event identity")
	}
	frame = marshalProtocol(t, ChatSay{Text: "hello"})
	if _, err := UnmarshalMessage(protocolFrame(KindChatSay, append(frame[messageHeaderBytes:], 1))); !errors.Is(err, ErrProtocol) {
		t.Fatal("chat accepted trailing forged fields")
	}
}

func TestMatchChatAssignsIdentityRatesAndKeepsIDsAcrossMaps(t *testing.T) {
	m, w := newResumeTestMatch(t, 35)
	a, _, err := m.Join(Hello{Compatibility: "test-content", Name: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := joinTestMatch(t, m)
	now := time.Unix(100, 0)
	for i := uint64(1); i <= chatBurst; i++ {
		event, err := m.chat(a, ChatSay{Text: "  hello   team  "}, now)
		if err != nil || event.ID != i || event.PlayerID != 1 || event.Name != "Alice" || event.Text != "hello team" {
			t.Fatalf("server chat identity/order incorrect: %+v %v", event, err)
		}
	}
	if _, err := m.chat(a, ChatSay{Text: "burst"}, now); !errors.Is(err, ErrChatThrottled) {
		t.Fatal("unbounded chat burst")
	}
	if _, err := m.chat(a, ChatSay{Text: "early"}, now.Add(chatTokenPeriod-time.Nanosecond)); !errors.Is(err, ErrChatThrottled) {
		t.Fatal("chat exceeded two messages/second refill")
	}
	if _, err := m.chat(b, ChatSay{Text: "independent"}, now); err != nil {
		t.Fatal("one sender throttled another")
	}
	if event, err := m.chat(a, ChatSay{Text: "refilled"}, now.Add(chatTokenPeriod)); err != nil || event.ID != 6 {
		t.Fatal("chat refill failed")
	}
	if w.tick != 0 || w.steps != 0 || m.players[a].lastActivity != 0 {
		t.Fatal("chat advanced gameplay or renewed input activity")
	}
	oldID := m.chatID
	m.completed = true
	if _, err := m.ResetEpoch(100, "next-content"); err != nil {
		t.Fatal(err)
	}
	event, err := m.chat(b, ChatSay{Text: "new map"}, now)
	if err != nil || event.ID != oldID+1 {
		t.Fatal("map reset reused chat event IDs")
	}
	m.Suspend(a)
	if _, err := m.chat(a, ChatSay{Text: "stale socket"}, now.Add(time.Second)); !errors.Is(err, ErrUnknownConnection) {
		t.Fatal("stale socket kept chat identity")
	}
}

func TestClientChatDeduplicatesAndBoundsQueues(t *testing.T) {
	c := &Client{chats: make(chan ChatEvent, 2), chatOutgoing: make(chan ChatSay, 1), done: make(chan struct{})}
	event := ChatEvent{ID: 2, PlayerID: 1, Name: "Marine", Text: "hello"}
	if err := c.receiveChat(event); err != nil {
		t.Fatal(err)
	}
	if err := c.receiveChat(event); err != nil {
		t.Fatal(err)
	}
	event.ID = 1
	if err := c.receiveChat(event); err != nil {
		t.Fatal(err)
	}
	if len(c.chats) != 1 {
		t.Fatal("duplicate/out-of-order chat was displayed twice")
	}
	if _, ok, err := c.PollChat(); !ok || err != nil {
		t.Fatal("chat could not drain without snapshots")
	}
	if err := c.SendChat("one"); err != nil {
		t.Fatal(err)
	}
	if err := c.SendChat("two"); !errors.Is(err, ErrChatQueueFull) {
		t.Fatal("outgoing chat grew without bound")
	}
	for id := uint64(3); id <= 4; id++ {
		event.ID = id
		if err := c.receiveChat(event); err != nil {
			t.Fatal(err)
		}
	}
	event.ID = 5
	if err := c.receiveChat(event); !errors.Is(err, ErrChatQueueFull) {
		t.Fatal("incoming reliable chat overflow was silent")
	}
}

func TestServerChatRelaysEchoAndIdentityToBothClients(t *testing.T) {
	m, _ := newTestMatch(t)
	m.config.DisconnectTicks = 350
	s, _ := NewServer(m)
	listener := newPipeListener()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})
	clients := make([]*Client, 0, 2)
	for _, name := range []string{"Alice", "Bob"} {
		c, err := Connect(ctx, &streamTransport{listener.dial(t)}, Hello{Compatibility: "test-content", Name: name})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		clients = append(clients, c)
	}
	if err := clients[1].SendChat("hello from second slot"); err != nil {
		t.Fatal(err)
	}
	for _, c := range clients {
		deadline := time.Now().Add(time.Second)
		for {
			event, ok, err := c.PollChat()
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				if event.ID != 1 || event.PlayerID != 2 || event.Name != "Bob" || event.Text != "hello from second slot" {
					t.Fatal("relay trusted incorrect identity")
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("reliable chat was not delivered")
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func TestClientLeaveSendsExplicitDisconnect(t *testing.T) {
	seen := make(chan any, 1)
	c, _ := pipeClient(t, func(remote net.Conn) { message, _ := ReadMessage(remote); seen <- message })
	if err := c.Leave(); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-seen:
		if _, ok := message.(Disconnect); !ok {
			t.Fatalf("explicit leave sent %T", message)
		}
	case <-time.After(time.Second):
		t.Fatal("leave was never sent")
	}
}
