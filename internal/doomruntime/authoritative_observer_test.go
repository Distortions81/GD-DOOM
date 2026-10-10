package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

func TestAuthoritativeObserverWaitsAndNeverPredictsOrSendsCommands(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	connection.welcome.PlayerID = 0
	now := time.Unix(100, 0)
	sample := func() demo.Tic { t.Fatal("spectator sampled gameplay input"); return demo.Tic{} }
	if err := g.updateAuthoritativeClientAt(now, sample); err != nil {
		t.Fatal(err)
	}
	if len(connection.sent) != 0 || g.clientPrediction.ready {
		t.Fatal("observer advanced before a player snapshot")
	}
	for range 12 {
		if err := a.Step(map[byte]demo.Tic{1: {Forward: 25}}); err != nil {
			t.Fatal(err)
		}
	}
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := g.updateAuthoritativeClientAt(now.Add(time.Second), sample); err != nil {
		t.Fatal(err)
	}
	if g.p != a.players[1].p || g.worldTic != 12 || g.clientPrediction.PredictedTic() != 12 || len(g.clientPrediction.history) != 0 {
		t.Fatal("observer did not preserve confirmed body and clock")
	}
	if len(connection.sent) != 1 || connection.sent[0].SnapshotAck != 1 || len(connection.sent[0].Inputs) != 0 {
		t.Fatal("observer must only acknowledge snapshots")
	}
	if err := g.updateAuthoritativeClientAt(now.Add(3*time.Second), sample); err != nil {
		t.Fatal(err)
	}
	if g.worldTic != 12 || len(connection.sent) != 1 {
		t.Fatal("observer predicted during silence")
	}
}

func TestAuthoritativeObserverSwitchesCameraAndRejectsPlayerAcknowledgment(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	connection.welcome.PlayerID = 0
	a.players[1].p.x = -64 * fracUnit
	if err := a.AddPlayer(2); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := g.updateAuthoritativeClientAt(now, nil); err != nil {
		t.Fatal(err)
	}
	data, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	connection.snapshots = []netgame.Snapshot{{Epoch: 7, ID: 2, Tick: a.Tic(), State: data}}
	if err := g.updateAuthoritativeClientAt(now.Add(time.Second), nil); err != nil {
		t.Fatal(err)
	}
	if g.clientPrediction.viewer != 2 || g.p != a.players[2].p || connection.Welcome().PlayerID != 0 {
		t.Fatal("camera change mutated connection identity or did not follow player two")
	}
	before := g.p
	connection.snapshots = []netgame.Snapshot{{Epoch: 7, ID: 3, Tick: a.Tic(), State: data, Finalized: netgame.InputAck{HasTick: true, HasSequence: true, Sequence: 1}}}
	if err := g.updateAuthoritativeClientAt(now, nil); err == nil || g.p != before {
		t.Fatal("accepted gameplay acknowledgment on spectator connection")
	}
}

func TestAuthoritativeObserverCameraUsesSnapshotTimeline(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 3)
	connection.welcome.PlayerID = 0
	g.opts.SourcePortMode = true
	now := time.Unix(100, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	if err := g.updateAuthoritativeClientAt(now, nil); err != nil {
		t.Fatal(err)
	}
	initial := g.p
	a.players[1].p.x += 20 * fracUnit
	a.players[1].p.y += 10 * fracUnit
	a.players[1].p.angle += 0x20000000
	a.g.worldTic += 2
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 2, netgame.InputAck{})}
	arrival := now.Add(time.Second * 2 / 35)
	if err := g.updateAuthoritativeClientAt(arrival, nil); err != nil {
		t.Fatal(err)
	}
	confirmed := g.p
	g.prepareRenderStateAt(arrival.Add(time.Second * 2 / 35))
	if abs(int64(g.renderPX*fracUnit)-(initial.x+10*fracUnit)) > 1 || abs(int64(g.renderPY*fracUnit)-(initial.y+5*fracUnit)) > 1 {
		t.Fatalf("observer camera snapped instead of interpolating: %v,%v", g.renderPX, g.renderPY)
	}
	if delta := int32(g.renderAngle - (initial.angle + 0x10000000)); delta < -8 || delta > 8 {
		t.Fatalf("observer yaw did not interpolate: %x", g.renderAngle)
	}
	if g.State.RenderCamX != g.renderPX || g.State.RenderCamY != g.renderPY {
		t.Fatal("observer automap and first-person camera use different timelines")
	}
	if g.p != confirmed || g.worldTic != 2 || len(g.clientPrediction.history) != 0 {
		t.Fatal("observer presentation changed confirmed state or predicted movement")
	}
	for _, batch := range connection.sent {
		if len(batch.Inputs) != 0 {
			t.Fatal("spectator interpolation sent player input")
		}
	}
}
