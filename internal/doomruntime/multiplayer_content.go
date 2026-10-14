package doomruntime

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"gddoom/internal/lobby"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/session"
)

type authorityContentReply struct {
	preparation runtimecfg.AuthorityContentPreparation
	err         error
}

type authorityContentAttempt struct {
	ctx         context.Context
	cancel      context.CancelFunc
	reply       chan authorityContentReply
	updates     chan runtimecfg.AuthorityContentProgress
	progress    runtimecfg.AuthorityContentProgress
	room        lobby.Room
	request     runtimecfg.AuthorityJoinRequest
	preparation *runtimecfg.AuthorityContentPreparation
	loadDrawn   bool
}

type authorityContentReplacement struct {
	bundle     runtimecfg.AuthorityContentBundle
	initialize func(session.Runtime)
}

func cancelContentPreparation(preparation runtimecfg.AuthorityContentPreparation) {
	if preparation.Cancel != nil {
		preparation.Cancel()
	}
}

func releaseAuthorityContentBundle(bundle runtimecfg.AuthorityContentBundle) {
	if bundle.Options.SharedPCSpeaker != nil {
		_ = bundle.Options.SharedPCSpeaker.Close()
	}
	if bundle.Options.AuthorityContentCleanup != nil {
		bundle.Options.AuthorityContentCleanup()
	}
}

func (sg *sessionGame) cancelAuthorityContent() {
	menu := &sg.multiplayer
	if job := menu.content; job != nil {
		menu.content = nil
		job.cancel()
		if job.preparation != nil {
			cancelContentPreparation(*job.preparation)
		} else {
			// A prepare callback may finish just after cancellation. Its private
			// result still owns resources, but can never install another session.
			go func() { cancelContentPreparation((<-job.reply).preparation) }()
		}
	}
	if replacement := menu.replacement; replacement != nil {
		menu.replacement = nil
		releaseAuthorityContentBundle(replacement.bundle)
	}
}

func (sg *sessionGame) beginAuthorityContent(room lobby.Room) {
	menu := &sg.multiplayer
	if sg.opts.AuthorityPrepareRoom == nil {
		menu.status = "LOAD MATCHING WAD FIRST"
		return
	}
	if menu.content != nil || menu.replacement != nil || menu.attempt != nil || sg.opts.AuthorityClient != nil {
		return
	}
	if sg.opts.LiveTicSource != nil || sg.opts.LiveTicSink != nil || sg.opts.CoopPeers != nil || sg.opts.DemoScript != nil || strings.TrimSpace(sg.opts.RecordDemoPath) != "" || strings.TrimSpace(sg.opts.DemoTracePath) != "" {
		menu.status = "FINISH THE RECORDING OR REPLAY FIRST"
		return
	}
	sg.cancelAuthorityLobbyRefresh()
	sg.cancelAuthorityServerRefresh()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	request := menu.request
	request.Address = room.Address
	room.Manifest.WADHashes = slices.Clone(room.Manifest.WADHashes)
	job := &authorityContentAttempt{ctx: ctx, cancel: cancel, room: room, request: request,
		reply: make(chan authorityContentReply, 1), updates: make(chan runtimecfg.AuthorityContentProgress, 1),
		progress: runtimecfg.AuthorityContentProgress{Stage: "CHECKING REQUIRED WADS", Name: room.Name}}
	menu.content, menu.status = job, "CHECKING REQUIRED WADS..."
	prepare, address := sg.opts.AuthorityPrepareRoom, sg.opts.AuthorityLobbyURL
	go func() {
		preparation, err := prepare(ctx, address, room, func(progress runtimecfg.AuthorityContentProgress) {
			// Keep only the newest progress sample and never stall network IO on
			// a menu frame. Old attempts have their own inaccessible channel.
			select {
			case <-job.updates:
			default:
			}
			select {
			case job.updates <- progress:
			default:
			}
		})
		job.reply <- authorityContentReply{preparation, err}
	}()
}

func (sg *sessionGame) pollAuthorityContent() {
	menu := &sg.multiplayer
	job := menu.content
	if job == nil {
		return
	}
	if err := job.ctx.Err(); err != nil {
		sg.cancelAuthorityContent()
		menu.status = "WAD DOWNLOAD TIMED OUT - TRY AGAIN"
		return
	}
	select {
	case job.progress = <-job.updates:
	default:
	}
	if job.preparation == nil {
		select {
		case reply := <-job.reply:
			if reply.err == nil && reply.preparation.Load == nil {
				reply.err = fmt.Errorf("content loader is unavailable")
			}
			if reply.err == nil {
				reply.err = job.ctx.Err()
			}
			if reply.err != nil {
				job.cancel()
				cancelContentPreparation(reply.preparation)
				menu.content, menu.status = nil, reply.err.Error()
				return
			}
			job.preparation = &reply.preparation
			job.progress = runtimecfg.AuthorityContentProgress{Stage: "LOADING WADS", Name: job.room.Name}
		default:
		}
		return
	}
	if !job.loadDrawn {
		return
	}
	current := sg.authorityContentCurrentOptions()
	bundle, err := job.preparation.Load(current)
	if err == nil && (bundle.Map == nil || bundle.Options.AuthorityJoin == nil || !slices.Equal(bundle.Options.AuthorityWADHashes, job.room.Manifest.WADHashes)) {
		err = fmt.Errorf("prepared game does not contain the room's matching WAD stack")
	}
	if err == nil {
		err = job.ctx.Err()
	}
	job.cancel()
	menu.content = nil
	if err != nil {
		releaseAuthorityContentBundle(bundle)
		cancelContentPreparation(*job.preparation)
		menu.status = err.Error()
		return
	}
	// The complete bundle now owns its resources. Closing the old session
	// must not cancel or unregister the new stack before its runtime starts.
	if bundle.Options.AuthorityContentCleanup == nil {
		bundle.Options.AuthorityContentCleanup = job.preparation.Cancel
	}
	bundle.Options.AuthorityAutoJoin = true
	bundle.Options.AuthorityJoinDefaults = job.request
	bundle.Options.AuthorityServers = make([]runtimecfg.AuthorityServerEntry, len(menu.servers))
	for i, server := range menu.servers {
		bundle.Options.AuthorityServers[i] = server.entry
	}
	settings := sg.runtimeSettingsSnapshot()
	servers, selected, scroll := slices.Clone(menu.servers), menu.selected, menu.scroll
	lobbyState, room := menu.lobby.state, job.room
	menu.replacement = &authorityContentReplacement{bundle: bundle, initialize: func(runtime session.Runtime) {
		fresh, ok := runtime.(*sessionGame)
		if !ok {
			return
		}
		fresh.applyRuntimeSettings(settings)
		fresh.applyPersistentSettingsToGame(fresh.g)
		fresh.multiplayer.servers, fresh.multiplayer.selected, fresh.multiplayer.scroll = servers, selected, scroll
		fresh.multiplayer.joinLabel = room.Name
		fresh.multiplayer.lobby.state = lobbyState
		fresh.multiplayer.lobby.selected = room.ID
		if row := slices.IndexFunc(lobbyState.Rooms, func(candidate lobby.Room) bool { return candidate.ID == room.ID }); row >= 0 {
			fresh.multiplayer.lobby.row = row
		}
		fresh.multiplayer.lobby.keepSelectionVisible()
	}}
}

// TakeAuthorityContentReplacement is consumed by doomsession.Session on the
// game thread. The wrapper closes the old runtime before constructing this one.
func (sg *sessionGame) TakeAuthorityContentReplacement() (runtimecfg.AuthorityContentBundle, func(session.Runtime), bool) {
	if sg == nil || sg.multiplayer.replacement == nil {
		return runtimecfg.AuthorityContentBundle{}, nil, false
	}
	replacement := sg.multiplayer.replacement
	sg.multiplayer.replacement = nil
	return replacement.bundle, replacement.initialize, true
}

func (sg *sessionGame) authorityContentCurrentOptions() Options {
	sg.capturePersistentSettings()
	sg.applyPersistentSettingsToOptions()
	current := sg.opts
	settings := sg.runtimeSettingsSnapshot()
	current.InitialDetailLevel, current.AutoDetail, current.InitialGammaLevel = settings.DetailLevel, settings.AutoDetail, settings.GammaLevel
	current.MouseLook, current.MouseInvert = settings.MouseLook, settings.MouseInvert
	current.AlwaysRun, current.AutoWeaponSwitch = settings.AlwaysRun, settings.AutoWeaponSwitch
	current.SourcePortThingRenderMode, current.CRTEffect = settings.ThingRenderMode, settings.CRTEffect
	if sg.g != nil {
		current.InputBindings = sg.g.opts.InputBindings
	}
	return current
}

func (sg *sessionGame) beginAuthorityAutoJoin() {
	menu := &sg.multiplayer
	request := sg.opts.AuthorityJoinDefaults
	if strings.TrimSpace(request.Name) == "" {
		request.Name = "Player"
	}
	// A room address is transient and must not be inserted into favorites by
	// initializeAuthorityBrowser's normal direct-address convenience behavior.
	menu.initialized = true
	menu.request.Name = request.Name
	sg.initializeAuthorityBrowser()
	menu.request = request
	if sg.authorityLobbyAvailable() {
		menu.lobby.page = authorityLobbyPageRooms
	}
	sg.frontend = frontendState{Active: true, MenuActive: true, Mode: frontendModeMultiplayer}
	if sg.rt != nil {
		sg.rt.sessionSetFrontendActive(true)
	}
	sg.beginAuthorityJoin()
}

func (sg *sessionGame) authorityRoomContentHint(room lobby.Room) string {
	if sg.opts.AuthorityPrepareRoom == nil {
		return "LOAD MATCHING WAD FIRST"
	}
	for _, pack := range sg.multiplayer.lobby.state.Packs {
		if pack.ID != room.Settings.PackID || len(pack.Files) == 0 {
			continue
		}
		var bytes int64
		for _, file := range pack.Files {
			if slices.Contains(sg.opts.AuthorityWADHashes, file.SHA256) {
				continue
			}
			if !file.Downloadable {
				return "ENTER: CHECK REQUIRED LOCAL WADS"
			}
			bytes += file.Size
		}
		if bytes > 0 {
			return "ENTER: GET WADS (" + authorityContentSize(bytes) + ")"
		}
		break
	}
	return "ENTER: LOAD REQUIRED WADS"
}

func authorityContentSize(bytes int64) string {
	if bytes >= 1<<20 {
		return fmt.Sprintf("%.1f MIB", float64(bytes)/(1<<20))
	}
	if bytes >= 1<<10 {
		return fmt.Sprintf("%.1f KIB", float64(bytes)/(1<<10))
	}
	return fmt.Sprintf("%d B", bytes)
}

func (sg *sessionGame) drawAuthorityContent(text func(string, int, int)) {
	job := sg.multiplayer.content
	if job == nil {
		return
	}
	fit := func(s string) string { return sg.ellipsizeIntermissionText(s, 272) }
	text(fit(strings.ToUpper(job.progress.Stage)), 24, 48)
	text(fit(job.progress.Name), 24, 74)
	if job.preparation != nil {
		text("LOADING THE COMPLETE WAD STACK", 24, 106)
		job.loadDrawn = true
	} else if job.progress.Total > 0 {
		received := job.progress.Received
		if received < 0 {
			received = 0
		}
		if received > job.progress.Total {
			received = job.progress.Total
		}
		text(authorityContentSize(received)+" / "+authorityContentSize(job.progress.Total), 24, 106)
	} else {
		text("ONLY APPROVED FILES DOWNLOAD", 24, 106)
	}
	text("ESC / BACK: CANCEL", 24, 164)
}
