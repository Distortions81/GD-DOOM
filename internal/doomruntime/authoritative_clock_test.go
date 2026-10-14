package doomruntime

import (
	"errors"
	"math"
	"sort"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

func TestAuthorityCommandClockKeepsInputsContiguousAcrossLatencyChanges(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	g.opts.SourcePortMode = true
	g.opts.SmoothCameraYaw = true
	connection.rtt = 60 * time.Millisecond
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	buffer, err := netgame.NewInputBuffer(netgame.InputBufferConfig{FirstTick: 1, FutureTicks: 35, HoldTicks: 2})
	if err != nil {
		t.Fatal(err)
	}
	type delivery struct {
		at       time.Time
		snapshot netgame.Snapshot
	}
	var deliveries []delivery
	start := time.Unix(100, 0)
	const period = time.Second / netgame.TickRate
	var lastSequence, lastTick, firstTick uint32
	snapshotID := uint32(1)
	applied := 0
	sample := func() demo.Tic { return demo.Tic{Forward: 10, AngleTurn: 1536} }
	for frame := range 600 {
		now := start.Add(time.Duration(frame) * time.Second / 120)
		for a.Tic() < uint32(now.Sub(start)/period) {
			input, err := buffer.Consume(a.Tic() + 1)
			if err != nil {
				t.Fatal(err)
			}
			if firstTick != 0 && input.Tick >= firstTick && !input.Received {
				t.Fatalf("scheduler left an empty server input slot at tic %d", input.Tick)
			}
			if err := a.Step(map[byte]demo.Tic{1: input.Command}); err != nil {
				t.Fatal(err)
			}
			if a.Tic()%2 == 0 {
				snapshotID++
				delay := []time.Duration{5, 50, 20, 35}[snapshotID%4] * time.Millisecond
				deliveries = append(deliveries, delivery{now.Add(delay), predictionSnapshot(t, a, snapshotID, buffer.Ack())})
			}
		}
		sort.Slice(deliveries, func(i, j int) bool { return deliveries[i].at.Before(deliveries[j].at) })
		for len(deliveries) > 0 && !now.Before(deliveries[0].at) {
			connection.snapshots = append(connection.snapshots, deliveries[0].snapshot)
			deliveries = deliveries[1:]
			applied++
		}
		// These changes used to jump the assigned input tic and simulate the
		// gaps as neutral, although the real server briefly holds movement.
		switch frame {
		case 45:
			connection.rtt = 150 * time.Millisecond
		case 180:
			connection.rtt = 70 * time.Millisecond
		case 300:
			connection.rtt = 190 * time.Millisecond
		case 420:
			connection.rtt = 60 * time.Millisecond
		}
		if err := g.updateAuthoritativeClientAt(now, sample); err != nil {
			t.Fatal(err)
		}
		g.prepareRenderStateAt(now)
		for _, batch := range connection.sent {
			for _, input := range batch.Inputs {
				if input.Sequence <= lastSequence {
					continue
				}
				if lastSequence != 0 && (input.Sequence != lastSequence+1 || input.Tick != lastTick+1) {
					t.Fatalf("latency change skipped input slots: previous=%d/%d next=%d/%d", lastSequence, lastTick, input.Sequence, input.Tick)
				}
				if err := buffer.Submit(input); err != nil && !errors.Is(err, netgame.ErrDuplicateInput) {
					t.Fatal(err)
				}
				if firstTick == 0 {
					firstTick = input.Tick
				}
				lastSequence, lastTick = input.Sequence, input.Tick
			}
		}
		connection.sent = nil
		correction := g.clientPrediction.renderCorrection
		if correction.x != 0 || correction.y != 0 || correction.z != 0 || correction.angle != 0 {
			t.Fatalf("matching movement needed correction at frame %d: %+v", frame, correction)
		}
	}
	if applied < 60 || lastSequence < 160 {
		t.Fatalf("insufficient moving snapshots/commands: %d/%d", applied, lastSequence)
	}
}

func TestAuthorityCommandClockHoldsCameraAtPredictionHorizon(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	g.opts.SourcePortMode = true
	connection.rtt = time.Second
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	start := time.Unix(100, 0)
	sample := func() demo.Tic { return demo.Tic{Forward: 25} }
	if err := g.updateAuthoritativeClientAt(start, sample); err != nil {
		t.Fatal(err)
	}
	if g.clientPrediction.PredictedTic() != netgame.TickRate {
		t.Fatal("test did not reach the prediction horizon")
	}
	// A missing baseline stops movement. Changing the latency estimate on a
	// host frame without a command must not restart the last movement blend.
	if err := g.updateAuthoritativeClientAt(start.Add(100*time.Millisecond), sample); err != nil {
		t.Fatal(err)
	}
	g.prepareRenderStateAt(start.Add(100 * time.Millisecond))
	x, y := g.renderPX, g.renderPY
	connection.rtt = 0
	if err := g.updateAuthoritativeClientAt(start.Add(101*time.Millisecond), sample); err != nil {
		t.Fatal(err)
	}
	g.prepareRenderStateAt(start.Add(101 * time.Millisecond))
	if g.renderPX != x || g.renderPY != y || g.clientUpdate.sequence != 1 {
		t.Fatal("latency adjustment rewound a camera held at the prediction horizon")
	}
}

func TestAuthorityCommandClockRateChangePreservesRenderPhase(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	g.opts.SourcePortMode = true
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	start := time.Unix(100, 0)
	sample := func() demo.Tic { return demo.Tic{Forward: 25, AngleTurn: 128} }
	if err := g.updateAuthoritativeClientAt(start, sample); err != nil {
		t.Fatal(err)
	}
	now := start.Add(10 * time.Millisecond)
	g.prepareRenderStateAt(now)
	x, y, angle, alpha := g.renderPX, g.renderPY, g.renderAngle, g.renderAlpha
	connection.rtt = 150 * time.Millisecond
	if err := g.updateAuthoritativeClientAt(now, sample); err != nil {
		t.Fatal(err)
	}
	g.prepareRenderStateAt(now)
	if g.clientUpdate.sequence != 1 || g.clientUpdate.step >= time.Second/netgame.TickRate {
		t.Fatal("latency change must adjust cadence without inventing an input")
	}
	if math.Abs(g.renderPX-x) > 1e-6 || math.Abs(g.renderPY-y) > 1e-6 || abs(int64(int32(g.renderAngle-angle))) > 2 || math.Abs(g.renderAlpha-alpha) > 1e-7 {
		t.Fatalf("clock rate change moved camera: position=(%v,%v) -> (%v,%v), angle=%d -> %d, phase=%v -> %v", x, y, g.renderPX, g.renderPY, angle, g.renderAngle, alpha, g.renderAlpha)
	}
}
