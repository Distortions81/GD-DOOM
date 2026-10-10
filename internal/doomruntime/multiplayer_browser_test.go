package doomruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

func pollBrowser(t *testing.T, sg *sessionGame) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for sg.multiplayer.refresh != nil && time.Now().Before(deadline) {
		sg.pollAuthorityServers()
		time.Sleep(time.Millisecond)
	}
	if sg.multiplayer.refresh != nil {
		t.Fatal("discovery did not settle")
	}
}

func browserInfo(mapName string) runtimecfg.AuthorityServerInfo {
	manifest := authorityResumeTestManifest(mapName)
	return runtimecfg.AuthorityServerInfo{Manifest: manifest, Ping: 42 * time.Millisecond, Players: 2, PlayerLimit: 4, Spectators: 1, Compatible: true}
}

func TestAuthorityBrowserSeedsExplicitEntriesAndBoundsList(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	sg.opts.AuthorityServers = []runtimecfg.AuthorityServerEntry{{Label: "Default", Address: "wss://default.test/netplay"}, {Address: " wss://custom.test/netplay "}, {Address: "wss://default.test/netplay"}}
	sg.opts.AuthorityJoinDefaults.Address = "wss://custom.test/netplay"
	sg.openFrontendMultiplayer()
	if len(sg.multiplayer.servers) != 2 || sg.multiplayer.selected != 1 || sg.multiplayer.row != 1 || sg.multiplayer.request.Address != "wss://custom.test/netplay" {
		t.Fatal("browser lost explicit entries, deduplication, or selected default")
	}
	for range 50 {
		sg.opts.AuthorityServers = append(sg.opts.AuthorityServers, runtimecfg.AuthorityServerEntry{Address: fmt.Sprintf("server-%d:6671", len(sg.opts.AuthorityServers))})
	}
	sg.multiplayer = authorityMenuState{}
	sg.openFrontendMultiplayer()
	if len(sg.multiplayer.servers) != authorityBrowserLimit {
		t.Fatalf("server list size=%d", len(sg.multiplayer.servers))
	}
	sg.beginAuthorityServerEdit(false, true)
	if sg.multiplayer.editing || !strings.Contains(sg.multiplayer.status, "FULL") {
		t.Fatal("full list permitted another add")
	}
}

func TestAuthorityBrowserRefreshShowsOnlineOfflineAndWADMismatch(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	sg.opts.AuthorityServers = []runtimecfg.AuthorityServerEntry{{Address: "online"}, {Address: "offline"}, {Address: "mismatch"}}
	sg.opts.AuthorityDiscover = func(_ context.Context, address string) (runtimecfg.AuthorityServerInfo, error) {
		info := browserInfo("MAP02")
		if address == "offline" {
			return runtimecfg.AuthorityServerInfo{}, errors.New("connection timed out")
		}
		if address == "mismatch" {
			info.Compatible, info.CompatibilityError = false, "loaded WAD differs"
		}
		return info, nil
	}
	sg.openFrontendMultiplayer()
	pollBrowser(t, sg)
	servers := sg.multiplayer.servers
	if servers[0].status != authorityServerOnline || servers[1].status != authorityServerOffline || servers[2].status != authorityServerOnline || servers[2].info.Compatible {
		t.Fatal("discovery states were not preserved")
	}
	details, content := authorityServerSummary(servers[0])
	if !strings.Contains(details, "MAP02") || !strings.Contains(details, "2/4 PLAYERS") || content != "WAD: MATCH  1 WATCHING" {
		t.Fatalf("online details=%q / %q", details, content)
	}
	_, content = authorityServerSummary(servers[2])
	if !strings.Contains(content, "MISMATCH") || !strings.Contains(content, "loaded WAD differs") {
		t.Fatalf("mismatch details=%q", content)
	}
	servers[0].info.Players, servers[0].info.PlayerLimit, servers[0].info.Spectators = -1, -1, -1
	details, content = authorityServerSummary(servers[0])
	if !strings.Contains(details, "PLAYERS ?") || strings.Contains(content, "WATCHING") {
		t.Fatal("unknown counts presented as real capacity")
	}
}

func TestAuthorityBrowserRefreshBoundPersistsAcrossCancellation(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	for i := range 9 {
		sg.opts.AuthorityServers = append(sg.opts.AuthorityServers, runtimecfg.AuthorityServerEntry{Address: fmt.Sprintf("server-%d", i)})
	}
	started := make(chan struct{}, 20)
	release := make(chan struct{})
	var active, peak atomic.Int32
	sg.opts.AuthorityDiscover = func(ctx context.Context, _ string) (runtimecfg.AuthorityServerInfo, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		started <- struct{}{}
		<-release // A slow callback must not create unbounded replacement queries.
		return browserInfo("MAP01"), ctx.Err()
	}
	sg.openFrontendMultiplayer()
	defer sg.closeAuthorityMultiplayer()
	for range 4 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("discovery workers did not start")
		}
	}
	sg.refreshAuthorityServers()
	select {
	case <-started:
		t.Fatal("refresh exceeded four concurrent probes")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	pollBrowser(t, sg)
	if peak.Load() > 4 {
		t.Fatalf("concurrent probes=%d", peak.Load())
	}
}

func TestAuthorityBrowserEditedAddressIgnoresLateRefresh(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	sg.opts.AuthorityServers = []runtimecfg.AuthorityServerEntry{{Address: "old"}}
	started, release := make(chan struct{}), make(chan struct{})
	sg.opts.AuthorityDiscover = func(_ context.Context, address string) (runtimecfg.AuthorityServerInfo, error) {
		if address == "old" {
			close(started)
			<-release
			return browserInfo("MAP01"), nil
		}
		return browserInfo("MAP02"), nil
	}
	sg.openFrontendMultiplayer()
	<-started
	sg.beginAuthorityServerEdit(false, false)
	sg.multiplayer.request.Address = "new"
	if !sg.commitAuthorityServerEdit() {
		t.Fatal("edit not accepted")
	}
	close(release)
	pollBrowser(t, sg)
	server := sg.multiplayer.servers[0]
	if server.entry.Address != "new" || server.info.Manifest.Map != "MAP02" || server.status != authorityServerOnline {
		t.Fatalf("late old response replaced edited server: %+v", server)
	}
}

func TestAuthorityBrowserAddCancelPersistSelectAndJoin(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	sg.opts.AuthorityServers = []runtimecfg.AuthorityServerEntry{{Label: "Default", Address: "default"}}
	sg.opts.AuthorityJoinDefaults.Name = "Doomer"
	var saved []runtimecfg.AuthorityServerEntry
	sg.opts.OnAuthorityServersChanged = func(entries []runtimecfg.AuthorityServerEntry) error { saved = entries; return nil }
	sg.openFrontendMultiplayer()
	sg.beginAuthorityServerEdit(false, true)
	sg.multiplayer.request.Address = "discarded"
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontendMultiplayer()
	if len(sg.multiplayer.servers) != 1 || sg.multiplayer.request.Address != "default" || saved != nil {
		t.Fatal("cancel mutated saved entries")
	}
	sg.beginAuthorityServerEdit(false, true)
	sg.multiplayer.request.Address = "wss://custom.test/netplay"
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	if len(saved) != 2 || sg.multiplayer.selected != 1 || sg.multiplayer.row != 1 {
		t.Fatal("new server was not saved and selected")
	}
	requests := make(chan runtimecfg.AuthorityJoinRequest, 1)
	sg.opts.AuthorityJoin = func(_ context.Context, request runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		requests <- request
		return runtimecfg.AuthorityJoinResult{}, errors.New("stop after request")
	}
	sg.multiplayer.row = sg.multiplayer.actionRow(authorityBrowserRole)
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	sg.multiplayer.row = 1
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	pollMenuJoin(t, sg)
	request := <-requests
	if request.Address != "wss://custom.test/netplay" || request.Name != "Doomer" || !request.Spectator {
		t.Fatalf("selected join request=%+v", request)
	}
}

func TestAuthorityBrowserEscapeCancelsDiscoveryAndScrollsSelection(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	for i := range 8 {
		sg.opts.AuthorityServers = append(sg.opts.AuthorityServers, runtimecfg.AuthorityServerEntry{Address: fmt.Sprintf("server-%d", i)})
	}
	started := make(chan context.Context, 4)
	sg.opts.AuthorityDiscover = func(ctx context.Context, _ string) (runtimecfg.AuthorityServerInfo, error) {
		started <- ctx
		<-ctx.Done()
		return runtimecfg.AuthorityServerInfo{}, ctx.Err()
	}
	sg.openFrontendMultiplayer()
	ctx := <-started
	for range 6 {
		menuKey(sg, ebiten.KeyArrowDown)
		_ = sg.tickFrontendMultiplayer()
	}
	if sg.multiplayer.selected != 6 || sg.multiplayer.scroll != 3 || sg.multiplayer.request.Address != "server-6" {
		t.Fatal("list navigation did not scroll selected server into view")
	}
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontendMultiplayer()
	if ctx.Err() == nil || sg.multiplayer.refresh != nil || sg.frontend.Mode != frontendModeTitle {
		t.Fatal("leaving browser did not cancel discovery")
	}
}
