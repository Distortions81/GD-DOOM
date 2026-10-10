package netgame

import (
	"errors"
	"testing"

	"gddoom/internal/demo"
)

func newResumeTestMatch(t *testing.T, grace uint32) (*Match, *testWorld) {
	t.Helper()
	m, w := newTestMatch(t)
	m.config.ResumeGraceTicks = grace
	return m, w
}

func resumeTestMatch(t *testing.T, m *Match, token [32]byte) (ConnectionID, Welcome) {
	t.Helper()
	handle, welcome, err := m.Join(Hello{Compatibility: m.config.Compatibility, Name: "returning", ResumeToken: token})
	if err != nil {
		t.Fatal(err)
	}
	return handle, welcome
}

func TestMatchResumePreservesBodyAndDiscardsOldInputs(t *testing.T) {
	m, w := newResumeTestMatch(t, 5)
	m.config.PlayerLimit = 1
	old, welcome := joinTestMatch(t, m)
	if welcome.ResumeToken == ([32]byte{}) || welcome.ResumeGraceTicks != 5 {
		t.Fatal("resume capability missing")
	}
	submitTestInput(t, m, old, 1, 10)
	submitTestInput(t, m, old, 4, 50)
	if _, err := m.Step(); err != nil {
		t.Fatal(err)
	}
	m.Suspend(old)
	if err := m.Submit(old, InputBatch{Epoch: 99}); !errors.Is(err, ErrUnknownConnection) {
		t.Fatal("suspended socket retained input authority")
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "stranger"}); !errors.Is(err, ErrMatchFull) {
		t.Fatal("grace period did not reserve body/slot")
	}
	for range 2 {
		result, err := m.Step()
		if err != nil || len(result.Snapshots) != 0 || w.last[welcome.PlayerID] != (demo.Tic{}) {
			t.Fatal("suspended player did not receive neutral simulation")
		}
	}
	fresh, resumed := resumeTestMatch(t, m, welcome.ResumeToken)
	if fresh == old || resumed.PlayerID != welcome.PlayerID || resumed.ResumeToken == welcome.ResumeToken || resumed.ServerTick != 3 || w.players[welcome.PlayerID] != 10 {
		t.Fatal("resume replaced the body or reused connection identity")
	}
	// Acknowledgments and command sequences belong to this new transport.
	if err := m.Submit(fresh, InputBatch{Epoch: 99, SnapshotAck: 1}); !errors.Is(err, ErrProtocol) {
		t.Fatal("resume accepted an old snapshot acknowledgment")
	}
	if err := m.Submit(fresh, InputBatch{Epoch: 99, Inputs: []Input{{Sequence: 0, Tick: 4, Command: demo.Tic{Forward: 5}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if w.players[welcome.PlayerID] != 15 {
		t.Fatal("queued pre-disconnect movement ran after resume")
	}
	if err := m.Submit(old, InputBatch{Epoch: 99}); !errors.Is(err, ErrUnknownConnection) {
		t.Fatal("old socket controls resumed body")
	}
	m.Suspend(old) // late transport cleanup cannot affect replacement
	if m.players[fresh].suspended {
		t.Fatal("old leave notice suspended new socket")
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "old token", ResumeToken: welcome.ResumeToken}); !errors.Is(err, ErrResumeUnavailable) {
		t.Fatal("confirmed resume left prior bearer token valid")
	}
}

func TestMatchResumeLostWelcomeCanRetryUntilConfirmed(t *testing.T) {
	m, _ := newResumeTestMatch(t, 5)
	old, welcome := joinTestMatch(t, m)
	first, _ := resumeTestMatch(t, m, welcome.ResumeToken)
	// The old socket need not have detected loss yet. Replacement is atomic.
	if m.players[old] != nil || m.PlayerCount() != 1 {
		t.Fatal("resume duplicated an active body")
	}
	m.Suspend(first)
	second, latest := resumeTestMatch(t, m, welcome.ResumeToken)
	if second == first {
		t.Fatal("retry reused stale connection identity")
	}
	if err := m.Submit(second, InputBatch{Epoch: 99}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "stale", ResumeToken: welcome.ResumeToken}); !errors.Is(err, ErrResumeUnavailable) {
		t.Fatal("old bearer survived confirmation")
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "latest", ResumeToken: latest.ResumeToken}); err != nil {
		t.Fatal("latest bearer was invalidated by acknowledgment")
	}
}

func TestMatchResumeGraceExpiresWhileEveryoneDisconnected(t *testing.T) {
	m, w := newResumeTestMatch(t, 3)
	handle, welcome := joinTestMatch(t, m)
	m.Suspend(handle)
	for tic := uint32(1); tic <= 3; tic++ {
		m.Suspend(handle) // duplicate transport notifications cannot extend grace
		if result, err := m.Step(); err != nil || result.Tick != tic {
			t.Fatalf("grace world stalled at tic %d: %v", tic, err)
		}
		if tic < 3 && m.PlayerCount() != 1 {
			t.Fatal("body expired before full grace")
		}
	}
	if m.PlayerCount() != 0 || len(w.players) != 0 {
		t.Fatal("grace expiry left a ghost body")
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "expired", ResumeToken: welcome.ResumeToken}); !errors.Is(err, ErrResumeUnavailable) {
		t.Fatal("expired token silently joined a new player")
	}
	_, fresh := joinTestMatch(t, m)
	if fresh.PlayerID != welcome.PlayerID || fresh.ResumeToken == welcome.ResumeToken {
		t.Fatal("new occupant reused prior session bearer")
	}
}

func TestMatchResumeGraceSurvivesMapEpochWithoutResettingDeadline(t *testing.T) {
	m, w := newResumeTestMatch(t, 3)
	handle, welcome := joinTestMatch(t, m)
	m.Suspend(handle)
	if _, err := m.Step(); err != nil {
		t.Fatal(err)
	}
	m.completed, w.tick = true, 0
	if _, err := m.ResetEpoch(100, "next-content"); err != nil {
		t.Fatal(err)
	}
	if m.players[handle].resumeTicks != 2 {
		t.Fatal("map transition refreshed reconnect grace")
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "wrong map", ResumeToken: welcome.ResumeToken}); !errors.Is(err, ErrCompatibility) {
		t.Fatal("old map resumed into new epoch")
	}
	fresh, resumed := resumeTestMatch(t, m, welcome.ResumeToken)
	if resumed.Epoch != 100 || resumed.PlayerID != welcome.PlayerID || fresh == handle {
		t.Fatal("resumed into wrong epoch or body")
	}
	m.Leave(fresh)
	if _, _, err := m.Join(Hello{Compatibility: "next-content", Name: "left", ResumeToken: resumed.ResumeToken}); !errors.Is(err, ErrResumeUnavailable) {
		t.Fatal("explicit leave retained session bearer")
	}
}

func TestMatchInactivityMovesPlayerIntoResumeGrace(t *testing.T) {
	m, _ := newResumeTestMatch(t, 3)
	m.config.DisconnectTicks = 2
	handle, welcome := joinTestMatch(t, m)
	for range 3 {
		if _, err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if p := m.players[handle]; p == nil || !p.suspended || p.resumeTicks != 2 {
		t.Fatal("inactivity deleted player or failed to start grace")
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "back", ResumeToken: welcome.ResumeToken}); err != nil {
		t.Fatal(err)
	}
}
