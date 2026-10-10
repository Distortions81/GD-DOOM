package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/netgame"
)

func TestAuthoritySnapshotPumpBoundsReconciliationAndPreservesEventsAndAcks(t *testing.T) {
	for _, observer := range []bool{false, true} {
		name := "player"
		if observer {
			name = "observer"
		}
		t.Run(name, func(t *testing.T) {
			a, g, connection := authorityClientTestWorld(t, 0)
			if observer {
				connection.welcome.PlayerID = 0
			}
			played := captureAuthoritySoundPlayback(g)
			now := time.Unix(100, 0)
			connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
			if err := g.updateAuthoritativeClientAt(now, nil); err != nil {
				t.Fatal(err)
			}
			sequence := g.clientUpdate.sequence
			connection.sent, connection.snapshotPolls = nil, 0
			// The real transport has a latest-only slot. A queued fake models a
			// second frame arriving while the first expensive restore is running.
			for i, kind := range []soundEvent{soundEventShootPistol, soundEventShootShotgun} {
				if err := a.Step(nil); err != nil {
					t.Fatal(err)
				}
				a.g.sectorCeil[0] = int64(120-i*8) * fracUnit
				a.g.emitSoundEvent(kind)
				ack := netgame.InputAck{HasTick: true, Tick: a.Tic()}
				if !observer {
					ack.HasSequence, ack.Sequence = true, sequence
				}
				connection.snapshots = append(connection.snapshots, predictionSnapshot(t, a, uint32(i+2), ack))
			}
			for i, kind := range []soundEvent{soundEventShootPistol, soundEventShootShotgun} {
				if err := g.updateAuthoritativeClientAt(now.Add(time.Duration(i+1)*time.Second/140), nil); err != nil {
					t.Fatal(err)
				}
				id, tic := uint32(i+2), uint32(i+1)
				if connection.snapshotPolls != i+1 || len(connection.snapshots) != 1-i || g.clientPrediction.SnapshotID() != id || g.worldTic != int(tic) {
					t.Fatalf("pump %d drained extra snapshots: polls=%d queued=%d id=%d world=%d", i+1, connection.snapshotPolls, len(connection.snapshots), g.clientPrediction.SnapshotID(), g.worldTic)
				}
				if g.sectorCeil[0] != int64(120-i*8)*fracUnit || g.m.Name != a.MapName() {
					t.Fatal("per-pump confirmed map state was lost")
				}
				if len(*played) != i+1 || (*played)[i] != kind || g.clientPrediction.soundCursor != uint64(i+1) {
					t.Fatalf("event history duplicated or dropped: played=%v cursor=%d", *played, g.clientPrediction.soundCursor)
				}
				if len(connection.sent) != i+1 || connection.sent[i].Epoch != 7 || connection.sent[i].SnapshotAck != id || len(connection.sent[i].Inputs) != 0 {
					t.Fatalf("per-pump acknowledgment wrong: %+v", connection.sent)
				}
				if g.clientUpdate.sequence != sequence || len(g.clientPrediction.PendingInputs()) != 0 {
					t.Fatal("snapshot-only pump invented commands or retained finalized input")
				}
			}
		})
	}
}
