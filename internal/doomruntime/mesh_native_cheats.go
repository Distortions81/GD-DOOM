package doomruntime

import (
	"gddoom/internal/mapdata"
)

// Completed cheats mutate state outside the command stream. Send a mandatory
// snapshot so connected watchers retain those changes as well as movement.
func (c *NativeCampaign) TypeCheats(chars []rune) (bool, error) {
	if c.Watching() {
		return false, nil
	}
	active, completed := false, false
	for _, ch := range chars {
		before := c.Game.g.typedCheatBuffer
		active = c.Game.TypeCheats([]rune{ch}) || active
		completed = completed || before != "" && c.Game.g.typedCheatBuffer == ""
	}
	if completed && c.Game.g.newGameRequestedMap == nil {
		return active, c.BroadcastMandatoryKeyframe()
	}
	return active, nil
}

// TypeCheats uses the main engine's parser. It reports cheat typing so the host
// can suppress conflicting shortcuts (P, R and weapon-number keys).
func (n *NativeMeshGame) TypeCheats(chars []rune) bool {
	g := n.g
	old := g.input.inputChars
	defer func() { g.input.inputChars = old }()
	active := false
	for _, r := range chars {
		active = active || nativeCheatPrefix(g.typedCheatBuffer)
		g.input.inputChars = []rune{r}
		g.consumeTypedCheatInput()
		active = nativeCheatPrefix(g.typedCheatBuffer) || active
	}
	return active
}

func nativeCheatPrefix(buffer string) bool {
	codes := []string{"iddqd", "idkfa", "idfa", "idclip", "idspispopd", "idmypos", "idchoppers", "iddt", "idbeholdv", "idbeholds", "idbeholdi", "idbeholdr", "idbeholda", "idbeholdl", "idclev##", "idmus##"}
	for _, code := range codes {
		for count := 1; count <= len(code) && count <= len(buffer); count++ {
			tail := buffer[len(buffer)-count:]
			match := true
			for i := range count {
				if code[i] == '#' {
					match = match && tail[i] >= '0' && tail[i] <= '9'
				} else {
					match = match && code[i] == tail[i]
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// TakeNewGameRequest transfers a validated IDCLEV map to the native session.
func (n *NativeMeshGame) TakeNewGameRequest() (*mapdata.Map, int, bool) {
	g := n.g
	m, skill := g.newGameRequestedMap, g.newGameRequestedSkill
	if m == nil {
		return nil, 0, false
	}
	g.sessionAcknowledgeNewGameRequest()
	return m, skill, true
}

func (n *NativeMeshGame) ClearCheatInput() { n.g.typedCheatBuffer = "" }

// Notify displays a short message through the shared Doom HUD message timer.
func (n *NativeMeshGame) Notify(message string) { n.g.setHUDMessage(message, 105) }
