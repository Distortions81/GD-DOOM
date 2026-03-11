package automap

import "gddoom/internal/gameplay"

func (g *game) sessionSignals() gameplay.SessionSignals {
	if g == nil {
		return gameplay.SessionSignals{}
	}
	sig := gameplay.SessionSignals{
		DemoActive:       g.opts.DemoScript != nil,
		NewGameMap:       g.newGameRequestedMap,
		NewGameSkill:     g.newGameRequestedSkill,
		QuitPrompt:       g.quitPromptRequested,
		ReadThis:         g.readThisRequested,
		LevelRestart:     g.levelRestartRequested,
		LevelExit:        g.levelExitRequested,
		SecretLevelExit:  g.secretLevelExit,
		SourcePortMode:   g.opts.SourcePortMode,
		ViewWidth:        g.viewW,
		ViewHeight:       g.viewH,
		LowDetail:        g.lowDetailMode(),
		HUDMessages:      g.hudMessagesEnabled,
		MouseLookSpeed:   g.opts.MouseLookSpeed,
		MusicVolume:      g.opts.MusicVolume,
		SFXVolume:        g.opts.SFXVolume,
		PaletteLUT:       g.paletteLUTEnabled,
		GammaLevel:       g.gammaLevel,
		CRTEnabled:       g.crtEnabled,
		WorldTic:         g.worldTic,
		SourcePortDetail: g.sourcePortDetailDivisor(),
	}
	if g.m != nil {
		sig.MapName = g.m.Name
	}
	return sig
}

func (g *game) sessionSetQuitPromptActive(active bool) {
	if g == nil {
		return
	}
	g.quitPromptActive = active
}

func (g *game) sessionAcknowledgeNewGameRequest() {
	if g == nil {
		return
	}
	g.newGameRequestedMap = nil
	g.newGameRequestedSkill = 0
}

func (g *game) sessionAcknowledgeQuitPrompt() {
	if g == nil {
		return
	}
	g.quitPromptRequested = false
}

func (g *game) sessionAcknowledgeReadThis() {
	if g == nil {
		return
	}
	g.readThisRequested = false
}
