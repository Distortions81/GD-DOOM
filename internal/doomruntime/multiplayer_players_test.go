package doomruntime

import (
	"strings"
	"testing"

	"gddoom/internal/netgame"
	"github.com/hajimehoshi/ebiten/v2"
)

type playersMenuClient struct {
	*menuAuthorityClient
	roster netgame.Roster
}

func (c *playersMenuClient) Roster() (netgame.Roster, bool) { return c.roster, true }

func TestMultiplayerPlayersPageScrollsLiveRosterAndKeepsNavigationInMatch(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	_, base := multiplayerMenuTestResult()
	client := &playersMenuClient{menuAuthorityClient: base, roster: netgame.Roster{Revision: 1, PlayerLimit: 4, Count: 6}}
	for i := range int(client.roster.Count) {
		client.roster.Players[i] = netgame.PlayerPresence{ID: uint64(i + 1), Name: "Watching", Spectator: true, Connected: true, PingMillis: 42}
	}
	sg.opts.AuthorityClient, sg.g.opts.AuthorityClient = client, client
	sg.openFrontendMultiplayer()
	sg.multiplayer.showPlayers = true
	menuKey(sg, ebiten.KeyPageDown)
	if err := sg.tickFrontendMultiplayer(); err != nil {
		t.Fatal(err)
	}
	if sg.multiplayer.playersScroll == 0 || !sg.multiplayer.showPlayers || client.leaves.Load() != 0 {
		t.Fatal("roster pagination left the match or failed to scroll")
	}
	// Membership can shrink while the page is open. Drawing and scrolling must
	// clamp against the current list without stale indices or a menu reset.
	client.roster.Revision++
	client.roster.Count = 1
	client.roster.Players[0].Name = "Last Spectator"
	var drawn []string
	sg.drawAuthorityPlayersPage(func(line string, _, y int) {
		if y < 16 || y > 178 {
			t.Fatalf("player list escaped menu panel: y=%d", y)
		}
		drawn = append(drawn, line)
	})
	if !strings.Contains(strings.ToUpper(strings.Join(drawn, " ")), "LAST SPECTATOR") {
		t.Fatalf("player list did not refresh: %q", drawn)
	}
	menuKey(sg, ebiten.KeyArrowDown)
	_ = sg.tickFrontendMultiplayer()
	if sg.multiplayer.playersScroll != 0 {
		t.Fatal("scroll did not clamp to the new roster")
	}
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	if sg.multiplayer.showPlayers || sg.multiplayer.row != 1 || sg.frontend.Mode != frontendModeMultiplayer || client.leaves.Load() != 0 {
		t.Fatal("Enter should return to Players in the connected menu")
	}
}
