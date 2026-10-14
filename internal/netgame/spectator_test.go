package netgame

import (
	"errors"
	"testing"

	"gddoom/internal/demo"
)

func joinSpectatorTest(t *testing.T, m *Match) (ConnectionID, Welcome) {
	t.Helper()
	h, welcome, err := m.Join(Hello{Compatibility: m.config.Compatibility, Name: "Observer", Spectator: true})
	if err != nil {
		t.Fatal(err)
	}
	return h, welcome
}

func TestSpectatorsHaveSeparateCapacityAndNoWorldBodies(t *testing.T) {
	m, w := newResumeTestMatch(t, 35)
	m.config.PlayerLimit = 1
	observers := make([]ConnectionID, 0, MaxSpectators)
	for range MaxSpectators {
		h, welcome := joinSpectatorTest(t, m)
		observers = append(observers, h)
		if welcome.PlayerID != 0 || welcome.ResumeToken != ([32]byte{}) || welcome.ResumeGraceTicks != 0 {
			t.Fatal("observer acquired a player identity or reservation")
		}
	}
	if m.PlayerCount() != 0 || len(w.players) != 0 {
		t.Fatal("observers created simulation bodies")
	}
	if _, _, err := m.Join(Hello{Compatibility: "test-content", Name: "extra", Spectator: true}); !errors.Is(err, ErrSpectatorsFull) {
		t.Fatal("observer capacity was unbounded")
	}
	if result, err := m.Step(); err != nil || result.Tick != 0 || len(result.Snapshots) != 0 || w.steps != 0 {
		t.Fatal("empty spectator lobby advanced simulation")
	}
	_, player := joinTestMatch(t, m)
	if player.PlayerID != 1 || m.PlayerCount() != 1 {
		t.Fatal("observers consumed gameplay slots")
	}
	if err := m.Submit(observers[0], InputBatch{Epoch: 99}); err != nil {
		t.Fatal("waiting observer heartbeat rejected")
	}
	if err := m.Submit(observers[0], InputBatch{Epoch: 99, Inputs: []Input{{Tick: 1, Command: demo.Tic{Forward: 50}}}}); !errors.Is(err, ErrProtocol) {
		t.Fatal("spectator could submit gameplay commands")
	}
	m.Suspend(observers[0])
	if m.players[observers[0]] != nil || m.PlayerCount() != 1 {
		t.Fatal("observer disconnect reserved a body or removed someone else's")
	}
}

func TestSpectatorFollowsPresentBodiesAndFallsBackAfterLeave(t *testing.T) {
	m, w := newTestMatch(t)
	a, _ := joinTestMatch(t, m)
	b, _ := joinTestMatch(t, m)
	observer, _ := joinSpectatorTest(t, m)
	result, err := m.Step()
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Snapshots[observer]; len(got.State) != 2 || got.State[1] != 1 || got.Finalized.HasTick {
		t.Fatal("observer did not initially follow first player")
	}
	if _, hasZero := w.last[0]; hasZero {
		t.Fatal("world received a spectator command")
	}
	if err := m.follow(observer, 0); err != nil {
		t.Fatal(err)
	}
	result, err = m.Step()
	if err != nil || result.Snapshots[observer].State[1] != 2 {
		t.Fatal("follow cycle did not switch camera")
	}
	if err := m.follow(observer, 4); !errors.Is(err, ErrCameraUnavailable) {
		t.Fatal("absent camera accepted")
	}
	if err := m.follow(a, 2); !errors.Is(err, ErrProtocol) {
		t.Fatal("player could select someone else's viewer baseline")
	}
	m.Leave(b)
	result, err = m.Step()
	if err != nil || result.Snapshots[observer].State[1] != 1 {
		t.Fatal("observer retained departed player camera")
	}
	m.Leave(a)
	result, err = m.Step()
	if err != nil || len(result.Snapshots) != 0 || m.players[observer] == nil {
		t.Fatal("last-player leave invalidated waiting observer")
	}
}

func TestSpectatorSurvivesInactivityAndMapChange(t *testing.T) {
	m, _ := newTestMatch(t)
	m.config.DisconnectTicks = 3
	player, _ := joinTestMatch(t, m)
	observer, welcome := joinSpectatorTest(t, m)
	for range 10 {
		if err := m.Submit(player, InputBatch{Epoch: 99}); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.players[observer] == nil {
		t.Fatal("idle observer timed out by world tic")
	}
	m.completed = true
	m.world.(*testWorld).tick = 0
	welcomes, err := m.ResetEpoch(100, "next-content")
	if err != nil || welcomes[observer].PlayerID != welcome.PlayerID || welcomes[observer].Epoch != 100 {
		t.Fatal("epoch transition lost observer identity")
	}
	result, err := m.Step()
	if err != nil || result.Snapshots[observer].Epoch != 100 || result.Snapshots[observer].State[1] != 1 {
		t.Fatal("observer did not follow into new map")
	}
}

func TestSpectatorWireRoleAndFollowBounds(t *testing.T) {
	for _, message := range []any{Hello{Compatibility: "test", Name: "watcher", Spectator: true}, Welcome{Epoch: 1, PlayerID: 0}, FollowPlayer{}, FollowPlayer{PlayerID: 4}} {
		if _, err := UnmarshalMessage(marshalProtocol(t, message)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := MarshalMessage(FollowPlayer{PlayerID: 5}); !errors.Is(err, ErrProtocol) {
		t.Fatal("invalid follow target accepted")
	}
	frame := marshalProtocol(t, Hello{Compatibility: "test", Name: "watcher", Spectator: true})
	frame[len(frame)-1] = 2
	if _, err := UnmarshalMessage(frame); !errors.Is(err, ErrProtocol) {
		t.Fatal("noncanonical spectator flag accepted")
	}
}
