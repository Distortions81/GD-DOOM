package netgame

import (
	"errors"
	"testing"

	"gddoom/internal/demo"
)

type testWorld struct {
	tick    uint32
	players map[byte]int
	steps   int
	last    map[byte]demo.Tic
	fail    bool
}

func (w *testWorld) Tic() uint32 { return w.tick }
func (w *testWorld) AddPlayer(id byte) error {
	if w.players == nil {
		w.players = make(map[byte]int)
	}
	w.players[id] = 0
	return nil
}
func (w *testWorld) RemovePlayer(id byte) { delete(w.players, id) }
func (w *testWorld) Step(commands map[byte]demo.Tic) error {
	if w.fail {
		return errors.New("broken simulation")
	}
	w.tick++
	w.steps++
	w.last = commands
	for id, cmd := range commands {
		w.players[id] += int(cmd.Forward)
	}
	return nil
}
func (w *testWorld) Snapshot(id byte) ([]byte, error) { return []byte{byte(w.tick), id}, nil }

func newTestMatch(t *testing.T) (*Match, *testWorld) {
	t.Helper()
	w := &testWorld{}
	m, err := NewMatch(w, w, MatchConfig{Epoch: 99, Compatibility: "test-content", PlayerLimit: 4, InputLead: 2, FutureTicks: 8, HoldTicks: 2, DisconnectTicks: 10, SnapshotInterval: 1})
	if err != nil {
		t.Fatal(err)
	}
	return m, w
}
func joinTestMatch(t *testing.T, m *Match) (ConnectionID, Welcome) {
	t.Helper()
	h, w, err := m.Join(Hello{Compatibility: "test-content", Name: "player"})
	if err != nil {
		t.Fatal(err)
	}
	return h, w
}
func submitTestInput(t *testing.T, m *Match, h ConnectionID, tick uint32, forward int8) {
	t.Helper()
	if err := m.Submit(h, InputBatch{Epoch: 99, Inputs: []Input{{Sequence: tick, Tick: tick, Command: demo.Tic{Forward: forward, Buttons: demo.ButtonAttack}}}}); err != nil {
		t.Fatal(err)
	}
}

func TestMatchLossDoesNotStopWorldOrGrantBacklogMovement(t *testing.T) {
	m, w := newTestMatch(t)
	a, wa := joinTestMatch(t, m)
	b, wb := joinTestMatch(t, m)
	for tick := uint32(1); tick <= 6; tick++ {
		submitTestInput(t, m, a, tick, 10)
		if tick == 1 || tick == 6 {
			submitTestInput(t, m, b, tick, 20)
		}
		result, err := m.Step()
		if err != nil {
			t.Fatal(err)
		}
		if result.Tick != tick || w.steps != int(tick) {
			t.Fatalf("world advanced incorrectly: %+v, steps=%d", result, w.steps)
		}
		if tick > 1 && tick < 6 && w.last[wb.PlayerID].Buttons != 0 {
			t.Fatal("packet loss repeated attack")
		}
		if ack := result.Snapshots[b].Finalized; ack.Tick != tick || !ack.HasTick {
			t.Fatalf("expired input not finalized: %+v", ack)
		}
	}
	if w.players[wa.PlayerID] != 60 || w.players[wb.PlayerID] != 80 {
		t.Fatalf("movement=%v", w.players)
	}
	// Redundant late inputs are ignored; they cannot be executed on a later tic.
	if err := m.Submit(b, InputBatch{Epoch: 99, Inputs: []Input{{Sequence: 2, Tick: 2, Command: demo.Tic{Forward: 50}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if w.players[wb.PlayerID] != 100 {
		t.Fatalf("late input replayed: %v", w.players)
	}
}

func TestMatchReconnectCannotBeControlledByOldConnection(t *testing.T) {
	m, _ := newTestMatch(t)
	old, first := joinTestMatch(t, m)
	m.Leave(old)
	fresh, second := joinTestMatch(t, m)
	if old == fresh || first.PlayerID != second.PlayerID {
		t.Fatal("slot reuse must issue fresh connection identity")
	}
	if err := m.Submit(old, InputBatch{Epoch: 99}); !errors.Is(err, ErrUnknownConnection) {
		t.Fatalf("old socket err=%v", err)
	}
	if err := m.Submit(fresh, InputBatch{Epoch: 98}); !errors.Is(err, ErrSessionEpoch) {
		t.Fatalf("old epoch err=%v", err)
	}
}

func TestMatchBatchValidationIsAtomic(t *testing.T) {
	m, w := newTestMatch(t)
	h, wel := joinTestMatch(t, m)
	err := m.Submit(h, InputBatch{Epoch: 99, Inputs: []Input{
		{Sequence: 1, Tick: 1, Command: demo.Tic{Forward: 40}},
		{Sequence: 2, Tick: 2, Command: demo.Tic{Forward: 127}},
	}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
	if _, err = m.Step(); err != nil {
		t.Fatal(err)
	}
	if w.players[wel.PlayerID] != 0 {
		t.Fatal("invalid batch partially applied")
	}
}

func TestMatchTimeoutRemovesOnlySilentPlayer(t *testing.T) {
	m, w := newTestMatch(t)
	a, _ := joinTestMatch(t, m)
	b, wb := joinTestMatch(t, m)
	for tick := uint32(1); tick <= 11; tick++ {
		submitTestInput(t, m, a, tick, 10)
		result, err := m.Step()
		if err != nil {
			t.Fatal(err)
		}
		if tick == 11 && (len(result.Dropped) != 1 || result.Dropped[0] != b) {
			t.Fatalf("dropped=%v", result.Dropped)
		}
	}
	if _, ok := w.players[wb.PlayerID]; ok {
		t.Fatal("silent player remains")
	}
	if w.tick != 11 || m.PlayerCount() != 1 {
		t.Fatalf("tick=%d players=%d", w.tick, m.PlayerCount())
	}
}

func TestMatchChecksCompatibilityLimitsAndSnapshotAcknowledgment(t *testing.T) {
	m, _ := newTestMatch(t)
	if _, _, err := m.Join(Hello{Compatibility: "wrong", Name: "player"}); !errors.Is(err, ErrCompatibility) {
		t.Fatalf("err=%v", err)
	}
	h, _ := joinTestMatch(t, m)
	for range 3 {
		joinTestMatch(t, m)
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "fifth"}); !errors.Is(err, ErrMatchFull) {
		t.Fatalf("err=%v", err)
	}
	if err := m.Submit(h, InputBatch{Epoch: 99, SnapshotAck: 1}); !errors.Is(err, ErrProtocol) {
		t.Fatalf("ack unsent snapshot err=%v", err)
	}
	if _, err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if err := m.Submit(h, InputBatch{Epoch: 99, SnapshotAck: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestMatchEmptyPausesAndFailedSimulationCannotBeRetried(t *testing.T) {
	m, w := newTestMatch(t)
	if _, err := m.Step(); err != nil || w.steps != 0 {
		t.Fatalf("empty match advanced: %v", err)
	}
	joinTestMatch(t, m)
	w.fail = true
	if _, err := m.Step(); err == nil {
		t.Fatal("missing simulation error")
	}
	w.fail = false
	if _, err := m.Step(); err == nil || w.steps != 0 {
		t.Fatal("failed tic retried")
	}
}

type completingTestWorld struct {
	testWorld
	completeAt uint32
	complete   bool
}

func (w *completingTestWorld) Step(commands map[byte]demo.Tic) error {
	if w.complete {
		return errors.New("step after ordinary match completion")
	}
	if err := w.testWorld.Step(commands); err != nil {
		return err
	}
	w.complete = w.tick == w.completeAt
	return nil
}

func (w *completingTestWorld) MatchCompletion() (bool, string) {
	if w.complete {
		return true, "frag limit"
	}
	return false, ""
}

func TestMatchCompletionForcesFinalSnapshotAndStopsSimulation(t *testing.T) {
	m, _ := newTestMatch(t)
	w := &completingTestWorld{completeAt: 2}
	m.world, m.snapshots, m.config.SnapshotInterval = w, w, 5
	h, _ := joinTestMatch(t, m)
	first, err := m.Step()
	if err != nil || first.Completed || len(first.Snapshots) != 0 {
		t.Fatalf("first tic result=%+v error=%v", first, err)
	}
	final, err := m.Step()
	if err != nil || !final.Completed || final.CompletionReason != "frag limit" {
		t.Fatalf("final result=%+v error=%v", final, err)
	}
	if snapshot := final.Snapshots[h]; snapshot.Tick != 2 || snapshot.Finalized.Tick != 2 || len(snapshot.State) == 0 {
		t.Fatalf("final tic was not snapshotted: %+v", snapshot)
	}
	for range 3 {
		result, err := m.Step()
		if err != nil || !result.Completed || result.Tick != 2 || len(result.Snapshots) != 0 || w.steps != 2 {
			t.Fatalf("completed world advanced/resnapshotted: %+v error=%v steps=%d", result, err, w.steps)
		}
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "late"}); !errors.Is(err, ErrMatchCompleted) {
		t.Fatalf("completed match join = %v", err)
	}
}

func TestMatchCompletionBetweenTicsRetainsFinalSnapshot(t *testing.T) {
	m, _ := newTestMatch(t)
	w := &completingTestWorld{completeAt: 100}
	m.world, m.snapshots = w, w
	h, _ := joinTestMatch(t, m)
	w.complete = true
	// A join may discover completion before Step. That must not suppress the
	// existing players' terminal state or cause an extra simulation tic.
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "late"}); !errors.Is(err, ErrMatchCompleted) {
		t.Fatalf("completed match join = %v", err)
	}
	result, err := m.Step()
	if err != nil || !result.Completed || result.Tick != 0 || w.steps != 0 || len(result.Snapshots[h].State) == 0 {
		t.Fatalf("completion between tics result=%+v error=%v steps=%d", result, err, w.steps)
	}
	if result.Snapshots[h].Finalized.HasTick {
		t.Fatal("completion finalized an unexecuted tic")
	}
}
