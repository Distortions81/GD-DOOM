package doomruntime

import (
	"errors"
	"math"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

type fakeAuthorityClient struct {
	welcome          netgame.Welcome
	transitions      []netgame.MapChange
	snapshots        []netgame.Snapshot
	sent             []netgame.InputBatch
	pollErr, sendErr error
	snapshotPolls    int
	rtt              time.Duration
}

func (f *fakeAuthorityClient) Welcome() netgame.Welcome     { return f.welcome }
func (f *fakeAuthorityClient) RoundTripTime() time.Duration { return f.rtt }
func (f *fakeAuthorityClient) PollTransition() (netgame.MapChange, bool, error) {
	if len(f.transitions) == 0 {
		return netgame.MapChange{}, false, nil
	}
	change := f.transitions[0]
	f.transitions = f.transitions[1:]
	f.welcome = change.Welcome
	return change, true, nil
}
func (f *fakeAuthorityClient) PollSnapshot() (netgame.Snapshot, bool, error) {
	f.snapshotPolls++
	if f.pollErr != nil {
		return netgame.Snapshot{}, false, f.pollErr
	}
	if len(f.snapshots) == 0 {
		return netgame.Snapshot{}, false, nil
	}
	s := f.snapshots[0]
	f.snapshots = f.snapshots[1:]
	return s, true, nil
}
func (f *fakeAuthorityClient) SendInputs(batch netgame.InputBatch) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	batch.Inputs = append([]netgame.Input(nil), batch.Inputs...)
	f.sent = append(f.sent, batch)
	return nil
}

func authorityClientTestWorld(t *testing.T, lead uint16) (*Authority, *game, *fakeAuthorityClient) {
	t.Helper()
	a, p := predictionTestWorld(t)
	f := &fakeAuthorityClient{welcome: netgame.Welcome{Epoch: 7, PlayerID: 1, InputLead: lead}}
	p.g.opts.AuthorityClient = f
	return a, p.g, f
}

func TestAuthorityClientWaitsForBaselineAndSchedulesLead(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	now := time.Unix(100, 0)
	sampled := 0
	sample := func() demo.Tic { sampled++; return demo.Tic{Forward: 50} }
	if err := g.updateAuthoritativeClientAt(now, sample); err != nil {
		t.Fatal(err)
	}
	if sampled != 0 || len(connection.sent) != 0 || g.clientPrediction.Ready() {
		t.Fatal("submitted controls before initial baseline")
	}
	g.alwaysRun = true
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := g.updateAuthoritativeClientAt(now.Add(time.Second), sample); err != nil {
		t.Fatal(err)
	}
	if sampled != 1 || len(connection.sent) != 1 {
		t.Fatal("initial baseline did not start prediction")
	}
	batch := connection.sent[0]
	if batch.Epoch != 7 || batch.SnapshotAck != 1 || len(batch.Inputs) != 1 || batch.Inputs[0].Tick != 4 || batch.Inputs[0].Sequence != 1 {
		t.Fatalf("incorrect scheduled input: %+v", batch)
	}
	for range 3 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Step(map[byte]demo.Tic{1: {Forward: 50}}); err != nil {
		t.Fatal(err)
	}
	if g.p != a.players[1].p || g.worldTic != 0 || g.clientPrediction.PredictedTic() != 4 {
		t.Fatal("lead prediction changed authoritative clock or body differs from server")
	}
	if !g.alwaysRun {
		t.Fatal("snapshot replaced client run preference")
	}
}

func TestAuthorityClientBoundsCatchupAndRedundantBatches(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	now := time.Unix(100, 0)
	for i := range 4 {
		if err := g.updateAuthoritativeClientAt(now.Add(time.Duration(i)*(time.Second/netgame.TickRate*4)), nil); err != nil {
			t.Fatal(err)
		}
	}
	if g.clientUpdate.sequence != 13 || len(connection.sent) != 4 {
		t.Fatal("unbounded or missing catchup commands")
	}
	batch := connection.sent[3]
	if len(batch.Inputs) != netgame.MaxInputBatch || batch.Inputs[0].Sequence != 6 || batch.Inputs[7].Sequence != 13 {
		t.Fatalf("redundant batch must contain newest eight: %+v", batch)
	}
	for _, input := range batch.Inputs {
		if input.Command != (demo.Tic{}) {
			t.Fatal("background menu pump sent nonneutral input")
		}
	}
	if g.worldTic != 0 {
		t.Fatal("client pump stepped server world")
	}
}

func TestAuthorityClientInterpolationUsesCommandClock(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	g.opts.SourcePortMode = true
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	start := time.Unix(100, 0)
	const ticDuration = time.Second / netgame.TickRate
	// Uneven render updates cross command boundaries at different phases.
	// Interpolation must retain the elapsed fraction rather than reset at
	// the update time, which produces a repeating sawtooth in movement speed.
	for _, elapsed := range []time.Duration{0, 16 * time.Millisecond, 34 * time.Millisecond, 49 * time.Millisecond, 67 * time.Millisecond, 84 * time.Millisecond, 101 * time.Millisecond, 130 * time.Millisecond} {
		now := start.Add(elapsed)
		if err := g.updateAuthoritativeClientAt(now, func() demo.Tic { return demo.Tic{Forward: 25} }); err != nil {
			t.Fatal(err)
		}
		wantStamp := start.Add(elapsed / ticDuration * ticDuration)
		if !g.lastUpdate.Equal(wantStamp) {
			t.Fatalf("at %v interpolation clock = %v, want %v", elapsed, g.lastUpdate.Sub(start), wantStamp.Sub(start))
		}
		g.prepareRenderStateAt(now)
		wantAlpha := now.Sub(wantStamp).Seconds() * netgame.TickRate
		if math.Abs(g.renderAlpha-wantAlpha) > 1e-8 {
			t.Fatalf("at %v interpolation alpha = %v, want %v", elapsed, g.renderAlpha, wantAlpha)
		}
	}
}

func TestAuthorityClientJitteredMatchingSnapshotsPreserveRenderedMotion(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	_, reference, referenceConnection := authorityClientTestWorld(t, 0)
	g.opts.SourcePortMode, reference.opts.SourcePortMode = true, true
	g.opts.SmoothCameraYaw, reference.opts.SmoothCameraYaw = true, true
	initial := predictionSnapshot(t, a, 1, netgame.InputAck{})
	connection.snapshots, referenceConnection.snapshots = []netgame.Snapshot{initial}, []netgame.Snapshot{initial}
	start := time.Unix(100, 0)
	const ticDuration = time.Second / netgame.TickRate
	type delayedSnapshot struct {
		at       time.Time
		snapshot netgame.Snapshot
	}
	var deliveries []delayedSnapshot
	inputs := make(map[uint32]demo.Tic)
	sample := func() demo.Tic { return demo.Tic{Forward: 25, AngleTurn: 128} }
	id := uint32(1)
	applied := 0
	// The reference predicts the same commands without intermediate baselines.
	// A matching authoritative baseline at any render phase should be invisible.
	// Delivery jitter deliberately puts most packets between local 35 Hz steps.
	for frame := range 90 {
		elapsed := time.Duration(frame) * time.Second / 120
		now := start.Add(elapsed)
		for a.Tic() < uint32(elapsed/ticDuration) {
			tick := a.Tic() + 1
			if err := a.Step(map[byte]demo.Tic{1: inputs[tick]}); err != nil {
				t.Fatal(err)
			}
			if tick%2 == 0 {
				id++
				jitter := []time.Duration{3, 21, 7, 15}[int(id)%4] * time.Millisecond
				deliveries = append(deliveries, delayedSnapshot{start.Add(time.Duration(tick)*ticDuration + jitter), predictionSnapshot(t, a, id, netgame.InputAck{HasTick: true, Tick: tick, HasSequence: true, Sequence: tick})})
			}
		}
		for len(deliveries) > 0 && !now.Before(deliveries[0].at) {
			connection.snapshots = append(connection.snapshots, deliveries[0].snapshot)
			deliveries = deliveries[1:]
			applied++
		}
		for _, clientGame := range []*game{g, reference} {
			if err := clientGame.updateAuthoritativeClientAt(now, sample); err != nil {
				t.Fatal(err)
			}
			clientGame.prepareRenderStateAt(now)
		}
		if len(connection.sent) > 0 {
			for _, input := range connection.sent[len(connection.sent)-1].Inputs {
				inputs[input.Tick] = input.Command
			}
		}
		if g.p != reference.p || math.Abs(g.renderPX-reference.renderPX) > 1e-6 || math.Abs(g.renderPY-reference.renderPY) > 1e-6 || g.renderAngle != reference.renderAngle {
			t.Fatalf("snapshot changed motion at frame %d: body equal=%v render=(%.6f, %.6f, %d), uninterrupted=(%.6f, %.6f, %d)", frame, g.p == reference.p, g.renderPX, g.renderPY, g.renderAngle, reference.renderPX, reference.renderPY, reference.renderAngle)
		}
	}
	if applied < 10 {
		t.Fatalf("insufficient jittered baselines: %d", applied)
	}
}

func TestAuthorityClientRTTLeadAndStalledSnapshotHorizon(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	connection.rtt = 100 * time.Millisecond
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	now := time.Unix(100, 0)
	if err := g.updateAuthoritativeClientAt(now, nil); err != nil {
		t.Fatal(err)
	}
	if input := connection.sent[0].Inputs[0]; input.Tick != 8 {
		t.Fatalf("100ms round-trip lead = %d, want 8", input.Tick)
	}
	if target := g.authorityInputTarget(now.Add(200 * time.Millisecond)); target != 15 {
		t.Fatalf("elapsed snapshot time ignored: %d", target)
	}
	connection.rtt = 5 * time.Second
	if target := g.authorityInputTarget(now); target != 35 {
		t.Fatalf("unbounded RTT lead: %d", target)
	}
	if err := g.updateAuthoritativeClientAt(now.Add(2*time.Second), nil); err != nil {
		t.Fatal(err)
	}
	if g.clientUpdate.sequence != 1 || len(connection.sent) != 1 {
		t.Fatal("stalled snapshot stream continued scheduling beyond bounded horizon")
	}
	connection.rtt = 100 * time.Millisecond
	for range 70 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 2, netgame.InputAck{HasTick: true, Tick: 70})}
	if err := g.updateAuthoritativeClientAt(now.Add(3*time.Second), nil); err != nil {
		t.Fatal(err)
	}
	batch := connection.sent[len(connection.sent)-1]
	if len(batch.Inputs) != 4 || batch.Inputs[0].Tick != 78 || batch.Inputs[0].Sequence != 2 {
		t.Fatalf("fresh baseline did not recover bounded catchup: %+v", batch)
	}
}

func TestAuthorityClientCorrectionExpiresInputsBeforeSampling(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	now := time.Unix(100, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := g.updateAuthoritativeClientAt(now, func() demo.Tic { return demo.Tic{Forward: 50} }); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 2, netgame.InputAck{HasTick: true, Tick: 20})}
	observedTic := -1
	if err := g.updateAuthoritativeClientAt(now.Add(time.Second/netgame.TickRate), func() demo.Tic {
		observedTic = g.worldTic
		return demo.Tic{Side: 80}
	}); err != nil {
		t.Fatal(err)
	}
	batch := connection.sent[len(connection.sent)-1]
	if observedTic != 20 || batch.SnapshotAck != 2 || len(batch.Inputs) != 1 || batch.Inputs[0].Tick != 24 || batch.Inputs[0].Sequence != 2 || g.worldTic != 20 {
		t.Fatalf("correction did not expire lost input before scheduling: %+v", batch)
	}
	// A repeated render update without another scheduled tic still sends an ack
	// when a new full snapshot arrives, and never invents another input.
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 3, netgame.InputAck{HasTick: true, Tick: 20})}
	if err := g.updateAuthoritativeClientAt(now.Add(time.Second/netgame.TickRate), nil); err != nil {
		t.Fatal(err)
	}
	if got := connection.sent[len(connection.sent)-1]; got.SnapshotAck != 3 || got.Inputs[0].Sequence != 2 {
		t.Fatalf("snapshot-only acknowledgment: %+v", got)
	}
}

func TestAuthorityClientDisplaysTransportFailureAndRejectsLegacyMix(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 0)
	failure := errors.New("closed connection")
	connection.pollErr = failure
	if err := g.updateAuthoritativeClientAt(time.Unix(100, 0), nil); err != nil {
		t.Fatalf("transport failure terminated rendering: %v", err)
	}
	if status := g.authorityConnectionStatus(); status.State != netgame.ConnectionDisconnected || status.Message != failure.Error() || len(connection.sent) != 0 {
		t.Fatalf("transport failure not displayed: %+v", status)
	}
	connection.pollErr = nil
	g.resetAuthorityClientPrediction()
	g.opts.RecordDemoPath = "demo.lmp"
	if err := g.updateAuthoritativeClientAt(time.Unix(100, 0), nil); err == nil {
		t.Fatal("accepted recording together with authoritative input")
	}
}
