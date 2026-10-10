package doomruntime

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"gddoom/internal/lobby"
	"gddoom/internal/netgame"
)

const (
	authorityLobbyPageNone = iota
	authorityLobbyPageRooms
	authorityLobbyPageCreate
	authorityLobbyPageRules
	authorityLobbyPageHome
	authorityLobbyPagePlayer
	authorityLobbyPageFiles
)

const authorityLobbyVisible = 5

const (
	authorityLobbyCreateAction = iota
	authorityLobbyRefreshAction
	authorityLobbyNameAction
	authorityLobbyRoleAction
	authorityLobbyDirectAction
	authorityLobbyBackAction
)

func (sg *sessionGame) authorityLobbyActions() []int {
	return []int{authorityLobbyRefreshAction, authorityLobbyBackAction}
}

func (sg *sessionGame) authorityLobbyActionRow(action int) int {
	return len(sg.multiplayer.lobby.state.Rooms) + slices.Index(sg.authorityLobbyActions(), action)
}

type authorityLobbyReply struct {
	state lobby.State
	err   error
}
type authorityLobbyRefresh struct {
	ctx    context.Context
	cancel context.CancelFunc
	reply  chan authorityLobbyReply
}
type authorityCreateReply struct {
	room lobby.Room
	err  error
}
type authorityCreateAttempt struct {
	ctx    context.Context
	cancel context.CancelFunc
	reply  chan authorityCreateReply
}
type authorityLobbyMenu struct {
	page, row, scroll int
	selected          string
	state             lobby.State
	refresh           *authorityLobbyRefresh
	creating          *authorityCreateAttempt
	uploading         *authorityUploadAttempt
	request           lobby.CreateRequest
	requestReady      bool
	editRoomName      bool
	homeRow           int
	playerReturnPage  int
	playerReturnRow   int
	initialRoomsFocus bool
}

func (sg *sessionGame) authorityLobbyAvailable() bool {
	return sg != nil && strings.TrimSpace(sg.opts.AuthorityLobbyURL) != "" && sg.opts.AuthorityLobby != nil
}

func (sg *sessionGame) openAuthorityLobby() {
	sg.cancelAuthorityServerRefresh()
	m := &sg.multiplayer
	m.lobby.page = authorityLobbyPageRooms
	m.lobby.row = 0
	m.lobby.initialRoomsFocus = len(m.lobby.state.Rooms) == 0
	for i, room := range m.lobby.state.Rooms {
		if room.ID == m.lobby.selected {
			m.lobby.row = i
			break
		}
	}
	m.lobby.keepSelectionVisible()
	sg.refreshAuthorityLobby()
}

func (m *authorityLobbyMenu) keepSelectionVisible() {
	if m.row >= 0 && m.row < len(m.state.Rooms) {
		m.selected = m.state.Rooms[m.row].ID
		if m.row < m.scroll {
			m.scroll = m.row
		}
		if m.row >= m.scroll+authorityLobbyVisible {
			m.scroll = m.row - authorityLobbyVisible + 1
		}
	}
	m.scroll = max(0, min(m.scroll, max(0, len(m.state.Rooms)-authorityLobbyVisible)))
}

func (sg *sessionGame) cancelAuthorityLobbyRefresh() {
	m := &sg.multiplayer.lobby
	if m.refresh != nil {
		m.refresh.cancel()
		m.refresh = nil
	}
}

func (sg *sessionGame) refreshAuthorityLobby() {
	sg.cancelAuthorityLobbyRefresh()
	if !sg.authorityLobbyAvailable() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	job := &authorityLobbyRefresh{ctx: ctx, cancel: cancel, reply: make(chan authorityLobbyReply, 1)}
	sg.multiplayer.lobby.refresh = job
	sg.multiplayer.status = "CHECKING GAMES..."
	fetch, address := sg.opts.AuthorityLobby, sg.opts.AuthorityLobbyURL
	go func() { state, err := fetch(ctx, address); job.reply <- authorityLobbyReply{state, err} }()
}

func (sg *sessionGame) pollAuthorityLobby() {
	m := &sg.multiplayer.lobby
	job := m.refresh
	if job == nil {
		return
	}
	select {
	case reply := <-job.reply:
		if reply.err == nil {
			reply.err = job.ctx.Err()
		}
		job.cancel()
		m.refresh = nil
		if reply.err != nil {
			sg.multiplayer.status = "LOBBY: " + reply.err.Error()
			return
		}
		// The HTTP client validates its catalog. Keep UI storage bounded too,
		// including for alternate callback implementations.
		reply.state.Rooms = slices.Clone(reply.state.Rooms[:min(len(reply.state.Rooms), authorityBrowserLimit)])
		reply.state.Packs = slices.Clone(reply.state.Packs[:min(len(reply.state.Packs), authorityBrowserLimit)])
		action := -1
		if m.page == authorityLobbyPageRooms && !m.initialRoomsFocus && m.row >= len(m.state.Rooms) {
			index := m.row - len(m.state.Rooms)
			if index < len(sg.authorityLobbyActions()) {
				action = sg.authorityLobbyActions()[index]
			}
		}
		filesBack := m.page == authorityLobbyPageFiles && m.row == sg.authorityFilesBackRow()
		oldRow := m.row
		m.state = reply.state
		if m.page == authorityLobbyPageRooms {
			m.row = max(0, min(oldRow, len(m.state.Rooms)-1))
			for i, room := range m.state.Rooms {
				if room.ID == m.selected {
					m.row = i
					break
				}
			}
			if action >= 0 {
				m.row = sg.authorityLobbyActionRow(action)
			}
			m.keepSelectionVisible()
			m.initialRoomsFocus = false
		}
		if filesBack {
			m.row = sg.authorityFilesBackRow()
		}
		if m.requestReady && m.request.Settings.PackID == "" {
			sg.selectInitialAuthorityPack()
		}
		sg.multiplayer.status = ""
	case <-job.ctx.Done():
		job.cancel()
		m.refresh = nil
		sg.multiplayer.status = "LOBBY TIMED OUT - REFRESH TO RETRY"
	default:
	}
}

func (sg *sessionGame) authorityPackMatches(hashes []string) bool {
	return len(hashes) != 0 && len(sg.opts.AuthorityWADHashes) != 0 && slices.Equal(hashes, sg.opts.AuthorityWADHashes)
}

func (m *authorityLobbyMenu) selectedPack() (lobby.Pack, bool) {
	for _, pack := range m.state.Packs {
		if pack.ID == m.request.Settings.PackID {
			return pack, true
		}
	}
	return lobby.Pack{}, false
}

func (sg *sessionGame) openAuthorityCreate() {
	m := &sg.multiplayer.lobby
	if sg.opts.AuthorityCreateGame == nil {
		sg.multiplayer.status = "THIS LOBBY DOES NOT ALLOW CREATION"
		return
	}
	if !m.requestReady {
		pack := lobby.Pack{}
		if len(m.state.Packs) > 0 {
			pack = m.state.Packs[0]
		}
		for _, candidate := range m.state.Packs {
			if sg.authorityPackMatches(candidate.WADHashes) {
				pack = candidate
				break
			}
		}
		name := strings.TrimSpace(sg.multiplayer.request.Name) + "'s game"
		for len(name) > 64 {
			_, size := utf8.DecodeLastRuneInString(name)
			name = name[:len(name)-size]
		}
		m.request = lobby.CreateRequest{Name: name, Settings: lobby.Settings{PackID: pack.ID, Mode: "coop", Skill: 3, PlayerLimit: 4}}
		if len(pack.Maps) > 0 {
			m.request.Settings.Map = pack.Maps[0]
		}
		m.requestReady = true
	}
	m.page, m.row = authorityLobbyPageCreate, 0
	sg.multiplayer.status = ""
	if len(m.state.Packs) == 0 {
		if m.refresh == nil {
			sg.refreshAuthorityLobby()
		} else {
			sg.multiplayer.status = "CHECKING GAME FILES..."
		}
	}
}

func (sg *sessionGame) cancelAuthorityCreate() {
	m := &sg.multiplayer.lobby
	if m.creating != nil {
		m.creating.cancel()
		m.creating = nil
	}
	// Keep the request ID: a timed-out/canceled POST may already have created
	// a room. Retrying unchanged settings must recover that room, not duplicate it.
}

func (sg *sessionGame) beginAuthorityCreate() {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	if m.creating != nil || m.uploading != nil || menu.attempt != nil || sg.opts.AuthorityClient != nil || sg.opts.AuthorityCreateGame == nil {
		return
	}
	pack, ok := m.selectedPack()
	if !ok || !sg.authorityPackMatches(pack.WADHashes) {
		if !ok && m.refresh != nil {
			menu.status = "WAIT FOR GAME FILES TO LOAD"
		} else {
			menu.status = "LOAD MATCHING WAD FIRST"
		}
		return
	}
	if !slices.Contains(pack.Maps, m.request.Settings.Map) {
		menu.status = "SELECT AN AVAILABLE MAP"
		return
	}
	if strings.TrimSpace(m.request.Name) == "" {
		menu.status = "ENTER A GAME NAME"
		m.row = 0
		return
	}
	if strings.TrimSpace(menu.request.Name) == "" {
		menu.status = "ENTER YOUR PLAYER NAME"
		sg.openAuthorityPlayerSetup(authorityLobbyPageCreate)
		return
	}
	if sg.opts.AuthorityJoin == nil {
		menu.status = "MULTIPLAYER JOIN IS UNAVAILABLE"
		return
	}
	if sg.opts.LiveTicSource != nil || sg.opts.LiveTicSink != nil || sg.opts.CoopPeers != nil || sg.opts.DemoScript != nil || strings.TrimSpace(sg.opts.RecordDemoPath) != "" || strings.TrimSpace(sg.opts.DemoTracePath) != "" {
		menu.status = "FINISH THE RECORDING OR REPLAY FIRST"
		return
	}
	if m.request.RequestID == "" {
		id, err := lobby.NewRequestID()
		if err != nil {
			menu.status = err.Error()
			return
		}
		m.request.RequestID = id
	}
	sg.cancelAuthorityLobbyRefresh()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	job := &authorityCreateAttempt{ctx: ctx, cancel: cancel, reply: make(chan authorityCreateReply, 1)}
	m.creating, menu.status = job, "CREATING GAME... ESC TO CANCEL"
	create, address, request := sg.opts.AuthorityCreateGame, sg.opts.AuthorityLobbyURL, m.request
	go func() { room, err := create(ctx, address, request); job.reply <- authorityCreateReply{room, err} }()
}

func (sg *sessionGame) pollAuthorityCreate() {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	job := m.creating
	if job == nil {
		return
	}
	select {
	case reply := <-job.reply:
		if reply.err == nil {
			reply.err = job.ctx.Err()
		}
		job.cancel()
		m.creating = nil
		if reply.err != nil {
			menu.status = "CREATE: " + reply.err.Error()
			return
		}
		if reply.room.ID == "" {
			menu.status = "LOBBY RETURNED AN INVALID ROOM"
			return
		}
		index := -1
		for i, room := range m.state.Rooms {
			if room.ID == reply.room.ID {
				index = i
				break
			}
		}
		if index >= 0 {
			m.state.Rooms[index] = reply.room
		} else {
			m.state.Rooms = append([]lobby.Room{reply.room}, m.state.Rooms...)
			m.state.Rooms = m.state.Rooms[:min(len(m.state.Rooms), authorityBrowserLimit)]
			index = 0
		}
		m.page, m.row, m.selected = authorityLobbyPageRooms, index, reply.room.ID
		m.keepSelectionVisible()
		m.request.RequestID = ""
		menu.status = "GAME CREATED"
		menu.request.Spectator = false
		sg.joinAuthorityRoom(reply.room)
	case <-job.ctx.Done():
		sg.cancelAuthorityCreate()
		menu.status = "CREATE TIMED OUT - RETRY TO CHECK RESULT"
	default:
	}
}

func (sg *sessionGame) joinAuthorityRoom(room lobby.Room) {
	if strings.TrimSpace(sg.multiplayer.request.Name) == "" {
		sg.multiplayer.status = "ENTER YOUR PLAYER NAME"
		sg.openAuthorityPlayerSetup(authorityLobbyPageRooms)
		return
	}
	if room.State != "ready" {
		sg.multiplayer.status = "GAME IS " + strings.ToUpper(room.State) + " - REFRESH TO CHECK"
		return
	}
	if room.Manifest.Simulation != netgame.SimulationVersion {
		sg.multiplayer.status = "GAME REQUIRES A MATCHING ENGINE"
		return
	}
	if !sg.authorityPackMatches(room.Manifest.WADHashes) {
		sg.beginAuthorityContent(room)
		return
	}
	sg.cancelAuthorityLobbyRefresh()
	sg.multiplayer.request.Address, sg.multiplayer.joinLabel = room.Address, room.Name
	sg.beginAuthorityJoin()
}

func (m *authorityLobbyMenu) ruleRows() []string {
	modeRule := "FRIENDLY FIRE"
	if m.request.Settings.Mode == "deathmatch" {
		modeRule = "FRAG LIMIT"
	}
	return []string{"PLAYERS", "MONSTERS", "FAST MONSTERS", "RESPAWN MONSTERS", modeRule, "TIME LIMIT", "BACK"}
}

func (m *authorityLobbyMenu) adjustRule(dir int) {
	s := &m.request.Settings
	switch m.row {
	case 0:
		s.PlayerLimit = (s.PlayerLimit-1+dir+4)%4 + 1
	case 1:
		s.NoMonsters = !s.NoMonsters
	case 2:
		s.FastMonsters = !s.FastMonsters
	case 3:
		s.RespawnMonsters = !s.RespawnMonsters
	case 4:
		if s.Mode == "deathmatch" {
			s.FragLimit = max(0, min(100, s.FragLimit+dir*5))
		} else {
			s.FriendlyFire = !s.FriendlyFire
		}
	case 5:
		s.TimeLimitSeconds = max(0, min(3600, s.TimeLimitSeconds+dir*300))
	}
}
