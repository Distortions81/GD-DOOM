package netgame

import (
	"errors"
	"reflect"
	"testing"
)

type batchSnapshotWorld struct {
	*testWorld
	viewers []byte
	calls   int
	err     error
	omit    byte
}

func (w *batchSnapshotWorld) Snapshot(byte) ([]byte, error) {
	return nil, errors.New("batch source used individual capture")
}

func (w *batchSnapshotWorld) SnapshotBatch(viewers []byte) (map[byte][]byte, error) {
	w.calls++
	w.viewers = append([]byte(nil), viewers...)
	if w.err != nil {
		return nil, w.err
	}
	states := make(map[byte][]byte, len(viewers))
	for _, viewer := range viewers {
		if viewer != w.omit {
			states[viewer] = []byte{byte(w.tick), viewer}
		}
	}
	return states, nil
}

func TestMatchSnapshotBatchSharesViewersKeepsConnectionAcks(t *testing.T) {
	m, world := newTestMatch(t)
	source := &batchSnapshotWorld{testWorld: world}
	m.snapshots = source
	a, _ := joinTestMatch(t, m)
	b, _ := joinTestMatch(t, m)
	observer, _ := joinSpectatorTest(t, m)
	submitTestInput(t, m, a, 1, 10)
	result, err := m.Step()
	if err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || !reflect.DeepEqual(source.viewers, []byte{1, 2}) {
		t.Fatalf("capture calls=%d viewers=%v", source.calls, source.viewers)
	}
	if len(result.Snapshots) != 3 || result.Snapshots[observer].State[1] != 1 || result.Snapshots[b].State[1] != 2 {
		t.Fatal("batched viewer routing changed")
	}
	if &result.Snapshots[a].State[0] != &result.Snapshots[observer].State[0] {
		t.Fatal("same viewer was captured/copied again for spectator")
	}
	if !result.Snapshots[a].Finalized.HasSequence || result.Snapshots[b].Finalized.HasSequence || result.Snapshots[observer].Finalized.HasTick {
		t.Fatal("per-connection acknowledgments were combined with shared state")
	}
	if err := m.follow(observer, 2); err != nil {
		t.Fatal(err)
	}
	result, err = m.Step()
	if err != nil || source.calls != 2 || result.Snapshots[observer].State[1] != 2 {
		t.Fatal("batch did not follow current spectator viewer")
	}
}

func TestMatchSnapshotBatchSkipsSuspendedRecipients(t *testing.T) {
	m, world := newResumeTestMatch(t, 35)
	source := &batchSnapshotWorld{testWorld: world}
	m.snapshots = source
	a, _ := joinTestMatch(t, m)
	b, _ := joinTestMatch(t, m)
	m.Suspend(b)
	result, err := m.Step()
	if err != nil || len(result.Snapshots) != 1 || len(result.Snapshots[a].State) == 0 || !reflect.DeepEqual(source.viewers, []byte{1}) {
		t.Fatalf("suspended recipient capture: snapshots=%v viewers=%v err=%v", result.Snapshots, source.viewers, err)
	}
}

func TestMatchSnapshotBatchRejectsMissingViewsOrCaptureErrors(t *testing.T) {
	for _, fail := range []bool{false, true} {
		m, world := newTestMatch(t)
		source := &batchSnapshotWorld{testWorld: world, omit: 2}
		want := ErrProtocol
		if fail {
			want = errors.New("capture failed")
			source.err = want
		}
		m.snapshots = source
		a, _ := joinTestMatch(t, m)
		joinTestMatch(t, m)
		result, err := m.Step()
		if !errors.Is(err, want) || len(result.Snapshots) != 0 || m.players[a].lastSnapshot != 0 {
			t.Fatalf("invalid batch was partially published: snapshots=%v last=%d err=%v", result.Snapshots, m.players[a].lastSnapshot, err)
		}
	}
}
