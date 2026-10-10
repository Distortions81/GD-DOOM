package doomruntime

import (
	"errors"
	"image/color"
	"strings"

	"gddoom/internal/netgame"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (g *game) setAuthorityConnectionFailure(err error) {
	status := netgame.ConnectionStatus{State: netgame.ConnectionDisconnected, Message: err.Error()}
	var server *netgame.ServerDisconnectError
	if errors.As(err, &server) && strings.HasPrefix(strings.ToLower(server.Reason), "match complete") {
		status.State = netgame.ConnectionComplete
		status.Message = server.Reason
	}
	g.authorityFailure = &status
	g.pendingUse = false
	g.demoWeaponSlot = 0
	g.input.mouseTurnRawAccum = 0
}

func (g *game) authorityConnectionStatus() netgame.ConnectionStatus {
	if g.authorityFailure != nil {
		return *g.authorityFailure
	}
	if status, ok := g.opts.AuthorityClient.(interface {
		Status() netgame.ConnectionStatus
	}); ok {
		value := status.Status()
		if value.State != netgame.ConnectionConnected {
			return value
		}
	}
	if g.authorityRules != nil && g.authorityRules.Ended {
		return netgame.ConnectionStatus{State: netgame.ConnectionComplete, Message: g.authorityRules.EndReason}
	}
	return netgame.ConnectionStatus{State: netgame.ConnectionConnected}
}

func (g *game) authorityConnectionBlocked() bool {
	return g.opts.AuthorityClient != nil && g.authorityConnectionStatus().State != netgame.ConnectionConnected
}

func (g *game) resetAuthorityClientPrediction() {
	g.clientPrediction = nil
	g.clientUpdate = authorityClientUpdateState{}
	g.authorityRender = nil
	g.authorityFailure = nil
	g.pendingUse = false
	g.demoWeaponSlot = 0
	g.input.mouseTurnRawAccum = 0
	g.clearPendingSoundState()
}

// Rendering happens after the host clears its sampled input, so presentation
// controls must retain this frame's held state during the Update input phase.
func (g *game) captureAuthorityScoreboardInput() {
	g.clientUpdate.scoreboardHeld = g.keyHeld(ebiten.KeyF6)
}

func (g *game) authorityStatusLines() []string {
	if g.opts.AuthorityClient == nil {
		return nil
	}
	status := g.authorityConnectionStatus()
	var lines []string
	switch status.State {
	case netgame.ConnectionReconnecting:
		lines = []string{"CONNECTION LOST", status.String(), "YOUR PLAYER REMAINS IN THE MATCH"}
	case netgame.ConnectionDisconnected:
		lines = []string{"DISCONNECTED", status.Message, "ESC: MENU"}
	case netgame.ConnectionComplete:
		lines = []string{"MATCH COMPLETE"}
		if g.authorityRules != nil && g.authorityRules.Ended {
			reason := strings.ReplaceAll(strings.ToUpper(g.authorityRules.EndReason), "_", " ")
			lines = append(lines, reason)
			lines = append(lines, g.authorityScoreLines()...)
		}
		lines = append(lines, "ESC: MENU")
	default:
		if g.opts.AuthorityClient.Welcome().PlayerID == 0 {
			// An observer keeps the last view after the final player leaves;
			// lack of new snapshots then is expected, not a stalled server.
			if roster := g.authorityNetworkHUD(); roster.known && roster.players == 0 {
				return []string{"WAITING FOR PLAYERS", "F12: CHANGE SPECTATOR VIEW"}
			}
		}
		if g.clientPrediction == nil || !g.clientPrediction.Ready() {
			if g.opts.AuthorityClient.Welcome().PlayerID == 0 {
				return []string{"WAITING FOR PLAYERS", "F12: CHANGE SPECTATOR VIEW"}
			}
			return []string{"WAITING FOR SERVER STATE"}
		}
		if g.clientUpdate.scoreboardHeld {
			title := "CO-OP PLAYERS"
			if g.opts.GameMode == gameModeDeathmatch {
				title = "DEATHMATCH PLAYERS"
			}
			lines = append([]string{title}, g.authorityScoreLines()...)
		}
	}
	return lines
}

func (sg *sessionGame) authorityOverlayLines() []string {
	// Keep local menus and the explicit quit confirmation usable during an
	// outage. The status remains visible when returning to the frozen world.
	if sg.g == nil || sg.quitPrompt.Active || (sg.frontend.Active && sg.frontend.MenuActive) {
		return nil
	}
	return sg.g.authorityStatusLines()
}

func (sg *sessionGame) drawAuthorityConnectionOverlay(screen *ebiten.Image) {
	lines := sg.authorityOverlayLines()
	if len(lines) == 0 {
		sg.drawAuthorityNetworkHUD(screen)
		return
	}
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	scale := float64(w) / 640
	if scale < 1 {
		scale = 1
	}
	if scale > 2 {
		scale = 2
	}
	lineHeight := 14 * scale
	wrapped := sg.g.wrapAuthorityOverlay(lines, int((float64(w)-32)/scale))
	if fit := float64(max(1, h-28)) / float64(max(1, len(wrapped))*14); fit < scale {
		scale = fit
		lineHeight = 14 * scale
	}
	panelHeight := float64(len(wrapped))*lineHeight + 20
	y := float64(h)/2 - panelHeight/2
	vector.DrawFilledRect(screen, 4, float32(y), float32(w-8), float32(panelHeight), color.RGBA{R: 0, G: 0, B: 0, A: 220}, false)
	for i, line := range wrapped {
		x := (float64(w) - float64(sg.g.huTextWidth(line))*scale) / 2
		sg.g.drawHUTextAt(screen, line, x, y+10+float64(i)*lineHeight, scale, scale)
	}
}
