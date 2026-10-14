package doomruntime

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const authorityPlayersVisibleLines = 8

func (sg *sessionGame) authorityPlayersPageLines() []string {
	if sg.g == nil {
		return []string{"WAITING FOR PLAYER LIST"}
	}
	return sg.g.authorityRosterLines()
}

func (sg *sessionGame) tickAuthorityPlayersPage(back, confirm bool) {
	menu := &sg.multiplayer
	if back || confirm {
		menu.showPlayers, menu.row = false, 1
		sg.playMenuBackSound()
		return
	}
	step := 0
	if sg.keyJustPressed(ebiten.KeyArrowUp) || sg.touchJustPressed(touchActionUp) {
		step = -2
	}
	if sg.keyJustPressed(ebiten.KeyArrowDown) || sg.keyJustPressed(ebiten.KeyTab) || sg.touchJustPressed(touchActionDown) {
		step = 2
	}
	if sg.keyJustPressed(ebiten.KeyPageUp) {
		step = -authorityPlayersVisibleLines
	}
	if sg.keyJustPressed(ebiten.KeyPageDown) {
		step = authorityPlayersVisibleLines
	}
	last := max(0, len(sg.authorityPlayersPageLines())-authorityPlayersVisibleLines)
	next := min(last, max(0, menu.playersScroll+step))
	if next != menu.playersScroll {
		sg.playMenuMoveSound()
	}
	menu.playersScroll = next
}

func (sg *sessionGame) drawAuthorityPlayersPage(text func(string, int, int)) {
	if sg.g != nil {
		text(sg.ellipsizeIntermissionText(sg.g.authorityNetworkSummaryAt(time.Now()), 272), 24, 34)
	}
	lines := sg.authorityPlayersPageLines()
	start := min(sg.multiplayer.playersScroll, max(0, len(lines)-authorityPlayersVisibleLines))
	end := min(len(lines), start+authorityPlayersVisibleLines)
	for i, line := range lines[start:end] {
		text(sg.ellipsizeIntermissionText(line, 272), 24, 52+i*12)
	}
	if len(lines) > authorityPlayersVisibleLines {
		text(fmt.Sprintf("UP/DOWN: MORE  %d-%d / %d", start+1, end, len(lines)), 24, 154)
	}
	text("BACK: ENTER / ESC", 24, 178)
}
