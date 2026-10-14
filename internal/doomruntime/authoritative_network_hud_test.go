package doomruntime

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"gddoom/internal/netgame"
)

type hudAuthorityClient struct {
	*fakeAuthorityClient
	roster netgame.Roster
	known  bool
}

func (c *hudAuthorityClient) Roster() (netgame.Roster, bool) { return c.roster, c.known }

func TestAuthorityNetworkHUDReportsNamedPlayersPingAndReconnects(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 0)
	connection.welcome.PlayerID = 2
	connection.rtt = 42500 * time.Microsecond
	client := &hudAuthorityClient{fakeAuthorityClient: connection, known: true,
		roster: netgame.Roster{Revision: 1, Count: 3, PlayerLimit: 3}}
	client.roster.Players[0] = netgame.PlayerPresence{ID: 1, PlayerID: 2, Name: "Marine", Connected: true, PingMillis: 43}
	client.roster.Players[1] = netgame.PlayerPresence{ID: 2, PlayerID: 1, Name: "Ranger"}
	client.roster.Players[2] = netgame.PlayerPresence{ID: 3, Name: "Watcher", Spectator: true, Connected: true, PingMillis: 1000}
	g.opts.AuthorityClient = client
	g.authorityRules = &authorityRulesState{}
	g.authorityRules.Scores[1] = authorityScoreState{Generation: 1, Frags: 3, Deaths: 4}
	g.authorityRules.Scores[2] = authorityScoreState{Generation: 2, Frags: -1, Deaths: 2}
	want := []string{
		"P1 RANGER", "  RECONNECTING  F 3 D 4",
		"P2 MARINE (YOU)", "  FRAGS -1 DEATHS 2 PING 43MS",
		"SPECTATOR WATCHER", "  WATCHING  PING 1000+MS",
	}
	if got := g.authorityRosterLines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("roster=%q want=%q", got, want)
	}
	if got := g.authorityScoreLines(); len(got) != 5 || got[4] != "SPECTATORS 1 - MENU: PLAYERS" {
		t.Fatalf("held scoreboard did not summarize spectators: %q", got)
	}
	now := time.Unix(100, 0)
	if got := g.authorityNetworkSummaryAt(now); got != "PLAYERS 1/3  PING 43MS" {
		t.Fatalf("summary=%q", got)
	}
	if allocations := testing.AllocsPerRun(100, func() {
		g.authorityNetworkSummaryAt(now)
		g.authorityRosterLines()
		g.authorityScoreLines()
	}); allocations != 0 {
		t.Fatalf("unchanged HUD allocates %g times per draw", allocations)
	}
	// Scores and local RTT can update between reliable presence messages.
	g.authorityRules.Scores[2].Frags = 7
	connection.rtt = 80 * time.Millisecond
	if got := g.authorityRosterLines()[3]; got != "  FRAGS 7 DEATHS 2 PING 43MS" {
		t.Fatalf("cached stale scores: %q", got)
	}
	if got := g.authorityNetworkSummaryAt(now); got != "PLAYERS 1/3  PING 80MS" {
		t.Fatalf("cached stale local RTT: %q", got)
	}
	client.roster.Revision++
	client.roster.Players[1].Connected = true
	client.roster.Players[1].PingMillis = 62
	if got := g.authorityRosterLines()[1]; got != "  FRAGS 3 DEATHS 4 PING 62MS" {
		t.Fatalf("cached stale connection state: %q", got)
	}
	if got := g.authorityNetworkSummaryAt(now); got != "PLAYERS 2/3  PING 80MS" {
		t.Fatalf("cached stale occupancy: %q", got)
	}
	// A spectator's zero slot must not mark every observer as local.
	connection.welcome.PlayerID = 0
	if got := strings.Join(g.authorityRosterLines(), " "); strings.Contains(got, "(YOU)") {
		t.Fatalf("spectator guessed a local session identity: %q", got)
	}
}

func TestAuthorityNetworkHUDHandlesUnknownPresenceAndNames(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 0)
	g.opts.AuthorityClient = &hudAuthorityClient{fakeAuthorityClient: connection}
	if got := g.authorityRosterLines(); !reflect.DeepEqual(got, []string{"WAITING FOR PLAYER LIST"}) {
		t.Fatalf("missing roster=%q", got)
	}
	if got := g.authorityNetworkSummaryAt(time.Now()); got != "PLAYERS --  PING --" {
		t.Fatalf("unknown occupancy/ping presented as measurement: %q", got)
	}
	name, detail := authorityPlayerStatusLines(netgame.PlayerPresence{PlayerID: 1, Name: "  玩家\nmarine " + strings.Repeat("Z", 40), Connected: true}, 1, authorityScoreState{})
	if len(name) > 33 || strings.ContainsAny(name, "\n\r\t") || !strings.HasSuffix(name, "... (YOU)") || !strings.Contains(name, "???MARINE") {
		t.Fatalf("name not bounded to renderable glyphs: %q", name)
	}
	if detail != "  FRAGS 0 DEATHS 0 PING --" {
		t.Fatalf("unknown player ping=%q", detail)
	}
}

func TestAuthorityNetworkWarningsPrioritizeSnapshotStalls(t *testing.T) {
	now := time.Unix(100, 0)
	for _, test := range []struct {
		name string
		age  time.Duration
		ping int
		want string
	}{
		{"healthy", 299 * time.Millisecond, 149, ""},
		{"high ping", 100 * time.Millisecond, 150, "HIGH PING"},
		{"delayed", 300 * time.Millisecond, 180, "SERVER UPDATES DELAYED"},
		{"stalled despite low ping", time.Second, 20, "SERVER UPDATES STALLED"},
		{"future clock", -time.Second, 40, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := authorityNetworkWarning(now, now.Add(-test.age), test.ping); got != test.want {
				t.Fatalf("warning=%q want=%q", got, test.want)
			}
		})
	}
	if got := authorityNetworkWarning(now, time.Time{}, 20); got != "" {
		t.Fatalf("observer waiting for first player shown as stalled: %q", got)
	}
}

func TestAuthorityNetworkHUDOnlyAppearsDuringGameplay(t *testing.T) {
	_, g, _ := authorityClientTestWorld(t, 0)
	sg := &sessionGame{g: g}
	if !sg.authorityNetworkHUDVisible() {
		t.Fatal("missing connected gameplay HUD")
	}
	sg.frontend.Active = true
	if sg.authorityNetworkHUDVisible() {
		t.Fatal("HUD covers frontend")
	}
	sg.frontend.Active = false
	sg.intermission.state.Active = true
	if sg.authorityNetworkHUDVisible() {
		t.Fatal("HUD covers intermission")
	}
	sg.intermission.state.Active = false
	sg.quitPrompt.Active = true
	if sg.authorityNetworkHUDVisible() {
		t.Fatal("HUD covers quit prompt")
	}
	sg.quitPrompt.Active = false
	g.authorityFailure = &netgame.ConnectionStatus{State: netgame.ConnectionReconnecting}
	if sg.authorityNetworkHUDVisible() {
		t.Fatal("HUD competes with reconnect overlay")
	}
	g.authorityFailure = nil
	g.opts.AuthorityClient = nil
	if sg.authorityNetworkHUDVisible() {
		t.Fatal("network HUD appeared in singleplayer")
	}
}

func TestAuthorityNetworkObserverWaitsAfterLastPlayerLeaves(t *testing.T) {
	_, g, connection := authorityClientTestWorld(t, 0)
	connection.welcome.PlayerID = 0
	g.opts.AuthorityClient = &hudAuthorityClient{fakeAuthorityClient: connection, known: true,
		roster: netgame.Roster{Revision: 7, PlayerLimit: 4}}
	g.clientPrediction = &ClientPrediction{ready: true}
	g.clientUpdate.snapshotAt = time.Now().Add(-time.Minute)
	if got := g.authorityStatusLines(); len(got) == 0 || got[0] != "WAITING FOR PLAYERS" {
		t.Fatalf("observer's old player view hid waiting status: %q", got)
	}
}
