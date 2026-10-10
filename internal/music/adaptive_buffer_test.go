package music

import (
	"testing"
	"time"
)

func TestAdaptiveMusicBufferNormalBatchedUpdates(t *testing.T) {
	var b adaptiveMusicBuffer
	now := time.Unix(1, 0)
	const minimum = 93 * time.Millisecond
	b.start(now, minimum)
	// Browser frames often deliver multiple 140 Hz host updates together.
	for i := 0; i < 600; i++ {
		now = now.Add(time.Second / 60)
		b.observe(now, 2*time.Millisecond)
		b.observe(now, 0)
		b.observe(now, 0)
	}
	if got := b.reserve(); got != minimum {
		t.Fatalf("regular browser updates grew reserve to %v, want %v", got, minimum)
	}
	if got := b.interval(); got != 12*time.Millisecond {
		t.Fatalf("normal refill interval = %v", got)
	}
}

func TestAdaptiveMusicBufferGrowsForLateRefillAndRenderCost(t *testing.T) {
	var b adaptiveMusicBuffer
	now := time.Unix(1, 0)
	b.start(now, 93*time.Millisecond)
	b.observe(now.Add(100*time.Millisecond), 15*time.Millisecond)
	if got, want := b.reserve(), 330*time.Millisecond; got != want {
		t.Fatalf("reserve = %v, want %v", got, want)
	}
	if got := b.interval(); got != 4*time.Millisecond {
		t.Fatalf("stressed refill interval = %v", got)
	}
}

func TestAdaptiveMusicBufferCapsLongSuspension(t *testing.T) {
	var b adaptiveMusicBuffer
	now := time.Unix(1, 0)
	b.start(now, 93*time.Millisecond)
	b.observe(now.Add(12*time.Hour), 12*time.Hour)
	if got := b.reserve(); got != 750*time.Millisecond {
		t.Fatalf("suspended reserve = %v, want 750ms", got)
	}
}

func TestAdaptiveMusicBufferShrinksOnlyAfterStableRecovery(t *testing.T) {
	var b adaptiveMusicBuffer
	now := time.Unix(1, 0)
	const minimum = 93 * time.Millisecond
	b.start(now, minimum)
	now = now.Add(100 * time.Millisecond)
	b.observe(now, 0)
	const grown = 300 * time.Millisecond
	for i := 0; i < 600; i++ {
		now = now.Add(time.Second / 60)
		b.observe(now, 2*time.Millisecond)
	}
	if got := b.reserve(); got != grown {
		t.Fatalf("reserve shrank before ten stable seconds: %v", got)
	}
	for i := 0; i < 2; i++ {
		now = now.Add(time.Second / 60)
		b.observe(now, 2*time.Millisecond)
	}
	firstReduction := b.reserve()
	if firstReduction >= grown || firstReduction <= minimum {
		t.Fatalf("first recovery reduction jumped from %v to %v", grown, firstReduction)
	}
	for i := 0; i < 30; i++ {
		now = now.Add(time.Second / 60)
		b.observe(now, 2*time.Millisecond)
	}
	if got := b.reserve(); got != firstReduction {
		t.Fatalf("recovery changed more than once per second: %v -> %v", firstReduction, got)
	}
	for i := 0; i < 20*60; i++ {
		now = now.Add(time.Second / 60)
		b.observe(now, 2*time.Millisecond)
	}
	if got := b.reserve(); got != minimum {
		t.Fatalf("reserve did not recover to baseline: %v", got)
	}
	if got := b.interval(); got != 12*time.Millisecond {
		t.Fatalf("recovered interval = %v", got)
	}
}

func TestAdaptiveMusicBufferIntermittentPressurePreventsShrink(t *testing.T) {
	var b adaptiveMusicBuffer
	now := time.Unix(1, 0)
	b.start(now, 93*time.Millisecond)
	now = now.Add(100 * time.Millisecond)
	b.observe(now, 0)
	const grown = 300 * time.Millisecond
	for second := 0; second < 30; second++ {
		for i := 0; i < 50; i++ {
			now = now.Add(time.Second / 60)
			b.observe(now, 0)
		}
		// This still needs most of the current reserve, without growing it.
		now = now.Add(80 * time.Millisecond)
		b.observe(now, 0)
	}
	if got := b.reserve(); got != grown {
		t.Fatalf("reserve shrank while refill delays persisted: %v", got)
	}
}

func TestAdaptiveMusicBufferTrackStartRetainsReserveResetsTiming(t *testing.T) {
	var b adaptiveMusicBuffer
	now := time.Unix(1, 0)
	b.start(now, 93*time.Millisecond)
	b.observe(now.Add(100*time.Millisecond), 0)
	const grown = 300 * time.Millisecond
	now = now.Add(time.Hour)
	b.start(now, 93*time.Millisecond)
	b.observe(now.Add(time.Second/60), 0)
	if got := b.reserve(); got != grown {
		t.Fatalf("track change lost learned reserve or counted idle time: %v", got)
	}
	// A larger requested baseline still takes effect immediately.
	b.start(now.Add(time.Second), 400*time.Millisecond)
	if got := b.reserve(); got != 400*time.Millisecond {
		t.Fatalf("larger stream baseline ignored: %v", got)
	}
}
