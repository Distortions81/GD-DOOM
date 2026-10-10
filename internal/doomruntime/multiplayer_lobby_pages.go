package doomruntime

import (
	"fmt"
	"slices"
	"strings"

	"gddoom/internal/lobby"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	authorityHomeFind = iota
	authorityHomeCreate
	authorityHomePlayer
	authorityHomeDirect
	authorityHomeBack
)

const (
	authorityCreateNameRow = iota
	authorityCreateModeRow
	authorityCreateSkillRow
	authorityCreateFilesRow
	authorityCreateRulesRow
	authorityCreateStartRow
	authorityCreateBackRow
)

func (sg *sessionGame) authorityHomeActions() []int {
	actions := []int{authorityHomeFind}
	if sg.authorityLobbyAvailable() && sg.opts.AuthorityCreateGame != nil {
		actions = append(actions, authorityHomeCreate)
	}
	actions = append(actions, authorityHomePlayer)
	if sg.authorityLobbyAvailable() {
		actions = append(actions, authorityHomeDirect)
	}
	return append(actions, authorityHomeBack)
}

func (sg *sessionGame) openAuthorityMultiplayerHome() {
	sg.cancelAuthorityLobbyRefresh()
	sg.cancelAuthorityServerRefresh()
	menu := &sg.multiplayer
	menu.editing, menu.lobby.editRoomName = false, false
	menu.lobby.page = authorityLobbyPageHome
	menu.lobby.row = min(menu.lobby.homeRow, len(sg.authorityHomeActions())-1)
	menu.status = ""
}

func (sg *sessionGame) openAuthorityPlayerSetup(returnPage int) {
	sg.cancelAuthorityLobbyRefresh()
	sg.cancelAuthorityServerRefresh()
	m := &sg.multiplayer.lobby
	m.playerReturnPage, m.playerReturnRow = returnPage, m.row
	m.page, m.row = authorityLobbyPagePlayer, 0
}

func (sg *sessionGame) openAuthorityDirectServers() {
	sg.cancelAuthorityLobbyRefresh()
	menu := &sg.multiplayer
	menu.lobby.page = authorityLobbyPageNone
	menu.joinLabel, menu.status = "", ""
	menu.showServers()
	menu.selectServer(menu.selected)
	sg.refreshAuthorityServers()
}

func (sg *sessionGame) authorityMultiplayerPageTitle() string {
	if sg.opts.AuthorityClient != nil && sg.multiplayer.showPlayers {
		return "PLAYERS"
	}
	switch sg.multiplayer.lobby.page {
	case authorityLobbyPageRooms:
		return "FIND GAME"
	case authorityLobbyPageCreate:
		return "CREATE GAME"
	case authorityLobbyPageRules:
		return "GAME RULES"
	case authorityLobbyPagePlayer:
		return "PLAYER SETUP"
	case authorityLobbyPageFiles:
		return "GAME FILES"
	case authorityLobbyPageNone:
		if sg.opts.AuthorityClient == nil {
			if sg.multiplayer.moreOptions {
				return "MANAGE SERVERS"
			}
			if sg.authorityLobbyAvailable() {
				return "SERVERS"
			}
			return "SAVED SERVERS"
		}
	}
	return "MULTIPLAYER"
}

func (sg *sessionGame) selectInitialAuthorityPack() {
	m := &sg.multiplayer.lobby
	if len(m.state.Packs) == 0 {
		return
	}
	pack := m.state.Packs[0]
	for _, candidate := range m.state.Packs {
		if sg.authorityPackMatches(candidate.WADHashes) {
			pack = candidate
			break
		}
	}
	m.request.Settings.PackID = pack.ID
	if len(pack.Maps) != 0 {
		m.request.Settings.Map = pack.Maps[0]
	}
	m.request.RequestID = ""
}

func (sg *sessionGame) authorityCreateActionRow() int { return authorityCreateStartRow }

func (sg *sessionGame) authorityFilesBackRow() int {
	if sg.authorityUploadAvailable() {
		return 3
	}
	return 2
}

func (sg *sessionGame) authorityLobbyBack() {
	m := &sg.multiplayer.lobby
	switch m.page {
	case authorityLobbyPageHome:
		sg.cancelAuthorityLobbyRefresh()
		m.page = authorityLobbyPageNone
		sg.frontend.Mode, sg.frontend.ItemOn = frontendModeTitle, frontendMultiplayerMenuItem
	case authorityLobbyPageRooms, authorityLobbyPageCreate:
		sg.openAuthorityMultiplayerHome()
	case authorityLobbyPageFiles:
		m.page, m.row = authorityLobbyPageCreate, authorityCreateFilesRow
	case authorityLobbyPageRules:
		m.page, m.row = authorityLobbyPageCreate, authorityCreateRulesRow
	case authorityLobbyPagePlayer:
		switch m.playerReturnPage {
		case authorityLobbyPageNone:
			sg.openAuthorityDirectServers()
		case authorityLobbyPageRooms:
			m.page, m.row = authorityLobbyPageRooms, m.playerReturnRow
			m.keepSelectionVisible()
		case authorityLobbyPageCreate:
			m.page, m.row = authorityLobbyPageCreate, m.playerReturnRow
		default:
			sg.openAuthorityMultiplayerHome()
		}
	}
	sg.playMenuBackSound()
}

func (sg *sessionGame) tickAuthorityLobby(escape, selectPressed bool) error {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	if escape {
		sg.authorityLobbyBack()
		return nil
	}
	count := 1
	switch m.page {
	case authorityLobbyPageHome:
		count = len(sg.authorityHomeActions())
	case authorityLobbyPagePlayer:
		count = 3
	case authorityLobbyPageRooms:
		count = len(m.state.Rooms) + len(sg.authorityLobbyActions())
	case authorityLobbyPageCreate:
		count = authorityCreateBackRow + 1
	case authorityLobbyPageRules:
		count = len(m.ruleRows())
	case authorityLobbyPageFiles:
		count = sg.authorityFilesBackRow() + 1
	}
	m.row = max(0, min(m.row, count-1))
	if sg.keyJustPressed(ebiten.KeyArrowUp) || sg.touchJustPressed(touchActionUp) {
		m.initialRoomsFocus = false
		m.row = (m.row + count - 1) % count
		sg.playMenuMoveSound()
	}
	if sg.keyJustPressed(ebiten.KeyArrowDown) || sg.keyJustPressed(ebiten.KeyTab) || sg.touchJustPressed(touchActionDown) {
		m.initialRoomsFocus = false
		m.row = (m.row + 1) % count
		sg.playMenuMoveSound()
	}
	if m.page == authorityLobbyPageHome {
		m.homeRow = m.row
		if !selectPressed {
			return nil
		}
		sg.playMenuConfirmSound()
		switch sg.authorityHomeActions()[m.row] {
		case authorityHomeFind:
			if sg.authorityLobbyAvailable() {
				sg.openAuthorityLobby()
			} else {
				sg.openAuthorityDirectServers()
			}
		case authorityHomeCreate:
			sg.openAuthorityCreate()
		case authorityHomePlayer:
			menu.status = ""
			sg.openAuthorityPlayerSetup(authorityLobbyPageHome)
		case authorityHomeDirect:
			sg.openAuthorityDirectServers()
		case authorityHomeBack:
			sg.authorityLobbyBack()
		}
		return nil
	}
	if m.page == authorityLobbyPageRooms {
		previous := m.selected
		m.keepSelectionVisible()
		if m.selected != previous {
			menu.status = ""
		}
		if sg.keyJustPressed(ebiten.KeyF5) {
			m.initialRoomsFocus = false
			sg.refreshAuthorityLobby()
		}
		watch := sg.keyJustPressed(ebiten.KeyS) && m.row < len(m.state.Rooms)
		if !selectPressed && !watch {
			return nil
		}
		m.initialRoomsFocus = false
		sg.playMenuConfirmSound()
		if m.row < len(m.state.Rooms) {
			if watch {
				menu.request.Spectator = true
			}
			sg.joinAuthorityRoom(m.state.Rooms[m.row])
		} else if m.row == sg.authorityLobbyActionRow(authorityLobbyRefreshAction) {
			sg.refreshAuthorityLobby()
		} else {
			sg.authorityLobbyBack()
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
	switch m.page {
	case authorityLobbyPagePlayer:
		switch m.row {
		case 0:
			if selectPressed {
				sg.beginAuthorityServerEdit(true, false)
			}
		case 1:
			menu.request.Spectator = !menu.request.Spectator
		case 2:
			if selectPressed {
				sg.authorityLobbyBack()
			}
		}
	case authorityLobbyPageRules:
		if m.row == len(m.ruleRows())-1 {
			if selectPressed {
				sg.authorityLobbyBack()
			}
		} else {
			m.adjustRule(dir)
		}
	case authorityLobbyPageFiles:
		switch m.row {
		case 0:
			index := slices.IndexFunc(m.state.Packs, func(p lobby.Pack) bool { return p.ID == m.request.Settings.PackID })
			if len(m.state.Packs) > 0 {
				pack := m.state.Packs[(max(0, index)+dir+len(m.state.Packs))%len(m.state.Packs)]
				m.request.Settings.PackID, m.request.Settings.Map = pack.ID, ""
				if len(pack.Maps) > 0 {
					m.request.Settings.Map = pack.Maps[0]
				}
			}
		case 1:
			if pack, ok := m.selectedPack(); ok && len(pack.Maps) > 0 {
				index := max(0, slices.Index(pack.Maps, m.request.Settings.Map))
				m.request.Settings.Map = pack.Maps[(index+dir+len(pack.Maps))%len(pack.Maps)]
			}
		default:
			if selectPressed {
				if m.row == sg.authorityFilesBackRow() {
					sg.authorityLobbyBack()
				} else {
					sg.beginAuthorityUpload()
				}
			}
		}
	case authorityLobbyPageCreate:
		switch m.row {
		case authorityCreateNameRow:
			if selectPressed {
				menu.editing, menu.replace, menu.editName, m.editRoomName = true, true, true, true
				menu.editOriginal = m.request.Name
			}
		case authorityCreateModeRow:
			if m.request.Settings.Mode == "coop" {
				m.request.Settings.Mode, m.request.Settings.NoMonsters = "deathmatch", true
				m.request.Settings.FriendlyFire = false
			} else {
				m.request.Settings.Mode, m.request.Settings.NoMonsters = "coop", false
				m.request.Settings.FragLimit = 0
			}
		case authorityCreateSkillRow:
			m.request.Settings.Skill = (m.request.Settings.Skill-1+dir+5)%5 + 1
		case authorityCreateFilesRow:
			if selectPressed {
				m.page, m.row = authorityLobbyPageFiles, 0
			}
		case authorityCreateRulesRow:
			if selectPressed {
				m.page, m.row = authorityLobbyPageRules, 0
			}
		case authorityCreateStartRow:
			if selectPressed {
				sg.beginAuthorityCreate()
			}
		case authorityCreateBackRow:
			if selectPressed {
				sg.authorityLobbyBack()
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

func (sg *sessionGame) drawAuthorityLobby(text func(string, int, int)) {
	menu, m := &sg.multiplayer, &sg.multiplayer.lobby
	fit := func(value string, width int) string { return sg.ellipsizeIntermissionText(value, width) }
	if m.uploading != nil || m.creating != nil {
		label := "CREATING GAME..."
		if m.uploading != nil {
			label = "UPLOADING LOADED WADS..."
		}
		text(label, 24, 64)
		text(fit(m.request.Name, 272), 24, 90)
		text("ESC / BACK: CANCEL", 24, 154)
		return
	}
	drawRows := func(labels []string, start, step int) {
		for row, label := range labels {
			y := start + row*step
			text(fit(label, 272), 36, y)
			if row == m.row {
				text(">", 20, y)
			}
		}
	}
	switch m.page {
	case authorityLobbyPageHome:
		labels := []string{}
		find := "FIND GAME"
		if !sg.authorityLobbyAvailable() {
			find = "SAVED SERVERS"
		}
		all := []string{find, "CREATE GAME", "PLAYER SETUP", "SERVERS", "BACK"}
		for _, action := range sg.authorityHomeActions() {
			labels = append(labels, all[action])
		}
		drawRows(labels, 56, 25)
	case authorityLobbyPagePlayer:
		role := "PLAYER"
		if menu.request.Spectator {
			role = "SPECTATOR"
		}
		drawRows([]string{"NAME: " + menu.request.Name, "JOIN AS: " + role, "BACK"}, 64, 32)
	case authorityLobbyPageRooms:
		heading := "ENTER: JOIN GAME    S: WATCH"
		if menu.request.Spectator {
			heading = "ENTER: WATCH GAME"
		}
		text(heading, 24, 36)
		if len(m.state.Rooms) == 0 {
			text("NO GAMES AVAILABLE", 24, 70)
		}
		for i := m.scroll; i < min(len(m.state.Rooms), m.scroll+authorityLobbyVisible); i++ {
			room := m.state.Rooms[i]
			y := 52 + (i-m.scroll)*16
			if i == m.row {
				text(">", 12, y)
			}
			text(fit(room.Name, 204), 24, y)
			status := strings.ToUpper(room.State)
			if room.State == "ready" {
				status = fmt.Sprintf("%d/%d", room.Players, room.PlayerLimit)
			}
			text(fit(status, 64), 240, y)
		}
		if i := slices.IndexFunc(m.state.Rooms, func(room lobby.Room) bool { return room.ID == m.selected }); i >= 0 {
			room := m.state.Rooms[i]
			mode := strings.ToUpper(room.Settings.Mode)
			if mode == "COOP" {
				mode = "CO-OP"
			}
			text(fit(fmt.Sprintf("%s  %s  SKILL %d", mode, room.Manifest.Map, room.Settings.Skill), 272), 24, 132)
			if menu.status == "" {
				hint := "MATCHING WAD LOADED"
				if !sg.authorityPackMatches(room.Manifest.WADHashes) {
					hint = sg.authorityRoomContentHint(room)
				}
				text(fit(hint, 272), 24, 144)
			}
		}
		for i, label := range []string{"REFRESH", "BACK"} {
			text(label, 36, 160+i*14)
			if m.row == len(m.state.Rooms)+i {
				text(">", 20, 160+i*14)
			}
		}
	case authorityLobbyPageCreate:
		mode := "CO-OP"
		if m.request.Settings.Mode == "deathmatch" {
			mode = "DEATHMATCH"
		}
		drawRows([]string{"NAME: " + m.request.Name, "MODE: " + mode, fmt.Sprintf("DIFFICULTY: %d / 5", m.request.Settings.Skill), "GAME FILES...", "RULES...", "CREATE & JOIN", "BACK"}, 52, 18)
		pack, _ := m.selectedPack()
		text(fit(pack.Name+"  "+m.request.Settings.Map, 272), 24, 176)
	case authorityLobbyPageFiles:
		pack, _ := m.selectedPack()
		labels := []string{"WAD: " + pack.Name, "LEVEL: " + m.request.Settings.Map}
		if sg.authorityUploadAvailable() {
			labels = append(labels, "UPLOAD LOADED WADS")
		}
		drawRows(append(labels, "BACK"), 56, 26)
		if menu.status == "" && !sg.authorityPackMatches(pack.WADHashes) {
			text("LOAD MATCHING WAD TO CREATE", 24, 168)
		}
	case authorityLobbyPageRules:
		s := m.request.Settings
		onoff := func(value bool) string {
			if value {
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
		drawRows([]string{fmt.Sprintf("PLAYERS: %d", s.PlayerLimit), "MONSTERS: " + onoff(!s.NoMonsters), "FAST MONSTERS: " + onoff(s.FastMonsters), "RESPAWN MONSTERS: " + onoff(s.RespawnMonsters), modeRule, "TIME LIMIT: " + limit, "BACK"}, 52, 18)
	}
	text(fit(menu.status, 296), 12, 188)
}
