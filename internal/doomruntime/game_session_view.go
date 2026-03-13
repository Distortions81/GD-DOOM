package doomruntime

import (
	"gddoom/internal/gameplay"

	"github.com/hajimehoshi/ebiten/v2"
)

func (g *game) sessionSignals() gameplay.SessionSignals {
	if g == nil {
		return gameplay.SessionSignals{}
	}
	sig := gameplay.SessionSignals{
		DemoActive:       g.opts.DemoScript != nil,
		FrontendMenu:     g.frontendMenuRequested,
		NewGameMap:       g.newGameRequestedMap,
		NewGameSkill:     g.newGameRequestedSkill,
		QuitPrompt:       g.quitPromptRequested,
		ReadThis:         g.readThisRequested,
		MusicPlayer:      g.musicPlayerRequested,
		LevelRestart:     g.levelRestartRequested,
		LevelExit:        g.levelExitRequested,
		SecretLevelExit:  g.secretLevelExit,
		SourcePortMode:   g.opts.SourcePortMode,
		ViewWidth:        g.viewW,
		ViewHeight:       g.viewH,
		LowDetail:        g.lowDetailMode(),
		HUDMessages:      g.hudMessagesEnabled,
		ShowPerf:         !g.opts.NoFPS,
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

func (g *game) sessionAcknowledgeMusicPlayer() {
	if g == nil {
		return
	}
	g.musicPlayerRequested = false
}

func (g *game) sessionAcknowledgeFrontendMenu() {
	if g == nil {
		return
	}
	g.frontendMenuRequested = false
}

func (g *game) sessionToggleHUDMessages() bool {
	if g == nil {
		return false
	}
	g.hudMessagesEnabled = !g.hudMessagesEnabled
	return g.hudMessagesEnabled
}

func (g *game) sessionTogglePerfOverlay() bool {
	if g == nil {
		return false
	}
	g.opts.NoFPS = !g.opts.NoFPS
	return !g.opts.NoFPS
}

func (g *game) sessionCycleDetail() int {
	if g == nil {
		return 0
	}
	if g.opts.SourcePortMode {
		g.cycleSourcePortDetailLevel()
	} else {
		g.cycleDetailLevel()
	}
	return g.detailLevel
}

func (g *game) sessionMouseLookSpeed() float64 {
	if g == nil {
		return 0
	}
	return g.opts.MouseLookSpeed
}

func (g *game) sessionSetMouseLookSpeed(v float64) {
	if g == nil {
		return
	}
	g.opts.MouseLookSpeed = v
}

func (g *game) sessionMusicVolume() float64 {
	if g == nil {
		return 0
	}
	return g.opts.MusicVolume
}

func (g *game) sessionSetMusicVolume(v float64) {
	if g == nil {
		return
	}
	g.opts.MusicVolume = v
}

func (g *game) sessionSFXVolume() float64 {
	if g == nil {
		return 0
	}
	return g.opts.SFXVolume
}

func (g *game) sessionSetSFXVolume(v float64) {
	if g == nil {
		return
	}
	g.opts.SFXVolume = v
	if g.snd != nil {
		g.snd.setSFXVolume(v)
	}
}

func (g *game) sessionPublishRuntimeSettings() {
	if g == nil {
		return
	}
	g.publishRuntimeSettingsIfChanged()
}

func (g *game) sessionDrawHUTextAt(screen *ebiten.Image, text string, x, y, sx, sy float64) {
	if g == nil {
		return
	}
	g.drawHUTextAt(screen, text, x, y, sx, sy)
}

func (g *game) sessionPlaySoundEvent(ev soundEvent) {
	if g == nil || g.snd == nil {
		return
	}
	g.snd.playEvent(ev)
}

func (g *game) sessionTickSound() {
	if g == nil || g.snd == nil {
		return
	}
	g.snd.tick()
}
