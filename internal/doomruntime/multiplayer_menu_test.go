package doomruntime

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

type menuAuthorityClient struct {
	*fakeAuthorityClient
	leaves    atomic.Int32
	left      chan struct{}
	leaveHook func()
}

func (c *menuAuthorityClient) Leave() error {
	if c.leaveHook != nil {
		c.leaveHook()
	}
	if c.leaves.Add(1) == 1 && c.left != nil {
		close(c.left)
	}
	return nil
}
func multiplayerMenuTestSession(t *testing.T) *sessionGame {
	t.Helper()
	m := predictionTestMap()
	opts := Options{Headless: true, GameMode: gameModeSingle, SkillLevel: 2, PlayerSlot: 1, WADHash: "local-hash", NoMonsters: true, ShowAllItems: true, AllCheats: true, CheatLevel: 2, Invulnerable: true, MusicVolume: .4, SFXVolume: .3}
	g := newGame(cloneMapForRestart(m), opts)
	sg := &sessionGame{g: g, rt: g, opts: opts, bootMap: cloneMapForRestart(m), current: m.Name, currentTemplate: cloneMapForRestart(m), frontend: frontendState{Active: true, Mode: frontendModeTitle, MenuActive: true}}
	sg.capturePersistentSettings()
	return sg
}
func multiplayerMenuTestResult() (runtimecfg.AuthorityJoinResult, *menuAuthorityClient) {
	m := predictionTestMap()
	m.Name = "MAP02"
	client := &menuAuthorityClient{fakeAuthorityClient: &fakeAuthorityClient{welcome: netgame.Welcome{Epoch: 7, PlayerID: 1}}, left: make(chan struct{})}
	manifest := authorityResumeTestManifest("MAP02")
	manifest.Mode, manifest.Skill = "deathmatch", 4
	return runtimecfg.AuthorityJoinResult{Client: client, Map: m, Manifest: manifest, MapLoader: func(netgame.MapChange) (*mapdata.Map, error) { return cloneMapForRestart(m), nil }}, client
}
func pollMenuJoin(t *testing.T, sg *sessionGame) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for sg.multiplayer.attempt != nil && time.Now().Before(deadline) {
		sg.pollAuthorityJoin()
		time.Sleep(time.Millisecond)
	}
	if sg.multiplayer.attempt != nil {
		t.Fatal("join did not settle")
	}
}
func menuKey(sg *sessionGame, key ebiten.Key) {
	sg.input = sessionInputSnapshot{justPressedKeys: map[ebiten.Key]int{key: 1}}
}

func TestMultiplayerMenuJoinLeaveRestoresRulesAndKeepsLiveSettings(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	result, client := multiplayerMenuTestResult()
	var joinCtx context.Context
	sg.opts.AuthorityJoin = func(ctx context.Context, request runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		joinCtx = ctx
		return result, nil
	}
	sg.opts.AuthorityJoinDefaults = runtimecfg.AuthorityJoinRequest{Address: "wss://example.test/netplay", Name: "Doomer"}
	sg.openFrontendMultiplayer()
	sg.openAuthorityDirectServers()
	// An attract demo can finish behind the menu while connection IO runs.
	sg.g.opts.DemoScript, sg.g.demoWorldDone = &demo.Script{}, true
	sg.beginAuthorityJoin()
	pollMenuJoin(t, sg)
	if sg.opts.AuthorityClient != client || sg.g.opts.AuthorityClient != client || sg.current != "MAP02" || sg.frontend.Active || sg.opts.GameMode != gameModeDeathmatch || sg.opts.SkillLevel != 4 || sg.opts.AllCheats || sg.opts.CheatLevel != 0 || sg.opts.Invulnerable || sg.opts.ShowAllItems || sg.g.opts.DemoScript != nil {
		t.Fatal("join did not install verified server rules and clean playback state")
	}
	if joinCtx.Err() != nil {
		t.Fatal("successful session inherited canceled opening context")
	}
	// Preference changes made during the match must survive leaving it.
	sg.g.opts.SFXVolume, sg.g.opts.MusicVolume, sg.g.opts.MouseLookSpeed = .8, .6, 1.4
	sg.g.alwaysRun = true
	sg.leaveAuthorityMatch()
	select {
	case <-client.left:
	case <-time.After(time.Second):
		t.Fatal("explicit leave not sent")
	}
	if sg.opts.AuthorityClient != nil || sg.g.opts.AuthorityClient != nil || sg.opts.AuthorityMapLoader != nil || sg.g.clientPrediction != nil || !sg.frontend.Active || !sg.frontend.MenuActive || sg.frontend.InGame || sg.current != sg.bootMap.Name {
		t.Fatal("leave did not return to a clean local title")
	}
	if sg.frontend.ItemOn != frontendMultiplayerMenuItem || sg.frontendMainMenuRow(sg.frontend.ItemOn) != 1 {
		t.Fatal("leave did not select Multiplayer in its new title-menu row")
	}
	if sg.opts.GameMode != gameModeSingle || sg.opts.SkillLevel != 2 || sg.opts.WADHash != "local-hash" || !sg.opts.AllCheats || !sg.opts.ShowAllItems || !sg.opts.Invulnerable {
		t.Fatal("leave failed to restore original local rules")
	}
	if sg.g.opts.SFXVolume != .8 || sg.g.opts.MusicVolume != .6 || sg.g.opts.MouseLookSpeed != 1.4 || !sg.g.alwaysRun {
		t.Fatal("leave discarded live presentation/input preferences")
	}
	if sg.opts.AuthorityJoin == nil || sg.multiplayer.request.Address != "wss://example.test/netplay" {
		t.Fatal("leave discarded future join settings")
	}
}

func TestMultiplayerMenuJoinIsNonblockingAndCanceledLateResultLeaves(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	result, client := multiplayerMenuTestResult()
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	sg.opts.AuthorityJoin = func(ctx context.Context, request runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		close(started)
		<-release // simulate a callback that returns success as cancellation races
		close(done)
		return result, nil
	}
	sg.opts.AuthorityJoinDefaults = runtimecfg.AuthorityJoinRequest{Address: "server:6666", Name: "Player"}
	sg.openFrontendMultiplayer()
	sg.openAuthorityDirectServers()
	menuKey(sg, ebiten.KeyEnter)
	if err := sg.tickFrontendMultiplayer(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("join worker did not start")
	}
	if sg.multiplayer.attempt == nil || sg.opts.AuthorityClient != nil {
		t.Fatal("join blocked or changed game before result")
	}
	attempt := sg.multiplayer.attempt
	menuKey(sg, ebiten.KeyEscape)
	if err := sg.tickFrontendMultiplayer(); err != nil {
		t.Fatal(err)
	}
	if sg.multiplayer.attempt != nil || attempt.ctx.Err() == nil || sg.frontend.Mode != frontendModeMultiplayer {
		t.Fatal("cancel did not release UI and opening context")
	}
	close(release)
	<-done
	select {
	case <-client.left:
	case <-time.After(time.Second):
		t.Fatal("late success was abandoned")
	}
	sg.pollAuthorityJoin()
	if sg.opts.AuthorityClient != nil || sg.current == "MAP02" {
		t.Fatal("canceled join changed current session")
	}
}

func TestMultiplayerMenuCancelCompletedResultLeavesBeforeCancelContext(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	result, client := multiplayerMenuTestResult()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempt := &authorityJoinAttempt{ctx: ctx, cancel: cancel, timer: time.NewTimer(time.Minute), result: make(chan authorityJoinReply, 1)}
	attempt.result <- authorityJoinReply{result: result}
	sg.multiplayer.attempt = attempt
	canceledAtLeave := make(chan bool, 1)
	client.leaveHook = func() { canceledAtLeave <- ctx.Err() != nil }
	sg.cancelAuthorityJoin()
	select {
	case canceled := <-canceledAtLeave:
		if canceled {
			t.Fatal("completed connection closed before explicit leave")
		}
	case <-time.After(time.Second):
		t.Fatal("completed result not cleaned up")
	}
}

func TestMultiplayerMenuEditingRetryAndUnavailableActions(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	sg.opts.AuthorityJoinDefaults = runtimecfg.AuthorityJoinRequest{Address: "old:6666", Name: "Old"}
	sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		return runtimecfg.AuthorityJoinResult{}, errors.New("server unavailable")
	}
	sg.frontend.ItemOn = frontendMultiplayerMenuItem
	menuKey(sg, ebiten.KeyEnter)
	if err := sg.tickFrontend(); err != nil {
		t.Fatal(err)
	}
	if sg.frontend.Mode != frontendModeMultiplayer {
		t.Fatal("main menu did not open multiplayer page")
	}
	if sg.multiplayer.lobby.page != authorityLobbyPageHome {
		t.Fatal("main menu did not open Multiplayer Home")
	}
	sg.openAuthorityDirectServers()
	sg.multiplayer.row = sg.multiplayer.actionRow(authorityBrowserMore)
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	sg.multiplayer.row = sg.multiplayer.actionRow(authorityBrowserEdit)
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	sg.input = sessionInputSnapshot{inputChars: []rune("wss://new.test/netplay")}
	_ = sg.tickFrontendMultiplayer()
	if sg.multiplayer.request.Address != "wss://new.test/netplay" {
		t.Fatal("editing failed to replace selected address")
	}
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontendMultiplayer()
	if sg.multiplayer.request.Address != "old:6666" {
		t.Fatal("edit cancel failed to restore prior value")
	}
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontendMultiplayer()
	sg.openAuthorityPlayerSetup(authorityLobbyPageNone)
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	sg.input = sessionInputSnapshot{inputChars: []rune(strings.Repeat("é", 100))}
	_ = sg.tickFrontendMultiplayer()
	if len(sg.multiplayer.request.Name) != 64 {
		t.Fatal("name UTF-8 byte limit not enforced")
	}
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	sg.multiplayer.lobby.row = 1
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	if !sg.multiplayer.request.Spectator {
		t.Fatal("spectator selection not toggled")
	}
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontendMultiplayer()
	sg.beginAuthorityJoin()
	pollMenuJoin(t, sg)
	if sg.multiplayer.status != "server unavailable" || sg.opts.AuthorityClient != nil {
		t.Fatal("failure did not remain retryable in menu")
	}
	result, client := multiplayerMenuTestResult()
	sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		return result, nil
	}
	sg.beginAuthorityJoin()
	pollMenuJoin(t, sg)
	defer sg.closeAuthorityMultiplayer()
	if sg.opts.AuthorityClient != client {
		t.Fatal("retry did not install new session")
	}
	for _, row := range []int{0, 2, 3} {
		if !sg.frontendMenuItemDisabled(row) {
			t.Fatalf("connected local mutation row %d enabled", row)
		}
	}
}

func TestMultiplayerMenuCLILeaveUsesOriginalLocalRules(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	_, client := multiplayerMenuTestResult()
	sg.opts.AuthorityClient, sg.g.opts.AuthorityClient = client, client
	sg.opts.GameMode, sg.opts.WADHash = gameModeDeathmatch, "network-content"
	sg.opts.AuthorityLocalRules = &runtimecfg.AuthorityLocalRules{GameMode: gameModeSingle, SkillLevel: 5, PlayerSlot: 1, WADHash: "original-cli-hash", ShowNoSkillItems: true}
	sg.leaveAuthorityMatch()
	if sg.opts.GameMode != gameModeSingle || sg.opts.SkillLevel != 5 || sg.opts.WADHash != "original-cli-hash" || !sg.opts.ShowNoSkillItems {
		t.Fatal("CLI network launch lost original local rules")
	}
	if sg.frontend.ItemOn != 0 {
		t.Fatal("CLI leave selected a nonexistent multiplayer row without a join callback")
	}
	select {
	case <-client.left:
	case <-time.After(time.Second):
		t.Fatal("CLI client not left")
	}
}

func TestMultiplayerMenuConnectedDefaultsToResumeAndKeepsLeaveClear(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	_, client := multiplayerMenuTestResult()
	sg.opts.AuthorityClient, sg.g.opts.AuthorityClient = client, client
	sg.frontend.InGame, sg.g.frontendActive = true, true
	sg.openFrontendMultiplayer()
	if sg.multiplayer.row != 0 {
		t.Fatal("connected menu did not default to Return to Game")
	}
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	if sg.frontend.Active || sg.frontend.MenuActive || sg.g.frontendActive || sg.opts.AuthorityClient != client || client.leaves.Load() != 0 {
		t.Fatal("Return to Game did not resume the current live match")
	}
	sg.frontend.Active, sg.frontend.InGame = true, true
	sg.openFrontendMultiplayer()
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontendMultiplayer()
	if sg.frontend.Mode != frontendModeTitle || !sg.frontend.Active || client.leaves.Load() != 0 {
		t.Fatal("Back from connected menu altered the match")
	}
	sg.openFrontendMultiplayer()
	menuKey(sg, ebiten.KeyArrowDown)
	_ = sg.tickFrontendMultiplayer()
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	if !sg.multiplayer.showPlayers || sg.authorityMultiplayerPageTitle() != "PLAYERS" || client.leaves.Load() != 0 || sg.opts.AuthorityClient != client {
		t.Fatal("Players should open the roster without leaving the match")
	}
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontendMultiplayer()
	if sg.multiplayer.showPlayers || sg.multiplayer.row != 1 || sg.frontend.Mode != frontendModeMultiplayer {
		t.Fatal("Back from Players should return to the connected menu")
	}
	menuKey(sg, ebiten.KeyArrowDown)
	_ = sg.tickFrontendMultiplayer()
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontendMultiplayer()
	select {
	case <-client.left:
	case <-time.After(time.Second):
		t.Fatal("Leave Match did not release the client")
	}
	if sg.opts.AuthorityClient != nil || !sg.frontend.Active || sg.frontend.InGame {
		t.Fatal("Leave Match did not return to local title")
	}
}
