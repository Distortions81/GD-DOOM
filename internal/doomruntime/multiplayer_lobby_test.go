package doomruntime

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/lobby"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

func lobbyMenuTestSession(t *testing.T) *sessionGame {
	t.Helper()
	sg := multiplayerMenuTestSession(t)
	sg.opts.AuthorityLobbyURL = "https://lobby.example.test"
	sg.opts.AuthorityWADHashes = []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}
	sg.opts.AuthorityJoinDefaults = runtimecfg.AuthorityJoinRequest{Address: "wss://favorite.test/netplay", Name: "Doomer"}
	sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		return runtimecfg.AuthorityJoinResult{}, errors.New("join failed")
	}
	sg.opts.AuthorityCreateGame = func(context.Context, string, lobby.CreateRequest) (lobby.Room, error) {
		return lobby.Room{}, errors.New("create failed")
	}
	sg.opts.AuthorityLobby = func(context.Context, string) (lobby.State, error) {
		return lobby.State{}, nil
	}
	sg.multiplayer.initialized = true
	sg.multiplayer.request = sg.opts.AuthorityJoinDefaults
	sg.initializeAuthorityBrowser()
	sg.multiplayer.lobby.state = lobby.State{Version: lobby.APIVersion, MaxRooms: 8, Packs: []lobby.Pack{
		{ID: "other", Name: "Other WAD", WADHashes: []string{strings.Repeat("c", 64)}, Maps: []string{"MAP01"}},
		{ID: "loaded", Name: "Loaded WADs", WADHashes: append([]string(nil), sg.opts.AuthorityWADHashes...), Maps: []string{"E1M1", "E1M2"}},
	}}
	t.Cleanup(sg.closeAuthorityMultiplayer)
	return sg
}

func lobbyMenuRoom(sg *sessionGame, id string) lobby.Room {
	return lobby.Room{ID: id, Name: "Room " + id, Address: "wss://lobby.example.test/rooms/" + id,
		State: "ready", PlayerLimit: 4, Settings: lobby.Settings{PackID: "loaded", Map: "E1M1", Mode: "coop", Skill: 3, PlayerLimit: 4},
		Manifest: netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: append([]string(nil), sg.opts.AuthorityWADHashes...)}}
}

func settleLobby(t *testing.T, pending func() bool, poll func()) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for pending() && time.Now().Before(deadline) {
		poll()
		time.Sleep(time.Millisecond)
	}
	if pending() {
		t.Fatal("lobby operation did not settle")
	}
}

func lobbyMenuKey(t *testing.T, sg *sessionGame, key ebiten.Key) {
	t.Helper()
	menuKey(sg, key)
	if err := sg.tickFrontendMultiplayer(); err != nil {
		t.Fatal(err)
	}
}

func TestMultiplayerLobbyCreateDefaultsAndRules(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	sg.openAuthorityCreate()
	m := &sg.multiplayer.lobby
	want := lobby.Settings{PackID: "loaded", Map: "E1M1", Mode: "coop", Skill: 3, PlayerLimit: 4}
	if m.page != authorityLobbyPageCreate || m.request.Settings != want || m.request.Name != "Doomer's game" {
		t.Fatalf("wrong initial form: %+v", m.request)
	}
	m.request.RequestID = strings.Repeat("1", 32)
	m.page, m.row = authorityLobbyPageFiles, 1
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.Map != "E1M2" || m.request.RequestID != "" {
		t.Fatal("map selection did not change settings and invalidate retry ID")
	}
	m.row = 0
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.PackID != "other" || m.request.Settings.Map != "MAP01" {
		t.Fatal("changing WAD did not select a map from the new pack")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	m.row = authorityCreateModeRow
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.request.Settings.Mode != "deathmatch" || !m.request.Settings.NoMonsters || m.ruleRows()[4] != "FRAG LIMIT" {
		t.Fatal("deathmatch defaults/rules not selected")
	}
	m.page, m.row = authorityLobbyPageRules, 4
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.FragLimit != 5 {
		t.Fatal("frag limit did not advance")
	}
	m.row = 5
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.TimeLimitSeconds != 300 {
		t.Fatal("time limit did not advance")
	}
	m.page, m.row = authorityLobbyPageCreate, authorityCreateModeRow
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.request.Settings.Mode != "coop" || m.request.Settings.FragLimit != 0 || m.request.Settings.NoMonsters || m.ruleRows()[4] != "FRIENDLY FIRE" {
		t.Fatal("co-op did not reset deathmatch-only rules")
	}
	m.page, m.row = authorityLobbyPageRules, 4
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if !m.request.Settings.FriendlyFire {
		t.Fatal("friendly fire option did not change")
	}
}

func TestMultiplayerLobbyFreeDMDefaultsFollowSelectionUntilModeConfigured(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	m.state.Packs[0].ID, m.state.Packs[0].Name = "freedm", "FreeDM Deathmatch"
	sg.openAuthorityCreate()
	if m.request.Settings.Mode != "coop" || m.request.Settings.NoMonsters {
		t.Fatal("ordinary loaded game did not keep co-op defaults")
	}
	m.page, m.row = authorityLobbyPageFiles, 0
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.PackID != "freedm" || m.request.Settings.Mode != "deathmatch" || !m.request.Settings.NoMonsters {
		t.Fatal("selecting FreeDM did not apply deathmatch defaults")
	}
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.PackID != "loaded" || m.request.Settings.Mode != "coop" || m.request.Settings.NoMonsters {
		t.Fatal("automatic defaults did not follow the next game")
	}
	// An intentional choice of co-op remains a choice even when browsing
	// deathmatch-oriented content afterwards.
	m.page, m.row = authorityLobbyPageCreate, authorityCreateModeRow
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	m.page, m.row = authorityLobbyPageFiles, 0
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.PackID != "freedm" || m.request.Settings.Mode != "coop" || m.request.Settings.NoMonsters {
		t.Fatal("catalog selection overwrote explicitly chosen mode")
	}
	// Reopening the draft must not reset the explicit preference.
	sg.openAuthorityCreate()
	if m.request.Settings.Mode != "coop" || !m.modeConfigured {
		t.Fatal("reopening create lost the configured mode")
	}
}

func TestMultiplayerLobbyLoadedFreeDMAndMonsterPreference(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	m.state.Packs[1].ID, m.state.Packs[1].Name = "freedm", "FreeDM Deathmatch"
	sg.openAuthorityCreate()
	if m.request.Settings.PackID != "freedm" || m.request.Settings.Mode != "deathmatch" || !m.request.Settings.NoMonsters {
		t.Fatal("loaded FreeDM did not default to deathmatch without monsters")
	}
	m.page, m.row = authorityLobbyPageRules, 1
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.request.Settings.NoMonsters || !m.monstersConfigured {
		t.Fatal("monster preference was not recorded")
	}
	m.page, m.row = authorityLobbyPageFiles, 0
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	lobbyMenuKey(t, sg, ebiten.KeyArrowRight)
	if m.request.Settings.PackID != "freedm" || m.request.Settings.Mode != "deathmatch" || m.request.Settings.NoMonsters {
		t.Fatal("automatic mode defaults overwrote configured monster preference")
	}
}

func TestMultiplayerLobbyColdFreeDMDefaultPreservesExplicitMode(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		sg := lobbyMenuTestSession(t)
		m := &sg.multiplayer.lobby
		m.state.Packs[1].ID = "freedm"
		m.request = lobby.CreateRequest{Settings: lobby.Settings{Mode: "coop", Skill: 3, PlayerLimit: 4}}
		m.modeConfigured = explicit
		sg.selectInitialAuthorityPack()
		if m.request.Settings.PackID != "freedm" {
			t.Fatal("initial selection did not find loaded FreeDM")
		}
		wantMode := "deathmatch"
		if explicit {
			wantMode = "coop"
		}
		if m.request.Settings.Mode != wantMode || m.request.Settings.NoMonsters != !explicit {
			t.Fatal("late catalog ignored FreeDM defaults or explicit mode")
		}
	}
}

func TestMultiplayerLobbyOrderedWADStackRequiredBeforeCreateOrJoin(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	var creates, joins atomic.Int32
	sg.opts.AuthorityCreateGame = func(context.Context, string, lobby.CreateRequest) (lobby.Room, error) {
		creates.Add(1)
		return lobby.Room{}, nil
	}
	sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		joins.Add(1)
		return runtimecfg.AuthorityJoinResult{}, errors.New("unexpected")
	}
	sg.openAuthorityCreate()
	room := lobbyMenuRoom(sg, "wrong")
	room.Manifest.WADHashes[0], room.Manifest.WADHashes[1] = room.Manifest.WADHashes[1], room.Manifest.WADHashes[0]
	sg.multiplayer.lobby.state.Packs[1].WADHashes = room.Manifest.WADHashes
	sg.beginAuthorityCreate()
	if sg.multiplayer.status != "LOAD MATCHING WAD FIRST" || sg.multiplayer.lobby.creating != nil {
		t.Fatal("create accepted the same files in a different order")
	}
	sg.joinAuthorityRoom(room)
	if sg.multiplayer.status != "LOAD MATCHING WAD FIRST" || sg.multiplayer.attempt != nil || creates.Load() != 0 || joins.Load() != 0 {
		t.Fatal("mismatched WAD stack reached a network mutation/join")
	}
	if sg.authorityPackMatches(nil) {
		t.Fatal("missing WAD hashes treated as compatible")
	}
}

func downloadableLobbyTestPack(sg *sessionGame) lobby.Pack {
	return lobby.Pack{ID: "catalog", Name: "Free game", WADHashes: []string{sg.opts.AuthorityWADHashes[0], strings.Repeat("c", 64)}, Maps: []string{"E1M1"}, Files: []lobby.PackFile{
		{Name: "BASE.WAD", Size: 10 << 20, SHA256: sg.opts.AuthorityWADHashes[0]},
		{Name: "FREE.WAD", Size: 2 << 20, SHA256: strings.Repeat("c", 64), Downloadable: true},
	}}
}

func TestMultiplayerLobbyCreatesDownloadablePackThenPreparesRoom(t *testing.T) {
	for _, standalone := range []bool{false, true} {
		t.Run(map[bool]string{false: "private loaded base", true: "standalone"}[standalone], func(t *testing.T) {
			sg := lobbyMenuTestSession(t)
			pack := downloadableLobbyTestPack(sg)
			if standalone {
				pack.WADHashes, pack.Files = pack.WADHashes[1:], pack.Files[1:]
			}
			sg.multiplayer.lobby.state.Packs = []lobby.Pack{pack}
			sg.openAuthorityCreate()
			room := lobbyMenuRoom(sg, "downloadable")
			room.Settings = sg.multiplayer.lobby.request.Settings
			room.Manifest.WADHashes = slices.Clone(pack.WADHashes)
			var creates, prepares, joins atomic.Int32
			sg.opts.AuthorityPrepareRoom = func(ctx context.Context, address string, got lobby.Room, _ func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
				prepares.Add(1)
				if address != sg.opts.AuthorityLobbyURL || got.ID != room.ID || !slices.Equal(got.Manifest.WADHashes, pack.WADHashes) {
					return runtimecfg.AuthorityContentPreparation{}, errors.New("wrong room passed for download")
				}
				<-ctx.Done()
				return runtimecfg.AuthorityContentPreparation{}, ctx.Err()
			}
			sg.opts.AuthorityCreateGame = func(_ context.Context, _ string, request lobby.CreateRequest) (lobby.Room, error) {
				creates.Add(1)
				if request.Settings.PackID != pack.ID {
					return lobby.Room{}, errors.New("wrong pack selected")
				}
				return room, nil
			}
			sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
				joins.Add(1)
				return runtimecfg.AuthorityJoinResult{}, errors.New("joined before loading matching content")
			}
			if hint := sg.authorityCreateContentHint(pack); hint != "CREATE & JOIN DOWNLOADS 2.0 MIB" {
				t.Fatalf("download size hint = %s", hint)
			}
			sg.beginAuthorityCreate()
			if sg.multiplayer.lobby.creating == nil {
				t.Fatalf("downloadable pack refused: %s", sg.multiplayer.status)
			}
			settleLobby(t, func() bool { return sg.multiplayer.lobby.creating != nil }, sg.pollAuthorityCreate)
			settleLobby(t, func() bool { return prepares.Load() == 0 }, func() {})
			if creates.Load() != 1 || joins.Load() != 0 || sg.multiplayer.content == nil || sg.multiplayer.content.room.ID != room.ID {
				t.Fatal("created room did not enter content preparation before joining")
			}
		})
	}
}

func TestMultiplayerLobbyCreateRejectsUnavailableOrUnverifiedDownloads(t *testing.T) {
	for _, name := range []string{"no loader", "private missing file", "missing metadata", "metadata hash mismatch", "metadata too large"} {
		t.Run(name, func(t *testing.T) {
			sg := lobbyMenuTestSession(t)
			pack := downloadableLobbyTestPack(sg)
			sg.opts.AuthorityPrepareRoom = func(context.Context, string, lobby.Room, func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
				return runtimecfg.AuthorityContentPreparation{}, errors.New("unexpected prepare")
			}
			switch name {
			case "no loader":
				sg.opts.AuthorityPrepareRoom = nil
			case "private missing file":
				pack.Files[1].Downloadable = false
			case "missing metadata":
				pack.Files = nil
			case "metadata hash mismatch":
				pack.Files[1].SHA256 = pack.Files[0].SHA256
			case "metadata too large":
				pack.Files[1].Size = lobby.MaxDownloadFileBytes + 1
			}
			sg.multiplayer.lobby.state.Packs = []lobby.Pack{pack}
			sg.openAuthorityCreate()
			var creates atomic.Int32
			sg.opts.AuthorityCreateGame = func(context.Context, string, lobby.CreateRequest) (lobby.Room, error) {
				creates.Add(1)
				return lobby.Room{}, nil
			}
			sg.beginAuthorityCreate()
			if sg.multiplayer.lobby.creating != nil || creates.Load() != 0 || sg.multiplayer.status == "" {
				t.Fatalf("invalid download created a room: %s", sg.multiplayer.status)
			}
			if name == "private missing file" && !strings.Contains(sg.multiplayer.status, "FREE.WAD") {
				t.Fatalf("private file refusal lost file name: %s", sg.multiplayer.status)
			}
			if hint := sg.authorityCreateContentHint(pack); hint != sg.multiplayer.status {
				t.Fatalf("file page hint disagrees with create refusal: %s != %s", hint, sg.multiplayer.status)
			}
		})
	}
}

func TestMultiplayerLobbyCreateCancellationRetryUsesSameRequest(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	sg.openAuthorityCreate()
	type call struct {
		ctx     context.Context
		req     lobby.CreateRequest
		release chan struct{}
	}
	calls := make(chan call, 2)
	room := lobbyMenuRoom(sg, "recovered")
	sg.opts.AuthorityCreateGame = func(ctx context.Context, _ string, request lobby.CreateRequest) (lobby.Room, error) {
		release := make(chan struct{})
		calls <- call{ctx, request, release}
		<-release
		return room, nil
	}
	sg.beginAuthorityCreate()
	first := <-calls
	if first.req.RequestID == "" || sg.multiplayer.lobby.creating == nil {
		t.Fatal("create was blocking or missing idempotency key")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if first.ctx.Err() == nil || sg.multiplayer.lobby.creating != nil {
		t.Fatal("cancel did not detach operation and cancel IO")
	}
	sg.beginAuthorityCreate()
	second := <-calls
	if second.req != first.req {
		t.Fatalf("retry changed request: first=%+v second=%+v", first.req, second.req)
	}
	close(first.release)
	// An old successful callback cannot replace the current request or autojoin.
	sg.pollAuthorityCreate()
	if sg.multiplayer.attempt != nil || len(sg.multiplayer.lobby.state.Rooms) != 0 {
		t.Fatal("canceled result was installed")
	}
	close(second.release)
	settleLobby(t, func() bool { return sg.multiplayer.lobby.creating != nil }, sg.pollAuthorityCreate)
	pollMenuJoin(t, sg)
	m := &sg.multiplayer.lobby
	if len(m.state.Rooms) != 1 || m.state.Rooms[0].ID != room.ID || m.selected != room.ID || sg.multiplayer.status != "join failed" {
		t.Fatal("created game disappeared when automatic join failed")
	}
	if m.request.RequestID != "" {
		t.Fatal("a subsequent deliberate creation would reuse the completed request")
	}
	if len(sg.multiplayer.servers) != 1 || sg.multiplayer.servers[0].entry.Address != "wss://favorite.test/netplay" {
		t.Fatal("temporary room endpoint changed saved server list")
	}
}

func TestMultiplayerLobbyNameEditPreservesRetryUntilAcceptedChange(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	sg.openAuthorityCreate()
	m := &sg.multiplayer.lobby
	m.request.RequestID = "retry-id"
	original := m.request.Name
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	sg.input = sessionInputSnapshot{inputChars: []rune("A new game")}
	_ = sg.tickFrontendMultiplayer()
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if m.request.Name != original || m.request.RequestID != "retry-id" || sg.multiplayer.request.Name != "Doomer" {
		t.Fatal("canceling game-name edit changed retry request/player name")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	sg.input = sessionInputSnapshot{inputChars: []rune("A new game")}
	_ = sg.tickFrontendMultiplayer()
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.request.Name != "A new game" || m.request.RequestID != "" || sg.multiplayer.request.Name != "Doomer" {
		t.Fatal("accepted game-name edit did not invalidate only its creation ID")
	}
}

func TestMultiplayerLobbyRefreshPreservesSelectionAndDiscardsCanceledResults(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	a, b := lobbyMenuRoom(sg, "a"), lobbyMenuRoom(sg, "b")
	m.page, m.row, m.selected = authorityLobbyPageRooms, 1, b.ID
	m.state.Rooms = []lobby.Room{a, b}
	type call struct {
		ctx     context.Context
		release chan lobby.State
	}
	calls := make(chan call, 2)
	sg.opts.AuthorityLobby = func(ctx context.Context, _ string) (lobby.State, error) {
		release := make(chan lobby.State, 1)
		calls <- call{ctx, release}
		return <-release, nil
	}
	sg.refreshAuthorityLobby()
	first := <-calls
	sg.refreshAuthorityLobby()
	second := <-calls
	if first.ctx.Err() == nil {
		t.Fatal("replacement refresh did not cancel earlier request")
	}
	first.release <- lobby.State{Rooms: []lobby.Room{lobbyMenuRoom(sg, "stale")}}
	second.release <- lobby.State{Rooms: []lobby.Room{b, a}}
	settleLobby(t, func() bool { return m.refresh != nil }, sg.pollAuthorityLobby)
	if m.row != 0 || m.selected != b.ID || len(m.state.Rooms) != 2 || m.state.Rooms[0].ID != b.ID {
		t.Fatalf("selection/result generation lost: row=%d selected=%s rooms=%+v", m.row, m.selected, m.state.Rooms)
	}
}

func TestMultiplayerLobbyJoinReadyOnlyAndWatchRole(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	room := lobbyMenuRoom(sg, "one")
	for _, state := range []string{"starting", "ended", "failed"} {
		room.State = state
		sg.joinAuthorityRoom(room)
		if sg.multiplayer.attempt != nil || !strings.Contains(sg.multiplayer.status, strings.ToUpper(state)) {
			t.Fatalf("non-ready room %s was not blocked", state)
		}
	}
	room.State = "ready"
	sg.multiplayer.lobby.state.Rooms = []lobby.Room{room}
	sg.multiplayer.lobby.page = authorityLobbyPageRooms
	requests := make(chan runtimecfg.AuthorityJoinRequest, 2)
	sg.opts.AuthorityJoin = func(_ context.Context, request runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		requests <- request
		return runtimecfg.AuthorityJoinResult{}, errors.New("test join")
	}
	lobbyMenuKey(t, sg, ebiten.KeyS)
	request := <-requests
	pollMenuJoin(t, sg)
	if !request.Spectator || request.Address != room.Address || request.Name != "Doomer" {
		t.Fatal("watch action lost role/address/player")
	}
	sg.openAuthorityPlayerSetup(authorityLobbyPageRooms)
	sg.multiplayer.lobby.row = 1
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	sg.multiplayer.lobby.row = 0
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	request = <-requests
	pollMenuJoin(t, sg)
	if request.Spectator {
		t.Fatal("ordinary room join retained previous spectator role")
	}
}

func TestMultiplayerLobbyDirectServersAndConnectedMenuRemainSeparate(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	sg.openAuthorityMultiplayerHome()
	m.row = slices.Index(sg.authorityHomeActions(), authorityHomeDirect)
	sg.multiplayer.request.Address = "wss://temporary.test/room"
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if m.page != authorityLobbyPageNone || sg.multiplayer.request.Address != sg.multiplayer.servers[0].entry.Address {
		t.Fatal("direct server list inherited transient room address")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if m.page != authorityLobbyPageHome {
		t.Fatal("back from direct servers did not return to multiplayer home")
	}
	sg.cancelAuthorityLobbyRefresh()
	_, client := multiplayerMenuTestResult()
	sg.opts.AuthorityClient = client
	sg.openFrontendMultiplayer()
	if m.page != authorityLobbyPageNone || sg.multiplayer.row != 0 {
		t.Fatal("connected menu did not return to Return to Game / Leave Match")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if sg.frontend.MenuActive || sg.opts.AuthorityClient != client {
		t.Fatal("Return to Game was intercepted by lobby navigation")
	}
}

func TestMultiplayerLobbyUploadsSelectMatchingStackAndDeduplicate(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	m.state.UploadsEnabled = true
	pack := lobby.Pack{ID: "custom", Name: "Custom WADs", WADHashes: append([]string(nil), sg.opts.AuthorityWADHashes...), Maps: []string{"MAP11", "MAP12"}}
	sg.opts.AuthorityUploadWADs = func(ctx context.Context, address, name string) (lobby.Pack, error) {
		if address != sg.opts.AuthorityLobbyURL || name != "Custom WADs" {
			return lobby.Pack{}, errors.New("wrong upload arguments")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 2*time.Minute || time.Until(deadline) < time.Minute {
			return lobby.Pack{}, errors.New("upload deadline missing")
		}
		return pack, nil
	}
	sg.openAuthorityCreate()
	m.page, m.row = authorityLobbyPageFiles, 2
	if sg.authorityFilesBackRow() != 3 {
		t.Fatal("upload action is not in the game files page")
	}
	for i := 0; i < 2; i++ {
		m.request.RequestID = "prior-request"
		wantMap, wantID := "MAP11", ""
		if i == 1 {
			m.request.Settings.Map = "MAP12"
			wantMap, wantID = "MAP12", "prior-request"
		}
		sg.beginAuthorityUpload()
		settleLobby(t, func() bool { return m.uploading != nil }, sg.pollAuthorityUpload)
		if m.request.Settings.PackID != pack.ID || m.request.Settings.Map != wantMap || m.request.RequestID != wantID || m.page != authorityLobbyPageFiles || m.row != 1 {
			t.Fatalf("uploaded catalog pack not selected: %+v status=%s", m.request, sg.multiplayer.status)
		}
	}
	if len(m.state.Packs) != 3 || !reflect.DeepEqual(m.state.Packs[0], pack) {
		t.Fatal("re-upload duplicated/changed the content-addressed pack")
	}
	m.state.UploadsEnabled = false
	if sg.authorityFilesBackRow() != 2 {
		t.Fatal("disabled lobby still advertised upload action")
	}
}

func TestMultiplayerLobbyUploadCancellationCannotSelectLatePack(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	m.state.UploadsEnabled = true
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	done := make(chan struct{})
	sg.opts.AuthorityUploadWADs = func(ctx context.Context, _, _ string) (lobby.Pack, error) {
		started <- ctx
		<-release
		close(done)
		return lobby.Pack{ID: "late", Maps: []string{"MAP99"}, WADHashes: append([]string(nil), sg.opts.AuthorityWADHashes...)}, nil
	}
	sg.openAuthorityCreate()
	before := m.request
	sg.beginAuthorityUpload()
	ctx := <-started
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if ctx.Err() == nil || m.uploading != nil {
		t.Fatal("upload cancel failed to release context/menu")
	}
	close(release)
	<-done
	sg.pollAuthorityUpload()
	if m.request != before || len(m.state.Packs) != 2 {
		t.Fatal("late upload completion changed form")
	}
}

func TestMultiplayerLobbyUploadAvailableWithEmptyCatalog(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	m.state.Packs = nil
	m.state.UploadsEnabled = true
	sg.opts.AuthorityUploadWADs = func(context.Context, string, string) (lobby.Pack, error) { return lobby.Pack{}, errors.New("test") }
	sg.openAuthorityCreate()
	if m.page != authorityLobbyPageCreate || m.request.Settings.Mode != "coop" {
		t.Fatal("empty upload-enabled catalog prevented creating a custom pack")
	}
	m.page = authorityLobbyPageFiles
	var labels []string
	sg.drawAuthorityLobby(func(label string, _, _ int) { labels = append(labels, label) })
	if !strings.Contains(strings.Join(labels, "\n"), "UPLOAD LOADED WADS") {
		t.Fatal("empty catalog does not expose upload action")
	}
}

func TestMultiplayerLobbyDeadlineRetainsRequestForRetry(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	sg.openAuthorityCreate()
	m := &sg.multiplayer.lobby
	started := make(chan context.Context, 1)
	sg.opts.AuthorityCreateGame = func(ctx context.Context, _ string, _ lobby.CreateRequest) (lobby.Room, error) {
		started <- ctx
		<-ctx.Done()
		return lobby.Room{}, ctx.Err()
	}
	sg.beginAuthorityCreate()
	ctx := <-started
	id := m.request.RequestID
	m.creating.cancel() // expire the operation without sleeping thirty seconds
	sg.pollAuthorityCreate()
	if m.creating != nil || m.request.RequestID != id || id == "" || ctx.Err() == nil {
		t.Fatal("expired creation did not retain its retry identity")
	}
	if len(m.state.Rooms) != 0 || sg.multiplayer.attempt != nil {
		t.Fatal("expired creation joined or installed an unknown room")
	}
}

func TestMultiplayerLobbyClosingCancelsAllPendingIO(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	refreshCtx, refreshCancel := context.WithCancel(context.Background())
	createCtx, createCancel := context.WithCancel(context.Background())
	uploadCtx, uploadCancel := context.WithCancel(context.Background())
	m := &sg.multiplayer.lobby
	m.refresh = &authorityLobbyRefresh{ctx: refreshCtx, cancel: refreshCancel}
	m.creating = &authorityCreateAttempt{ctx: createCtx, cancel: createCancel}
	m.uploading = &authorityUploadAttempt{ctx: uploadCtx, cancel: uploadCancel}
	sg.closeAuthorityMultiplayer()
	if refreshCtx.Err() == nil || createCtx.Err() == nil || uploadCtx.Err() == nil || m.refresh != nil || m.creating != nil || m.uploading != nil {
		t.Fatal("session cleanup retained pending lobby operations")
	}
}

func TestMultiplayerLobbyUnavailableCreationHidden(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	sg.opts.AuthorityCreateGame = nil
	sg.openAuthorityMultiplayerHome()
	var labels []string
	sg.drawAuthorityLobby(func(label string, _, _ int) { labels = append(labels, label) })
	for _, label := range labels {
		if label == "CREATE GAME" {
			t.Fatal("read-only lobby exposed creation action")
		}
	}
	sg.multiplayer.lobby.row = slices.Index(sg.authorityHomeActions(), authorityHomeDirect)
	lobbyMenuKey(t, sg, ebiten.KeyEnter)
	if sg.multiplayer.lobby.page != authorityLobbyPageNone {
		t.Fatal("hiding creation broke remaining room-list actions")
	}
}

func TestMultiplayerLobbyStatusUsesOneLineAndRowsStayInPanel(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	m := &sg.multiplayer.lobby
	room := lobbyMenuRoom(sg, "mismatch")
	room.Manifest.WADHashes = []string{strings.Repeat("d", 64)}
	m.page, m.selected, m.state.Rooms = authorityLobbyPageRooms, room.ID, []lobby.Room{room}
	sg.multiplayer.status = "LOAD MATCHING WAD FIRST"
	statusLines := 0
	sg.drawAuthorityLobby(func(label string, _, y int) {
		if y == 188 && label != "" {
			statusLines++
		}
		if y > 188 {
			t.Fatalf("menu text outside panel: %s at y%d", label, y)
		}
	})
	if statusLines != 1 {
		t.Fatalf("status and compatibility messages overlap: %d lines", statusLines)
	}
}
