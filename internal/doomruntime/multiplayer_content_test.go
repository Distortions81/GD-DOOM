package doomruntime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gddoom/internal/lobby"
	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

func contentMenuTestRoom(sg *sessionGame) lobby.Room {
	room := lobbyMenuRoom(sg, "different-wads")
	room.Manifest.WADHashes = []string{strings.Repeat("c", 64)}
	return room
}

func waitContentPreparation(t *testing.T, sg *sessionGame) {
	t.Helper()
	settleLobby(t, func() bool { return sg.multiplayer.content != nil && sg.multiplayer.content.preparation == nil }, sg.pollAuthorityContent)
	if sg.multiplayer.content == nil {
		t.Fatalf("preparation failed: %s", sg.multiplayer.status)
	}
}

func TestMultiplayerContentCanceledLatePreparationReleasesWithoutLoading(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	room := contentMenuTestRoom(sg)
	started, release, cleaned := make(chan context.Context, 1), make(chan struct{}), make(chan struct{})
	var loads atomic.Int32
	sg.opts.AuthorityPrepareRoom = func(ctx context.Context, _ string, _ lobby.Room, progress func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
		started <- ctx
		for i := int64(0); i < 10000; i++ {
			progress(runtimecfg.AuthorityContentProgress{Stage: "DOWNLOADING", Received: i, Total: 10000})
		}
		<-release
		return runtimecfg.AuthorityContentPreparation{Load: func(Options) (runtimecfg.AuthorityContentBundle, error) {
			loads.Add(1)
			return runtimecfg.AuthorityContentBundle{}, nil
		}, Cancel: func() { close(cleaned) }}, nil
	}
	sg.joinAuthorityRoom(room)
	ctx := <-started
	if sg.multiplayer.content == nil || sg.multiplayer.attempt != nil {
		t.Fatal("mismatch did not begin asynchronous content preparation")
	}
	lobbyMenuKey(t, sg, ebiten.KeyEscape)
	if ctx.Err() == nil || sg.multiplayer.content != nil {
		t.Fatal("cancel did not detach download and cancel context")
	}
	close(release)
	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("late preparation resources leaked")
	}
	if loads.Load() != 0 || sg.multiplayer.replacement != nil || !slices.Equal(sg.opts.AuthorityWADHashes, []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}) {
		t.Fatal("canceled download changed loaded content")
	}
}

func TestMultiplayerContentLoadWaitsForVisibleFrameAndTransfersOwnership(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	room := contentMenuTestRoom(sg)
	sg.multiplayer.request.Spectator = true
	sg.multiplayer.lobby.state.Rooms = []lobby.Room{lobbyMenuRoom(sg, "first"), room}
	sg.g.detailLevel, sg.g.gammaLevel = 2, 3
	sg.g.opts.MouseLook, sg.g.opts.MouseInvert, sg.g.alwaysRun = true, true, true
	sg.g.hudMessagesEnabled = false
	var loads, cleanups atomic.Int32
	var current Options
	var cleanupOnce sync.Once
	cleanup := func() { cleanupOnce.Do(func() { cleanups.Add(1) }) }
	sg.opts.AuthorityPrepareRoom = func(_ context.Context, address string, got lobby.Room, progress func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
		if address != sg.opts.AuthorityLobbyURL || got.ID != room.ID {
			return runtimecfg.AuthorityContentPreparation{}, errors.New("wrong room request")
		}
		progress(runtimecfg.AuthorityContentProgress{Stage: "DOWNLOADING", Name: "CUSTOM.WAD", Received: 8 << 20, Total: 8 << 20})
		return runtimecfg.AuthorityContentPreparation{Cancel: cleanup, Load: func(options Options) (runtimecfg.AuthorityContentBundle, error) {
			current = options
			loads.Add(1)
			options.AuthorityWADHashes = slices.Clone(room.Manifest.WADHashes)
			options.AuthorityContentCleanup = cleanup
			return runtimecfg.AuthorityContentBundle{Map: predictionTestMap(), Options: options}, nil
		}}, nil
	}
	sg.joinAuthorityRoom(room)
	waitContentPreparation(t, sg)
	sg.pollAuthorityContent()
	if loads.Load() != 0 {
		t.Fatal("asset loading began before rendering a loading frame")
	}
	var labels []string
	sg.drawAuthorityContent(func(label string, _, _ int) { labels = append(labels, label) })
	if !slices.Contains(labels, "LOADING WADS") {
		t.Fatal("loading status was not presented")
	}
	sg.pollAuthorityContent()
	if loads.Load() != 1 || cleanups.Load() != 0 || sg.multiplayer.content != nil || sg.multiplayer.replacement == nil {
		t.Fatal("successful load did not hand off resource ownership")
	}
	if current.InitialDetailLevel != 2 || current.InitialGammaLevel != 3 || !current.MouseLook || !current.MouseInvert || !current.AlwaysRun {
		t.Fatal("loader received stale startup preferences")
	}
	bundle, initialize, ok := sg.TakeAuthorityContentReplacement()
	if !ok || !bundle.Options.AuthorityAutoJoin || bundle.Options.AuthorityJoinDefaults.Address != room.Address || !bundle.Options.AuthorityJoinDefaults.Spectator || len(bundle.Options.AuthorityServers) != 1 {
		t.Fatal("replacement lost selected room, role, or favorites")
	}
	if _, _, again := sg.TakeAuthorityContentReplacement(); again {
		t.Fatal("replacement could be taken twice")
	}
	sg.closeAuthorityMultiplayer()
	if cleanups.Load() != 0 {
		t.Fatal("old runtime cleanup released new content")
	}
	fresh := multiplayerMenuTestSession(t)
	initialize(fresh)
	if fresh.g.hudMessagesEnabled || fresh.multiplayer.joinLabel != room.Name || fresh.multiplayer.lobby.selected != room.ID || fresh.multiplayer.lobby.row != 1 || fresh.multiplayer.servers[0].entry.Address != "wss://favorite.test/netplay" {
		t.Fatal("replacement lost HUD/browser state")
	}
	bundle.Options.AuthorityContentCleanup()
	if cleanups.Load() != 1 {
		t.Fatal("new runtime did not own the prepared resources")
	}
}

func TestMultiplayerContentRoomSelectionClearsPreviousError(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	private, approved := contentMenuTestRoom(sg), contentMenuTestRoom(sg)
	private.ID, approved.ID = "private", "approved"
	sg.multiplayer.lobby.state.Rooms = []lobby.Room{private, approved}
	sg.multiplayer.lobby.page = authorityLobbyPageRooms
	sg.multiplayer.lobby.keepSelectionVisible()
	sg.multiplayer.status = "LOAD PRIVATE.WAD LOCALLY FIRST"
	lobbyMenuKey(t, sg, ebiten.KeyArrowDown)
	if sg.multiplayer.lobby.selected != approved.ID || sg.multiplayer.status != "" {
		t.Fatal("new room retained the previous room's content error")
	}
	// Redrawing/updating the same selection must preserve a current error.
	sg.multiplayer.status = "APPROVED ROOM JOIN FAILED"
	sg.input = sessionInputSnapshot{}
	if err := sg.tickFrontendMultiplayer(); err != nil {
		t.Fatal(err)
	}
	if sg.multiplayer.status == "" {
		t.Fatal("unchanged selection discarded its own error")
	}
}

func TestMultiplayerContentLoadFailureKeepsCurrentSessionAndCleans(t *testing.T) {
	for _, wrongHashes := range []bool{false, true} {
		t.Run(map[bool]string{false: "loader error", true: "wrong content"}[wrongHashes], func(t *testing.T) {
			sg := lobbyMenuTestSession(t)
			original := sg.g
			var cleanups atomic.Int32
			sg.opts.AuthorityPrepareRoom = func(context.Context, string, lobby.Room, func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
				return runtimecfg.AuthorityContentPreparation{Cancel: func() { cleanups.Add(1) }, Load: func(current Options) (runtimecfg.AuthorityContentBundle, error) {
					if wrongHashes {
						return runtimecfg.AuthorityContentBundle{Map: predictionTestMap(), Options: current}, nil
					}
					return runtimecfg.AuthorityContentBundle{}, errors.New("invalid WAD resources")
				}}, nil
			}
			sg.joinAuthorityRoom(contentMenuTestRoom(sg))
			waitContentPreparation(t, sg)
			sg.drawAuthorityContent(func(string, int, int) {})
			sg.pollAuthorityContent()
			if sg.g != original || sg.multiplayer.replacement != nil || sg.multiplayer.content != nil || cleanups.Load() != 1 || sg.multiplayer.status == "" {
				t.Fatal("failed content load replaced live assets or leaked preparation")
			}
		})
	}
}

func TestMultiplayerContentMissingPrivateWADIsClearAndNonDestructive(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	original := sg.g
	sg.opts.AuthorityPrepareRoom = func(context.Context, string, lobby.Room, func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
		return runtimecfg.AuthorityContentPreparation{}, errors.New("DOOM2.WAD is not downloadable; load your own copy first")
	}
	sg.joinAuthorityRoom(contentMenuTestRoom(sg))
	settleLobby(t, func() bool { return sg.multiplayer.content != nil }, sg.pollAuthorityContent)
	if !strings.Contains(sg.multiplayer.status, "DOOM2.WAD") || !strings.Contains(sg.multiplayer.status, "not downloadable") || sg.g != original || sg.multiplayer.attempt != nil {
		t.Fatal("private-WAD refusal hid its cause or changed the active game")
	}
}

func TestMultiplayerContentCancelReadyPreparationAndAbandonedReplacement(t *testing.T) {
	for _, loaded := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "loaded"}[loaded], func(t *testing.T) {
			sg := lobbyMenuTestSession(t)
			room := contentMenuTestRoom(sg)
			var canceled atomic.Int32
			sg.opts.AuthorityPrepareRoom = func(context.Context, string, lobby.Room, func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
				return runtimecfg.AuthorityContentPreparation{Cancel: func() { canceled.Add(1) }, Load: func(current Options) (runtimecfg.AuthorityContentBundle, error) {
					current.AuthorityWADHashes = slices.Clone(room.Manifest.WADHashes)
					return runtimecfg.AuthorityContentBundle{Map: predictionTestMap(), Options: current}, nil
				}}, nil
			}
			sg.joinAuthorityRoom(room)
			waitContentPreparation(t, sg)
			if loaded {
				sg.drawAuthorityContent(func(string, int, int) {})
				sg.pollAuthorityContent()
			}
			sg.closeAuthorityMultiplayer()
			if canceled.Load() != 1 || sg.multiplayer.content != nil || sg.multiplayer.replacement != nil {
				t.Fatal("closing session leaked prepared or unconsumed loaded resources")
			}
		})
	}
}

func TestMultiplayerContentDownloadHintUsesOnlyMissingApprovedFiles(t *testing.T) {
	sg := lobbyMenuTestSession(t)
	room := contentMenuTestRoom(sg)
	room.Settings.PackID = "files"
	sg.opts.AuthorityPrepareRoom = func(context.Context, string, lobby.Room, func(runtimecfg.AuthorityContentProgress)) (runtimecfg.AuthorityContentPreparation, error) {
		return runtimecfg.AuthorityContentPreparation{}, nil
	}
	sg.multiplayer.lobby.state.Packs = []lobby.Pack{{ID: "files", Files: []lobby.PackFile{
		{Name: "BASE.WAD", SHA256: sg.opts.AuthorityWADHashes[0], Size: 10 << 20},
		{Name: "ADDON.WAD", SHA256: room.Manifest.WADHashes[0], Size: 2 << 20, Downloadable: true},
	}}}
	if hint := sg.authorityRoomContentHint(room); !strings.Contains(hint, "2.0 MIB") {
		t.Fatalf("already loaded base counted as download: %s", hint)
	}
	sg.multiplayer.lobby.state.Packs[0].Files[1].Size = 28
	if hint := sg.authorityRoomContentHint(room); !strings.Contains(hint, "28 B") {
		t.Fatalf("small nonzero download displayed as zero: %s", hint)
	}
	sg.multiplayer.lobby.state.Packs[0].Files[1].Size = 1536
	if hint := sg.authorityRoomContentHint(room); !strings.Contains(hint, "1.5 KIB") {
		t.Fatalf("kilobyte-sized download not readable: %s", hint)
	}
	sg.multiplayer.lobby.state.Packs[0].Files[1].Downloadable = false
	if hint := sg.authorityRoomContentHint(room); !strings.Contains(hint, "LOCAL WADS") {
		t.Fatalf("private file advertised as downloadable: %s", hint)
	}
}

func TestMultiplayerContentAutoJoinDoesNotSaveTransientRoom(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	requests := make(chan runtimecfg.AuthorityJoinRequest, 1)
	sg.opts.AuthorityJoin = func(_ context.Context, request runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		requests <- request
		return runtimecfg.AuthorityJoinResult{}, errors.New("test failure")
	}
	sg.opts.AuthorityServers = []runtimecfg.AuthorityServerEntry{{Label: "Favorite", Address: "wss://favorite.test/netplay"}}
	sg.opts.AuthorityJoinDefaults = runtimecfg.AuthorityJoinRequest{Address: "wss://rooms.test/temporary", Name: "Doomer", Spectator: true}
	sg.beginAuthorityAutoJoin()
	request := <-requests
	pollMenuJoin(t, sg)
	defer sg.closeAuthorityMultiplayer()
	if request != sg.opts.AuthorityJoinDefaults || len(sg.multiplayer.servers) != 1 || sg.multiplayer.servers[0].entry.Address != "wss://favorite.test/netplay" || !sg.frontend.MenuActive {
		t.Fatal("autojoin lost request or persisted a room endpoint")
	}
}
