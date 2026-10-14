package doomruntime

import (
	"reflect"
	"strings"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
)

type noticeAuthorityClient struct {
	*fakeResumingAuthorityClient
	roster    netgame.Roster
	available bool
}

func (c *noticeAuthorityClient) Roster() (netgame.Roster, bool) {
	return c.roster, c.available
}

func noticeTestRoster(revision uint64, players ...netgame.PlayerPresence) netgame.Roster {
	roster := netgame.Roster{Revision: revision, Count: uint8(len(players))}
	copy(roster.Players[:], players)
	return roster
}

func noticeTestWorld(t *testing.T) (*game, *noticeAuthorityClient) {
	t.Helper()
	_, g, connection := authorityClientTestWorld(t, 0)
	client := &noticeAuthorityClient{fakeResumingAuthorityClient: &fakeResumingAuthorityClient{
		fakeAuthorityClient: connection,
		status:              netgame.ConnectionStatus{State: netgame.ConnectionConnected},
	}}
	g.opts.AuthorityClient = client
	return g, client
}

func noticeTexts(g *game) []string {
	var lines []string
	for _, entry := range g.chatHistory {
		lines = append(lines, entry.Text)
	}
	return lines
}

func TestAuthorityNoticesAnnounceRosterChangesWithoutStartupFlood(t *testing.T) {
	g, client := noticeTestWorld(t)
	alice := netgame.PlayerPresence{ID: 11, PlayerID: 1, Name: "Alice", Connected: true}
	bob := netgame.PlayerPresence{ID: 12, PlayerID: 2, Name: "Bob", Connected: true}
	viewer := netgame.PlayerPresence{ID: 13, Name: "  The\t Viewer\n", Spectator: true, Connected: true}
	g.updateAuthorityNotices()
	client.roster, client.available = noticeTestRoster(1, alice, bob), true
	g.updateAuthorityNotices()
	g.updateAuthorityNotices()
	if got := noticeTexts(g); !reflect.DeepEqual(got, []string{"CONNECTED - F6: PLAYERS"}) {
		t.Fatalf("initial roster spammed arrivals: %q", got)
	}
	client.roster = noticeTestRoster(2, alice, bob, viewer)
	g.updateAuthorityNotices()
	client.roster = noticeTestRoster(3, alice, viewer)
	g.updateAuthorityNotices()
	// Ping refreshes and repeated reads do not produce presence notices.
	client.roster.Revision++
	client.roster.Players[0].PingMillis = 42
	g.updateAuthorityNotices()
	g.updateAuthorityNotices()
	want := []string{"CONNECTED - F6: PLAYERS", "The Viewer (SPECTATOR) JOINED THE GAME", "Bob (P2) LEFT THE GAME"}
	if got := noticeTexts(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("notices=%q want=%q", got, want)
	}
	if allocations := testing.AllocsPerRun(100, g.updateAuthorityNotices); allocations != 0 {
		t.Fatalf("unchanged presence allocates %g times per update", allocations)
	}
}

func TestAuthorityNoticesDistinguishResumeAndSlotReuse(t *testing.T) {
	g, client := noticeTestWorld(t)
	alice := netgame.PlayerPresence{ID: 11, PlayerID: 1, Name: "Alice", Connected: true}
	bob := netgame.PlayerPresence{ID: 12, PlayerID: 2, Name: "Bob", Connected: true}
	client.roster, client.available = noticeTestRoster(1, alice, bob), true
	g.updateAuthorityNotices()
	g.chatHistory = nil
	bob.Connected = false
	client.roster = noticeTestRoster(2, alice, bob)
	g.updateAuthorityNotices()
	bob.Connected = true
	client.roster = noticeTestRoster(3, alice, bob)
	g.updateAuthorityNotices()
	carol := netgame.PlayerPresence{ID: 14, PlayerID: 2, Name: "Carol", Connected: true}
	client.roster = noticeTestRoster(4, alice, carol)
	g.updateAuthorityNotices()
	want := []string{"Bob (P2) LOST CONNECTION", "Bob (P2) RECONNECTED", "Bob (P2) LEFT THE GAME", "Carol (P2) JOINED THE GAME"}
	if got := noticeTexts(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("notices=%q want=%q", got, want)
	}
}

func TestAuthorityNoticesReconnectDoesNotResetPresence(t *testing.T) {
	g, client := noticeTestWorld(t)
	alice := netgame.PlayerPresence{ID: 11, PlayerID: 1, Name: "Alice", Connected: true}
	bob := netgame.PlayerPresence{ID: 12, PlayerID: 2, Name: "Bob", Connected: true}
	client.roster, client.available = noticeTestRoster(1, alice, bob), true
	g.updateAuthorityNotices()
	g.chatHistory = nil
	client.status.State = netgame.ConnectionReconnecting
	g.updateAuthorityNotices()
	client.status.Attempt++
	client.roster = noticeTestRoster(2, alice) // Ignore stale/offline telemetry.
	g.updateAuthorityNotices()
	g.resetAuthorityClientPrediction()
	client.status.State = netgame.ConnectionConnected
	client.roster = noticeTestRoster(3, alice, bob)
	g.updateAuthorityNotices()
	g.updateAuthorityNotices()
	want := []string{"CONNECTION LOST - RECONNECTING", "RECONNECTED"}
	if got := noticeTexts(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("notices=%q want=%q", got, want)
	}
	client.status.State = netgame.ConnectionDisconnected
	g.updateAuthorityNotices()
	g.updateAuthorityNotices()
	want = append(want, "DISCONNECTED")
	if got := noticeTexts(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("terminal notices=%q want=%q", got, want)
	}
}

func TestAuthorityNoticesSurviveMapChangeWithoutReannouncingConnection(t *testing.T) {
	g, client := noticeTestWorld(t)
	alice := netgame.PlayerPresence{ID: 11, PlayerID: 1, Name: "Alice", Connected: true}
	client.roster, client.available = noticeTestRoster(1, alice), true
	g.updateAuthorityNotices()
	sg := &sessionGame{g: g, rt: g, opts: g.opts, current: g.m.Name}
	sg.opts.AuthorityMapLoader = func(change netgame.MapChange) (*mapdata.Map, error) {
		m := cloneMapForRestart(g.m)
		m.Name = mapdata.MapName(change.Map)
		return m, nil
	}
	if err := sg.applyAuthorityMapChange(netgame.MapChange{Map: "MAP02", Welcome: netgame.Welcome{Epoch: 8, PlayerID: 1}}); err != nil {
		t.Fatal(err)
	}
	sg.g.updateAuthorityNotices()
	if got := noticeTexts(sg.g); len(got) != 0 {
		t.Fatalf("map change repeated initial connection: %q", got)
	}
	bob := netgame.PlayerPresence{ID: 12, PlayerID: 2, Name: "Bob", Connected: true}
	client.roster = noticeTestRoster(2, alice, bob)
	sg.g.updateAuthorityNotices()
	if got := noticeTexts(sg.g); !reflect.DeepEqual(got, []string{"Bob (P2) JOINED THE GAME"}) {
		t.Fatalf("map change lost roster history: %q", got)
	}
}

func TestAuthorityNoticeRenderingIsReadableSeparateAndBounded(t *testing.T) {
	for _, viewport := range []struct {
		width int
		scale float64
	}{{320, 1}, {640, 1}, {1280, 2}} {
		g := &game{viewW: viewport.width, viewH: viewport.width * 9 / 16}
		g.appendAuthorityNotice("OLD")
		g.appendAuthorityNotice("ONE")
		g.appendChatHistory("P2", "hello")
		g.appendAuthorityNotice("TWO")
		g.appendAuthorityNotice("THREE")
		var rendered []string
		g.drawChatOverlayText(func(text string, x, y, sx, sy float64) {
			rendered = append(rendered, text)
			if text == "P2: hello" {
				if x <= float64(g.viewW)/2 || sx != 1 || sy != 1 {
					t.Fatalf("width%d changed normal chat position/scale: x%g scale%g/%g", viewport.width, x, sx, sy)
				}
				return
			}
			if x >= float64(g.viewW)/4 || y < 20*viewport.scale || sx != viewport.scale || sy != viewport.scale {
				t.Fatalf("width%d unreadable/overlapping notice %q: x%g y%g scale%g/%g", viewport.width, text, x, y, sx, sy)
			}
		})
		if want := []string{"ONE", "TWO", "THREE", "P2: hello"}; !reflect.DeepEqual(rendered, want) {
			t.Fatalf("width%d rendered notices twice or outside latest three: %q", viewport.width, rendered)
		}
		for range chatHistoryTTL {
			g.tickChatHistory()
		}
		g.drawChatOverlayText(func(string, float64, float64, float64, float64) {
			t.Fatal("expired notice remained visible")
		})
	}
}

func TestAuthorityNoticeLongNamesWrapAwayFromChat(t *testing.T) {
	g := &game{viewW: 1280, viewH: 720}
	g.appendAuthorityNotice(strings.Repeat("A", 64) + " (P2) JOINED THE GAME")
	lines := 0
	g.drawChatOverlayText(func(text string, x, y, sx, sy float64) {
		lines++
		if x+float64(g.huTextWidth(text))*sx > float64(g.viewW)/2 {
			t.Fatalf("notice overlaps chat column: %q", text)
		}
	})
	if lines < 2 {
		t.Fatal("long notice did not wrap")
	}
}
