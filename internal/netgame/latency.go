package netgame

import (
	"math"
	"time"
)

// Ping/Pong measure this connection's end-to-end delivery delay, including its
// writer queue. Nonces have no authority over world time or input deadlines.
type Ping struct{ Nonce uint64 }
type Pong struct{ Nonce uint64 }

const pingInterval = time.Second
const maximumRoundTrip = time.Second
const pingExpiry = 2 * time.Second

type latencyProbe struct {
	nonce     uint64
	pending   uint64
	sent      time.Time
	roundTrip time.Duration
}

func clampRoundTrip(sample time.Duration) time.Duration {
	if sample < 0 {
		return 0
	}
	if sample > maximumRoundTrip {
		return maximumRoundTrip
	}
	return sample
}

func (p *latencyProbe) begin(now time.Time) (Ping, bool) {
	if p.pending != 0 && now.Sub(p.sent) < pingExpiry {
		return Ping{}, false
	}
	if p.nonce == math.MaxUint64 {
		return Ping{}, false
	}
	p.nonce++
	p.pending, p.sent = p.nonce, now
	return Ping{Nonce: p.pending}, true
}

func (p *latencyProbe) receive(pong Pong, now time.Time) {
	if p.pending == 0 || pong.Nonce != p.pending {
		return
	}
	elapsed := now.Sub(p.sent)
	p.pending = 0
	if elapsed < 0 || elapsed > pingExpiry {
		return
	}
	sample := clampRoundTrip(elapsed)
	if p.roundTrip == 0 {
		p.roundTrip = sample
	} else {
		p.roundTrip = (p.roundTrip*7 + sample) / 8
	}
}

// RoundTripTime is a bounded EWMA initialized by the reliable handshake and
// updated by one nonce-bound probe at a time. It is safe on the render thread.
func (c *Client) RoundTripTime() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latency.roundTrip
}
