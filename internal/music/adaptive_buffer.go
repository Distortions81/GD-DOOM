package music

import "time"

const (
	adaptiveMusicMaxReserve     = 750 * time.Millisecond
	adaptiveMusicStablePeriod   = 10 * time.Second
	adaptiveMusicRecoveryStep   = time.Second
	adaptiveMusicNormalInterval = 12 * time.Millisecond
	adaptiveMusicFastInterval   = 4 * time.Millisecond
)

// adaptiveMusicBuffer keeps a little more synthesized music ready when servicing
// the stream becomes irregular. Its reserve survives track changes, but idle
// time between tracks must not count as a missed refill.
type adaptiveMusicBuffer struct {
	minimum     time.Duration
	lookahead   time.Duration
	lastService time.Time
	stableSince time.Time
	lastShrink  time.Time
	started     bool
	stable      bool
}

func (b *adaptiveMusicBuffer) start(now time.Time, minimum time.Duration) {
	if minimum <= 0 {
		minimum = 100 * time.Millisecond
	}
	if minimum > adaptiveMusicMaxReserve {
		minimum = adaptiveMusicMaxReserve
	}
	b.minimum = minimum
	if b.lookahead < minimum {
		b.lookahead = minimum
	}
	if b.lookahead > adaptiveMusicMaxReserve {
		b.lookahead = adaptiveMusicMaxReserve
	}
	b.lastService = now
	b.stableSince = time.Time{}
	b.lastShrink = now
	b.started = true
	b.stable = false
}

// observe uses wall-clock gaps, rather than game ticks or source-queue emptiness:
// the audio backend can drain that queue in large, harmless prefetch bursts.
// renderCost is the time spent synthesizing the most recent refill.
func (b *adaptiveMusicBuffer) observe(now time.Time, renderCost time.Duration) {
	if !b.started {
		b.start(now, 0)
	}
	gap := now.Sub(b.lastService)
	b.lastService = now
	// Besides bounding the reserve, clamp before arithmetic so suspended tabs
	// and unusual clock changes cannot overflow duration calculations.
	gap = min(max(gap, 0), adaptiveMusicMaxReserve)
	renderCost = min(max(renderCost, 0), adaptiveMusicMaxReserve)
	demand := min(3*gap+2*renderCost, adaptiveMusicMaxReserve)
	wanted := max(b.minimum, demand)
	if wanted > b.lookahead {
		b.lookahead = wanted
		b.stable = false
		b.lastShrink = now
		return
	}
	if b.lookahead <= b.minimum {
		b.stable = false
		return
	}
	// Require meaningful spare capacity before reducing it. A workload that
	// still needs the learned reserve must not make it oscillate every frame.
	if demand > b.minimum && demand > b.lookahead*2/3 {
		b.stable = false
		return
	}
	if !b.stable {
		b.stable = true
		b.stableSince = now
	}
	if now.Sub(b.stableSince) < adaptiveMusicStablePeriod || now.Sub(b.lastShrink) < adaptiveMusicRecoveryStep {
		return
	}
	b.lookahead = max(b.minimum, b.lookahead-b.lookahead/8)
	b.lastShrink = now
}

func (b *adaptiveMusicBuffer) reserve() time.Duration {
	return b.lookahead
}

func (b *adaptiveMusicBuffer) interval() time.Duration {
	if b.lookahead > b.minimum {
		return adaptiveMusicFastInterval
	}
	return adaptiveMusicNormalInterval
}
