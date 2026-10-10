package doomruntime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"gddoom/internal/lobby"
	"github.com/hajimehoshi/ebiten/v2"
)

func chooseAuthorityHome(t *testing.T, sg *sessionGame, action int) {
	t.Helper()
	sg.multiplayer.lobby.row = slices.Index(sg.authorityHomeActions(), action)
	if sg.multiplayer.lobby.row < 0 {
		t.Fatal("requested home action is unavailable")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
}

func TestMultiplayerHomeAndPlayerSetupWorkWithoutLobby(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	sg.opts.AuthorityLobbyURL, sg.opts.AuthorityLobby, sg.opts.AuthorityCreateGame = "", nil, nil
	sg.openFrontendMultiplayer()
	m := &sg.multiplayer.lobby
	if m.page != authorityLobbyPageHome || !slices.Equal(sg.authorityHomeActions(), []int{authorityHomeFind, authorityHomePlayer, authorityHomeBack}) {
		t.Fatal("direct-only multiplayer did not open its focused home")
	}
	chooseAuthorityHome(t, sg, authorityHomeFind)
	if m.page != authorityLobbyPageNone {
		t.Fatal("saved servers unavailable without lobby")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if m.page != authorityLobbyPageHome {
		t.Fatal("saved servers did not return home")
	}
	chooseAuthorityHome(t, sg, authorityHomePlayer)
	if m.page != authorityLobbyPagePlayer {
		t.Fatal("player setup did not open")
	}
	m.row = 1
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if !sg.multiplayer.request.Spectator {
		t.Fatal("player setup did not set spectator role")
	}
	m.row = 0
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	sg.input = sessionInputSnapshot{inputChars: []rune("New player"), justPressedKeys: map[ebiten.Key]int{ebiten.KeyEnter: 1}}
	if err := sg.tickFrontendMultiplayer(); err != nil {
		t.Fatal(err)
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if m.page != authorityLobbyPageHome || m.row != slices.Index(sg.authorityHomeActions(), authorityHomePlayer) || sg.multiplayer.request.Name != "New player" || !sg.multiplayer.request.Spectator {
		t.Fatal("player setup lost identity, role, or return focus")
	}
	chooseAuthorityHome(t, sg, authorityHomeBack)
	if sg.frontend.Mode != frontendModeTitle || sg.frontend.ItemOn != frontendMultiplayerMenuItem {
		t.Fatal("home Back did not return to main menu")
	}
}

func TestMultiplayerCreateFilesAndRulesHaveIndependentBackHierarchy(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	sg.openAuthorityMultiplayerHome()
	chooseAuthorityHome(t, sg, authorityHomeCreate)
	m := &sg.multiplayer.lobby
	m.request.RequestID = "retry-unchanged"
	m.row = authorityCreateFilesRow
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.page != authorityLobbyPageFiles {
		t.Fatal("create did not open game files")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if m.page != authorityLobbyPageCreate || m.row != authorityCreateFilesRow || m.request.RequestID != "retry-unchanged" {
		t.Fatal("files Back changed retry identity or parent focus")
	}
	m.row = authorityCreateRulesRow
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.page != authorityLobbyPageRules {
		t.Fatal("create did not open rules")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if m.page != authorityLobbyPageCreate || m.row != authorityCreateRulesRow || m.request.RequestID != "retry-unchanged" {
		t.Fatal("rules Back changed retry identity or parent focus")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if m.page != authorityLobbyPageHome || m.row != slices.Index(sg.authorityHomeActions(), authorityHomeCreate) {
		t.Fatal("create Back did not restore home focus")
	}
	chooseAuthorityHome(t, sg, authorityHomeCreate)
	if m.request.RequestID != "retry-unchanged" || m.request.Settings.PackID != "loaded" {
		t.Fatal("reopening create reset unchanged draft")
	}
}

func TestMultiplayerColdCreateFetchesFilesAndPreservesFilesBack(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	state := m.state
	state.UploadsEnabled = true
	m.state = lobby.State{}
	started, release := make(chan struct{}), make(chan lobby.State, 1)
	sg.opts.AuthorityLobby = func(ctx context.Context, _ string) (lobby.State, error) {
		close(started)
		select {
		case result := <-release:
			return result, nil
		case <-ctx.Done():
			return lobby.State{}, ctx.Err()
		}
	}
	sg.opts.AuthorityUploadWADs = func(context.Context, string, string) (lobby.Pack, error) { return lobby.Pack{}, errors.New("unused") }
	var creates atomic.Int32
	sg.opts.AuthorityCreateGame = func(context.Context, string, lobby.CreateRequest) (lobby.Room, error) {
		creates.Add(1)
		return lobby.Room{}, nil
	}
	sg.openAuthorityMultiplayerHome()
	chooseAuthorityHome(t, sg, authorityHomeCreate)
	<-started
	if m.page != authorityLobbyPageCreate || m.refresh == nil || !m.requestReady {
		t.Fatal("cold create requires visiting Find Game first")
	}
	m.request.Name = "My draft"
	sg.beginAuthorityCreate()
	if creates.Load() != 0 || m.creating != nil {
		t.Fatal("creation started before catalog validation")
	}
	m.row = authorityCreateModeRow
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	m.row = authorityCreateFilesRow
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	m.row = sg.authorityFilesBackRow()
	release <- state
	settleLobby(t, func() bool { return m.refresh != nil }, sg.pollAuthorityLobby)
	if m.page != authorityLobbyPageFiles || m.row != sg.authorityFilesBackRow() || m.row != 3 {
		t.Fatal("new upload capability changed focused Back into Upload")
	}
	if m.request.Name != "My draft" || m.request.Settings.Mode != "deathmatch" || m.request.Settings.PackID != "loaded" || m.request.Settings.Map != "E1M1" {
		t.Fatal("catalog arrival overwrote draft or failed to choose matching files")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.page != authorityLobbyPageCreate || m.row != authorityCreateFilesRow {
		t.Fatal("remapped Back action did not return to create")
	}
}

func TestMultiplayerLeavingColdCreateCancelsLateCatalog(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	state := m.state
	m.state = lobby.State{}
	started := make(chan context.Context, 1)
	release, done := make(chan struct{}), make(chan struct{})
	sg.opts.AuthorityLobby = func(ctx context.Context, _ string) (lobby.State, error) {
		started <- ctx
		<-release
		close(done)
		return state, nil
	}
	sg.openAuthorityMultiplayerHome()
	chooseAuthorityHome(t, sg, authorityHomeCreate)
	ctx := <-started
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if ctx.Err() == nil || m.refresh != nil || m.page != authorityLobbyPageHome {
		t.Fatal("leaving create retained catalog operation")
	}
	sg.multiplayer.status = "HOME STATUS"
	close(release)
	<-done
	sg.pollAuthorityLobby()
	if len(m.state.Packs) != 0 || sg.multiplayer.status != "HOME STATUS" {
		t.Fatal("late catalog changed a retired page")
	}
}

func TestMultiplayerRoomRefreshKeepsActionAndInitialDefaultDistinct(t *testing.T) {
	for _, action := range []int{authorityLobbyRefreshAction, authorityLobbyBackAction} {
		sg := lobbyMenuTestSession(t)
		m := &sg.multiplayer.lobby
		a, b := lobbyMenuRoom(sg, "a"), lobbyMenuRoom(sg, "b")
		m.page, m.selected, m.state.Rooms = authorityLobbyPageRooms, b.ID, []lobby.Room{a, b}
		m.row = sg.authorityLobbyActionRow(action)
		sg.opts.AuthorityLobby = func(context.Context, string) (lobby.State, error) {
			return lobby.State{Rooms: []lobby.Room{b, a, lobbyMenuRoom(sg, "c")}}, nil
		}
		sg.refreshAuthorityLobby()
		settleLobby(t, func() bool { return m.refresh != nil }, sg.pollAuthorityLobby)
		if m.row != sg.authorityLobbyActionRow(action) || m.selected != b.ID {
			t.Fatal("refresh lost explicit action focus or room detail selection")
		}
	}
	for _, explicitBack := range []bool{false, true} {
		sg := lobbyMenuTestSession(t)
		m := &sg.multiplayer.lobby
		release := make(chan lobby.State, 1)
		sg.opts.AuthorityLobby = func(ctx context.Context, _ string) (lobby.State, error) {
			select {
			case state := <-release:
				return state, nil
			case <-ctx.Done():
				return lobby.State{}, ctx.Err()
			}
		}
		sg.openAuthorityLobby()
		if explicitBack {
			lobbyMenuKey(t, sg, ebiten.KeyArrowDown)
		}
		room := lobbyMenuRoom(sg, "first")
		release <- lobby.State{Rooms: []lobby.Room{room}}
		settleLobby(t, func() bool { return m.refresh != nil }, sg.pollAuthorityLobby)
		if explicitBack {
			if m.row != sg.authorityLobbyActionRow(authorityLobbyBackAction) {
				t.Fatal("first catalog overwrote explicitly focused Back")
			}
		} else if m.row != 0 || m.selected != room.ID {
			t.Fatal("initial room listing selected Refresh instead of first game")
		}
	}
}

func TestMultiplayerMissingPlayerNameReturnsToCreateOrRoom(t *testing.T) {
	for _, create := range []bool{false, true} {
		sg := lobbyMenuTestSession(t)
		sg.multiplayer.request.Name = ""
		m := &sg.multiplayer.lobby
		wantPage, wantRow := authorityLobbyPageRooms, 1
		if create {
			sg.openAuthorityCreate()
			m.request.Name = "Existing draft"
			m.request.RequestID = "existing-retry"
			m.row = authorityCreateStartRow
			wantPage, wantRow = authorityLobbyPageCreate, authorityCreateStartRow
			sg.beginAuthorityCreate()
		} else {
			m.state.Rooms = []lobby.Room{lobbyMenuRoom(sg, "a"), lobbyMenuRoom(sg, "b")}
			m.page, m.row = authorityLobbyPageRooms, 1
			sg.joinAuthorityRoom(m.state.Rooms[1])
		}
		if m.page != authorityLobbyPagePlayer || m.row != 0 {
			t.Fatal("missing player name did not open dedicated player setup")
		}
		sg.multiplayer.request.Name = "Named player"
		lobbyMenuKey(t, sg, ebiten.KeyEscape)
		if m.page != wantPage || m.row != wantRow {
			t.Fatal("player setup forgot originating action")
		}
		if create && m.request.RequestID != "existing-retry" {
			t.Fatal("identity setup invalidated unchanged creation request")
		}
	}
}

func TestMultiplayerRoomsScrollThroughFiveEntries(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	m.page = authorityLobbyPageRooms
	for _, id := range []string{"a", "b", "c", "d", "e", "f"} {
		m.state.Rooms = append(m.state.Rooms, lobbyMenuRoom(sg, id))
	}
	for i := 0; i < 5; i++ {
		lobbyMenuKey(t, sg, ebiten.KeyArrowDown)
	}
	if m.row != 5 || m.scroll != 1 || m.selected != "f" {
		t.Fatal("room navigation did not keep fifth visible entry selected")
	}
	lobbyMenuKey(t, sg, ebiten.KeyArrowDown)
	if m.row != sg.authorityLobbyActionRow(authorityLobbyRefreshAction) || m.scroll != 1 {
		t.Fatal("room list did not advance to focused Refresh")
	}
	var roomRows int
	sg.drawAuthorityLobby(func(label string, _, y int) {
		if strings.HasPrefix(label, "Room ") {
			roomRows++
			if y < 52 || y > 116 {
				t.Fatal("room row outside five-entry list")
			}
		}
	})
	if roomRows != 5 {
		t.Fatalf("visible room count=%d, want 5", roomRows)
	}
}
