package sessionflow

import "strings"

const QuitPromptExitDelayTics = 53

type QuitPrompt struct {
	Active       bool
	Lines        []string
	ExitDelayTic int
}

func DefaultQuitPromptLines() []string {
	return []string{"ARE YOU SURE YOU WANT TO", "QUIT THIS GREAT GAME?", "(PRESS Y TO QUIT)"}
}

func StartQuitPrompt(seq int, messages []string) (QuitPrompt, int) {
	seq++
	msg := "are you sure you want to\nquit this great game?"
	if len(messages) > 0 {
		msg = messages[(seq-1)%len(messages)]
	}
	lines := strings.Split(strings.ToUpper(msg), "\n")
	lines = append(lines, "(PRESS Y TO QUIT)")
	return QuitPrompt{
		Active:       true,
		Lines:        lines,
		ExitDelayTic: 0,
	}, seq
}

func TickQuitPrompt(state QuitPrompt, confirm, cancel bool) (QuitPrompt, bool) {
	if !state.Active {
		return state, false
	}
	if state.ExitDelayTic > 0 {
		state.ExitDelayTic--
		if state.ExitDelayTic == 0 {
			return state, true
		}
		return state, false
	}
	if confirm {
		state.ExitDelayTic = QuitPromptExitDelayTics
		return state, false
	}
	if cancel {
		return QuitPrompt{}, false
	}
	return state, false
}
