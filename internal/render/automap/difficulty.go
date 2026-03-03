package automap

import "gddoom/internal/mapdata"

const (
	skillEasyBits   = 0x0001
	skillMediumBits = 0x0002
	skillHardBits   = 0x0004
	skillMask       = skillEasyBits | skillMediumBits | skillHardBits
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

func thingSpawnsForSkill(t mapdata.Thing, skill int) bool {
	if isPlayerStart(t.Type) {
		return true
	}
	bits := int(t.Flags) & skillMask
	if bits == 0 {
		// Vanilla-compatible default: no skill bits means available in all skills.
		return true
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
