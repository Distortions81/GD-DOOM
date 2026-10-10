package doomruntime

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"gddoom/internal/lobby"
	"gddoom/internal/netgame"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	authorityLobbyPageNone = iota
	authorityLobbyPageRooms
	authorityLobbyPageCreate
	authorityLobbyPageRules
)

const authorityLobbyVisible = 3

const (
	authorityLobbyCreateAction = iota
	authorityLobbyRefreshAction
	authorityLobbyNameAction
	authorityLobbyRoleAction
	authorityLobbyDirectAction
	authorityLobbyBackAction
)

func (sg *sessionGame) authorityLobbyActions() []int {
	actions := []int{authorityLobbyCreateAction, authorityLobbyRefreshAction, authorityLobbyNameAction, authorityLobbyRoleAction, authorityLobbyDirectAction, authorityLobbyBackAction}
	if sg.opts.AuthorityCreateGame == nil {
		return actions[1:]
	}
	return actions
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
}

func (sg *sessionGame) authorityLobbyAvailable() bool {
	return sg != nil && strings.TrimSpace(sg.opts.AuthorityLobbyURL) != "" && sg.opts.AuthorityLobby != nil
}

func (sg *sessionGame) openAuthorityLobby() {
	sg.cancelAuthorityServerRefresh()
	m := &sg.multiplayer
	m.lobby.page = authorityLobbyPageRooms
	m.lobby.row = 0
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
		m.state = reply.state
		if m.page == authorityLobbyPageRooms {
			m.row = 0
			for i, room := range m.state.Rooms {
				if room.ID == m.selected {
					m.row = i
					break
				}
			}
			m.keepSelectionVisible()
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
	if len(m.state.Packs) == 0 && !sg.authorityUploadAvailable() {
		sg.multiplayer.status = "REFRESH TO LOAD AVAILABLE WADS"
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
		menu.status = "LOAD MATCHING WAD FIRST"
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
		m.page, m.row = authorityLobbyPageRooms, sg.authorityLobbyActionRow(authorityLobbyNameAction)
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
		sg.multiplayer.lobby.row = sg.authorityLobbyActionRow(authorityLobbyNameAction)
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

func (sg *sessionGame) tickAuthorityLobby(escape, selectPressed bool) error {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	if escape {
		switch m.page {
		case authorityLobbyPageRooms:
			sg.cancelAuthorityLobbyRefresh()
			m.page = authorityLobbyPageNone
			sg.frontend.Mode, sg.frontend.ItemOn = frontendModeTitle, frontendMultiplayerMenuItem
		case authorityLobbyPageCreate:
			m.page, m.row = authorityLobbyPageRooms, 0
			m.keepSelectionVisible()
		case authorityLobbyPageRules:
			m.page, m.row = authorityLobbyPageCreate, 5
		}
		sg.playMenuBackSound()
		return nil
	}
	count := len(m.state.Rooms) + len(sg.authorityLobbyActions())
	if m.page == authorityLobbyPageCreate {
		count = sg.authorityCreateActionRow() + 2
	} else if m.page == authorityLobbyPageRules {
		count = len(m.ruleRows())
	}
	if sg.keyJustPressed(ebiten.KeyArrowUp) || sg.touchJustPressed(touchActionUp) {
		m.row = (m.row + count - 1) % count
		sg.playMenuMoveSound()
	}
	if sg.keyJustPressed(ebiten.KeyArrowDown) || sg.keyJustPressed(ebiten.KeyTab) || sg.touchJustPressed(touchActionDown) {
		m.row = (m.row + 1) % count
		sg.playMenuMoveSound()
	}
	if m.page == authorityLobbyPageRooms {
		previousRoom := m.selected
		m.keepSelectionVisible()
		if m.selected != previousRoom {
			// A join or content error describes the previously selected room.
			// Let the newly selected room show its own WAD availability hint.
			menu.status = ""
		}
		if sg.keyJustPressed(ebiten.KeyF5) {
			sg.refreshAuthorityLobby()
		}
		if m.row == sg.authorityLobbyActionRow(authorityLobbyRoleAction) && (sg.keyJustPressed(ebiten.KeyArrowLeft) || sg.keyJustPressed(ebiten.KeyArrowRight) || sg.touchJustPressed(touchActionLeft) || sg.touchJustPressed(touchActionRight)) {
			menu.request.Spectator = !menu.request.Spectator
			sg.playMenuMoveSound()
			return nil
		}
		watch := sg.keyJustPressed(ebiten.KeyS) && m.row < len(m.state.Rooms)
		if !selectPressed && !watch {
			return nil
		}
		sg.playMenuConfirmSound()
		if m.row < len(m.state.Rooms) {
			if watch {
				menu.request.Spectator = true
			}
			sg.joinAuthorityRoom(m.state.Rooms[m.row])
			return nil
		}
		switch sg.authorityLobbyActions()[m.row-len(m.state.Rooms)] {
		case authorityLobbyCreateAction:
			sg.openAuthorityCreate()
		case authorityLobbyRefreshAction:
			sg.refreshAuthorityLobby()
		case authorityLobbyNameAction:
			sg.beginAuthorityServerEdit(true, false)
		case authorityLobbyRoleAction:
			menu.request.Spectator = !menu.request.Spectator
		case authorityLobbyDirectAction:
			sg.cancelAuthorityLobbyRefresh()
			m.page = authorityLobbyPageNone
			menu.joinLabel = ""
			menu.showServers()
			menu.selectServer(menu.selected)
			sg.refreshAuthorityServers()
		case authorityLobbyBackAction:
			return sg.tickAuthorityLobby(true, false)
		}
		return nil
	}
	dir := 0
	if sg.keyJustPressed(ebiten.KeyArrowLeft) || sg.touchJustPressed(touchActionLeft) {
		dir = -1
	}
	if sg.keyJustPressed(ebiten.KeyArrowRight) || sg.touchJustPressed(touchActionRight) {
		dir = 1
	}
	if selectPressed && dir == 0 {
		dir = 1
	}
	if dir == 0 {
		return nil
	}
	before := m.request.Settings
	if m.page == authorityLobbyPageRules {
		if m.row == len(m.ruleRows())-1 && selectPressed {
			m.page, m.row = authorityLobbyPageCreate, 5
		} else {
			m.adjustRule(dir)
		}
	} else {
		if m.row >= 6 {
			if selectPressed {
				switch m.row {
				case sg.authorityCreateActionRow():
					sg.beginAuthorityCreate()
				case sg.authorityCreateActionRow() + 1:
					return sg.tickAuthorityLobby(true, false)
				default:
					sg.beginAuthorityUpload()
				}
			}
			sg.playMenuMoveSound()
			return nil
		}
		switch m.row {
		case 0:
			if selectPressed {
				menu.editing, menu.replace, menu.editName, m.editRoomName = true, true, true, true
				menu.editOriginal = m.request.Name
			}
		case 1:
			index := 0
			for i, pack := range m.state.Packs {
				if pack.ID == m.request.Settings.PackID {
					index = i
				}
			}
			if len(m.state.Packs) > 0 {
				pack := m.state.Packs[(index+dir+len(m.state.Packs))%len(m.state.Packs)]
				m.request.Settings.PackID, m.request.Settings.Map = pack.ID, ""
				if len(pack.Maps) > 0 {
					m.request.Settings.Map = pack.Maps[0]
				}
			}
		case 2:
			if pack, ok := m.selectedPack(); ok && len(pack.Maps) > 0 {
				index := max(0, slices.Index(pack.Maps, m.request.Settings.Map))
				m.request.Settings.Map = pack.Maps[(index+dir+len(pack.Maps))%len(pack.Maps)]
			}
		case 3:
			if m.request.Settings.Mode == "coop" {
				m.request.Settings.Mode, m.request.Settings.NoMonsters = "deathmatch", true
				m.request.Settings.FriendlyFire = false
			} else {
				m.request.Settings.Mode, m.request.Settings.NoMonsters = "coop", false
				m.request.Settings.FragLimit = 0
			}
		case 4:
			m.request.Settings.Skill = (m.request.Settings.Skill-1+dir+5)%5 + 1
		case 5:
			if selectPressed {
				m.page, m.row = authorityLobbyPageRules, 0
			}
		}
	}
	if before != m.request.Settings {
		m.request.RequestID = ""
		menu.status = ""
	}
	sg.playMenuMoveSound()
	return nil
}

func (sg *sessionGame) authorityCreateActionRow() int {
	if sg.authorityUploadAvailable() {
		return 7
	}
	return 6
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

func (sg *sessionGame) drawAuthorityLobby(text func(string, int, int)) {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	fit := func(s string, w int) string { return sg.ellipsizeIntermissionText(s, w) }
	if m.uploading != nil {
		text("UPLOADING LOADED WADS...", 24, 56)
		text("KEEP THIS GAME OPEN", 24, 82)
		text("ESC / BACK: CANCEL", 24, 154)
		return
	}
	if m.creating != nil {
		text("CREATING GAME...", 32, 56)
		text(fit(m.request.Name, 256), 32, 82)
		text("ESC / BACK: CANCEL", 32, 154)
		return
	}
	if m.page == authorityLobbyPageRooms {
		heading := "GAMES - ENTER TO JOIN"
		if menu.request.Spectator {
			heading = "GAMES - ENTER TO WATCH"
		}
		text(heading, 24, 36)
		if len(m.state.Rooms) == 0 {
			empty := "NO GAMES AVAILABLE"
			if sg.opts.AuthorityCreateGame != nil {
				empty = "NO GAMES YET - CREATE ONE"
			}
			text(empty, 24, 66)
		}
		for i := m.scroll; i < min(len(m.state.Rooms), m.scroll+authorityLobbyVisible); i++ {
			room := m.state.Rooms[i]
			y := 54 + (i-m.scroll)*16
			if i == m.row {
				text(">", 12, y)
			}
			text(fit(room.Name, 204), 24, y)
			state := strings.ToUpper(room.State)
			if room.State == "ready" {
				state = fmt.Sprintf("%d/%d", room.Players, room.PlayerLimit)
			}
			text(fit(state, 64), 240, y)
		}
		if i := slices.IndexFunc(m.state.Rooms, func(r lobby.Room) bool { return r.ID == m.selected }); i >= 0 {
			room := m.state.Rooms[i]
			mode := strings.ToUpper(room.Settings.Mode)
			if mode == "COOP" {
				mode = "CO-OP"
			}
			text(fit(fmt.Sprintf("%s  %s  SKILL %d", mode, room.Manifest.Map, room.Settings.Skill), 272), 24, 105)
			if menu.status == "" && !sg.authorityPackMatches(room.Manifest.WADHashes) {
				text(fit(sg.authorityRoomContentHint(room), 272), 24, 117)
			} else if menu.status == "" {
				text("MATCHING WAD LOADED", 24, 117)
			}
		}
		if menu.status != "" {
			text(fit(menu.status, 296), 12, 117)
		}
		role := "JOIN AS: PLAYER"
		if menu.request.Spectator {
			role = "JOIN AS: SPECTATOR"
		}
		labels := []string{"CREATE GAME", "REFRESH GAMES", "PLAYER: " + menu.request.Name, role, "DIRECT SERVERS", "BACK"}
		for i, action := range sg.authorityLobbyActions() {
			label := labels[action]
			y := 128 + i*12
			text(fit(label, 272), 36, y)
			if m.row == len(m.state.Rooms)+i {
				text(">", 20, y)
			}
		}
		return
	}
	labels := []string{}
	if m.page == authorityLobbyPageCreate {
		text("CREATE GAME", 24, 36)
		pack, _ := m.selectedPack()
		mode := "CO-OP"
		if m.request.Settings.Mode == "deathmatch" {
			mode = "DEATHMATCH"
		}
		labels = []string{"NAME: " + m.request.Name, "WAD: " + pack.Name, "MAP: " + m.request.Settings.Map, "MODE: " + mode, fmt.Sprintf("DIFFICULTY: %d / 5", m.request.Settings.Skill), "RULES..."}
		if sg.authorityUploadAvailable() {
			labels = append(labels, "UPLOAD LOADED WADS")
		}
		labels = append(labels, "CREATE & JOIN", "BACK")
		if menu.status == "" && !sg.authorityPackMatches(pack.WADHashes) {
			text("LOAD MATCHING WAD FIRST", 24, 188)
		}
	} else {
		text("GAME RULES", 24, 36)
		s := m.request.Settings
		onoff := func(on bool) string {
			if on {
				return "ON"
			}
			return "OFF"
		}
		limit := "NONE"
		if s.TimeLimitSeconds > 0 {
			limit = fmt.Sprintf("%d MIN", s.TimeLimitSeconds/60)
		}
		modeRule := "FRIENDLY FIRE: " + onoff(s.FriendlyFire)
		if s.Mode == "deathmatch" {
			modeRule = "FRAG LIMIT: NONE"
			if s.FragLimit > 0 {
				modeRule = fmt.Sprintf("FRAG LIMIT: %d", s.FragLimit)
			}
		}
		labels = []string{fmt.Sprintf("PLAYERS: %d", s.PlayerLimit), "MONSTERS: " + onoff(!s.NoMonsters), "FAST MONSTERS: " + onoff(s.FastMonsters), "RESPAWN MONSTERS: " + onoff(s.RespawnMonsters), modeRule, "TIME LIMIT: " + limit, "BACK"}
	}
	for row, label := range labels {
		spacing := 16
		if len(labels) > 8 {
			spacing = 14
		}
		y := 54 + row*spacing
		text(fit(label, 272), 36, y)
		if row == m.row {
			text(">", 20, y)
		}
	}
	text(fit(menu.status, 296), 12, 188)
}
