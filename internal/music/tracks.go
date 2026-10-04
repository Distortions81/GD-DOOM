package music

import (
	"fmt"
	"strings"
)

// MapLumpName is the shared Doom map-to-music mapping, including episode 4 aliases.
func MapLumpName(name string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(name))
	switch s {
	case "E4M1":
		return "D_E3M4", true
	case "E4M2":
		return "D_E3M2", true
	case "E4M3":
		return "D_E3M3", true
	case "E4M4":
		return "D_E1M5", true
	case "E4M5":
		return "D_E2M7", true
	case "E4M6":
		return "D_E2M4", true
	case "E4M7":
		return "D_E2M6", true
	case "E4M8":
		return "D_E2M5", true
	case "E4M9":
		return "D_E1M9", true
	}
	if len(s) == 4 && s[0] == 'E' && s[2] == 'M' &&
		s[1] >= '1' && s[1] <= '9' && s[3] >= '1' && s[3] <= '9' {
		return "D_" + s, true
	}
	if strings.HasPrefix(s, "MAP") && len(s) == 5 && s[3] >= '0' && s[3] <= '9' && s[4] >= '0' && s[4] <= '9' {
		n := int(s[3]-'0')*10 + int(s[4]-'0')
		switch n {
		case 1:
			return "D_RUNNIN", true
		case 2:
			return "D_STALKS", true
		case 3:
			return "D_COUNTD", true
		case 4:
			return "D_BETWEE", true
		case 5:
			return "D_DOOM", true
		case 6:
			return "D_THE_DA", true
		case 7:
			return "D_SHAWN", true
		case 8:
			return "D_DDTBLU", true
		case 9:
			return "D_IN_CIT", true
		case 10:
			return "D_DEAD", true
		case 11:
			return "D_STLKS2", true
		case 12:
			return "D_THE_DA2", true
		case 13:
			return "D_DOOM2", true
		case 14:
			return "D_DDTBL2", true
		case 15:
			return "D_RUNNI2", true
		case 16:
			return "D_DEAD2", true
		case 17:
			return "D_STLKS3", true
		case 18:
			return "D_ROMERO", true
		case 19:
			return "D_SHAWN2", true
		case 20:
			return "D_MESSAG", true
		case 21:
			return "D_COUNT2", true
		case 22:
			return "D_DDTBL3", true
		case 23:
			return "D_AMPIE", true
		case 24:
			return "D_THEDA3", true
		case 25:
			return "D_ADRIAN", true
		case 26:
			return "D_MESSG2", true
		case 27:
			return "D_ROMER2", true
		case 28:
			return "D_TENSE", true
		case 29:
			return "D_SHAWN3", true
		case 30:
			return "D_OPENIN", true
		case 31:
			return "D_EVIL", true
		case 32:
			return "D_ULTIMA", true
		}
	}
	return "", false
}

// CheatSelection resolves IDMUS targets without depending on an audio backend.
func CheatSelection(currentMapName, code string) (string, string, bool) {
	currentMapName = strings.ToUpper(strings.TrimSpace(currentMapName))
	code = strings.TrimSpace(code)
	if len(code) != 2 || code[0] < '0' || code[0] > '9' || code[1] < '0' || code[1] > '9' {
		return "", "", false
	}
	if strings.HasPrefix(currentMapName, "MAP") {
		n := int(code[0]-'0')*10 + int(code[1]-'0')
		switch {
		case n == 0:
			return "", "", true
		case n >= 1 && n <= 32:
			return fmt.Sprintf("MAP%02d", n), "", true
		case n == 33:
			return "", "D_READ_M", true
		case n == 34:
			return "", "D_DM2TTL", true
		case n == 35:
			return "", "D_DM2INT", true
		default:
			return "", "", false
		}
	}
	if len(currentMapName) == 4 && currentMapName[0] == 'E' && currentMapName[2] == 'M' {
		if code[0] < '1' || code[0] > '9' || code[1] < '1' || code[1] > '9' {
			return "", "", false
		}
		return fmt.Sprintf("E%cM%c", code[0], code[1]), "", true
	}
	return "", "", false
}
