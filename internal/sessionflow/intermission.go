package sessionflow

import "gddoom/internal/mapdata"

const (
	IntermissionPhaseWaitTics      = 8
	IntermissionEnteringWaitTics   = 35 * 2
	IntermissionYouAreHereWaitTics = 35 * 3
	IntermissionSkipInputDelayTics = 35 / 3
	IntermissionSkipExitHoldTics   = 12
)

const (
	PhaseKills = iota
	PhaseItems
	PhaseSecrets
	PhaseTime
	PhaseEntering
	PhaseYouAreHere
)

type Stats struct {
	MapName      mapdata.MapName
	NextMapName  mapdata.MapName
	KillsPct     int
	ItemsPct     int
	SecretsPct   int
	TimeSec      int
	KillsFound   int
	KillsTotal   int
	ItemsFound   int
	ItemsTotal   int
	SecretsFound int
	SecretsTotal int
}

type Intermission struct {
	Active            bool
	Phase             int
	WaitTic           int
	Tic               int
	StageSoundCounter int
	ShowEntering      bool
	ShowYouAreHere    bool
	EnteringWait      int
	YouAreHereWait    int
	Show              Stats
	Target            Stats
}

type TickResult struct {
	PlayTick bool
	PlayDone bool
	Finished bool
}

func Start(stats Stats) (Intermission, TickResult) {
	showEntering := ShouldShowEnteringScreen(stats.MapName, stats.NextMapName)
	showYouAreHere := showEntering && ShouldShowYouAreHere(stats.MapName, stats.NextMapName)
	enteringWait := IntermissionEnteringWaitTics
	youAreHereWait := IntermissionYouAreHereWaitTics
	if !showEntering {
		enteringWait = 0
		youAreHereWait = 1
	} else if !showYouAreHere {
		youAreHereWait = 1
	}
	return Intermission{
		Active:         true,
		Phase:          PhaseKills,
		ShowEntering:   showEntering,
		ShowYouAreHere: showYouAreHere,
		EnteringWait:   enteringWait,
		YouAreHereWait: youAreHereWait,
		Show: Stats{
			MapName:      stats.MapName,
			NextMapName:  stats.NextMapName,
			KillsFound:   stats.KillsFound,
			KillsTotal:   stats.KillsTotal,
			ItemsFound:   stats.ItemsFound,
			ItemsTotal:   stats.ItemsTotal,
			SecretsFound: stats.SecretsFound,
			SecretsTotal: stats.SecretsTotal,
		},
		Target: stats,
	}, TickResult{PlayTick: true}
}

func Tick(state Intermission, skipPressed bool) (Intermission, TickResult) {
	if !state.Active {
		return state, TickResult{}
	}
	state.Tic++
	if skipPressed && state.Tic <= IntermissionSkipInputDelayTics {
		skipPressed = false
	}
	if skipPressed && state.Phase != PhaseYouAreHere {
		state.Show.KillsPct = state.Target.KillsPct
		state.Show.ItemsPct = state.Target.ItemsPct
		state.Show.SecretsPct = state.Target.SecretsPct
		state.Show.TimeSec = state.Target.TimeSec
		state.Phase = PhaseYouAreHere
		state.WaitTic = IntermissionSkipExitHoldTics
		return state, TickResult{PlayDone: true}
	}
	if state.WaitTic > 0 {
		state.WaitTic--
		return state, TickResult{}
	}
	switch state.Phase {
	case PhaseKills:
		state.Show.KillsPct = StepCounter(state.Show.KillsPct, state.Target.KillsPct, 2)
		res := TickResult{PlayTick: counterSound(&state, state.Show.KillsPct, state.Target.KillsPct)}
		if state.Show.KillsPct >= state.Target.KillsPct {
			state.Phase = PhaseItems
			state.WaitTic = IntermissionPhaseWaitTics
			state.StageSoundCounter = 0
			res.PlayTick = true
		}
		return state, res
	case PhaseItems:
		state.Show.ItemsPct = StepCounter(state.Show.ItemsPct, state.Target.ItemsPct, 2)
		res := TickResult{PlayTick: counterSound(&state, state.Show.ItemsPct, state.Target.ItemsPct)}
		if state.Show.ItemsPct >= state.Target.ItemsPct {
			state.Phase = PhaseSecrets
			state.WaitTic = IntermissionPhaseWaitTics
			state.StageSoundCounter = 0
			res.PlayTick = true
		}
		return state, res
	case PhaseSecrets:
		state.Show.SecretsPct = StepCounter(state.Show.SecretsPct, state.Target.SecretsPct, 2)
		res := TickResult{PlayTick: counterSound(&state, state.Show.SecretsPct, state.Target.SecretsPct)}
		if state.Show.SecretsPct >= state.Target.SecretsPct {
			state.Phase = PhaseTime
			state.WaitTic = IntermissionPhaseWaitTics
			state.StageSoundCounter = 0
			res.PlayTick = true
		}
		return state, res
	case PhaseTime:
		state.Show.TimeSec = StepCounter(state.Show.TimeSec, state.Target.TimeSec, 3)
		res := TickResult{PlayTick: counterSound(&state, state.Show.TimeSec, state.Target.TimeSec)}
		if state.Show.TimeSec >= state.Target.TimeSec {
			if state.ShowEntering {
				state.Phase = PhaseEntering
				state.WaitTic = state.EnteringWait
				state.StageSoundCounter = 0
				res.PlayDone = true
			} else {
				state.Phase = PhaseYouAreHere
				state.WaitTic = state.YouAreHereWait
			}
		}
		return state, res
	case PhaseEntering:
		state.Phase = PhaseYouAreHere
		state.WaitTic = state.YouAreHereWait
		if state.ShowYouAreHere {
			return state, TickResult{PlayTick: true}
		}
		return state, TickResult{}
	default:
		if state.WaitTic <= 0 {
			return state, TickResult{PlayDone: true, Finished: true}
		}
		return state, TickResult{}
	}
}

func ShouldShowYouAreHere(current, next mapdata.MapName) bool {
	epCur, _, okCur := episodeMapSlot(current)
	epNext, _, okNext := episodeMapSlot(next)
	if !okCur || !okNext {
		return false
	}
	return epCur == epNext
}

func ShouldShowEnteringScreen(current, next mapdata.MapName) bool {
	_, _, okCur := episodeMapSlot(current)
	if !okCur {
		return false
	}
	_, _, okNext := episodeMapSlot(next)
	return okNext
}

func StepCounter(cur, target, step int) int {
	if step < 1 {
		step = 1
	}
	if cur >= target {
		return target
	}
	cur += step
	if cur > target {
		cur = target
	}
	return cur
}

func counterSound(state *Intermission, cur, target int) bool {
	if state == nil || cur >= target {
		return false
	}
	state.StageSoundCounter++
	return state.StageSoundCounter%6 == 0
}
