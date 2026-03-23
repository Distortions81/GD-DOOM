package doomruntime

func isPlayerStart(typ int16) bool {
	return typ >= 1 && typ <= 4
}

func isDeathmatchStart(typ int16) bool {
	return typ == 11
}

func isMonster(typ int16) bool {
	switch typ {
	case 7, 9, 16, 58, 64, 65, 66, 67, 68, 69, 71, 84:
		return true
	case 3001, 3002, 3003, 3004, 3005, 3006:
		return true
	default:
		return false
	}
}
