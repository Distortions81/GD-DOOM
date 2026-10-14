package netgame

import (
	"errors"
	"testing"

	"gddoom/internal/demo"
)

func TestMatchResetEpochRetainsConnectionsAndDiscardsOldTimeline(t *testing.T) {
	m, _ := newTestMatch(t)
	w := &completingTestWorld{completeAt: 1}
	m.world, m.snapshots = w, w
	a, wa := joinTestMatch(t, m)
	b, wb := joinTestMatch(t, m)
	aPresence, bPresence, rosterRevision := m.players[a].presenceID, m.players[b].presenceID, m.rosterRevision
	submitTestInput(t, m, a, 1, 10)
	submitTestInput(t, m, a, 3, 50)
	old, err := m.Step()
	if err != nil || !old.Completed {
		t.Fatalf("old round did not complete: %+v %v", old, err)
	}
	w.tick, w.complete, w.completeAt = 0, false, 100
	welcomes, err := m.ResetEpoch(100, "next-content")
	if err != nil {
		t.Fatal(err)
	}
	if m.PlayerCount() != 2 || welcomes[a].PlayerID != wa.PlayerID || welcomes[b].PlayerID != wb.PlayerID || welcomes[a].Epoch != 100 || welcomes[a].ServerTick != 0 {
		t.Fatalf("epoch reset replaced authenticated membership: %+v", welcomes)
	}
	if m.players[a].presenceID != aPresence || m.players[b].presenceID != bPresence || m.rosterRevision != rosterRevision {
		t.Fatal("map transition replaced participant identity or membership")
	}
	if err := m.Submit(a, InputBatch{Epoch: 99, SnapshotAck: old.Snapshots[a].ID}); !errors.Is(err, ErrSessionEpoch) {
		t.Fatalf("old epoch input reached new timeline: %v", err)
	}
	if err := m.Submit(a, InputBatch{Epoch: 100, SnapshotAck: old.Snapshots[a].ID}); !errors.Is(err, ErrProtocol) {
		t.Fatalf("old snapshot acknowledgment reached new timeline: %v", err)
	}
	for tick := uint32(1); tick <= 3; tick++ {
		// The first new input may restart its sequence counter at zero.
		if tick == 1 {
			if err := m.Submit(b, InputBatch{Epoch: 100, Inputs: []Input{{Sequence: 0, Tick: 1, Command: demo.Tic{Forward: 5}}}}); err != nil {
				t.Fatal(err)
			}
		}
		result, err := m.Step()
		if err != nil {
			t.Fatal(err)
		}
		if result.Completed || result.Snapshots[a].Epoch != 100 || result.Snapshots[a].ID != tick || w.last[wa.PlayerID].Forward != 0 {
			t.Fatalf("old epoch leaked commands/completion/snapshot IDs: %+v commands=%+v", result, w.last)
		}
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "old content"}); !errors.Is(err, ErrCompatibility) {
		t.Fatalf("old-map compatibility was accepted: %v", err)
	}
}

func TestMatchResetEpochRejectsInvalidTransitionAtomically(t *testing.T) {
	m, w := newTestMatch(t)
	h, _ := joinTestMatch(t, m)
	if _, err := m.ResetEpoch(100, "next"); err == nil {
		t.Fatal("unfinished match accepted reset")
	}
	m.completed = true
	for _, tc := range []struct {
		epoch uint64
		key   string
		tick  uint32
	}{{99, "next", 0}, {98, "next", 0}, {100, "", 0}, {100, "next", 1}} {
		w.tick = tc.tick
		before := m.players[h].input
		if _, err := m.ResetEpoch(tc.epoch, tc.key); err == nil {
			t.Fatalf("accepted invalid reset %+v", tc)
		}
		if m.Epoch() != 99 || m.players[h].input != before || !m.completed {
			t.Fatal("invalid reset mutated match history")
		}
	}
}
