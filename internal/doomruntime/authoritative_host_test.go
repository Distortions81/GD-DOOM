package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
	"gddoom/internal/session"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestAuthorityHostFramePumpsControlsAndRetainsMenuRequest(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	g.input = gameInputSnapshot{pressedKeys: map[ebiten.Key]struct{}{ebiten.KeyW: {}}}
	sg := &sessionGame{g: g, rt: g, opts: g.opts}
	if err := sg.UpdateHostFrame(); err != nil {
		t.Fatal(err)
	}
	if g.clientUpdate.sequence != 1 || len(connection.sent) != 1 || connection.sent[0].Inputs[0].Command.Forward == 0 {
		t.Fatal("extra host frame did not sample and predict local movement")
	}
	g.input.justPressedKeys = map[ebiten.Key]struct{}{ebiten.KeyEscape: {}}
	if err := sg.UpdateHostFrame(); err != nil {
		t.Fatal(err)
	}
	if !g.frontendMenuRequested || sg.frontend.Active {
		t.Fatal("extra host frame consumed the session-owned menu request")
	}
	if len(g.input.pressedKeys) != 0 || len(g.input.justPressedKeys) != 0 {
		t.Fatal("consumed gameplay samples could repeat on another host frame")
	}
	if g.worldTic != 0 || sg.hostFramePhase != 0 {
		t.Fatal("extra host pump advanced authoritative world or session cadence")
	}
}

func TestAuthorityHostFrameMenuIsNeutralAndPreservesSessionInput(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	g.input = gameInputSnapshot{pressedKeys: map[ebiten.Key]struct{}{ebiten.KeyW: {}}, justPressedKeys: map[ebiten.Key]struct{}{ebiten.KeyF4: {}}}
	sg := &sessionGame{g: g, rt: g, opts: g.opts, frontend: frontendState{Active: true, MenuActive: true, Tic: 42}}
	sg.input.justPressedKeys = map[ebiten.Key]int{ebiten.KeyArrowDown: 1}
	sg.touch.latchedJustPressed = 7
	if err := sg.UpdateHostFrame(); err != nil {
		t.Fatal(err)
	}
	if len(connection.sent) != 1 || connection.sent[0].Inputs[0].Command != (demo.Tic{}) {
		t.Fatal("menu pump stopped prediction or sent active movement")
	}
	if sg.frontend.Tic != 42 || sg.input.justPressedKeys[ebiten.KeyArrowDown] != 1 || sg.touch.latchedJustPressed != 7 {
		t.Fatal("extra network pump consumed fixed-rate menu input or timers")
	}
	if len(g.input.justPressedKeys) != 0 || len(g.input.pressedKeys) != 0 || g.soundMenuRequested {
		t.Fatal("menu-owned frame retained or acted on gameplay samples")
	}
}

func TestAuthorityHostFrameDoesNothingForLocalPlay(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	sg.g.input.justPressedKeys = map[ebiten.Key]struct{}{ebiten.KeyF4: {}}
	before := sg.g.p
	if err := sg.UpdateHostFrame(); err != nil {
		t.Fatal(err)
	}
	if sg.g.p != before || sg.g.worldTic != 0 || len(sg.g.input.justPressedKeys) != 1 || sg.g.clientPrediction != nil {
		t.Fatal("network host-frame hook changed singleplayer simulation/input")
	}
}

type authorityCadenceRuntime struct {
	g                   *game
	now                 time.Time
	updates, pumps      int
	multipleCommandRuns int
}

func (r *authorityCadenceRuntime) Update() error {
	r.updates++
	return r.pump()
}
func (r *authorityCadenceRuntime) pump() error {
	r.pumps++
	before := r.g.clientUpdate.sequence
	err := r.g.updateAuthoritativeClientAt(r.now, func() demo.Tic { return demo.Tic{Forward: 25} })
	if r.g.clientUpdate.sequence-before > 1 {
		r.multipleCommandRuns++
	}
	return err
}
func (r *authorityCadenceRuntime) Draw(*ebiten.Image)         {}
func (r *authorityCadenceRuntime) Layout(w, h int) (int, int) { return w, h }

type authorityPumpedCadenceRuntime struct{ *authorityCadenceRuntime }

func (r *authorityPumpedCadenceRuntime) UpdateHostFrame() error { return r.pump() }

func TestAuthorityHostCadenceAvoidsDoubleGatedCatchup(t *testing.T) {
	a, oldGame, oldClient := authorityClientTestWorld(t, 0)
	_, fixedGame, fixedClient := authorityClientTestWorld(t, 0)
	baseline := predictionSnapshot(t, a, 1, netgame.InputAck{})
	oldClient.snapshots, fixedClient.snapshots = []netgame.Snapshot{baseline}, []netgame.Snapshot{baseline}
	old, fixed := &authorityCadenceRuntime{g: oldGame}, &authorityCadenceRuntime{g: fixedGame}
	oldHost, fixedHost := session.New(old), session.New(&authorityPumpedCadenceRuntime{fixed})
	start := time.Unix(100, 0)
	for frame := range 96 {
		elapsed := time.Duration(frame) * time.Second / 140
		// Alternate an early ordinary tic with an on-time one. Millisecond
		// host jitter is enough for two independent 35 Hz gates to produce
		// repeated zero-command/two-command bursts without any network loss.
		if frame%8 == 4 {
			elapsed -= 2 * time.Millisecond
		}
		old.now, fixed.now = start.Add(elapsed), start.Add(elapsed)
		if err := oldHost.Update(); err != nil {
			t.Fatal(err)
		}
		if err := fixedHost.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if old.multipleCommandRuns == 0 {
		t.Fatal("fixture did not reproduce the old double-gate catch-up bursts")
	}
	if fixed.multipleCommandRuns != 0 || fixed.pumps != 96 || fixed.updates != 24 || old.updates != 24 {
		t.Fatalf("network-only pumps must remove catch-up without speeding session: old=%+v fixed=%+v", old, fixed)
	}
	if fixedGame.clientUpdate.sequence < oldGame.clientUpdate.sequence || fixedGame.clientUpdate.sequence > oldGame.clientUpdate.sequence+1 {
		t.Fatalf("host pump changed command rate: old=%d fixed=%d", oldGame.clientUpdate.sequence, fixedGame.clientUpdate.sequence)
	}
}
