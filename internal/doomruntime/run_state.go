package doomruntime

import (
	"fmt"
	"strings"

	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/runtimehost"
	"gddoom/internal/sessionaudio"
	"gddoom/internal/sessionflow"
	"gddoom/internal/sessionmusic"
	"gddoom/internal/sessiontransition"

	"github.com/hajimehoshi/ebiten/v2"
)

type NextMapFunc = runtimehost.NextMapFunc

const (
	bootSplashHoldTics = 2 * doomTicsPerSecond
	// Sourceport melt uses Doom-like 2-pixel column pairs over a 320-wide
	// virtual layout, i.e. 160 moving slices.
	sourcePortMeltInitCols = 160
	sourcePortMeltMoveCols = sourcePortMeltInitCols

	menuSkullBlinkTics = 8
)

type transitionKind = sessiontransition.Kind

const (
	transitionNone  = sessiontransition.KindNone
	transitionBoot  = sessiontransition.KindBoot
	transitionLevel = sessiontransition.KindLevel
)

type sessionIntermission struct {
	state   intermissionState
	nextMap *mapdata.Map
}

type sessionFinale = sessionflow.Finale

type frontendMode = sessionflow.FrontendMode

const (
	frontendModeNone                     = sessionflow.FrontendModeNone
	frontendModeTitle                    = sessionflow.FrontendModeTitle
	frontendModeReadThis                 = sessionflow.FrontendModeReadThis
	frontendModeOptions                  = sessionflow.FrontendModeOptions
	frontendModeSound                    = sessionflow.FrontendModeSound
	frontendModeEpisode                  = sessionflow.FrontendModeEpisode
	frontendModeSkill                    = sessionflow.FrontendModeSkill
	frontendModeMusicPlayer frontendMode = 100
)

type frontendState = sessionflow.Frontend

type quitPromptState = sessionflow.QuitPrompt

const (
	intermissionPhaseKills = iota
	intermissionPhaseItems
	intermissionPhaseSecrets
	intermissionPhaseTime
	intermissionPhaseEntering
	intermissionPhaseYouAreHere
)

type sessionGame struct {
	g                       *game
	rt                      sessionRuntime
	gameFactory             gameplay.RuntimeFactory[Options, *game]
	bootMap                 *mapdata.Map
	current                 mapdata.MapName
	currentTemplate         *mapdata.Map
	opts                    Options
	demoRecord              []DemoTic
	settings                gameplay.PersistentSettings
	nextMap                 NextMapFunc
	err                     error
	musicCtl                *sessionmusic.Playback
	secretVisited           bool
	levelCarryover          *playerLevelCarryover
	faithfulSurface         *ebiten.Image
	faithfulNearest         *ebiten.Image
	crtShader               *ebiten.Shader
	crtPost                 *ebiten.Image
	crtUniforms             map[string]any
	gameplaySurface         *ebiten.Image
	frontendSurface         *ebiten.Image
	bootSplashImage         *ebiten.Image
	menuPatchImages         map[string]*ebiten.Image
	intermissionImages      map[string]*ebiten.Image
	statusPatchImages       map[string]*ebiten.Image
	spritePatchImages       map[string]*ebiten.Image
	messageFontImages       map[rune]*ebiten.Image
	flatImages              map[string]*ebiten.Image
	menuSfx                 *sessionaudio.MenuController
	finalScreenDrawOp       ebiten.DrawImageOptions
	transition              sessiontransition.Controller
	intermission            sessionIntermission
	finale                  sessionFinale
	frontend                frontendState
	frontendMenuPending     bool
	frontendMusicConfig     frontendMusicConfigPending
	startupMusicLocked      bool
	startupMusicVisualReady bool
	startupMusicPending     musicPlaybackSource
	transitionMusicPending  musicPlaybackSource
	musicPlayer             frontendMusicPlayerState
	currentMusicSource      musicPlaybackSource
	nowPlayingLevel         string
	nowPlayingMusic         string
	quitPrompt              quitPromptState
	quitMessageSeq          int
	input                   sessionInputSnapshot
}

type frontendMusicConfigPending struct {
	active        bool
	backend       music.Backend
	soundFontPath string
}

type sessionInputSnapshot struct {
	justPressedKeys         map[ebiten.Key]int
	justPressedMouseButtons map[ebiten.MouseButton]int
}

type sessionRuntime interface {
	SampleInput()
	Update() error
	Draw(*ebiten.Image)
	Layout(int, int) (int, int)
	sessionSignals() gameplay.SessionSignals
	clearPendingSoundState()
	clearSpritePatchCache()
	initSkyLayerShader()
	setSkyOutputSize(int, int)
	sessionAcknowledgeSaveGame()
	sessionAcknowledgeLoadGame()
	sessionSetQuitPromptActive(bool)
	sessionSetFrontendActive(bool)
	sessionAcknowledgeNewGameRequest()
	sessionAcknowledgeQuitPrompt()
	sessionAcknowledgeReadThis()
	sessionAcknowledgeLevelRestart()
	sessionAcknowledgeMusicPlayer()
	sessionAcknowledgeFrontendMenu()
	sessionToggleHUDMessages() bool
	sessionTogglePerfOverlay() bool
	sessionCycleDetail() int
	sessionMouseLookSpeed() float64
	sessionSetMouseLookSpeed(float64)
	sessionMusicVolume() float64
	sessionSetMusicVolume(float64)
	sessionSFXVolume() float64
	sessionSetSFXVolume(float64)
	sessionPublishRuntimeSettings()
	sessionDrawHUTextAt(*ebiten.Image, string, float64, float64, float64, float64)
	sessionPlaySoundEvent(soundEvent)
	sessionTickSound()
}

const (
	attractPageTitleNonCommercial = 170
	attractPageTitleCommercial    = 35 * 11
	attractPageInfo               = 200
)

var doomQuitMessages = []string{
	"please don't leave, there's more\ndemons to toast!",
	"let's beat it -- this is turning\ninto a bloodbath!",
	"i wouldn't leave if i were you.\ndos is much worse.",
	"you're trying to say you like dos\nbetter than me, right?",
	"don't leave yet -- there's a\ndemon around that corner!",
	"ya know, next time you come in here\ni'm gonna toast ya.",
	"go ahead and leave. see if i care.",
	"you want to quit?\nthen, thou hast lost an eighth!",
	"don't go now, there's a \ndimensional shambler waiting\nat the dos prompt!",
	"get outta here and go back\nto your boring programs.",
	"if i were your boss, i'd \n deathmatch ya in a minute!",
	"look, bud. you leave now\nand you forfeit your body count!",
	"just leave. when you come\nback, i'll be waiting with a bat.",
	"you're lucky i don't smack\nyou for thinking about leaving.",
	"fuck you, pussy!\nget the fuck out!",
	"you quit and i'll jizz\nin your cystholes!",
	"if you leave, i'll make\nthe lord drink my jizz.",
	"hey, ron! can we say\n'fuck' in the game?",
	"i'd leave: this is just\nmore monsters and levels.\nwhat a load.",
	"suck it down, asshole!\nyou're a fucking wimp!",
	"don't quit now! we're \nstill spending your money!",
}

func cloneMapForRestart(src *mapdata.Map) *mapdata.Map { return gameplay.CloneMapForRestart(src) }

func clampDetailLevelForMode(level int, sourcePort bool) int {
	return gameplay.ClampDetailLevel(level, sourcePort, len(detailPresets), len(sourcePortDetailDivisors))
}

func normalizeRevealForMode(mode revealMode, sourcePort bool) revealMode {
	return revealMode(gameplay.NormalizeReveal(int(mode), sourcePort, int(revealNormal), int(revealAllMap)))
}

func clampIDDT(v int) int {
	return gameplay.ClampIDDT(v)
}

func clampGamma(level int) int {
	return gameplay.ClampGamma(level, doomGammaLevels)
}

func clampVolume(v float64) float64 {
	return gameplay.ClampVolume(v)
}

func clampOPLVolume(v float64) float64 {
	return gameplay.ClampOPLVolume(v, music.MaxOutputGain)
}

func (sg *sessionGame) capturePersistentSettings() {
	if sg == nil || sg.g == nil {
		return
	}
	g := sg.g
	sg.settings = gameplay.PersistentSettings{
		DetailLevel:      g.detailLevel,
		RotateView:       g.rotateView,
		MouseLook:        g.opts.MouseLook,
		MouseLookSpeed:   g.opts.MouseLookSpeed,
		MusicVolume:      g.opts.MusicVolume,
		OPLVolume:        g.opts.OPLVolume,
		SFXVolume:        g.opts.SFXVolume,
		HUDMessages:      g.hudMessagesEnabled,
		AlwaysRun:        g.alwaysRun,
		AutoWeaponSwitch: g.autoWeaponSwitch,
		LineColorMode:    g.opts.LineColorMode,
		ThingRenderMode:  g.opts.SourcePortThingRenderMode,
		ShowLegend:       g.showLegend,
		PaletteLUT:       g.paletteLUTEnabled,
		GammaLevel:       g.gammaLevel,
		CRTEnabled:       g.crtEnabled,
		Reveal:           int(g.parity.reveal),
		IDDT:             g.parity.iddt,
	}
}

func (sg *sessionGame) applyPersistentSettingsToOptions() {
	applyOptionStateToOptions(&sg.opts, gameplay.ApplyPersistentSettingsToOptions(sg.optionState(), sg.settings, music.MaxOutputGain))
}

func (sg *sessionGame) applyPersistentSettingsToGame(g *game) {
	if sg == nil || g == nil {
		return
	}
	applied := gameplay.ApplyPersistentSettings(
		sg.settings,
		g.opts.SourcePortMode,
		len(detailPresets),
		len(sourcePortDetailDivisors),
		doomGammaLevels,
		music.MaxOutputGain,
		g.opts.KageShader,
		len(g.opts.DoomPaletteRGBA) == 256*4,
		int(revealNormal),
		int(revealAllMap),
	)
	g.detailLevel = applied.DetailLevel
	g.rotateView = applied.RotateView
	g.opts.MouseLook = applied.MouseLook
	g.opts.MouseLookSpeed = applied.MouseLookSpeed
	g.opts.MusicVolume = applied.MusicVolume
	g.opts.OPLVolume = applied.OPLVolume
	g.opts.SFXVolume = applied.SFXVolume
	if g.snd != nil {
		g.snd.setSFXVolume(g.opts.SFXVolume)
	}
	if sg.musicCtl != nil {
		sg.musicCtl.SetVolume(g.opts.MusicVolume)
	}
	if sg.musicCtl != nil {
		sg.musicCtl.SetOutputGain(g.opts.OPLVolume)
	}
	g.alwaysRun = applied.AlwaysRun
	g.autoWeaponSwitch = applied.AutoWeaponSwitch
	g.hudMessagesEnabled = applied.HUDMessages
	g.opts.LineColorMode = applied.LineColorMode
	g.opts.SourcePortThingRenderMode = normalizeSourcePortThingRenderMode(applied.ThingRenderMode, g.opts.SourcePortMode)
	g.showLegend = applied.ShowLegend
	g.paletteLUTEnabled = applied.PaletteLUT
	g.setGammaLevel(applied.GammaLevel)
	g.crtEnabled = applied.CRTEnabled
	g.parity.reveal = revealMode(applied.Reveal)
	g.parity.iddt = applied.IDDT
	g.runtimeSettingsSeen = true
	g.runtimeSettingsLast = g.runtimeSettingsSnapshot()
}

func (sg *sessionGame) applyRuntimeSettings(s RuntimeSettings) {
	if sg == nil {
		return
	}
	result := gameplay.ApplyRuntimeSettings(
		sg.settings,
		s,
		sg.opts.SourcePortMode,
		len(detailPresets),
		len(sourcePortDetailDivisors),
		doomGammaLevels,
		music.MaxOutputGain,
	)
	next := result.Settings
	sg.settings.DetailLevel = next.DetailLevel
	sg.settings.MouseLook = next.MouseLook
	sg.settings.MusicVolume = next.MusicVolume
	sg.settings.OPLVolume = next.OPLVolume
	sg.settings.SFXVolume = next.SFXVolume
	sg.settings.HUDMessages = next.HUDMessages
	sg.settings.AlwaysRun = next.AlwaysRun
	sg.settings.AutoWeaponSwitch = next.AutoWeaponSwitch
	sg.settings.LineColorMode = next.LineColorMode
	sg.settings.ThingRenderMode = normalizeSourcePortThingRenderMode(next.ThingRenderMode, sg.opts.SourcePortMode)
	sg.settings.GammaLevel = next.GammaLevel
	sg.settings.CRTEnabled = next.CRTEnabled && sg.opts.KageShader
	sg.opts.MusicVolume = sg.settings.MusicVolume
	sg.opts.OPLVolume = sg.settings.OPLVolume
	sg.opts.SFXVolume = sg.settings.SFXVolume
	if backend, err := music.ParseBackend(s.MusicBackend); err == nil {
		sg.opts.MusicBackend = backend
	}
	sg.opts.MusicSoundFontPath = strings.TrimSpace(s.MusicSoundFontPath)
	if sg.menuSfx != nil {
		sg.menuSfx.SetVolume(sg.settings.SFXVolume)
	}
	if sg.musicCtl != nil {
		sg.musicCtl.SetOutputGain(sg.opts.OPLVolume)
	}
	switch result.MusicAction {
	case gameplay.MusicActionStop:
		sg.stopAndClearMusic()
	case gameplay.MusicActionRestart:
		if sg.frontend.Active {
			sg.playTitleMusic()
		} else if sg.g != nil && sg.g.m != nil {
			sg.playMusicForMap(sg.g.m.Name)
		} else {
			sg.playMusicForMap(sg.current)
		}
	case gameplay.MusicActionUpdateVolume:
		if sg.musicCtl != nil {
			sg.musicCtl.SetVolume(sg.settings.MusicVolume)
		}
	}
}

func (sg *sessionGame) runtimeSettingsSnapshot() RuntimeSettings {
	if sg == nil {
		return RuntimeSettings{}
	}
	if sg.g != nil {
		return sg.g.runtimeSettingsSnapshot()
	}
	return RuntimeSettings{
		DetailLevel:        sg.settings.DetailLevel,
		GammaLevel:         sg.settings.GammaLevel,
		MusicVolume:        sg.opts.MusicVolume,
		MUSPanMax:          sg.opts.MUSPanMax,
		OPLVolume:          sg.opts.OPLVolume,
		MusicBackend:       string(sg.opts.MusicBackend),
		MusicSoundFontPath: sg.opts.MusicSoundFontPath,
		SFXVolume:          sg.opts.SFXVolume,
		HUDMessages:        sg.settings.HUDMessages,
		MouseLook:          sg.opts.MouseLook,
		AlwaysRun:          sg.opts.AlwaysRun,
		AutoWeaponSwitch:   sg.opts.AutoWeaponSwitch,
		LineColorMode:      sg.opts.LineColorMode,
		ThingRenderMode:    sg.opts.SourcePortThingRenderMode,
		CRTEffect:          sg.settings.CRTEnabled && sg.opts.KageShader,
	}
}

func (sg *sessionGame) rebuildGameWithPersistentSettings(next *mapdata.Map) {
	if sg == nil || next == nil {
		return
	}
	sg.capturePersistentSettings()
	result := gameplay.RebuildRuntime(gameplay.RebuildRequest[Options, *game]{
		Next:           next,
		Current:        sg.g,
		DemoRecord:     sg.demoRecord,
		CurrentOptions: sg.optionState(),
		Settings:       sg.settings,
		MaxOPLGain:     music.MaxOutputGain,
		PendingDemo: func(g *game) []DemoTic {
			if g == nil {
				return nil
			}
			return g.demoRecord
		},
		SetPendingDemo: func(g *game, remaining []DemoTic) {
			if g != nil {
				g.demoRecord = remaining
			}
		},
		ClearBeforeBuild: func(g *game) {
			if g != nil {
				g.clearSpritePatchCache()
			}
		},
		ApplyOptions: func(state gameplay.OptionState) Options {
			opts := sg.opts
			applyOptionStateToOptions(&opts, state)
			return opts
		},
		Build:           sg.buildGame,
		ApplyPersistent: sg.applyPersistentSettingsToGame,
	})
	sg.demoRecord = result.DemoRecord
	sg.opts = result.Options
	sg.g = result.Runtime
	sg.rt = result.Runtime
}

func (sg *sessionGame) buildGame(m *mapdata.Map, opts Options) *game {
	if sg == nil {
		return nil
	}
	factory := sg.gameFactory
	if factory == nil {
		factory = newGame
	}
	g := gameplay.BuildRuntime(factory, m, opts)
	if g == nil {
		return nil
	}
	if sg.menuPatchImages == nil {
		sg.menuPatchImages = make(map[string]*ebiten.Image, 32)
	}
	if sg.intermissionImages == nil {
		sg.intermissionImages = make(map[string]*ebiten.Image, 96)
	}
	if sg.statusPatchImages == nil {
		sg.statusPatchImages = make(map[string]*ebiten.Image, 96)
	}
	if sg.spritePatchImages == nil {
		sg.spritePatchImages = make(map[string]*ebiten.Image, 256)
	}
	if sg.messageFontImages == nil {
		sg.messageFontImages = make(map[rune]*ebiten.Image, 96)
	}
	if sg.flatImages == nil {
		sg.flatImages = make(map[string]*ebiten.Image, 64)
	}
	g.menuPatchImg = sg.menuPatchImages
	g.statusPatchImg = sg.statusPatchImages
	g.spritePatchImg = sg.spritePatchImages
	g.messageFontImg = sg.messageFontImages
	g.flatImgCache = sg.flatImages
	sg.prewarmMapAssetCaches(g)
	return g
}

func (sg *sessionGame) prewarmWADAssetCaches() {
	if sg == nil {
		return
	}
	if sg.menuPatchImages == nil {
		sg.menuPatchImages = make(map[string]*ebiten.Image, 32)
	}
	if sg.intermissionImages == nil {
		sg.intermissionImages = make(map[string]*ebiten.Image, 96)
	}
	if sg.statusPatchImages == nil {
		sg.statusPatchImages = make(map[string]*ebiten.Image, 96)
	}
	if sg.spritePatchImages == nil {
		sg.spritePatchImages = make(map[string]*ebiten.Image, 256)
	}
	if sg.messageFontImages == nil {
		sg.messageFontImages = make(map[rune]*ebiten.Image, 96)
	}
	if sg.flatImages == nil {
		sg.flatImages = make(map[string]*ebiten.Image, 64)
	}
	if sg.bootSplashImage == nil {
		p := sg.opts.BootSplash
		if p.Width > 0 && p.Height > 0 && len(p.RGBA) == p.Width*p.Height*4 {
			sg.bootSplashImage = newUnmanagedImage(p.Width, p.Height)
			sg.bootSplashImage.WritePixels(p.RGBA)
		}
	}
	if len(sg.menuPatchImages) == 0 {
		for key, p := range sg.opts.MenuPatchBank {
			if p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
				continue
			}
			img := newDebugImage("prewarm:menu:"+key, p.Width, p.Height)
			img.WritePixels(p.RGBA)
			sg.menuPatchImages[key] = img
		}
	}
	if len(sg.intermissionImages) == 0 {
		for key, p := range sg.opts.IntermissionPatchBank {
			if p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
				continue
			}
			img := newDebugImage("prewarm:intermission:"+key, p.Width, p.Height)
			img.WritePixels(p.RGBA)
			sg.intermissionImages[key] = img
		}
	}
	if len(sg.statusPatchImages) == 0 {
		for key, p := range sg.opts.StatusPatchBank {
			if p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
				continue
			}
			img := newDebugImage("prewarm:status:"+key, p.Width, p.Height)
			img.WritePixels(p.RGBA)
			sg.statusPatchImages[key] = img
		}
	}
	if len(sg.messageFontImages) == 0 {
		for ch, p := range sg.opts.MessageFontBank {
			if p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
				continue
			}
			img := newDebugImage("prewarm:font:"+string(ch), p.Width, p.Height)
			img.WritePixels(p.RGBA)
			sg.messageFontImages[ch] = img
		}
	}
	if len(sg.spritePatchImages) == 0 {
		for key, p := range sg.opts.SpritePatchBank {
			if p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
				continue
			}
			img := newDebugImage("prewarm:sprite:"+key, p.Width, p.Height)
			img.WritePixels(p.RGBA)
			sg.spritePatchImages[key] = img
		}
	}
}

func (sg *sessionGame) prewarmMapAssetCaches(g *game) {
	if sg == nil || g == nil {
		return
	}
	if len(sg.flatImages) == 0 {
		g.precacheMapTextureAssets()
	}
}

func (sg *sessionGame) optionState() gameplay.OptionState {
	if sg == nil {
		return gameplay.OptionState{}
	}
	return gameplay.OptionState{
		MouseLook:        sg.opts.MouseLook,
		MouseLookSpeed:   sg.opts.MouseLookSpeed,
		MusicVolume:      sg.opts.MusicVolume,
		OPLVolume:        sg.opts.OPLVolume,
		SFXVolume:        sg.opts.SFXVolume,
		AlwaysRun:        sg.opts.AlwaysRun,
		AutoWeaponSwitch: sg.opts.AutoWeaponSwitch,
		LineColorMode:    sg.opts.LineColorMode,
		ThingRenderMode:  sg.opts.SourcePortThingRenderMode,
	}
}

func applyOptionStateToOptions(opts *Options, state gameplay.OptionState) {
	if opts == nil {
		return
	}
	opts.MouseLook = state.MouseLook
	opts.MouseLookSpeed = state.MouseLookSpeed
	opts.MusicVolume = state.MusicVolume
	opts.OPLVolume = state.OPLVolume
	opts.SFXVolume = state.SFXVolume
	opts.AlwaysRun = state.AlwaysRun
	opts.AutoWeaponSwitch = state.AutoWeaponSwitch
	opts.LineColorMode = state.LineColorMode
	opts.SourcePortThingRenderMode = state.ThingRenderMode
}

func (sg *sessionGame) collectDemoRecord() {
	if sg == nil || sg.g == nil || len(sg.g.demoRecord) == 0 {
		return
	}
	sg.demoRecord, sg.g.demoRecord = gameplay.CollectDemoRecord(sg.demoRecord, sg.g.demoRecord)
}

func (sg *sessionGame) effectiveDemoRecord() []DemoTic {
	if sg == nil {
		return nil
	}
	sg.collectDemoRecord()
	return sg.demoRecord
}

func (sg *sessionGame) restartMapForRespawn() *mapdata.Map {
	if sg == nil || sg.g == nil {
		return nil
	}
	return gameplay.RestartMapForRespawn(sg.g.m, sg.currentTemplate, normalizeGameMode(sg.opts.GameMode) == gameModeSingle)
}

func (sg *sessionGame) initMusicPlayback() {
	if sg == nil || clampVolume(sg.opts.MusicVolume) <= 0 || (sg.opts.MapMusicLoader == nil && sg.opts.TitleMusicLoader == nil) {
		return
	}
	ctl, err := sessionmusic.NewPlayback(
		clampVolume(sg.opts.MusicVolume),
		sg.opts.MUSPanMax,
		sg.opts.OPLVolume,
		sg.opts.AudioPreEmphasis,
		sg.opts.MusicBackend,
		sg.opts.MusicPatchBank,
		sg.opts.MusicSoundFont,
		sg.opts.MapMusicLoader,
		sg.opts.TitleMusicLoader,
		sg.opts.IntermissionMusicLoader,
	)
	if err != nil {
		return
	}
	sg.musicCtl = ctl
}

func (sg *sessionGame) closeMusicPlayback() {
	if sg == nil || sg.musicCtl == nil {
		return
	}
	sg.musicCtl.Close()
	sg.musicCtl = nil
}

func (sg *sessionGame) stopAndClearMusic() {
	if sg == nil || sg.musicCtl == nil {
		return
	}
	sg.musicCtl.StopAndClear()
	sg.transitionMusicPending = musicPlaybackSource{}
	sg.setNowPlayingLevel("")
	sg.setNowPlayingMusic("")
}

func (sg *sessionGame) playMusicForMap(name mapdata.MapName) {
	if sg == nil || sg.musicCtl == nil {
		return
	}
	if sg.startupMusicLocked {
		sg.startupMusicPending = musicPlaybackSource{
			kind:    musicPlaybackSourceMap,
			mapName: name,
		}
		return
	}
	if sg.transitionActive() && sg.transition.Kind() == transitionLevel {
		sg.transitionMusicPending = musicPlaybackSource{
			kind:    musicPlaybackSourceMap,
			mapName: name,
		}
		return
	}
	sg.musicCtl.PlayMap(name, clampVolume(sg.opts.MusicVolume))
	sg.currentMusicSource = musicPlaybackSource{
		kind:    musicPlaybackSourceMap,
		mapName: name,
	}
	if sg.opts.MapMusicInfo != nil {
		levelLabel, musicName := sg.opts.MapMusicInfo(string(name))
		sg.currentMusicSource.levelLabel = levelLabel
		sg.currentMusicSource.musicName = musicName
		sg.setNowPlayingLevel(levelLabel, string(name))
		sg.setNowPlayingMusic(musicName, string(name))
	}
}

func (sg *sessionGame) playIntermissionMusic(commercial bool) {
	if sg == nil || sg.musicCtl == nil {
		return
	}
	if sg.startupMusicLocked {
		sg.startupMusicPending = musicPlaybackSource{
			kind:       musicPlaybackSourceIntermission,
			commercial: commercial,
		}
		return
	}
	sg.musicCtl.PlayIntermission(commercial, clampVolume(sg.opts.MusicVolume))
	sg.currentMusicSource = musicPlaybackSource{
		kind:       musicPlaybackSourceIntermission,
		commercial: commercial,
	}
	sg.setNowPlayingLevel("")
	sg.setNowPlayingMusic("Intermission")
}

func (sg *sessionGame) releaseTransitionMusicIfReady() {
	if sg == nil || sg.transitionActive() {
		return
	}
	pending := sg.transitionMusicPending
	if pending.kind == musicPlaybackSourceNone {
		return
	}
	sg.transitionMusicPending = musicPlaybackSource{}
	switch pending.kind {
	case musicPlaybackSourceMap:
		sg.playMusicForMap(pending.mapName)
	}
}

func (sg *sessionGame) releaseStartupMusicIfReady() {
	if sg == nil || !sg.startupMusicLocked || sg.transitionActive() || !sg.startupMusicVisualReady {
		return
	}
	pending := sg.startupMusicPending
	sg.startupMusicLocked = false
	sg.startupMusicVisualReady = false
	sg.startupMusicPending = musicPlaybackSource{}
	switch pending.kind {
	case musicPlaybackSourceTitle:
		sg.playTitleMusic()
	case musicPlaybackSourceMap:
		sg.playMusicForMap(pending.mapName)
	case musicPlaybackSourceIntermission:
		sg.playIntermissionMusic(pending.commercial)
	}
}

func (sg *sessionGame) announceMapMusic(name mapdata.MapName) {
	if sg == nil || sg.g == nil || sg.opts.MapMusicInfo == nil {
		return
	}
	levelLabel, musicName := sg.opts.MapMusicInfo(string(name))
	levelLabel = strings.TrimSpace(levelLabel)
	musicName = strings.TrimSpace(musicName)
	switch {
	case levelLabel != "" && musicName != "":
		sg.g.setHUDMessage(fmt.Sprintf("%s\nSONG: %s", levelLabel, musicName), 70)
	case levelLabel != "":
		sg.g.setHUDMessage(levelLabel, 70)
	case musicName != "":
		sg.g.setHUDMessage(fmt.Sprintf("SONG: %s", musicName), 70)
	}
}

func (sg *sessionGame) initSession() {
	if sg == nil {
		return
	}
	sg.prewarmWADAssetCaches()
	sg.startupMusicLocked = sg.shouldShowBootSplash()
	sg.startupMusicVisualReady = false
	sg.startupMusicPending = musicPlaybackSource{}
	runtimehost.RunBootstrap(runtimehost.Bootstrap{
		BuildRuntime: func() {
			sg.g = sg.buildGame(sg.bootMap, sg.opts)
			sg.rt = sg.g
			sg.g.initSkyLayerShader()
		},
		InitMusicPlayback:     sg.initMusicPlayback,
		ShouldStartInFrontend: sg.shouldStartInFrontend,
		StartFrontend:         sg.startFrontend,
		StartMapMusic: func() {
			sg.playMusicForMap(sg.current)
			sg.announceMapMusic(sg.current)
		},
		CaptureSettings:      sg.capturePersistentSettings,
		ShouldShowBootSplash: sg.shouldShowBootSplash,
		QueueBootSplash: func() {
			sg.queueTransition(transitionBoot, bootSplashHoldTics)
		},
		InitPost: sg.initFaithfulPalettePost,
	})
}
