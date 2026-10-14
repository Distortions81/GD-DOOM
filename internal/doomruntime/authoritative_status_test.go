package doomruntime

import (
	"reflect"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestAuthorityLiveScoreboardHeldF6UsesServerScores(t *testing.T) {
	for _, mode := range []struct{ mode, title string }{{gameModeCoop, "CO-OP PLAYERS"}, {gameModeDeathmatch, "DEATHMATCH PLAYERS"}} {
		t.Run(mode.mode, func(t *testing.T) {
			_, g, _ := authorityClientTestWorld(t, 0)
			g.opts.GameMode = mode.mode
			g.clientPrediction = &ClientPrediction{ready: true}
			g.authorityRules = &authorityRulesState{}
			g.authorityRules.Scores[1] = authorityScoreState{Generation: 2, Frags: 7, Deaths: 3}
			g.authorityRules.Scores[3] = authorityScoreState{Generation: 1, Frags: -1, Deaths: 4}
			sg := &sessionGame{g: g}
			if lines := sg.authorityOverlayLines(); len(lines) != 0 {
				t.Fatalf("scoreboard visible without held key: %q", lines)
			}
			g.input.pressedKeys = map[ebiten.Key]struct{}{ebiten.KeyF6: {}}
			g.captureAuthorityScoreboardInput()
			g.clearSampledInput() // the host clears input before drawing
			want := []string{mode.title, "PLAYER 1  FRAGS 7  DEATHS 3", "PLAYER 3  FRAGS -1  DEATHS 4"}
			if got := sg.authorityOverlayLines(); !reflect.DeepEqual(got, want) {
				t.Fatalf("scores=%q want=%q", got, want)
			}
			// The next confirmed score is reflected without mutating gameplay.
			g.authorityRules.Scores[1].Frags = 8
			want[1] = "PLAYER 1  FRAGS 8  DEATHS 3"
			if got := sg.authorityOverlayLines(); !reflect.DeepEqual(got, want) {
				t.Fatalf("updated scores=%q want=%q", got, want)
			}
			delete(g.input.pressedKeys, ebiten.KeyF6)
			g.captureAuthorityScoreboardInput()
			g.clearSampledInput()
			if lines := sg.authorityOverlayLines(); len(lines) != 0 {
				t.Fatalf("scoreboard remained after release: %q", lines)
			}
		})
	}
}

func TestAuthorityLiveScoreboardYieldsToStatusAndMenus(t *testing.T) {
	_, g, _ := authorityClientTestWorld(t, 0)
	g.clientPrediction = &ClientPrediction{ready: true}
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1].Generation = 1
	g.input.pressedKeys = map[ebiten.Key]struct{}{ebiten.KeyF6: {}}
	g.captureAuthorityScoreboardInput()
	g.clearSampledInput()
	sg := &sessionGame{g: g}
	for _, status := range []struct {
		state netgame.ConnectionState
		first string
	}{
		{netgame.ConnectionReconnecting, "CONNECTION LOST"},
		{netgame.ConnectionDisconnected, "DISCONNECTED"},
		{netgame.ConnectionComplete, "MATCH COMPLETE"},
	} {
		g.authorityFailure = &netgame.ConnectionStatus{State: status.state, Message: "test", Attempt: 1}
		if lines := sg.authorityOverlayLines(); len(lines) == 0 || lines[0] != status.first {
			t.Fatalf("held F6 covered %s: %q", status.state, lines)
		}
	}
	g.authorityFailure = nil
	sg.frontend.Active, sg.frontend.MenuActive = true, true
	if lines := sg.authorityOverlayLines(); len(lines) != 0 {
		t.Fatalf("scoreboard covered frontend menu: %q", lines)
	}
	sg.frontend.Active, sg.frontend.MenuActive = false, false
	sg.quitPrompt.Active = true
	if lines := sg.authorityOverlayLines(); len(lines) != 0 {
		t.Fatalf("scoreboard covered quit confirmation: %q", lines)
	}
	sg.quitPrompt.Active = false
	g.clientPrediction.ready = false
	if lines := sg.authorityOverlayLines(); len(lines) == 0 || lines[0] != "WAITING FOR SERVER STATE" {
		t.Fatalf("held F6 covered initial baseline status: %q", lines)
	}
	g.opts.AuthorityClient = nil
	if lines := sg.authorityOverlayLines(); len(lines) != 0 {
		t.Fatalf("network scoreboard appeared in local game: %q", lines)
	}
}

func TestAuthorityLiveScoreboardBackgroundPumpClearsHeldState(t *testing.T) {
	a, g, connection := authorityClientTestWorld(t, 0)
	connection.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	now := time.Unix(100, 0)
	if err := g.updateAuthoritativeClientAt(now, func() demo.Tic { return demo.Tic{} }); err != nil {
		t.Fatal(err)
	}
	g.input.pressedKeys = map[ebiten.Key]struct{}{ebiten.KeyF6: {}}
	g.captureAuthorityScoreboardInput()
	g.clearSampledInput()
	if lines := g.authorityStatusLines(); len(lines) == 0 || lines[0] != "CO-OP PLAYERS" {
		t.Fatalf("frame lost scoreboard after clearing sampled input: %q", lines)
	}
	// Menus run the neutral pump instead of capturing gameplay controls. A key
	// released while a menu is open must not reappear when the menu closes.
	if err := g.updateAuthoritativeClientAt(now.Add(time.Second), nil); err != nil {
		t.Fatal(err)
	}
	if lines := g.authorityStatusLines(); len(lines) != 0 {
		t.Fatalf("background pump retained stale held scoreboard: %q", lines)
	}
}
