package doomruntime

import (
	"fmt"
	"math"
	"strings"

	"gddoom/internal/netgame"
)

// Presence is presentation state, independent of prediction resets and map
// baselines. A resumed player keeps its stable roster identity.
type authorityNoticeState struct {
	connection  netgame.ConnectionState
	roster      netgame.Roster
	rosterReady bool
}

func (g *game) appendAuthorityNotice(text string) {
	text = normalizeChatText(text)
	if text == "" {
		return
	}
	g.appendChatHistoryEntry(chatHistoryEntry{Text: text, Tics: chatHistoryTTL, Notice: true})
}

// Presence stays readable beside the ordinary pickup message, without sharing
// the frame counter's corner or duplicating entries in the chat column.
func (g *game) drawAuthorityNoticesText(draw func(string, float64, float64, float64, float64)) {
	const visibleNotices = 3
	start, count := len(g.chatHistory), 0
	for i := len(g.chatHistory) - 1; i >= 0; i-- {
		if g.chatHistory[i].Notice {
			start, count = i, count+1
			if count == visibleNotices {
				break
			}
		}
	}
	if count == 0 {
		return
	}
	scale := math.Min(2, math.Max(1, float64(g.viewW)/640))
	x, y := float64(chatMarginX)*scale, 20*scale
	maxWidth := max(40, int((float64(g.viewW)/2-x)/scale)-chatWrapPadding)
	for _, entry := range g.chatHistory[start:] {
		if !entry.Notice {
			continue
		}
		for _, line := range g.wrapChatText(entry.Text, maxWidth, "", "  ") {
			draw(line, x, y, scale, scale)
			y += chatLineAdvance * scale
		}
		y += 3 * scale
	}
}

func authorityPresenceLabel(player netgame.PlayerPresence) string {
	name := strings.Join(strings.Fields(player.Name), " ")
	if name == "" {
		name = "PLAYER"
	}
	if player.Spectator {
		return name + " (SPECTATOR)"
	}
	return fmt.Sprintf("%s (P%d)", name, player.PlayerID)
}

func authorityRosterPlayer(roster netgame.Roster, id uint64) (netgame.PlayerPresence, bool) {
	for _, player := range roster.Players[:min(int(roster.Count), len(roster.Players))] {
		if player.ID == id {
			return player, true
		}
	}
	return netgame.PlayerPresence{}, false
}

func (g *game) updateAuthorityNotices() {
	client := g.opts.AuthorityClient
	if client == nil {
		return
	}
	notices := &g.authorityNotices
	status := g.authorityConnectionStatus().State
	if status != notices.connection {
		switch status {
		case netgame.ConnectionConnected:
			if notices.connection == "" {
				g.appendAuthorityNotice("CONNECTED - F6: PLAYERS")
			} else {
				g.appendAuthorityNotice("RECONNECTED")
			}
		case netgame.ConnectionReconnecting:
			g.appendAuthorityNotice("CONNECTION LOST - RECONNECTING")
		case netgame.ConnectionDisconnected:
			g.appendAuthorityNotice("DISCONNECTED")
		}
		notices.connection = status
	}
	// Cached telemetry from the old socket must not become a new baseline
	// while the reconnect handshake is still incomplete.
	if status != netgame.ConnectionConnected {
		return
	}
	source, ok := client.(interface{ Roster() (netgame.Roster, bool) })
	if !ok {
		return
	}
	roster, available := source.Roster()
	if !available || (notices.rosterReady && roster.Revision == notices.roster.Revision) {
		return
	}
	if notices.rosterReady {
		for _, previous := range notices.roster.Players[:min(int(notices.roster.Count), len(notices.roster.Players))] {
			current, present := authorityRosterPlayer(roster, previous.ID)
			switch {
			case !present:
				g.appendAuthorityNotice(authorityPresenceLabel(previous) + " LEFT THE GAME")
			case previous.Connected && !current.Connected:
				g.appendAuthorityNotice(authorityPresenceLabel(current) + " LOST CONNECTION")
			case !previous.Connected && current.Connected:
				g.appendAuthorityNotice(authorityPresenceLabel(current) + " RECONNECTED")
			}
		}
		for _, current := range roster.Players[:min(int(roster.Count), len(roster.Players))] {
			if _, present := authorityRosterPlayer(notices.roster, current.ID); !present {
				g.appendAuthorityNotice(authorityPresenceLabel(current) + " JOINED THE GAME")
			}
		}
	}
	// Existing players form the first baseline silently; joining a busy server
	// should show one local confirmation, not a flood of old arrival notices.
	notices.roster, notices.rosterReady = roster, true
}
