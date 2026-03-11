package sessionflow

import "gddoom/internal/mapdata"

const FinaleHoldTics = 35 * 7

type Finale struct {
	Active  bool
	Tic     int
	WaitTic int
	MapName mapdata.MapName
	Screen  string
}

func StartFinale(current mapdata.MapName, secret bool) (Finale, bool) {
	screen, ok := EpisodeFinaleScreen(current, secret)
	if !ok {
		return Finale{}, false
	}
	return Finale{
		Active:  true,
		MapName: current,
		Screen:  screen,
		WaitTic: FinaleHoldTics,
	}, true
}

func TickFinale(state Finale, skipPressed bool) (Finale, bool) {
	if !state.Active {
		return state, false
	}
	state.Tic++
	if skipPressed && state.Tic <= IntermissionSkipInputDelayTics {
		skipPressed = false
	}
	if skipPressed && state.WaitTic > IntermissionSkipExitHoldTics {
		state.WaitTic = IntermissionSkipExitHoldTics
	}
	if state.WaitTic > 0 {
		state.WaitTic--
		return state, false
	}
	return Finale{}, true
}

func EpisodeFinaleScreen(current mapdata.MapName, secret bool) (string, bool) {
	if secret {
		return "", false
	}
	ep, slot, ok := episodeMapSlot(current)
	if !ok || slot != 8 {
		return "", false
	}
	switch ep {
	case 1:
		return "CREDIT", true
	case 2:
		return "VICTORY2", true
	case 3, 4:
		return "ENDPIC", true
	default:
		return "", false
	}
}

func episodeMapSlot(name mapdata.MapName) (episode int, slot int, ok bool) {
	s := string(name)
	if len(s) != 4 || s[0] != 'E' || s[2] != 'M' {
		return 0, 0, false
	}
	e := int(s[1] - '0')
	m := int(s[3] - '0')
	if e < 1 || e > 9 || m < 1 || m > 9 {
		return 0, 0, false
	}
	return e, m, true
}
