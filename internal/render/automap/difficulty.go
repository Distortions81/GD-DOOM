package automap

import "gddoom/internal/mapdata"

const (
	skillEasyBits      = 0x0001
	skillMediumBits    = 0x0002
	skillHardBits      = 0x0004
	skillMask          = skillEasyBits | skillMediumBits | skillHardBits
	thingFlagAmbush    = 0x0008
	thingFlagNotSingle = 0x0010
	thingFlagNotDM     = 0x0020
	thingFlagNotCoop   = 0x0040
	gameModeSingle     = "single"
	gameModeCoop       = "coop"
	gameModeDeathmatch = "deathmatch"
	defaultGameMode    = gameModeSingle
)

func normalizeSkillLevel(skill int) int {
	if skill < 1 {
		return 3
	}
	if skill > 5 {
		return 5
	}
	return skill
}

func normalizeGameMode(mode string) string {
	switch mode {
	case gameModeCoop:
		return gameModeCoop
	case gameModeDeathmatch:
		return gameModeDeathmatch
	default:
		return defaultGameMode
	}
}

func normalizeKeyboardTurnSpeed(v float64) float64 {
	if v <= 0 {
		return 1.0
	}
	if v > 4.0 {
		return 4.0
	}
	return v
}

func normalizeMouseLookSpeed(v float64) float64 {
	if v <= 0 {
		return 1.0
	}
	if v > 8.0 {
		return 8.0
	}
	return v
}

func thingSpawnsForSkill(t mapdata.Thing, skill int) bool {
	if isPlayerStart(t.Type) {
		return true
	}
	bits := int(t.Flags) & skillMask
	if bits == 0 {
		// Vanilla Doom: non-player things with no skill bits do not spawn.
		return false
	}
	switch normalizeSkillLevel(skill) {
	case 1, 2:
		return bits&skillEasyBits != 0
	case 3:
		return bits&skillMediumBits != 0
	default: // 4, 5
		return bits&skillHardBits != 0
	}
}

func thingSpawnsForGameMode(t mapdata.Thing, mode string) bool {
	if isPlayerStart(t.Type) {
		return true
	}
	flags := int(t.Flags)
	switch normalizeGameMode(mode) {
	case gameModeSingle:
		return (flags & thingFlagNotSingle) == 0
	case gameModeCoop:
		return (flags & thingFlagNotCoop) == 0
	default: // deathmatch
		return (flags & thingFlagNotDM) == 0
	}
}

func thingSpawnsInSession(t mapdata.Thing, skill int, mode string) bool {
	return thingSpawnsForSkill(t, skill) && thingSpawnsForGameMode(t, mode)
}
