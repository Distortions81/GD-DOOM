package doomruntime

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"gddoom/internal/netgame"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Formatting is refreshed only when reliable presence, scores, or rounded ping
// changes. Drawing never scans world objects or formats a roster every frame.
type authorityNetworkHUDCache struct {
	initialized bool
	known       bool
	revision    uint64
	local       byte
	scores      [netgame.MaxPlayers + 1]authorityScoreState
	rosterLines []string
	scoreLines  []string
	players     int
	limit       int
	pingMillis  int
	summary     string
}

func (g *game) authorityNetworkHUD() *authorityNetworkHUDCache {
	cache := &g.clientUpdate.networkHUD
	client := g.opts.AuthorityClient
	if client == nil {
		return cache
	}
	var roster netgame.Roster
	var known bool
	if source, ok := client.(interface{ Roster() (netgame.Roster, bool) }); ok {
		roster, known = source.Roster()
	}
	var scores [netgame.MaxPlayers + 1]authorityScoreState
	if g.authorityRules != nil {
		scores = g.authorityRules.Scores
	}
	local := client.Welcome().PlayerID
	changed := !cache.initialized || cache.known != known || cache.revision != roster.Revision || cache.local != local || cache.scores != scores
	if changed {
		cache.initialized, cache.known, cache.revision, cache.local = true, known, roster.Revision, local
		cache.scores = scores
		cache.players, cache.limit = 0, int(roster.PlayerLimit)
		cache.rosterLines = cache.rosterLines[:0]
		cache.scoreLines = cache.scoreLines[:0]
		count := min(int(roster.Count), len(roster.Players))
		// Slot order stays stable when a reconnecting player returns. The wire
		// roster is instead ordered by session identity for notice processing.
		for slot := byte(1); slot <= netgame.MaxPlayers; slot++ {
			found := false
			for _, presence := range roster.Players[:count] {
				if presence.Spectator || presence.PlayerID != slot {
					continue
				}
				found = true
				if presence.Connected {
					cache.players++
				}
				name, detail := authorityPlayerStatusLines(presence, local, scores[slot])
				cache.rosterLines = append(cache.rosterLines, name, detail)
				cache.scoreLines = append(cache.scoreLines, name, detail)
				break
			}
			// Scores can arrive before the first reliable roster. Retain the
			// confirmed scores while identity metadata is still on its way.
			if !known && !found && scores[slot].Generation != 0 {
				score := scores[slot]
				line := fmt.Sprintf("PLAYER %d  FRAGS %d  DEATHS %d", slot, score.Frags, score.Deaths)
				cache.scoreLines = append(cache.scoreLines, line)
			}
		}
		spectators := 0
		for _, presence := range roster.Players[:count] {
			if !presence.Spectator {
				continue
			}
			spectators++
			cache.rosterLines = append(cache.rosterLines, "SPECTATOR "+authorityRosterName(presence.Name), "  WATCHING  "+authorityPingText(int(presence.PingMillis)))
		}
		if spectators != 0 {
			cache.scoreLines = append(cache.scoreLines, fmt.Sprintf("SPECTATORS %d - MENU: PLAYERS", spectators))
		}
		if len(cache.rosterLines) == 0 {
			if known {
				cache.rosterLines = append(cache.rosterLines, "NO PLAYERS CONNECTED")
			} else {
				cache.rosterLines = append(cache.rosterLines, "WAITING FOR PLAYER LIST")
			}
		}
		if len(cache.scoreLines) == 0 {
			cache.scoreLines = append(cache.scoreLines, cache.rosterLines[0])
		}
	}
	ping := 0
	if latency, ok := client.(interface{ RoundTripTime() time.Duration }); ok {
		if rtt := latency.RoundTripTime(); rtt > 0 {
			ping = max(1, int((rtt+time.Millisecond/2)/time.Millisecond))
		}
	}
	if changed || ping != cache.pingMillis || cache.summary == "" {
		cache.pingMillis = ping
		if known {
			cache.summary = fmt.Sprintf("PLAYERS %d/%d  %s", cache.players, cache.limit, authorityPingText(ping))
		} else {
			cache.summary = "PLAYERS --  " + authorityPingText(ping)
		}
	}
	return cache
}

func authorityRosterName(name string) string {
	var out strings.Builder
	length := 0
	for _, char := range strings.TrimSpace(name) {
		if length == 20 {
			out.WriteString("...")
			break
		}
		if char >= 'a' && char <= 'z' {
			char -= 'a' - 'A'
		}
		if char < ' ' || char > '_' {
			char = '?'
		}
		out.WriteRune(char)
		length++
	}
	if length == 0 {
		return "PLAYER"
	}
	return out.String()
}

func authorityPingText(millis int) string {
	if millis <= 0 {
		return "PING --"
	}
	if millis >= 1000 {
		return "PING 1000+MS"
	}
	return fmt.Sprintf("PING %dMS", millis)
}

func authorityPlayerStatusLines(presence netgame.PlayerPresence, local byte, score authorityScoreState) (string, string) {
	name := fmt.Sprintf("P%d %s", presence.PlayerID, authorityRosterName(presence.Name))
	if local != 0 && presence.PlayerID == local {
		name += " (YOU)"
	}
	if !presence.Connected {
		return name, fmt.Sprintf("  RECONNECTING  F %d D %d", score.Frags, score.Deaths)
	}
	return name, fmt.Sprintf("  FRAGS %d DEATHS %d %s", score.Frags, score.Deaths, authorityPingText(int(presence.PingMillis)))
}

// The menu paginates the complete roster, including spectators. The held F6
// overlay keeps the four player rows together and summarizes spectators.
func (g *game) authorityRosterLines() []string { return g.authorityNetworkHUD().rosterLines }
func (g *game) authorityScoreLines() []string  { return g.authorityNetworkHUD().scoreLines }

func (g *game) authorityNetworkSummaryAt(now time.Time) string {
	if g == nil || g.opts.AuthorityClient == nil {
		return ""
	}
	status := g.authorityConnectionStatus()
	if status.State != netgame.ConnectionConnected {
		return strings.ToUpper(string(status.State))
	}
	cache := g.authorityNetworkHUD()
	return cache.summary
}

// Snapshot health is independent of a healthy transport ping: a stalled server
// may still answer probes. The stronger snapshot warning always takes priority.
func authorityNetworkWarning(now, snapshotAt time.Time, pingMillis int) string {
	if !snapshotAt.IsZero() {
		age := now.Sub(snapshotAt)
		if age >= time.Second {
			return "SERVER UPDATES STALLED"
		}
		if age >= 300*time.Millisecond {
			return "SERVER UPDATES DELAYED"
		}
	}
	if pingMillis >= 150 {
		return "HIGH PING"
	}
	return ""
}

func (sg *sessionGame) authorityNetworkHUDVisible() bool {
	return sg != nil && sg.g != nil && sg.g.opts.AuthorityClient != nil && !sg.frontend.Active &&
		!sg.quitPrompt.Active && !sg.intermission.state.Active && !sg.finale.Active && !sg.transitionActive() &&
		!sg.g.authorityConnectionBlocked()
}

func (sg *sessionGame) drawAuthorityNetworkHUD(screen *ebiten.Image) {
	if !sg.authorityNetworkHUDVisible() {
		return
	}
	g := sg.g
	cache := g.authorityNetworkHUD()
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	scale := math.Min(2, math.Max(1, float64(w)/640))
	warning := authorityNetworkWarning(time.Now(), g.clientUpdate.snapshotAt, cache.pingMillis)
	line2 := "F6: PLAYERS"
	touch := sg.shouldDrawTouchControls() && sg.gameplayTouchUsesPads()
	if touch {
		line2 = "MENU: PLAYERS"
	}
	width := float64(max(g.huTextWidth(cache.summary), max(g.huTextWidth(line2), g.huTextWidth(warning)))) * scale
	if width > float64(w)-16 {
		scale *= (float64(w) - 16) / width
		width = float64(w) - 16
	}
	// Sit above the status bar and chat composer, leaving pickups, join/leave
	// notices, and the frame counter unobscured at the top of the screen.
	bottom := float64(h) - 8*scale
	if g.statusBarVisible() {
		bottom = math.Min(bottom, float64(h)-32*g.hudScaleValue()*float64(h)/float64(max(1, g.viewH))-8*scale)
	}
	if g.chatComposeOpen {
		bottom -= 40 * scale
	}
	if touch {
		localW, localH := sg.touchControlLayoutSize(w, h)
		pad, _ := sg.gameplayTouchPads(localW, localH)
		pad = touchPadToScreen(newTouchLayoutTransform(w, h, localW, localH), pad)
		bottom = math.Min(bottom, pad.cy-pad.radius-8*scale)
	}
	height := 25 * scale
	if warning != "" {
		height += 11 * scale
	}
	y := math.Max(4, bottom-height)
	background := color.RGBA{A: 176}
	if warning != "" {
		background = color.RGBA{R: 80, A: 220}
	}
	vector.DrawFilledRect(screen, 4, float32(y), float32(width+8*scale), float32(height), background, false)
	g.drawHUTextAt(screen, cache.summary, 4+4*scale, y+4*scale, scale, scale)
	g.drawHUTextAt(screen, line2, 4+4*scale, y+15*scale, scale, scale)
	if warning != "" {
		g.drawHUTextAt(screen, warning, 4+4*scale, y+26*scale, scale, scale)
	}
}

func (g *game) wrapAuthorityOverlay(lines []string, maxWidth int) []string {
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.ToUpper(line)
		start, width := 0, 0
		for offset, char := range line {
			advance := g.huTextWidth(string(char))
			if width+advance > maxWidth && offset > start {
				wrapped = append(wrapped, line[start:offset])
				start, width = offset, 0
			}
			width += advance
		}
		if start < len(line) {
			wrapped = append(wrapped, line[start:])
		}
	}
	return wrapped
}
