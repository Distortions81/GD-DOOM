package doomruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gddoom/internal/runtimecfg"
)

const (
	authorityBrowserLimit   = 32
	authorityBrowserVisible = 4
	authorityBrowserWorkers = 4
)

const (
	authorityBrowserRefreshAction = iota
	authorityBrowserAdd
	authorityBrowserEdit
	authorityBrowserName
	authorityBrowserRole
	authorityBrowserMore
	authorityBrowserBack
)

var authorityBrowserPrimaryActions = []int{authorityBrowserName, authorityBrowserMore, authorityBrowserBack}
var authorityBrowserEmptyActions = []int{authorityBrowserAdd, authorityBrowserName, authorityBrowserMore, authorityBrowserBack}
var authorityBrowserMoreActions = []int{authorityBrowserRefreshAction, authorityBrowserAdd, authorityBrowserEdit, authorityBrowserRole, authorityBrowserBack}

type authorityServerStatus uint8

const (
	authorityServerUnchecked authorityServerStatus = iota
	authorityServerLoading
	authorityServerOnline
	authorityServerOffline
)

type authorityBrowserServer struct {
	entry  runtimecfg.AuthorityServerEntry
	status authorityServerStatus
	info   runtimecfg.AuthorityServerInfo
	err    string
}

type authorityBrowserReply struct {
	index   int
	address string
	info    runtimecfg.AuthorityServerInfo
	err     error
}

type authorityBrowserRefresh struct {
	cancel  context.CancelFunc
	replies chan authorityBrowserReply
	pending int
}

func (m *authorityMenuState) actions() []int {
	if m.moreOptions {
		return authorityBrowserMoreActions
	}
	if len(m.servers) == 0 {
		return authorityBrowserEmptyActions
	}
	return authorityBrowserPrimaryActions
}

func (m *authorityMenuState) actionStart() int {
	if m.moreOptions {
		return 0
	}
	return len(m.servers)
}

func (m *authorityMenuState) actionRow(action int) int {
	for row, candidate := range m.actions() {
		if candidate == action {
			return m.actionStart() + row
		}
	}
	return -1
}

func (m *authorityMenuState) actionAtRow() int {
	row := m.row - m.actionStart()
	if row < 0 || row >= len(m.actions()) {
		return -1
	}
	return m.actions()[row]
}

func (m *authorityMenuState) showServers() {
	m.moreOptions = false
	m.row = m.selected
	if len(m.servers) == 0 {
		m.row = m.actionRow(authorityBrowserAdd)
	}
}

func (m *authorityMenuState) selectServer(index int) {
	if index < 0 || index >= len(m.servers) {
		return
	}
	m.selected = index
	m.request.Address = m.servers[index].entry.Address
	if index < m.scroll {
		m.scroll = index
	}
	if index >= m.scroll+authorityBrowserVisible {
		m.scroll = index - authorityBrowserVisible + 1
	}
}

func (sg *sessionGame) initializeAuthorityBrowser() {
	m := &sg.multiplayer
	seen := make(map[string]bool)
	add := func(entry runtimecfg.AuthorityServerEntry) {
		entry.Address = strings.TrimSpace(entry.Address)
		entry.Label = strings.TrimSpace(entry.Label)
		if entry.Address == "" || len(entry.Address) > 512 || seen[entry.Address] || len(m.servers) == authorityBrowserLimit {
			return
		}
		seen[entry.Address] = true
		m.servers = append(m.servers, authorityBrowserServer{entry: entry})
	}
	for _, entry := range sg.opts.AuthorityServers {
		add(entry)
	}
	add(runtimecfg.AuthorityServerEntry{Address: m.request.Address})
	for index := range m.servers {
		if m.servers[index].entry.Address == strings.TrimSpace(m.request.Address) {
			m.selected = index
			break
		}
	}
	m.selectServer(m.selected)
}

func (sg *sessionGame) cancelAuthorityServerRefresh() {
	m := &sg.multiplayer
	if m.refresh != nil {
		m.refresh.cancel()
		m.refresh = nil
	}
	for index := range m.servers {
		if m.servers[index].status == authorityServerLoading {
			m.servers[index].status = authorityServerUnchecked
		}
	}
}

// Each refresh owns its results channel. A canceled worker can never update a
// subsequent refresh or an entry whose address was edited while it was in flight.
func (sg *sessionGame) refreshAuthorityServers() {
	sg.cancelAuthorityServerRefresh()
	m := &sg.multiplayer
	discover := sg.opts.AuthorityDiscover
	if discover == nil || len(m.servers) == 0 || sg.opts.AuthorityClient != nil {
		return
	}
	if m.probeSlots == nil {
		m.probeSlots = make(chan struct{}, authorityBrowserWorkers)
	}
	slots := m.probeSlots
	ctx, cancel := context.WithCancel(context.Background())
	refresh := &authorityBrowserRefresh{cancel: cancel, replies: make(chan authorityBrowserReply, len(m.servers)), pending: len(m.servers)}
	m.refresh = refresh
	m.status = ""
	jobs := make(chan authorityBrowserReply, len(m.servers))
	for index := range m.servers {
		m.servers[index].status, m.servers[index].err = authorityServerLoading, ""
		jobs <- authorityBrowserReply{index: index, address: m.servers[index].entry.Address}
	}
	close(jobs)
	for range min(authorityBrowserWorkers, len(m.servers)) {
		go func() {
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				select {
				case slots <- struct{}{}:
				case <-ctx.Done():
					return
				}
				if ctx.Err() != nil {
					<-slots
					return
				}
				query, stop := context.WithTimeout(ctx, 15*time.Second)
				job.info, job.err = discover(query, job.address)
				<-slots
				if job.err == nil && query.Err() != nil {
					job.err = query.Err()
				}
				stop()
				select {
				case refresh.replies <- job:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func (sg *sessionGame) pollAuthorityServers() {
	m := &sg.multiplayer
	refresh := m.refresh
	if refresh == nil {
		return
	}
	for refresh.pending > 0 {
		select {
		case reply := <-refresh.replies:
			refresh.pending--
			if reply.index >= len(m.servers) || m.servers[reply.index].entry.Address != reply.address {
				continue
			}
			entry := &m.servers[reply.index]
			entry.info = reply.info
			entry.status, entry.err = authorityServerOnline, ""
			if reply.err != nil {
				entry.status, entry.err = authorityServerOffline, reply.err.Error()
			}
		default:
			return
		}
	}
	refresh.cancel()
	m.refresh = nil
}

func (sg *sessionGame) beginAuthorityServerEdit(name, add bool) {
	m := &sg.multiplayer
	if !name && !add && len(m.servers) == 0 {
		add = true
	}
	if add && len(m.servers) == authorityBrowserLimit {
		m.status = "SERVER LIST FULL - EDIT AN ENTRY"
		return
	}
	if !name {
		sg.cancelAuthorityServerRefresh()
	}
	m.editing, m.replace, m.editName, m.editNew = true, true, name, add
	m.editOriginal, m.status = m.request.Address, ""
	if name {
		m.editOriginal = m.request.Name
	}
	if add {
		m.request.Address = ""
	}
}

func (sg *sessionGame) commitAuthorityServerEdit() bool {
	m := &sg.multiplayer
	address := strings.TrimSpace(m.request.Address)
	if address == "" {
		m.status = "ENTER A SERVER ADDRESS"
		return false
	}
	for index := range m.servers {
		if m.servers[index].entry.Address == address && (m.editNew || index != m.selected) {
			if !m.editNew {
				m.status = "SERVER ADDRESS ALREADY SAVED"
				return false
			}
			m.selectServer(index)
			m.showServers()
			m.status = "SERVER ALREADY SAVED"
			return true
		}
	}
	if m.editNew {
		m.servers = append(m.servers, authorityBrowserServer{entry: runtimecfg.AuthorityServerEntry{Address: address}})
		m.selected = len(m.servers) - 1
	} else {
		entry := &m.servers[m.selected]
		if entry.entry.Address != address {
			*entry = authorityBrowserServer{entry: runtimecfg.AuthorityServerEntry{Address: address}}
		}
	}
	m.selectServer(m.selected)
	m.showServers()
	m.status = "SERVER SAVED"
	if save := sg.opts.OnAuthorityServersChanged; save != nil {
		entries := make([]runtimecfg.AuthorityServerEntry, len(m.servers))
		for i := range m.servers {
			entries[i] = m.servers[i].entry
		}
		if err := save(entries); err != nil {
			m.status = "COULD NOT SAVE SERVER LIST: " + err.Error()
		}
	}
	// Only the newly selected entry needs fresh data; querying the explicit
	// bounded list also clears stale state after editing a default/favorite.
	status := m.status
	sg.refreshAuthorityServers()
	m.status = status
	return true
}

func authorityServerLabel(server authorityBrowserServer) string {
	if server.entry.Label != "" {
		return server.entry.Label
	}
	label := server.entry.Address
	if _, after, ok := strings.Cut(label, "://"); ok {
		label = after
	}
	return strings.TrimSuffix(label, "/netplay")
}

func authorityServerSummary(server authorityBrowserServer) (string, string) {
	switch server.status {
	case authorityServerLoading:
		return "CHECKING SERVER...", "WAD: WAITING FOR REPLY"
	case authorityServerOffline:
		return "OFFLINE - REFRESH TO RETRY", server.err
	case authorityServerUnchecked:
		return "NOT CHECKED - SELECT REFRESH", "JOIN VERIFIES YOUR LOADED WADS"
	}
	info := server.info
	counts := "PLAYERS ?"
	if info.Players >= 0 && info.PlayerLimit > 0 {
		counts = fmt.Sprintf("%d/%d PLAYERS", info.Players, info.PlayerLimit)
	}
	details := fmt.Sprintf("ONLINE %s  %s  %s", info.Manifest.Map, strings.ToUpper(info.Manifest.Mode), counts)
	compatibility := "WAD: MATCH"
	if !info.Compatible {
		compatibility = "WAD/ENGINE MISMATCH"
		if info.CompatibilityError != "" {
			compatibility += ": " + info.CompatibilityError
		}
	} else if info.Spectators >= 0 {
		compatibility += fmt.Sprintf("  %d WATCHING", info.Spectators)
	}
	return details, compatibility
}

func (sg *sessionGame) drawAuthorityBrowser(text func(string, int, int)) {
	m := &sg.multiplayer
	fit := func(value string, width int) string { return sg.ellipsizeIntermissionText(value, width) }
	if m.attempt != nil {
		text("JOINING GAME...", 32, 56)
		label := m.request.Address
		if m.selected < len(m.servers) {
			label = authorityServerLabel(m.servers[m.selected])
		}
		text(fit(label, 256), 32, 82)
		role := "PLAYING AS "
		if m.request.Spectator {
			role = "WATCHING AS "
		}
		text(fit(role+m.request.Name, 256), 32, 106)
		text("ESC / BACK: CANCEL", 32, 154)
		return
	}
	if m.editing {
		title, value := "EDIT SERVER ADDRESS", m.request.Address
		if m.editNew {
			title = "ADD SERVER"
		}
		if m.editName {
			title, value = "PLAYER NAME", m.request.Name
		}
		text(title, 24, 48)
		if m.replace {
			value = "[" + value + "]"
		} else {
			value += "_"
		}
		text(fit(value, 272), 24, 78)
		if !m.editName {
			text("HTTPS / WSS URL OR NATIVE HOST:PORT", 16, 108)
		}
		text("TYPE TO REPLACE  CTRL+A: SELECT ALL", 16, 136)
		text("ENTER: SAVE  ESC: CANCEL", 32, 156)
		text(fit(m.status, 296), 12, 180)
		return
	}
	if m.moreOptions {
		text("MORE OPTIONS", 24, 38)
		if len(m.servers) > 0 {
			server := m.servers[m.selected]
			text(fit(authorityServerLabel(server), 272), 24, 56)
			text(fit(server.entry.Address, 272), 24, 68)
		}
		role := "JOIN AS: PLAYER"
		if m.request.Spectator {
			role = "JOIN AS: SPECTATOR"
		}
		labels := []string{"REFRESH SERVERS", "ADD SERVER", "EDIT SELECTED SERVER", role, "BACK TO SERVERS"}
		for row, label := range labels {
			y := 90 + row*18
			text(label, 36, y)
			if m.row == row {
				text(">", 20, y)
			}
		}
		status := m.status
		if status == "" && len(m.servers) > 0 {
			server := m.servers[m.selected]
			if server.status == authorityServerOnline {
				status = fmt.Sprintf("RESPONSE %d MS", max(0, int(server.info.Ping.Milliseconds())))
			} else if server.status == authorityServerOffline {
				status = server.err
			}
		}
		text(fit(status, 296), 12, 186)
		return
	}
	verb := "JOIN"
	if m.request.Spectator {
		verb = "WATCH"
	}
	hint := "ENTER / TAP USE TO SELECT"
	if m.row < len(m.servers) {
		hint = "ENTER / TAP USE TO " + verb
	}
	text(hint, 24, 36)
	if len(m.servers) == 0 {
		text("NO SAVED SERVERS YET", 32, 68)
	}
	for index := m.scroll; index < min(len(m.servers), m.scroll+authorityBrowserVisible); index++ {
		server := m.servers[index]
		y := 54 + (index-m.scroll)*16
		label := authorityServerLabel(server)
		if index == m.row {
			text(">", 12, y)
			label = verb + " " + label
		}
		text(fit(label, 208), 24, y)
		state := ""
		switch server.status {
		case authorityServerLoading:
			state = "CHECKING"
		case authorityServerOffline:
			state = "OFFLINE"
		case authorityServerOnline:
			state = "ONLINE"
			if server.info.Players >= 0 && server.info.PlayerLimit > 0 {
				state = fmt.Sprintf("%d/%d", server.info.Players, server.info.PlayerLimit)
			}
			if !server.info.Compatible {
				state = "MISMATCH"
			}
		}
		text(fit(state, 64), 240, y)
	}
	if len(m.servers) > 0 {
		server := m.servers[m.selected]
		details, readiness := "", "ENTER TO CONNECT"
		switch server.status {
		case authorityServerLoading:
			readiness = "CHECKING SERVER - YOU CAN STILL JOIN"
		case authorityServerOffline:
			readiness = "SERVER OFFLINE - ENTER TO RETRY"
		case authorityServerOnline:
			mode := strings.ToUpper(server.info.Manifest.Mode)
			if mode == "COOP" {
				mode = "CO-OP"
			}
			details = mode + "  " + server.info.Manifest.Map
			readiness = "READY TO " + verb
			if server.info.Spectators > 0 {
				details += fmt.Sprintf("  %d WATCHING", server.info.Spectators)
			}
			if !server.info.Compatible {
				readiness = "REQUIRES MATCHING GAME FILES"
			}
		}
		text(fit(details, 272), 24, 120)
		if m.status != "" {
			readiness = m.status
		}
		text(fit(readiness, 296), 12, 132)
	} else if m.status != "" {
		text(fit(m.status, 296), 12, 90)
	}
	for index, action := range m.actions() {
		label := ""
		switch action {
		case authorityBrowserAdd:
			label = "ADD A SERVER"
		case authorityBrowserName:
			label = "PLAYER: " + m.request.Name
		case authorityBrowserMore:
			label = "MORE OPTIONS"
		case authorityBrowserBack:
			label = "BACK"
		}
		y := 154 + index*16
		if len(m.servers) == 0 {
			y = 110 + index*24
		}
		text(fit(label, 272), 36, y)
		if m.row == m.actionRow(action) {
			text(">", 20, y)
		}
	}
}
