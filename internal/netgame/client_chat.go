package netgame

import (
	"net"
	"time"
)

// SendChat queues one bounded reliable message. Identity is always supplied by
// the server; callers cannot choose the name or slot attached to a chat line.
func (c *Client) SendChat(text string) error {
	say := ChatSay{Text: text}
	if _, err := validateMessage(say); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
		return c.err
	default:
	}
	select {
	case c.chatOutgoing <- say:
		return nil
	default:
		return ErrChatQueueFull
	}
}

// PollChat drains chat independently of snapshot/menu state. The snapshot
// stream reports session termination after delivering its final world state.
func (c *Client) PollChat() (ChatEvent, bool, error) {
	select {
	case event := <-c.chats:
		return event, true, nil
	default:
		return ChatEvent{}, false, nil
	}
}

func (c *Client) receiveChat(event ChatEvent) error {
	if _, err := validateMessage(event); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if event.ID <= c.lastChatID {
		return nil
	}
	select {
	case c.chats <- event:
		c.lastChatID = event.ID
		return nil
	default:
		return ErrChatQueueFull
	}
}

type clientLeaveRequest struct{ done chan error }

// Leave is an explicit user quit. It gives the reliable writer a short chance
// to release the player's slot immediately, then closes the transport. Close
// remains abrupt for internal cleanup after loss or connection replacement.
func (c *Client) Leave() error {
	request := clientLeaveRequest{done: make(chan error, 1)}
	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	select {
	case c.leaving <- request:
	case <-c.done:
		return c.Close()
	case <-timer.C:
		return c.Close()
	}
	select {
	case <-request.done:
	case <-c.done:
	case <-timer.C:
	}
	return c.Close()
}

func (c *Client) writeLeave(request clientLeaveRequest) {
	err := c.transport.WriteMessage(Disconnect{Reason: "Player left"})
	request.done <- err
	c.fail(net.ErrClosed)
}
