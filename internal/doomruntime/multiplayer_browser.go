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
	authorityBrowserJoin
	authorityBrowserActionCount
)

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

func (m *authorityMenuState) actionRow(action int) int { return len(m.servers) + action }

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
	sg.cancelAuthorityServerRefresh()
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
			m.row = index
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
	m.row = m.selected
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
	text("SERVER BROWSER", 16, 32)
	text(fmt.Sprintf("%d SAVED", len(m.servers)), 240, 32)
	if len(m.servers) == 0 {
		text("NO SERVERS - CHOOSE ADD", 32, 61)
	}
	for index := m.scroll; index < min(len(m.servers), m.scroll+authorityBrowserVisible); index++ {
		server := m.servers[index]
		y := 47 + (index-m.scroll)*14
		marker := " "
		if index == m.selected {
			marker = "*"
		}
		if index == m.row {
			marker = ">"
		}
		text(marker, 10, y)
		text(fit(authorityServerLabel(server), 184), 24, y)
		state := "UNCHECKED"
		switch server.status {
		case authorityServerLoading:
			state = "LOADING"
		case authorityServerOffline:
			state = "OFFLINE"
		case authorityServerOnline:
			ping := server.info.Ping.Milliseconds()
			if ping < 0 {
				ping = 0
			}
			state = fmt.Sprintf("%dMS", ping)
			if !server.info.Compatible {
				state = "MISMATCH"
			}
		}
		text(fit(state, 80), 224, y)
	}
	if len(m.servers) > 0 {
		server := m.servers[m.selected]
		text(fit(server.entry.Address, 296), 12, 104)
		details, compatibility := authorityServerSummary(server)
		text(fit(details, 296), 12, 116)
		text(fit(compatibility, 296), 12, 128)
	}
	role := "PLAY"
	if m.request.Spectator {
		role = "SPECTATE"
	}
	join := "JOIN"
	if m.attempt != nil {
		join = "JOINING"
	}
	actions := []struct {
		label       string
		x, y, width int
	}{
		{"REFRESH", 24, 144, 76}, {"ADD", 122, 144, 48}, {"EDIT", 222, 144, 72},
		{"NAME: " + m.request.Name, 24, 158, 272}, {"AS: " + role, 24, 172, 164}, {join, 222, 172, 80},
	}
	for index, action := range actions {
		if m.row == m.actionRow(index) {
			text(">", action.x-12, action.y)
		}
		text(fit(action.label, action.width), action.x, action.y)
	}
	footer := "ENTER: JOIN  F5: REFRESH  F2: EDIT"
	if m.status != "" {
		footer = m.status
	}
	text(fit(footer, 296), 12, 188)
}
